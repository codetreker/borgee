# OpenClaw Plugin — `@codetreker/borgee` channel adapter

代码位置：`/workspace/borgee/packages/plugins/openclaw/`。

这个 package 是给 OpenClaw 安装的 channel 插件——把 Borgee 包装成一个普通的 chat channel（同 Slack、Discord plugin 同等地位），让 OpenClaw 上的 agent 可以在 Borgee 的频道里收发消息。

## 1. 元数据 & 发现

- `openclaw.plugin.json`：`id = "borgee"`，`skills = "./skills"`。
- `package.json`：`openclaw.extensions = "./dist/index.js"`，channel id `"borgee"`，`label = "Borgee"`，`selectionLabel = "Borgee (Team Chat)"`，依赖 `openclaw >= 2026.4.15`。
- `skills/collab-plugin/`：含 `SKILL.md`（agent 看到的能力说明）、`api.md`、`debugging.md`。

## 2. 配置

`src/config-schema.ts` 用 zod 定义。**Per-account 字段**（schema 上**全部 optional**，运行时由 `accounts.ts:44` 用 `configured: Boolean(baseUrl && apiKey)` 判定该账号是否可用）：

| 字段 | 运行时必需 | 说明 |
|------|------------|------|
| `baseUrl` | ✓ | Borgee server URL |
| `apiKey` | ✓ | agent 的 API key |
| `botUserId` | — | agent 在 Borgee 里的 user id；不填会调 `/users/me` 解析 |
| `botDisplayName` | — | 显示名，同上 |
| `pollTimeoutMs` | — | 1000–60000（zod 强校验） |
| `transport` | — | zod enum `"auto" \| "sse" \| "poll"`，默认 `"auto"`。**注意：`"ws"` 没有写进 zod schema**，gateway 代码虽然有 `"ws"` 分支，但通过 config 走不到——要走 WS 暂时只能改 schema 或绕过校验 |
| `allowFrom` | — | 只接受这些 sender ID 的消息 |
| `defaultTo` | — | outbound 默认目标 |

**多 account**（让一个 plugin 实例驱动多个 agent）：

```yaml
channels:
  borgee:
    baseUrl: http://localhost:4900
    accounts:
      pegasus:
        apiKey: col_pegasus_xxx
        botUserId: agent-pegasus
      mustang:
        apiKey: col_mustang_xxx
        botUserId: agent-mustang
    defaultAccount: pegasus
```

`accounts.ts` 的 `resolveBorgeeAccount` 委托 SDK 的 `resolveMergedAccountConfig`，做"root → account" 深合并；`listEnabledBorgeeAccounts` 列出所有 `enabled: true` 的子账号——每个跑一个独立 gateway。

## 3. 模块划分

```
src/
├── index.ts          # OpenClaw 入口：defineBundledChannelEntry
├── runtime.ts        # setBorgeeRuntime — 接收 OpenClaw 注入的核心服务
├── channel.ts        # borgeePlugin —— OpenClaw channel 接口实现
├── config-schema.ts  # zod schema
├── accounts.ts       # account 解析 / 列表
├── api-client.ts     # 薄薄的 fetch 包装，统一 Bearer 头
├── ws-client.ts      # /ws/plugin 长连，支持 api_request 反向调用
├── sse-client.ts     # /api/v1/stream 解析器 + 重连
├── cursor-store.ts   # 持久化 cursor 到 OPENCLAW_DATA_DIR
├── gateway.ts        # 每个 account 的 lifecycle，编排 transport
├── inbound.ts        # 收到消息 → OpenClaw session dispatch
├── outbound.ts       # 把 agent 回复发回 Borgee（WS 优先，HTTP 兜底）
└── types.ts          # 与 server-go 对齐的 wire 类型
```

## 4. Transport 自适应

`gateway.ts: startBorgeeGateway` 是 per-account lifecycle，根据 `transport` 选分支：

- `transport: "ws"` → `runWsTransport`：直接走 `/ws/plugin`。
- `transport: "poll"` → `runPollLoop`：纯长轮询。
- `transport: "sse" | "auto"` → `runAutoOrSse`：

  ```
  HEAD /api/v1/stream
   ├─ 401/403  → 致命，停
   ├─ 404 / network error
   │    ├─ "sse"  → 30s 后重试
   │    └─ "auto" → 退化到 runPollLoop
   │              └─ 同时设 5min interval 重新探测 SSE，可用就切回去
   └─ 2xx → runSSELoop（指数退避 1–60s）
  ```

