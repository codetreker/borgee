# Overview — 系统全景

## 一、组件与边界

| 组件 | 语言 | 主要职责 | 入口 |
|------|------|----------|------|
| `server-go` | Go 1.22 | HTTP API、SQLite 存储、WebSocket Hub、SSE、长轮询、事件日志 | `cmd/collab/main.go` |
| `client` | TS / React 18 | 用户 SPA + admin SPA（同一构建产物，双 HTML 入口） | `src/main.tsx`、`src/admin/main.tsx` |
| `remote-agent` | TS / Node 22 | 在用户机器上以 daemon 形式运行，对 server-go 暴露受限的本机目录读取 | `src/index.ts` |
| `plugins/openclaw` | TS / Node 22 | 让 OpenClaw agent 把 Borgee 当作一个 channel adapter（类似接入 Slack） | `src/index.ts` |

**Agent 的两条接入路径**

1. **OpenClaw plugin**（推荐）：agent 由 OpenClaw 平台运行，plugin 负责把 Borgee 的 inbound 消息映射成 OpenClaw 的 session，把 agent 的回复发回 Borgee。
2. **裸 API key**：任何持有 agent `api_key` 的进程都可以直接走 REST + WS 接收/发送消息（OpenClaw plugin 内部就是这么做的）。

`remote-agent` **不是 agent 本身**，而是一个让 server 能远程列出/读取用户机器上文件的 daemon——绑定到某个 channel 后，channel 中的 agent 可以查阅用户开放的目录。

## 二、关键术语

- **User**：人类或 agent，一行 `users` 记录，`role = "member" | "agent" | "system"`（**ADM-0.3 后** `admin` 不在此 enum；admin 走独立 `admins` 表）。
- **Admin**：独立子系统，自己的 `admins` 表 + `borgee_admin_session` cookie + `/admin-api/*` 路由。详见 [`admin/README.md`](admin/README.md)。
- **Agent**：仍是一行 `users` 记录，`role = "agent"`，`owner_id` 指向所属 user。Agent 通过 `api_key` 鉴权。
- **Organization**：`organizations` 表（CM-1.1，v=2）。1 person = 1 org，UI 永不暴露，数据层一等公民。注册 user 时自动建对应 org（CM-1.2），agent 继承 owner 的 `org_id`。
- **Channel**：`channels` 表，`type = "channel"`（普通频道） / `"dm"`（私聊） / `"system"`（CM-onboarding 起，`#welcome` 私属频道用），用 `visibility = "public" | "private"` 区分可见性，soft delete via `deleted_at`。
- **DM**：作为 `type="dm"` 的 channel 存储，name 形如 `dm:<uid_low>_<uid_high>`，两个用户 ID 排序后拼接，避免重复。
- **Welcome channel**：`type="system"` 的 per-user 私属频道，注册时自动建（CM-onboarding，v=7），含一条 `sender_id='system'` 的欢迎消息 + `quick_action` JSON 按钮。
- **Channel Group**：`channel_groups`，用于侧边栏分组，与 channel 一样用 LexoRank 排序。
- **Agent invitation**：`agent_invitations` 表（CM-4.0，v=3） + HTTP API（CM-4.1） + ws push（RT-0 server，#237）。跨 org 拉 agent 进 channel 的状态机（`pending → approved/rejected/expired`，blueprint §4.2 流程 B）。
- **Event**：`events` 表（自增 cursor），是所有 push 通道（WS / SSE / 长轮询）的统一事实来源。
- **Workspace**：每个 channel 一棵虚拟文件树（`workspace_files`），既能上传，也会自动收录从消息中产生的文件。
- **Remote Node / Binding**：用户在自己机器上跑 `remote-agent`，注册成 node；node 可以与 channel 上的某个 path 绑定，channel 内的 agent 通过 server 代理读取该路径。
- **Schema migrations**：`schema_migrations` 表（INFRA-1a，v=1） + `internal/migrations/registry.go` 版本化迁移引擎。Phase 1+ 所有新 schema 改动走此引擎；旧 `Store.Migrate()` 大杂烩仅保 v0 表骨架。当前注册 v=1..10（v=6 跳号）。详见 [`server/migrations.md`](server/migrations.md)。

## 三、写路径扇出

任何"会被订阅者看见"的状态变化（发消息、加入频道、改 topic、reaction…）都遵循同一个模式：

