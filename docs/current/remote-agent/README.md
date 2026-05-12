# Remote Agent

remote-agent 是用户显式启动的本机目录暴露进程。它通过 server 的 `/ws/remote` WebSocket 反连到 Borgee，只处理远端文件浏览和读取请求；它不是 host-bridge/helper，不读取 `host_grants`，不走 Unix Domain Socket，也不提供安装、执行或网络出站能力。

```text
user browser / API client
  |
  | user cookie / Bearer API key
  v
server REST /api/v1/remote/*
  |
  | Hub.ProxyRequest(node_id, action, params)
  v
server WebSocket /ws/remote
  |
  | remote node connection_token
  v
borgee-remote-agent --server --token --dirs
  |
  | process-level path allowlist
  v
local filesystem read/list/stat
```

## 负责什么

remote-agent 负责把用户登记的 `RemoteNode` 连接到 server，并在 agent 进程本地执行 `ls`、`read`、`stat`。CLI 要求 `--server`、`--token`、`--dirs`，目录列表来自逗号分隔的 `--dirs` 参数；启动后 agent 连接 `${serverUrl}/ws/remote?token=...`，30 秒发送一次 ping，并用 1s 到 30s 的退避重连。证据：`packages/remote-agent/src/index.ts`、`packages/remote-agent/src/agent.ts`。

server 负责 remote node 和 binding 的用户侧管理，包括节点创建/删除、binding 创建/删除、按 channel 查询 binding、节点在线状态、远程目录列举和远程文件读取。所有这些 REST 路由都挂在 `authMw` 后面，并在具体 handler 中校验 `node.UserID == user.ID`。证据：`packages/server-go/internal/api/remote.go`、`packages/server-go/internal/server/server.go`。

WebSocket hub 负责把已认证的 remote connection 注册为在线节点，并为 REST handler 提供 `IsNodeOnline` 和 `ProxyRequest` 代理。`/ws/remote` 接受 `Authorization: Bearer <token>` 或 `?token=`，通过 `GetRemoteNodeByToken` 解析 `RemoteNode`，连接后更新 `last_seen_at` 并注册到 `Hub.remotes`。证据：`packages/server-go/internal/ws/remote.go`、`packages/server-go/internal/ws/hub.go`、`packages/server-go/internal/store/queries_phase3.go`。

## 不负责什么

remote-agent 不做 host grant 授权。host grants 的 REST、SQLite 消费、ACL、audit、Landlock/sandbox-exec 都属于 host-bridge/helper 路径，不在 remote-agent 中实现。证据：remote-agent 代码只导入 `fs-ops` 和 WebSocket 依赖；host grant 代码位于 `packages/server-go/internal/api/host_grants.go` 与 `packages/borgee-helper/internal/*`。

remote-agent 不提供 OS sandbox。它的边界是 Node.js 进程中的 `path.resolve` 前缀检查、2 MiB 文件大小上限，以及同步只读文件操作。证据：`packages/remote-agent/src/fs-ops.ts`。

remote-agent 不是 plugin rail。plugin 通过 `/ws/plugin` 使用 agent 的 API key 连接，能发送 `api_request` 回打 server handler；remote-agent 通过 `/ws/remote` 使用 remote node `connection_token`，只处理 remote file actions。证据：`packages/server-go/internal/ws/plugin.go`、`packages/server-go/internal/ws/remote.go`。

## 和其他模块的接口

| 接口 | 方向 | 身份 | 数据 | 证据 |
| --- | --- | --- | --- | --- |
| `/api/v1/remote/nodes` | user API -> server | `borgee_token` cookie / Bearer API key / dev bypass | list/create/delete `RemoteNode` | `packages/server-go/internal/api/remote.go`, `packages/server-go/internal/auth/middleware.go` |
| `/api/v1/remote/nodes/{nodeId}/bindings` | user API -> server | same user auth | channel/path/label binding | `packages/server-go/internal/api/remote.go`, `packages/server-go/internal/store/models.go` |
| `/api/v1/channels/{channelId}/remote-bindings` | user API -> server | same user auth | channel-scoped binding list filtered by node owner | `packages/server-go/internal/store/queries_phase2b.go` |
| `/api/v1/remote/nodes/{nodeId}/ls` | user API -> server -> agent | user owns node; agent token already connected | path query, remote JSON response | `packages/server-go/internal/api/remote.go`, `packages/server-go/internal/server/server.go` |
| `/api/v1/remote/nodes/{nodeId}/read` | user API -> server -> agent | user owns node; agent token already connected | path query, remote JSON response | `packages/server-go/internal/api/remote.go`, `packages/remote-agent/src/fs-ops.ts` |
| `/ws/remote` | agent -> server | remote node `connection_token` | `{type,id,data}` request/response envelope | `packages/server-go/internal/ws/remote.go`, `packages/remote-agent/src/agent.ts` |

## 子模块

- `protocol.md` 展开 REST 管理面、WebSocket tunnel、token、timeout、错误映射和当前 wire mismatch。
- `filesystem-boundary.md` 展开 `--dirs`、路径解析、读大小、MIME、只读行为和与 host-bridge 的边界差异。

## Known risk / unknown

- `CreateRemoteNode` 生成 32 字节随机 hex `ConnectionToken`，但 `RemoteNode.ConnectionToken` JSON 标记为 `json:"-"`；在已核对代码里未看到把 token 返回给用户的单独 endpoint。当前 token 交付路径未确定。证据：`packages/server-go/internal/store/queries_phase2b.go`、`packages/server-go/internal/store/models.go`、`packages/server-go/internal/api/remote.go`。
- server 代理发送 `{action, params:{path}}`，TypeScript agent 当前读取 `data.path`；这会导致当前 wire shape 不一致。证据：`packages/server-go/internal/server/server.go`、`packages/remote-agent/src/agent.ts`。
