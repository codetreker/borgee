# Remote Agent Protocol

本协议分两层：user REST 管理面负责节点、binding、状态和文件代理入口；remote WebSocket 数据面负责 server 到 agent 的请求/响应 tunnel。两层使用不同凭据，不能互换。

```text
REST management plane
  user auth -> /api/v1/remote/* -> Store + Hub

WebSocket data plane
  remote token -> /ws/remote -> Hub.RegisterRemote -> RemoteConn.SendRequest
```

## 负责什么

REST 管理面负责 owner-only remote resources。`RemoteHandler.RegisterRoutes` 挂载 `GET/POST/DELETE /api/v1/remote/nodes`、node binding CRUD、channel binding list、node status、`ls` 和 `read`，并通过 `authMw` 进入 user rail。每个需要 node 的 handler 都读取 node 后检查 `node.UserID != user.ID` 时返回 403。证据：`packages/server-go/internal/api/remote.go`。

WebSocket 数据面负责 remote node 上线和 RPC-style 请求。`/ws/remote` 从 Bearer header 或 `?token=` 读取 token，用 `GetRemoteNodeByToken` 解析节点，成功后更新 `last_seen_at` 并注册 remote connection；server 发出的请求 envelope 是 `{type:"request", id, data:<payload>}`，agent 返回 `{type:"response", id, data:<result>}`。证据：`packages/server-go/internal/ws/remote.go`、`packages/server-go/internal/store/queries_phase3.go`。

server proxy 负责 timeout 和错误映射。`RemoteConn.SendRequest` 等待 10 秒，超时返回 `context.DeadlineExceeded`；REST 层把 proxy 超时映射为 504，把 remote body 中的 `path_not_allowed`、`file_not_found`、`file_too_large` 分别映射为 403、404、413，其他 remote error 映射为 502。证据：`packages/server-go/internal/ws/remote.go`、`packages/server-go/internal/api/remote.go`。

## 不负责什么

remote protocol 不负责 helper daemon 的 UDS IPC。helper IPC 使用 JSON line、handshake `{agent_id}`、action `read_file/list_files/network_egress`；remote-agent 使用 WebSocket，action 是 `ls/read/stat`。证据：`packages/borgee-helper/internal/ipc/ipc.go`、`packages/remote-agent/src/agent.ts`。

remote protocol 不负责 plugin API bridge。plugin WebSocket 用 user API key 认证，并能通过 `api_request` 把请求回放进 server handler；remote WebSocket 使用 remote node token，只 resolve server 发出的文件请求。证据：`packages/server-go/internal/ws/plugin.go`、`packages/server-go/internal/ws/remote.go`。

remote protocol 不负责 admin override。remote REST routes 只挂 user `authMw`，没有 `/admin-api` remote route。证据：`packages/server-go/internal/api/remote.go`、`packages/server-go/internal/server/server.go`。

## 和其他模块的接口

### REST 管理面

| Route | 行为 | 权限 | 证据 |
| --- | --- | --- | --- |
| `GET /api/v1/remote/nodes` | list current user's nodes | `authMw`; `ListRemoteNodes(user.ID)` | `packages/server-go/internal/api/remote.go` |
| `POST /api/v1/remote/nodes` | create node with `machine_name` | `authMw`; token generated in store | `packages/server-go/internal/api/remote.go`, `packages/server-go/internal/store/queries_phase2b.go` |
| `DELETE /api/v1/remote/nodes/{id}` | delete node and bindings | owner check | `packages/server-go/internal/api/remote.go`, `packages/server-go/internal/store/queries_phase2b.go` |
| `GET/POST/DELETE /api/v1/remote/nodes/{nodeId}/bindings` | bind node path to channel | owner check on node | `packages/server-go/internal/api/remote.go` |
| `GET /api/v1/channels/{channelId}/remote-bindings` | list bindings for channel/user | query joins `remote_nodes` by user | `packages/server-go/internal/store/queries_phase2b.go` |
| `GET /api/v1/remote/nodes/{nodeId}/status` | `online` from hub | owner check, hub optional | `packages/server-go/internal/api/remote.go` |
| `GET /api/v1/remote/nodes/{nodeId}/ls?path=` | proxy `ls` | owner check + online remote | `packages/server-go/internal/api/remote.go` |
| `GET /api/v1/remote/nodes/{nodeId}/read?path=` | proxy `read` | owner check + online remote | `packages/server-go/internal/api/remote.go` |

### WebSocket 数据面

| Message | Producer | Consumer | Shape | 证据 |
| --- | --- | --- | --- | --- |
| auth connect | agent | server | Bearer token or `?token=` | `packages/server-go/internal/ws/remote.go`, `packages/remote-agent/src/agent.ts` |
| heartbeat | agent/server | peer | `{type:"ping"}` / `{type:"pong"}` | `packages/server-go/internal/ws/remote.go`, `packages/remote-agent/src/agent.ts` |
| request | server | agent | `{type:"request", id, data}` | `packages/server-go/internal/ws/remote.go` |
| response | agent | server | `{type:"response", id, data}` | `packages/server-go/internal/ws/remote.go`, `packages/remote-agent/src/agent.ts` |

## Flow: `read`

```text
GET /api/v1/remote/nodes/{nodeId}/read?path=P
  -> authMw resolves user
  -> handler loads node and checks node.UserID == user.ID
  -> handler checks Hub.IsNodeOnline(nodeId)
  -> hubRemoteAdapter.ProxyRequest(nodeId, "read", {path:P})
  -> RemoteConn.SendRequest waits up to 10s
  -> remote-agent handleRequest dispatches readFile
  -> response body returned or mapped to HTTP error
```

## Known risk / unknown

- 当前 server payload 是 `{"action": action, "params": params}`，但 agent 读取 `data.path` 而不是 `data.params.path`。这不是文档推测，是代码形状不一致。证据：`packages/server-go/internal/server/server.go`、`packages/remote-agent/src/agent.ts`。
- `RemoteNode.ConnectionToken` 被 `json:"-"` 隐藏，`POST /api/v1/remote/nodes` 返回的 `node` 不会包含 token；已核对代码中未看到 token retrieve endpoint。证据：`packages/server-go/internal/store/models.go`、`packages/server-go/internal/api/remote.go`。
- remote response parser 只识别 `file_not_found`，但 `ls/stat` 的 missing path 返回 `path_not_found`，REST 层会把它落到 502。证据：`packages/remote-agent/src/fs-ops.ts`、`packages/server-go/internal/api/remote.go`。
