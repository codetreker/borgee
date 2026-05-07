# remote-agent — 暴露本地目录给 Borgee server

代码位置：`/workspace/borgee/packages/remote-agent/`
对偶 server 代码：`/workspace/borgee/packages/server-go/internal/api/remote.go`、`internal/ws/remote.go`

## 1. 这是什么

`remote-agent` 是一个**用户在自己机器上跑的 Node 守护进程**。它向 Borgee server 长连一条 WebSocket（`/ws/remote`），server 可以通过这条连接反向读取该机器上**白名单目录**里的文件，让 channel 里的 agent 看到这些内容。

它**不是聊天 agent 的运行时**——不参与消息流，不调用 LLM。它是一个"远程文件视图"的 daemon，作用对标 SSH 的只读子集，但通过既有的 Borgee channel 路径授权。

设计意图：

- 运维不必把代码 push 到云端就能让 LLM agent "看到"它；
- 用户对暴露范围有显式控制（启动参数 `--dirs` 白名单 + per-channel 绑定）；
- server 完全主导请求节奏，agent 只被动响应。

## 2. 启动

二进制名 `borgee-remote-agent`（`package.json: bin`），底层 `commander` 解析参数，三个 flag 都是 `requiredOption`：

```bash
borgee-remote-agent \
  --server ws://your-borgee-host:4900 \
  --token <connection_token> \
  --dirs /home/me/projects,/srv/data
```

| Flag | 作用 |
|------|------|
| `--server` | Borgee server 的 WS 基础 URL（如 `ws://localhost:4900`，prod 用 `wss://`）。daemon 启动时拼成 `${server}/ws/remote?token=...` |
| `--token` | 用户在 Borgee UI 上注册 node 时返回的 `remote_nodes.connection_token`（UNIQUE） |
| `--dirs` | 逗号分隔的目录白名单。启动时 `split(',').map(trim).filter(Boolean)`，空数组直接 `process.exit(1)` |

启动后日志会打印 `[remote-agent] Allowed directories: ...`。`SIGINT` / `SIGTERM` 调 `agent.close()` 后 `process.exit(0)`。

## 3. 协议

**形态：服务端主导的 RPC over WebSocket**——daemon 永远是被动方，只回应 `request`，不主动发业务请求。

连接地址：`{serverUrl}/ws/remote?token=${encodeURIComponent(token)}`。token 通过 query 参数带（不是 Header）。

### 消息类型

| `type` | 方向 | payload | 含义 |
|--------|------|---------|------|
| `ping` | agent → server | — | 30s 一次心跳 |
| `pong` | server → agent | — | 心跳响应（agent 收到就 no-op） |
| `request` | server → agent | `{id, data:{action, path}}` | 服务端发起的 RPC |
| `response` | agent → server | `{id, data: <result \| {error}>}` | 与 `request.id` 配对的应答 |

`action` 当前只有三种：`ls` / `read` / `stat`，其它一律返回 `{error: "Unknown action: <name>"}`。

### 心跳与重连

- `setInterval` 每 30 s 发一次 `{type:"ping"}`（`agent.ts:114`）。
- `close` 后 `scheduleReconnect`：起步 `reconnectDelay = 1000`，每次 ×2，封顶 `30_000`（`agent.ts:13–14, 124–130`）。
- 调用 `close()` 后 `closed = true`，重连循环就停。

## 4. 文件系统沙箱（`fs-ops.ts`）

**只有一道安全墙**：路径白名单 `isPathAllowed`（`fs-ops.ts:52–58`）：

```ts
const resolved = path.resolve(targetPath);
return allowedDirs.some(dir => {
  const resolvedDir = path.resolve(dir);
  return resolved === resolvedDir || resolved.startsWith(resolvedDir + path.sep);
});
```

要点：

- `path.resolve` 把相对路径与 `..` 全部规范化，杜绝 traversal；
- `+ path.sep` 防 `/srv/data-extra` 这种**前缀误匹配**；
- 白名单"等于"或"在子目录里"才放行。

每个导出函数（`ls` / `readFile` / `stat`）**第一行就是 isPathAllowed 校验**，失败返回 `{error:"path_not_allowed"}`。

### `readFile` 额外限制

- **2 MiB 上限**：`MAX_FILE_SIZE = 2 * 1024 * 1024`，超过返回 `{error:"file_too_large"}`（`fs-ops.ts:50, 101`）。
- **拒绝目录**：先 `fs.statSync` 后判 `isDirectory()`，命中返回 `{error:"is_directory"}`（`fs-ops.ts:98`）。
- **总是按 `utf-8` 读**——二进制文件会被解码成 mojibake。MIME 是查表（25+ 扩展，命中默认 `application/octet-stream`）。

### `ls` 行为

- `fs.readdirSync(targetPath, { withFileTypes:true })`，返回 `{name, isDirectory, size, mtime}` 列表。
- 子项 `statSync` 失败时静默忽略（保持 `size=0, mtime=""`）。
- ENOENT 返回 `{error:"path_not_found"}`，其它错误 `{error: String(err)}`。

### 显式不做的事

