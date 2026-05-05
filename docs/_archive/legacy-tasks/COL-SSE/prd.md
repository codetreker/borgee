# SSE 推送改造 — PRD

日期：2026-04-20 | 状态：Draft

## 背景

Collab 当前 OpenClaw Plugin 通过 HTTP 长轮询（long polling）从服务端获取消息。长轮询存在以下问题：

1. **消息丢失风险**：每次 poll 都是新请求，频道列表快照过期导致丢消息
2. **高延迟**：30 秒超时窗口，最坏情况延迟 30 秒才能收到新消息
3. **cursor 管理复杂**：超时时推不推进、推多少，逻辑脆弱且难以调试

改用 SSE（Server-Sent Events）可以解决以上全部问题：持久连接消除快照过期，实时推送消除延迟，服务端维护推送位置消除客户端 cursor 管理。

## 目标用户

- **AI Agent（通过 OpenClaw Plugin 接入）**：需要实时接收频道消息并快速响应
- **系统运维**：需要 Plugin 升级平滑、无需改配置

## 核心需求

### 需求 1: SSE 推送端点

Collab 服务端新增 SSE 端点 `GET /api/v1/stream`，客户端建立持久 HTTP 连接，服务端实时推送事件，支持 `Last-Event-ID` 断线续传。

- **用户故事**：作为 AI agent，我想通过 SSE 实时接收频道消息，以便零延迟响应用户
- **验收标准**：
  - [ ] SSE 连接建立后，新消息在 1 秒内推送到客户端
  - [ ] 支持按 channel_members 过滤（只推送 agent 是成员的频道消息）
  - [ ] 断线重连后通过 `Last-Event-ID` 补发错过的消息
  - [ ] 认证走 API key（query param 或 header 均支持）

#### 事件格式

```
event: message
id: <单调递增的事件 ID>
data: <JSON，与现有 poll 返回的消息格式一致>

event: heartbeat
data: {}
```

- `event: message` — 新消息推送，data 字段复用现有消息 JSON 结构
- `event: heartbeat` — 保活空事件，防止连接被中间代理超时断开
- 每个事件都携带 `id` 字段，客户端断线重连时通过 `Last-Event-ID` header 告知服务端续传起点

### 需求 2: Plugin 改造

OpenClaw Plugin 从长轮询改为 SSE 客户端，保持现有的 inbound/outbound 消息格式不变。

- **用户故事**：作为系统运维，Plugin 改用 SSE 后不需要修改 OpenClaw 配置
- **验收标准**：
  - [ ] Plugin 启动后自动建立 SSE 连接
  - [ ] 收到 SSE 事件后正常 dispatch 到 agent session
  - [ ] 断线自动重连（指数退避，初始 1 秒，上限 60 秒）
  - [ ] 发消息仍走 REST API（`POST /messages`），不走 SSE

#### 连接生命周期

```
Plugin 启动
  → 建立 SSE 连接 GET /api/v1/stream
  → 收到 event: message → dispatch 到 agent session
  → 连接断开 → 指数退避重连（携带 Last-Event-ID）
  → 重连成功 → 补发错过的消息 → 恢复正常推送
```

### 需求 3: 向后兼容

保留长轮询 `/api/v1/poll` 端点不删，SSE 和长轮询可以共存。

- **验收标准**：
  - [ ] 旧版 Plugin 仍可使用长轮询，行为不变
  - [ ] 新版 Plugin 默认使用 SSE

## 不在本需求范围

- **前端（浏览器）改用 SSE** — 浏览器已有 WebSocket，不需要改
- **双向通信** — SSE 是单向推送（服务端 → 客户端），发消息仍走 REST
- **WebSocket 方案** — SSE 满足需求且实现更简单，不引入 WebSocket

## 成功指标

| 指标 | 改造前（长轮询） | 改造后（SSE） |
|------|-------------------|---------------|
| 消息延迟 | 最高 30 秒 | < 1 秒 |
| cursor 丢消息 | 偶发 | 不再出现 |
| 连接数（稳态） | 每 30 秒一个新请求 | 1 个持久连接 |

## 开放问题

| # | 问题 | 建议 | 状态 |
|---|------|------|------|
| 1 | SSE 心跳间隔——多久发一次空事件保活？ | 15 秒 | 待确认 |
| 2 | `requireMention` 过滤在服务端还是客户端？ | 服务端（减少不必要的网络传输） | 待确认 |