```
HTTP/WS handler
   │
   ▼
Store.CreateXxx (transactional)
   ├── INSERT 业务表（messages / channel_members / …）
   └── INSERT events 表 (cursor++, kind, channel_id, payload JSON)
   │
   ▼
Hub.BroadcastEventToChannel(channelId, kind, payload)
   ├── 立即推送到订阅了该 channel 的 WS clients
   └── Hub.SignalNewEvents() 唤醒所有长轮询 / SSE 等待者
       └── waiter 醒来后自己再去 GetEventsSinceWithChanges(cursor) 拉新事件
```

代码位置：`internal/store/queries.go: CreateMessageFull`、`internal/api/messages.go`、`internal/ws/hub.go`、`internal/api/poll.go`、`internal/api/sse.go`。

事件 cursor 的好处：

- **断线续传**：客户端记住最近一个 cursor，重连时 `Last-Event-ID` / poll body 带上，server 回放之后所有事件。
- **统一三种传输**：WS、SSE、长轮询从同一张 `events` 表出数据，业务侧只需调用一次 `Hub.BroadcastEventToChannel`。

## 四、跨进程消息流（典型场景）

### 4.1 人 → agent 在 channel 中收到消息

```
浏览器 MessageInput
  │ (1) sendWsMessage chat_message {client_message_id, content}
  ▼
server-go /ws  ── handleSendMessage ──▶ Store.CreateMessageFull
                                          ├── messages 行
                                          └── events 行 (kind=new_message)
                                       Hub.BroadcastEventToChannel
                                          ├── 推 WS 给所有订阅了该 channel 的浏览器
                                          └── 唤醒 SSE / poll 等待者
  │
  │ (2) plugin 通过 SSE / poll / WS 收到 new_message event
  ▼
OpenClaw plugin (gateway.ts: handleEvent)
  │ 过滤 self、过滤 requireMention 但未 @ 的消息
  ▼
inbound.ts → dispatchInboundReplyWithBase (OpenClaw SDK)
  │ session key = agent:<id>:borgee:channel:<uuid>
  ▼
agent 生成回复
  │
  ▼
outbound.ts → POST /api/v1/channels/{id}/messages
  │  （或走已有 WS api_request 通道）
  ▼
server-go 又走一次写路径扇出，浏览器就看到 agent 的回复
```

### 4.2 agent 列出 user 本机目录

```
agent 在 OpenClaw 内调用 fs tool
  │
  ▼
plugin 转成 GET /api/v1/remote/nodes/{id}/ls?path=...
  │
  ▼
server-go api/remote.go → hubRemoteAdapter.ProxyRequest
  │ 通过 /ws/remote 给目标 remote-agent 发 type=request
  ▼
remote-agent agent.ts:onMessage → fs-ops.ls
  │ isPathAllowed 校验 → 在白名单 dirs 内才执行
  ▼
type=response 回到 server-go → HTTP 响应给 plugin
```

### 4.3 浏览器乐观发送

`MessageInput.tsx` 在按下回车时：

1. `dispatch(ADD_PENDING_MESSAGE)`，生成 `client_message_id`。
2. `sendWsMessage({type:'chat_message', client_message_id, ...})`。
3. 启动 10s 计时器，到时未收到 `message_ack` 则降级为 `api.sendMessage()` REST 调用。
4. 服务端的 `message_ack` / `new_message` 到达后，`AppContext` reducer 把 pending 替换为已确认消息。

## 五、与 PRD 的对应关系

PRD 定义了三种角色（admin / user / agent）和"agent 由 owner 拉入 channel"的归属规则。代码里的实现要点：

- 角色靠 `users.role` 区分（**ADM-0.3 后只剩 `member / agent / system`**, `admin` 走 `admins` 表）；`owner_id` 表示 agent 归属。
- 权限走 `user_permissions` 表。**AP-0 + AP-0-bis (R3 决议 #1)** 默认: human (`role=member`) → `(*, *)`一行全权; agent (`role=agent`) → `(message.send, *) + (message.read, *)` 两行; admin → 不写默认行 (admin 权威只活在 `/admin-api/*` 一轨)。频道创建者回填 `channel.delete / channel.manage_members / channel.manage_visibility`，scope `channel:<id>`。
- 中间件 `auth.RequirePermission` 在路由级别强制权限；**ADM-0.2 起**不再有 `users.role == "admin"` 短路，纯 `user_permissions` 表查。

## 六、不在系统内的东西

- **Agent 本身的 LLM 推理**：Borgee 是接入平台，不跑模型。
- **画布 / 文档协作**：v3 才规划，代码里目前只有 per-channel workspace（文件树），不是 RFR-style 渲染。
- **音视频 / 推送**：未实现。
- **Cloudflare Access**：README 历史版本提过，当前 server-go 没有这块代码。
