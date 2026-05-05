# COL-B21: Plugin SSE → WS — Task Breakdown

## T1: Server WS Endpoint + PluginManager

**目标**：新建 `/ws/plugin` WebSocket endpoint，Plugin 通过 apiKey 认证后注册到 PluginManager。

**改动文件**：
| 文件 | 动作 | 预估行数 |
|------|------|----------|
| `packages/server/package.json` | 加 `@fastify/websocket` 依赖 | ~1 |
| `packages/server/src/plugin-manager.ts` | **新建**：PluginManager 类（connections Map, register/unregister/pushEvent/broadcast） | ~80 |
| `packages/server/src/routes/ws-plugin.ts` | **新建**：`/ws/plugin` endpoint，query param apiKey 认证（复用 `Q.getUserByApiKey`），消息分发 | ~60 |
| `packages/server/src/app.ts` 或 `index.ts` | 注册 `@fastify/websocket` 插件 + ws-plugin route | ~5 |

**验证**：
- `wscat -c "ws://localhost:3000/ws/plugin?apiKey=xxx"` 连接成功
- 无效 apiKey 返回 4001 close
- 多个 Plugin 同时连接，PluginManager.connections 正确维护

**依赖**：无

---

## T2: 事件推送迁移（SSE → WS 双路推送）

**目标**：现有 `notifySSEClients()` 事件推送逻辑同时推送给 WS Plugin 连接。SSE 保留不动。

**改动文件**：
| 文件 | 动作 | 预估行数 |
|------|------|----------|
| `packages/server/src/plugin-manager.ts` | 增加 `pushEvent(agentId, event, data)` 实现，JSON 格式 `{ type: "event", event, data }` | ~15 |
| `packages/server/src/routes/stream.ts` | `notifySSEClients()` 末尾调用 `pluginManager.broadcastEvent(kind, payload)`，或在事件写入点（如 `createMessage` 后的广播处）统一调用 | ~10 |
| `packages/server/src/routes/ws-plugin.ts` | 处理 Plugin 端 `ping`/`pong` 心跳 | ~10 |

**验证**：
- Plugin WS 连接后，在 Web 端发消息，Plugin WS 收到 `{ type: "event", event: "new_message", data: {...} }`
- SSE 连接同时仍然正常工作（回归测试）
- 所有事件类型（message, message_edited, message_deleted, reaction_update）均通过 WS 推送

**依赖**：T1

---

## T3: Server → Plugin 请求通道

**目标**：PluginManager 支持向 Plugin 发送请求并等待响应（request/response 模式，带超时）。

**改动文件**：
| 文件 | 动作 | 预估行数 |
|------|------|----------|
| `packages/server/src/plugin-manager.ts` | 增加 `request(agentId, data, timeoutMs?)` — 生成 `req_xxx` ID，发 `{ type: "request", id, data }`，注册 pending Promise Map，超时 reject | ~50 |
| `packages/server/src/routes/ws-plugin.ts` | `on('message')` 处理 `type: "response"` — 按 `id` 匹配 pending 并 resolve | ~15 |

**验证**：
- 单元测试：`pluginManager.request(agentId, { action: "read_file", path: "/foo" })` → Plugin 回 response → Promise resolve
- 超时测试：Plugin 不回 response → 30s 后 reject
- 连接断开时所有 pending request reject

**依赖**：T1

---

## T4: Plugin WS Client + 重连

**目标**：Plugin 端新建 `PluginWsClient`，替代 SSE 作为主连接通道，支持事件监听 + server 请求响应 + 指数退避重连。

**改动文件**：
| 文件 | 动作 | 预估行数 |
|------|------|----------|
| `packages/plugin/src/ws-client.ts` | **新建**：`PluginWsClient` 类 — `connect()`, `onEvent()`, `onRequest()`, `apiCall()`, 指数退避重连（1s→2s→4s→…→30s cap） | ~150 |
| `packages/plugin/src/gateway.ts` | `startCollabGateway` 中增加 `transport === "ws"` 分支，优先尝试 WS，失败降级到 SSE/poll | ~30 |
| `packages/plugin/src/types.ts` | `ResolvedCollabAccount.transport` 增加 `"ws"` 选项 | ~2 |

**验证**：
- Plugin 通过 WS 连接 server，收到事件并正确 dispatch 到 `handleCollabInbound`
- 断开连接后自动重连（观察 backoff 时间递增）
- 稳定连接 30s 后断开，backoff 重置为 1s
- SSE 降级仍可用

**依赖**：T1, T2

---

## T5: Plugin API 请求走 WS（outbound 改造）

**目标**：Plugin 的 outbound 操作（send_message, add_reaction, edit, delete）从 HTTP 改为通过 WS `api_request`/`api_response` 通道。

**改动文件**：
| 文件 | 动作 | 预估行数 |
|------|------|----------|
| `packages/plugin/src/ws-client.ts` | `apiCall(method, path, body?)` 实现 — 生成 `api_xxx` ID，发 `{ type: "api_request", id, data: { method, path, body } }`，等 `api_response` | ~30 |
| `packages/plugin/src/outbound.ts` | `sendCollabText` / `handleCollabReaction` / `handleCollabMessageEdit` / `handleCollabMessageDelete` 检测 WS 连接可用时走 `wsClient.apiCall()`，否则降级 HTTP | ~40 |
| `packages/server/src/routes/ws-plugin.ts` | `on('message')` 处理 `type: "api_request"` — 提取 method/path/body，通过 `fastify.inject()` 或直接调 service 层执行，返回 `api_response` | ~40 |

**验证**：
- Plugin 通过 WS 发消息（`apiCall('POST', '/api/v1/channels/:id/messages', {...})`）→ server 返回 201 + message
- add_reaction / edit / delete 均通过 WS 成功
- WS 断开时自动降级到 HTTP（outbound 不中断）
- 并发 API 请求正确按 id 关联响应

**依赖**：T1, T4

---

## 依赖关系图

```
T1 (WS Endpoint + PluginManager)
├── T2 (事件推送迁移)
├── T3 (请求通道)
└── T4 (Plugin WS Client)
    └── T5 (API 走 WS)
```

## 总预估

| Task | 预估行数 | 复杂度 |
|------|----------|--------|
| T1 | ~150 | 中 |
| T2 | ~35 | 低 |
| T3 | ~65 | 中 |
| T4 | ~180 | 高 |
| T5 | ~110 | 中 |
| **合计** | **~540** | |
