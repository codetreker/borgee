# COL-B19: Remote Explorer — Task Breakdown

## T1: 数据库 + Node/Binding CRUD API

**依赖**: 无

**改动文件**:

| 文件 | 改动 | 预估行数 |
|------|------|---------|
| `packages/server/src/db.ts` | 新增 `remote_nodes` + `remote_bindings` 表迁移（ALTER 模式，同现有 migration 风格） | +30 |
| `packages/server/src/queries.ts` | 新增 node/binding CRUD query 函数（createRemoteNode, deleteRemoteNode, listRemoteNodes, createRemoteBinding, deleteRemoteBinding, listRemoteBindings, listChannelRemoteBindings） | +60 |
| `packages/server/src/routes/remote.ts` | **新建**：6 个 REST endpoint（见 design §4.1 + §4.2），owner-only 校验 | +150 |
| `packages/server/src/index.ts` | import + register remoteRoutes；auth hook 白名单加 `/ws/remote` | +5 |

**预估总行数**: ~245

**验证方式**:
- 单元测试 `src/__tests__/remote.test.ts`：创建 node → 获取 token → 创建 binding → 列表 → 删除
- `curl` 手动验证各 endpoint 返回 + owner-only 403

---

## T2: WS Endpoint + RemoteNodeManager

**依赖**: T1（需要 remote_nodes 表 + query 已存在）

**改动文件**:

| 文件 | 改动 | 预估行数 |
|------|------|---------|
| `packages/server/src/remote-node-manager.ts` | **新建**：参照 `plugin-manager.ts`（103 行），实现 register/unregister/isOnline/request + pending request 超时 | +110 |
| `packages/server/src/routes/ws-remote.ts` | **新建**：参照 `ws-plugin.ts`（130 行），`/ws/remote?token=xxx` endpoint，token 认证，注册到 RemoteNodeManager，消息分发 | +100 |
| `packages/server/src/index.ts` | import registerWsRemoteRoutes + 调用 | +3 |

**预估总行数**: ~213

**验证方式**:
- 用 `wscat` 或脚本模拟 Agent 连接：`wscat -c ws://localhost:4900/ws/remote?token=xxx`
- 验证 token 错误返回 4001；连接后发 request 能收到 response
- 测试 `remote-node-manager.test.ts`：register → isOnline → request/response → unregister → pending reject

---

## T3: Remote Agent 包

**依赖**: T2（需要 WS endpoint 可用来端到端测试）

**改动文件**:

| 文件 | 改动 | 预估行数 |
|------|------|---------|
| `packages/remote-agent/package.json` | **新建**：name `@collab/remote-agent`，bin 入口，依赖 ws + commander | +25 |
| `packages/remote-agent/tsconfig.json` | **新建**：参照 packages/plugin/tsconfig.json | +15 |
| `packages/remote-agent/src/index.ts` | **新建**：CLI 入口（commander 解析 --server --token --dirs），启动 Agent | +40 |
| `packages/remote-agent/src/agent.ts` | **新建**：RemoteAgent 类 — WS 连接、token 认证、心跳、指数退避重连、request/response 分发 | +120 |
| `packages/remote-agent/src/fs-ops.ts` | **新建**：ls/read/stat 实现 + isPathAllowed 目录白名单检查 | +80 |

**预估总行数**: ~280

**验证方式**:
- `npx tsx packages/remote-agent/src/index.ts --server ws://localhost:4900 --token xxx --dirs /tmp` 连接成功
- Server 侧 RemoteNodeManager.isOnline 返回 true
- 通过 T2 的 WS 发 ls request → Agent 返回目录列表
- 白名单外路径返回 error
- 断开连接后自动重连

---

## T4: 文件代理 API

**依赖**: T1 + T2（需要 DB + RemoteNodeManager）

**改动文件**:

| 文件 | 改动 | 预估行数 |
|------|------|---------|
| `packages/server/src/routes/remote.ts` | 追加 2 个 endpoint：`GET /api/v1/remote/nodes/:nodeId/ls` + `GET /api/v1/remote/nodes/:nodeId/read`，校验 owner → 调用 remoteNodeManager.request → 返回结果 | +60 |

**预估总行数**: ~60

**验证方式**:
- T3 Agent 在线时：`curl /api/v1/remote/nodes/:id/ls?path=/tmp` 返回目录列表
- `curl /api/v1/remote/nodes/:id/read?path=/tmp/test.txt` 返回文件内容
- Agent 离线时返回 503 + 合适错误信息
- 非 owner 请求返回 403

---

## T5: 前端 Remote Tab + 文件树

**依赖**: T4（需要文件代理 API 可调用）

**改动文件**:

| 文件 | 改动 | 预估行数 |
|------|------|---------|
| `packages/client/src/lib/api.ts` | 新增 API 调用函数：fetchRemoteBindings, remoteLs, remoteReadFile | +30 |
| `packages/client/src/components/RemotePanel.tsx` | **新建**：Remote tab 容器，列出 channel 绑定的远程目录，选中后展开 RemoteTree | +80 |
| `packages/client/src/components/RemoteTree.tsx` | **新建**：远程文件树组件（参照 WorkspacePanel 的文件列表模式），懒加载子目录，点击文件调 RemoteFileViewer | +120 |
| `packages/client/src/components/RemoteFileViewer.tsx` | 已存在，修改为调用 remoteReadFile → 复用 FileViewer | +40 |
| `packages/client/src/components/ChannelView.tsx` | tab 状态 `'chat' \| 'workspace' \| 'remote'`，新增 Remote tab 按钮 + 渲染 RemotePanel | +20 |
| `packages/client/src/index.css` | remote panel / tree 相关样式 | +40 |

**预估总行数**: ~330

**验证方式**:
- 浏览器中 ChannelView 出现 Remote tab
- 点击 Remote tab → 显示绑定的远程目录列表
- 点击目录 → 展开子文件/文件夹
- 点击文件 → FileViewer 预览（代码高亮、Markdown、图片）
- Agent 离线时显示离线提示
- owner-only：非 owner 不可见 Remote tab

---

## T6: Node 管理 UI

**依赖**: T1 + T5（需要 Node API + 前端框架就绪）

**改动文件**:

| 文件 | 改动 | 预估行数 |
|------|------|---------|
| `packages/client/src/lib/api.ts` | 新增：fetchRemoteNodes, createRemoteNode, deleteRemoteNode, createRemoteBinding, deleteRemoteBinding | +40 |
| `packages/client/src/components/NodeManager.tsx` | **新建**：Node 管理 UI — 添加 node（显示 token + 复制命令）、在线状态指示、绑定目录到 channel、解绑 | +200 |
| `packages/client/src/components/ChannelView.tsx` 或 `App.tsx` | 入口：channel 设置区域加 "Remote Nodes" 入口 | +15 |
| `packages/client/src/index.css` | node-manager 相关样式 | +30 |

**预估总行数**: ~285

**验证方式**:
- 打开 Node 管理页面，创建 node → 显示 token + 一行启动命令
- 复制命令后在远程机器执行，Node 管理页显示在线（绿点）
- 绑定目录到 channel → 回到 ChannelView Remote tab 能看到
- 删除 node → 确认弹窗 → 删除成功

---

## 依赖关系图

```
T1 ─────┬──→ T2 ──→ T3
        │         ↘
        ├──────→ T4 (T1+T2)
        │              ↘
        └──────────→ T5 (T4) ──→ T6 (T1+T5)
```

**推荐实施顺序**: T1 → T2 → T3 → T4 → T5 → T6

**总预估行数**: ~1413 行新增/修改代码