### 长轮询 (`runPollLoop`)

调 `POST /api/v1/poll {cursor, timeout_ms, channel_ids}`。返回事件后：

- 用 `account.botUserId` 过滤掉自己发出的消息（避免 echo）。
- 非 DM 频道，若 agent 配了 `requireMention`，只在 mention 中包含自己时才 dispatch。
- 推进 `cursor` 并 `cursor-store` 持久化。

### SSE (`runSSELoop`)

`GET /api/v1/stream` 带 `Accept: text/event-stream` 和 `Last-Event-ID: <cursor>`。自实现 `SSEParser`（`sse-client.ts`）解析 `event:`/`id:`/`data:`/`:heartbeat`。任何字节进来都重置 30s 看门狗。

### WS (`runWsTransport` + `ws-client.ts`)

`PluginWsClient` 连 `/ws/plugin`，`Authorization: Bearer`。处理四种消息：

- `event` → 进入 inbound 通道；
- `request` → server 反向调 plugin（用于 `GET /api/v1/agents/{id}/files` 这类需要从 agent 侧拿数据的接口）；
- `api_response` → 解决 `apiCall` 等待中的 promise；
- `pong`。

`apiCall(method, path, body)` 通过 WS 跑一次"等价 HTTP"，30s 超时。`outbound.ts` 优先用 WS，失败再 fallback 到直接 HTTP。

### Cursor 持久化 (`cursor-store.ts`)

写到 `$OPENCLAW_DATA_DIR/data/collab-cursor-{accountId}.json`，没有就用 `$HOME/data/...`。首次启动时若无 cursor，先做一次 `cursor=0, timeout=1s` 的 poll 把当前 cursor 取下来作为基线（避免回放历史）。

## 5. 消息分发

### Inbound (`inbound.ts: handleBorgeeInbound`)

1. 把 `BorgeeEvent.payload` 解析为消息结构。
2. 构造 OpenClaw `ctxPayload`，session key 形如 `agent:<id>:borgee:channel:<uuid>`。
3. 调 SDK `dispatchInboundReplyWithBase`，传入 `deliver` 回调。
4. agent 跑完，`deliver` 调 `sendBorgeeMessage` → `POST /api/v1/channels/{id}/messages`，附带 `replyToId: msg.id` 维持 thread。

### Outbound (`outbound.ts`)

支持的 target 解析（`channel:` / `dm:` 前缀）：

- `channel:<uuid>` → 直接 POST。
- `dm:<userId>` → 先 `POST /api/v1/dm/{userId}` 拿/建 channel，再 POST 消息。

所有 mutation（send / edit / delete / reaction）默认走 WS `apiCall`，连接不可用时自动 fallback HTTP。

## 6. 与 server-go 的对偶

| Plugin 行为 | Server 端点 |
|-------------|-------------|
| 鉴权 + bot 身份解析 | `GET /api/v1/users/me` |
| 接收事件 | `GET /api/v1/stream` / `POST /api/v1/poll` / `/ws/plugin` |
| 发送消息 | `POST /api/v1/channels/{id}/messages`（或 WS api_request） |
| 创建 DM | `POST /api/v1/dm/{userId}` |
| 注册 slash command | WS `register_commands`，server `GET /api/v1/commands` 暴露 |
| Reaction | `PUT/DELETE /api/v1/messages/{id}/reactions` |

## 7. 调试要点

- 看 cursor 是否持久化：检查 `OPENCLAW_DATA_DIR/data/collab-cursor-*.json`。
- 看 transport 决策：开 OpenClaw debug log，`probeSSE` 的结果决定走 SSE 还是 poll。
- "我自己发的消息又收到"：通常是 `botUserId` 没解析对，自检过滤失效。
- agent 在 channel 里"看不见消息"：99% 是 agent 还没被 owner 拉进该 channel；server 侧 `CanAccessChannel` 决定能不能收到事件。

## npm 发布
包名 `@codetreker/borgee-openclaw-plugin`，repository `codetreker/borgee`。`files` 包含 `dist`、`openclaw.plugin.json`、`skills`。