- **没有写、删、改名、exec、symlink 解析、follow link 控制、隐藏文件过滤**——只读、扁平。
- **没有进程级隔离**：没有 seccomp / chroot / namespace。整套靠 userland 路径检查；如果担心二次提权，跑在低权限 user 下。
- 没有审计日志；只 `console.log` 连接生命周期。

## 5. 与 server-go 的对偶

server 端实现在 `internal/api/remote.go`，全部走 `authMw` 包装的普通用户 JWT，路由：

| Method | Path | 行为 |
|--------|------|------|
| GET | `/api/v1/remote/nodes` | 列出当前 user 的 nodes |
| POST | `/api/v1/remote/nodes` | 注册 node，body `{machine_name}`，返回 `{node}`（含 `connection_token`） |
| DELETE | `/api/v1/remote/nodes/{id}` | 删除 node（owner 校验） |
| GET | `/api/v1/remote/nodes/{nodeId}/bindings` | 列出该 node 的所有 channel 绑定 |
| POST | `/api/v1/remote/nodes/{nodeId}/bindings` | body `{channel_id, path, label}`，把 path 暴露到 channel |
| DELETE | `/api/v1/remote/nodes/{nodeId}/bindings/{id}` | 删绑定 |
| GET | `/api/v1/channels/{channelId}/remote-bindings` | channel 角度的 binding 列表 |
| GET | `/api/v1/remote/nodes/{nodeId}/status` | `{online: bool}`，由 `Hub.IsNodeOnline(nodeID)` 判定（owner 或 admin 可看） |
| GET | `/api/v1/remote/nodes/{nodeId}/ls?path=...` | 通过 `Hub.ProxyRequest(nodeID,"ls",{path})` 走 `/ws/remote` 反向请求 daemon |
| GET | `/api/v1/remote/nodes/{nodeId}/read?path=...` | 同上，action=`read` |

### 鉴权

- node 与 binding 的 CRUD 走**普通用户 JWT** + `node.UserID == user.ID` 校验；admin **不能**替别人改 node/binding，但 `status / ls / read` 三条 admin 可读。
- daemon 与 server 的 WS 用 `connection_token`（query），不是 user JWT——服务器端把 token 反查到 node、关联 owner。

### 错误转换 (`writeRemoteResponse`)

daemon 返回的 JSON `{error:"..."}` 会被映射成 HTTP 状态：

| daemon error | HTTP status |
|--------------|-------------|
| `path_not_allowed` | 403 |
| `file_not_found` / `path_not_found`* | 404 |
| `file_too_large` | 413 |
| `timeout` | 504 |
| 其它 | 502 |

\* 注意 `ls` / `stat` 的 ENOENT 返回 `path_not_found`，但 `writeRemoteResponse` 只显式映射 `file_not_found`——`path_not_found` 会落到 default 的 502。这是个小不一致，调用方需注意。

如果 node 当前不在线，server 直接返回 `503 node_offline`，不进 RPC；超时（`context.DeadlineExceeded` 从 hub 传上来）返回 504。

### 数据库片段

```sql
remote_nodes:
  id, user_id, machine_name,
  connection_token UNIQUE,
  last_seen_at,
  created_at

remote_bindings:
  id, node_id, channel_id, path, label,
  UNIQUE(node_id, channel_id, path)
```

## 6. 客户端入口

浏览器侧的 `<NodeManager/>`（`packages/client/src/components/NodeManager.tsx`）通过 `lib/api.ts` 调以上端点，提供 node 注册 → 拿 token → 配 binding → 看 status / browse 的全套界面。channel 详情里也有 binding 入口。

## 7. 部署建议

- 用专用低权限 user 跑 daemon；不要 `sudo`。
- `--dirs` 给最小、最具体的目录；尽量是只读的内容仓库（数据集、文档、构建产物），不要交出源码或 secret 目录。
- `connection_token` 是 long-lived 凭证，泄露等于把对应 node 上白名单内的文件全交出去——把它当 SSH key 一样保存，必要时删了 node 重新注册即可换 token。
- `wss://` 在跨网络部署时是必须的（token 走 query string，明文 ws 等于裸传）。
- 容器化时把白名单挂成 `:ro` volume，给沙箱再加一层文件系统层面的只读保证。

## 8. 已知 quirks

1. `path_not_found` 在 server 这一侧没有专门 mapping，会落到 502（见 §5）。
2. `readFile` 用 `utf-8` 解码所有文件——二进制内容会损坏。当前没有 base64 通道，也没有按 MIME 切分支。
3. 没有任何配额：单 daemon 没限速、server 端 `Hub.ProxyRequest` 也只有超时；恶意/失控调用方可以把 daemon 推到 IO 瓶颈。
4. 心跳是 30 s 单向 ping，server 不回 pong——daemon 不会"探测自己被对端忘记"，只能等下次 send 失败或 TCP 断开。

## npm 发布
包名 `@codetreker/borgee-remote-agent`，repository `codetreker/borgee`。

## 发布流程

1. Dev 开 PR bump `packages/remote-agent/package.json` 版本号 → CI 通过 → 合并
2. 在 GitHub Actions 手动触发 `Publish Remote Agent` workflow
3. Workflow 自动检查版本是否已发布，未发布则 `npm publish --provenance`
4. npm registry 兜底：同版本不允许覆盖
