# COL-R01: Go Server 重写 — 技术设计

日期：2026-04-25 | 状态：Draft | 作者：Collab 架构师

## 1. 背景与问题

Collab 当前 server 位于 `packages/server/src/`，基于 TypeScript、Fastify、`@fastify/websocket`、`better-sqlite3` 实现。它同时承担浏览器 REST API、客户端 WebSocket、Plugin WebSocket、Remote Explorer WebSocket、SSE/长轮询、文件上传、workspace 文件、前端静态文件和 SQLite schema 初始化。

本次重写目标是在 `packages/server-go/` 中用 Go 替换 server，前端、Plugin、Remote 节点和现有 SQLite 数据库零改动。Go 版本必须复制 TS server 的 wire protocol，而不是重新设计产品行为。

当前 TS server 的主要约束：

- 路由和响应格式已经被前端、插件和集成测试固化，Go server 需要 1:1 兼容。
- SQLite schema 由应用启动时创建和迁移，Go 版本需要能直接打开现有 `data/collab.db`。
- 事件系统同时服务 WS、SSE、长轮询和 Plugin WS，任何漏发、重复或权限过滤差异都会表现为实时同步 bug。
- 鉴权同时支持 JWT Cookie、API Key Bearer、开发环境 bypass，以及 WS/SSE 特有的兼容入口。
- 文件系统状态分为 `UPLOAD_DIR` 和 `WORKSPACE_DIR`，需要与 DB 记录保持一致。

## 2. 目标

可量化验收标准：

1. 前端代码零修改，`packages/client/dist` 由 Go server serve 后，核心页面和现有交互正常。
2. Go server 覆盖 TS server 全部公开端点：REST、`/ws`、`/ws/plugin`、`/ws/remote`、`/api/v1/stream`、`/api/v1/poll`、`/uploads/*`、SPA fallback。
3. REST API 路径、状态码、JSON 字段、错误 `{ "error": "..." }` 结构与 TS server 兼容。
4. WS/SSE 消息类型兼容；客户端、agent plugin、remote node 不需要改协议。
5. 现有 SQLite DB 文件无需迁移即可启动；Go 初始化 schema 使用 GORM `AutoMigrate` 做兼容同步，并保留必要缺列/索引 backfill。
6. Go 测试覆盖率 ≥ 85%；关键路径必须有 `httptest`/WS 集成测试：auth、channels、messages、reactions、workspace、plugin WS、remote WS、SSE replay。
7. Docker 镜像使用多阶段构建，最终镜像仅包含 Go binary、client dist、运行时数据目录和 CGO 运行依赖。

## 3. 技术栈与边界

已确定技术栈：

- HTTP：Go 1.22+ `net/http` + `http.ServeMux`
- WebSocket：`github.com/coder/websocket`
- ORM：`gorm.io/gorm`
- SQLite driver：`gorm.io/driver/sqlite`（项目只依赖 GORM driver API，不直接依赖底层 SQLite driver API）
- JWT：`github.com/golang-jwt/jwt/v5`
- Password hash：`golang.org/x/crypto/bcrypt`
- 测试：`github.com/stretchr/testify`

保持不变：

- URL namespace：`/api/v1/*`、`/ws*`、`/uploads/*`
- SQLite schema 和毫秒时间戳语义
- API Key 前缀 `col_`、JWT Cookie 名 `collab_token`
- 图片上传限制：10MB，`image/jpeg|png|gif|webp`
- Plugin/Remote request-response envelope：`{type:"request", id, data}` / `{type:"response", id, data, error}`

## 4. 整体架构

```
Browser / Agent / Remote Node
          |
          v
packages/server-go/cmd/collab
  |
  +-- internal/server       net/http server, middleware chain, graceful shutdown
  +-- internal/config       env parsing and defaults
  +-- internal/auth         JWT cookie, API key, dev bypass, permission check
  +-- internal/api          REST handlers grouped by domain
  +-- internal/ws           client WS, plugin WS, remote WS, hub managers
  +-- internal/store        GORM models, AutoMigrate, ORM queries, transactions
  +-- internal/model        shared request/response/domain structs
  +-- internal/static       uploads and client dist file serving helpers
  +-- scripts/lib/coverage  Go coverage helper, adapted from /workspace/syntrix
          |
          v
SQLite data/collab.db + data/uploads + data/workspaces
```

核心设计原则：

- `api` 层只做 HTTP decode/validate/encode，不拼查询条件。
- `store` 层封装 GORM model、查询和事务，保留 TS 查询语义；禁止在业务代码中手写裸 SQL CRUD。
- `ws.Hub` 是实时广播的唯一出口，REST handler 通过接口调用广播，避免循环依赖。
- `events` 表是 Poll/SSE/Plugin event bridge 的事实来源，所有消息和频道变更继续写入。
- 所有时间戳兼容 TS：业务表大多为 Unix ms `INTEGER`，workspace/remote 表保留 SQLite `datetime('now')` 文本。

## 5. 项目结构

```
packages/server-go/
├── cmd/collab/main.go
├── internal/
│   ├── api/
│   │   ├── admin.go
│   │   ├── agents.go
│   │   ├── auth.go
│   │   ├── channels.go
│   │   ├── channel_groups.go
│   │   ├── commands.go
│   │   ├── dm.go
│   │   ├── messages.go
│   │   ├── poll.go
│   │   ├── reactions.go
│   │   ├── remote.go
│   │   ├── upload.go
│   │   └── workspace.go
│   ├── auth/
│   │   ├── middleware.go
│   │   ├── password.go
│   │   └── permissions.go
│   ├── config/config.go
│   ├── model/*.go
│   ├── server/server.go
│   ├── store/
│   │   ├── db.go
│   │   ├── migrations.go
│   │   ├── models.go
│   │   ├── queries.go
│   │   └── lexorank.go
│   └── ws/
│       ├── client.go
│       ├── hub.go
│       ├── plugin.go
│       └── remote.go
├── scripts/lib/coverage/
├── Dockerfile
├── Makefile
└── go.mod
```

`scripts/lib/coverage/` 参考 `/workspace/syntrix/scripts/lib/coverage/`，保留 AST priority/report 能力，适配 `go test -coverprofile` 输出，用于 CI 标注未覆盖关键路径。

`store/` 只暴露按业务命名的方法，例如 `CreateMessage`、`ListChannelMessages`、`GrantPermission`。这些方法内部使用 GORM model 和 `Where`/`First`/`Find`/`Create`/`Save`/`Delete`/`Transaction`，避免 handler 或 store 方法散落裸 SQL。

## 6. 配置管理

Go `internal/config.Config` 从环境变量读取，启动时集中校验：

| Env | 默认值 | 用途 |
|---|---:|---|
| `PORT` | `4900` | HTTP listen port |
| `HOST` | `0.0.0.0` | HTTP listen host |
| `LOG_LEVEL` | `info` | 日志级别 |
| `NODE_ENV` | 空 | `development` 放宽 CORS/cookie/dev bypass |
| `CORS_ORIGIN` | `https://collab.codetrek.work` | 非开发环境允许 origin |
| `DATABASE_PATH` | `data/collab.db` | SQLite 文件 |
| `UPLOAD_DIR` | `data/uploads` | `/uploads/*` 静态目录 |
| `WORKSPACE_DIR` | `data/workspaces` | workspace 文件数据 |
| `CLIENT_DIST` | `packages/client/dist` | 前端构建产物目录；Docker 内为 `/app/client/dist` |
| `JWT_SECRET` | `dev-secret`（development）/ 空（production） | JWT 签发与验证密钥；生产环境必填 |
| `DEV_AUTH_BYPASS` | `false` | development only |
| `ADMIN_EMAIL` / `ADMIN_PASSWORD` | 空 | bootstrap admin |
| `AGENT_*_API_KEY` | 随机 | legacy seed agents |

生产环境如果 `JWT_SECRET` 为空，启动失败；开发环境默认使用 `dev-secret`，确保本地签发和验证 JWT cookie 使用同一密钥。`DEV_AUTH_BYPASS` 只影响认证兜底，不改变 JWT 校验逻辑。

## 7. 中间件链

推荐链路：

```
Recover
  -> RequestID / Logger
  -> CORS
  -> SecurityHeaders
  -> RateLimiter
  -> StaticBypass
  -> AuthMiddleware
  -> Route Handler
  -> JSON Error Adapter
```

`RateLimiter` 先按 client IP 做轻量内存限流；`POST /api/v1/auth/register` 固定限制为 10 req/min，超限返回 `429 {"error":"Rate limit exceeded"}`。其他端点可按 TS 行为保留更宽松默认值或仅记录。

鉴权白名单与 TS 保持一致：

- `/health`
- `/api/v1/poll`
- `/api/v1/stream` 和 `/api/v1/stream?*` query variant
- `/api/v1/auth/*`
- `/assets/*`
- `/uploads/*`
- `/ws`、`/ws?*`、`/ws/plugin*`、`/ws/remote*`
- `/`、`/favicon.ico`
- 非 `/api/` 且非 `/ws` 的 frontend route

`AuthMiddleware` 成功后把 `model.User` 写入 request context。`RequirePermission(permission, scopeResolver)` 复刻 TS 逻辑：admin 直接通过；非 admin 查询 `user_permissions`，允许 `scope='*'` 或精确 scope。

## 8. Auth 流程

REST API：

1. Cookie `collab_token`：用 `jwt/v5` 校验 `{userId,email}`，查用户，拒绝 `deleted_at` 或 `disabled`。
2. Header `Authorization: Bearer <api_key>`：查 `users.api_key`，同样检查禁用/删除。
3. Dev bypass：仅 `NODE_ENV=development && DEV_AUTH_BYPASS=true`；优先 `x-dev-user-id`，否则选择第一个 admin。
4. 失败返回 `401 {"error":"Authentication required"}` 或更具体错误。

浏览器注册/登录：

```json
POST /api/v1/auth/login
{ "email": "a@example.com", "password": "password123" }

200
Set-Cookie: collab_token=...; HttpOnly; Path=/; SameSite=Lax; Max-Age=604800
{ "user": { "id": "u1", "display_name": "Alice", "role": "member" } }
```

Go 实现密码哈希使用 `golang.org/x/crypto/bcrypt`，必须兼容现有 `password_hash`，并用于新用户注册、管理员创建/重置密码和登录校验。

WS/SSE 鉴权兼容入口：

- `/ws`：Bearer API key、deprecated query `token`、JWT cookie、dev query `user_id`。
- `/ws/plugin`：Bearer API key、deprecated query `apiKey`。
- `/ws/remote`：Bearer remote node `connection_token`、deprecated query `token`。
- `/api/v1/stream`：JWT cookie、Bearer API key、deprecated query `api_key`。
- `/api/v1/poll`：Bearer API key 优先，body `api_key` deprecated fallback。

## 9. REST API 映射

以下表格是 Go handler 必须实现的公开 surface。示例只展示关键字段，实际响应保留 TS server 返回的全部字段。

### 9.1 系统与认证

| Method | Path | 说明 |
|---|---|---|
| `GET` | `/health` | `{status,timestamp,uptime,ws_clients}` |
| `GET` | `/api/v1/online` | WS 在线 + poll/SSE `last_seen_at` 合并 |
| `POST` | `/api/v1/auth/register` | invite code 注册并设置 cookie |
| `POST` | `/api/v1/auth/login` | 邮箱密码登录并设置 cookie |
| `POST` | `/api/v1/auth/logout` | 清空 cookie |
| `GET` | `/api/v1/users/me` | 当前用户，不返回 `api_key/password_hash`，附 permissions |
| `GET` | `/api/v1/me/permissions` | 当前用户权限详情 |

```json
POST /api/v1/auth/register
{
  "invite_code": "8b6c...",
  "email": "alice@example.com",
  "password": "password123",
  "display_name": "Alice"
}

201
{ "user": { "id": "uuid", "display_name": "Alice", "role": "member", "email": "alice@example.com" } }
```

注册校验保持服务端强约束：`password` UTF-8 bytes 长度必须为 8-72（bcrypt 限制），`display_name` 字符长度必须为 1-50，email trim/lowercase 后唯一。注册成功后自动加入所有未删除的 public channels，并设置登录 cookie。

### 9.2 用户、Admin、Agent

| Method | Path | Body / Query | 响应 |
|---|---|---|---|
| `GET` | `/api/v1/users` | - | `{users}` |
| `GET` | `/api/v1/admin/users` | admin | `{users}` |
| `POST` | `/api/v1/admin/users` | `{id?,email?,password?,display_name,role}` | `201 {user}` |
| `PATCH` | `/api/v1/admin/users/:id` | `{display_name?,password?,role?,require_mention?,disabled?}` | `{user}` |
| `DELETE` | `/api/v1/admin/users/:id` | - | `{ok:true}` soft delete |
| `POST` | `/api/v1/admin/users/:id/api-key` | - | `{api_key}` |
| `DELETE` | `/api/v1/admin/users/:id/api-key` | - | `{ok:true}` |
| `GET` | `/api/v1/admin/users/:id/permissions` | - | `{user_id,role,permissions,details}` |
| `POST` | `/api/v1/admin/users/:id/permissions` | `{permission,scope?}` | `201 {ok,permission,scope}` |
| `DELETE` | `/api/v1/admin/users/:id/permissions` | `{permission,scope?}` | `{ok:true}` |
| `POST` | `/api/v1/admin/invites` | `{expires_in_hours?,note?}` | `201 {invite}` |
| `GET` | `/api/v1/admin/invites` | - | `{invites}` |
| `DELETE` | `/api/v1/admin/invites/:code` | - | `{ok:true}` |
| `GET` | `/api/v1/admin/channels` | - | `{channels}` |
| `DELETE` | `/api/v1/admin/channels/:id/force` | - | `{ok:true}` |
| `POST` | `/api/v1/agents` | `{display_name,avatar_url?,permissions?,id?}` | `201 {agent}` |
| `GET` | `/api/v1/agents` | - | `{agents}` owner-scoped/admin all |
| `GET` | `/api/v1/agents/:id` | - | `{agent}` with `api_key` |
| `DELETE` | `/api/v1/agents/:id` | - | `{ok:true}` |
| `POST` | `/api/v1/agents/:id/rotate-api-key` | - | `{api_key}` |
| `GET` | `/api/v1/agents/:id/permissions` | - | `{agent_id,permissions,details}` |
| `PUT` | `/api/v1/agents/:id/permissions` | `{permissions:[{permission,scope?}]}` | `{agent_id,permissions,details}` |
| `GET` | `/api/v1/agents/:id/files?path=...` | owner only | proxied plugin file result |

Admin 用户操作业务错误需与 TS 行为兼容：不能删除当前登录用户，不能修改当前登录用户自己的 `role`，soft delete 用户时需要级联 soft delete 其 owner-scoped agents 并失效这些 agents 的 API key/在线连接。

```json
POST /api/v1/agents
{ "id": "agent-build", "display_name": "Build Bot", "avatar_url": "/bot.png" }

201
{
  "agent": {
    "id": "agent-build",
    "display_name": "Build Bot",
    "role": "agent",
    "owner_id": "admin-jianjun",
    "api_key": "col_..."
  }
}
```

### 9.3 Channels、DM、Groups、Messages

| Method | Path | Body / Query | 响应 |
|---|---|---|---|
| `GET` | `/api/v1/channels` | - | `{channels,groups}` |
| `POST` | `/api/v1/channels` | `{name,topic?,member_ids?,visibility?}` | `201 {channel}` |
| `GET` | `/api/v1/channels/:channelId` | - | `{channel}` with members |
| `GET` | `/api/v1/channels/:channelId/preview` | - | public 24h `{messages,channel}` |
| `PUT` | `/api/v1/channels/:channelId` | `{name?,topic?,visibility?}` | `{channel}` |
| `PUT` | `/api/v1/channels/:channelId/topic` | `{topic}` | `{channel}` |
| `POST` | `/api/v1/channels/:channelId/join` | - | `{ok:true}` |
| `POST` | `/api/v1/channels/:channelId/leave` | - | `{ok:true}` |
| `POST` | `/api/v1/channels/:channelId/members` | `{user_id}` | `201 {ok:true}` |
| `DELETE` | `/api/v1/channels/:channelId/members/:userId` | - | `{ok:true}` |
| `GET` | `/api/v1/channels/:channelId/members` | - | `{members}` |
| `PUT` | `/api/v1/channels/:channelId/read` | - | `{ok:true}` |
| `DELETE` | `/api/v1/channels/:channelId` | - | `204` or `{ok:true}` |
| `PUT` | `/api/v1/channels/reorder` | `{channel_id,after_id,group_id?}` | `{channel:{id,position,group_id}}` |
| `GET` | `/api/v1/channel-groups` | - | `{groups}` |
| `POST` | `/api/v1/channel-groups` | `{name}` | `201 {group}` |
| `PUT` | `/api/v1/channel-groups/:groupId` | `{name}` | `{group}` |
| `DELETE` | `/api/v1/channel-groups/:groupId` | - | `{ok,ungrouped_channel_ids}` |
| `PUT` | `/api/v1/channel-groups/reorder` | `{group_id,after_id}` | `{group}` |
| `POST` | `/api/v1/dm/:userId` | - | `{channel,peer}` |
| `GET` | `/api/v1/dm` | - | `{channels}` |
| `GET` | `/api/v1/channels/:channelId/messages?before=&after=&limit=` | limit max 200 | `{messages,has_more}` |
| `GET` | `/api/v1/channels/:channelId/messages/search?q=&limit=` | limit max 50 | `{messages}` |
| `POST` | `/api/v1/channels/:channelId/messages` | `{content,content_type?,reply_to_id?,mentions?}` | `201 {message}` |
| `PUT` | `/api/v1/messages/:messageId` | `{content}` | `{message}` |
| `DELETE` | `/api/v1/messages/:messageId` | - | `204` |

```json
POST /api/v1/channels
{
  "name": "Eng Chat",
  "topic": "Build and deploy",
  "visibility": "private",
  "member_ids": ["user-2", "agent-build"]
}

201
{ "channel": { "id": "uuid", "name": "eng-chat", "topic": "Build and deploy", "visibility": "private" } }
```

```json
POST /api/v1/channels/ch1/messages
{
  "content": "deploy <@agent-build>",
  "content_type": "text",
  "reply_to_id": null,
  "mentions": ["agent-build"]
}

201
{ "message": { "id": "m1", "channel_id": "ch1", "sender_id": "u1", "content": "deploy <@agent-build>", "mentions": ["agent-build"] } }
```

### 9.4 Reactions、Commands、Poll、SSE

| Method | Path | Body / Query | 响应 |
|---|---|---|---|
| `PUT` | `/api/v1/messages/:messageId/reactions` | `{emoji}` | `{ok:true,reactions}` |
| `DELETE` | `/api/v1/messages/:messageId/reactions` | `{emoji}` | `{ok:true,reactions}` |
| `GET` | `/api/v1/messages/:messageId/reactions` | - | `{reactions}` |
| `GET` | `/api/v1/commands?channelId=` | - | `{builtin,agent}` |
| `POST` | `/api/v1/poll` | `{api_key?,cursor?,since_id?,timeout_ms?,channel_ids?}` | `{cursor,events}` |
| `HEAD` | `/api/v1/stream` | - | `200` probe |
| `GET` | `/api/v1/stream` | `Last-Event-ID` optional | `text/event-stream` |

```json
POST /api/v1/poll
Authorization: Bearer col_xxx
{ "cursor": 100, "timeout_ms": 30000, "channel_ids": ["ch1"] }

200
{
  "cursor": 101,
  "events": [
    { "cursor": 101, "kind": "message", "channel_id": "ch1", "payload": "{...}", "created_at": 1777075200000 }
  ]
}
```

`/api/v1/poll` 必须把请求中的 `channel_ids` 与当前用户可访问频道集合取交集；无权限频道静默过滤，事件查询只使用过滤后的 channel IDs，避免 private channel 泄露。未传 `channel_ids` 时使用用户当前可访问的全部频道。

SSE event 格式：

```text
event: message
id: 101
data: {"id":"m1","channel_id":"ch1","sender_id":"u2","content":"hello"}

event: heartbeat
id: 101
data: {}
```

SSE 连接建立并完成 header flush 后先写一帧注释 `:connected\n\n`，方便客户端区分网络已连通但尚无业务事件的状态。

### 9.5 Upload、Workspace、Remote

| Method | Path | Body / Query | 响应 |
|---|---|---|---|
| `POST` | `/api/v1/upload` | multipart `file` | `201 {url,content_type}` |
| `GET` | `/api/v1/channels/:channelId/workspace?parentId=` | - | `{files}` |
| `POST` | `/api/v1/channels/:channelId/workspace/upload?parentId=` | multipart `file` | `201 {file}` |
| `GET` | `/api/v1/channels/:channelId/workspace/files/:id` | - | bytes, inline disposition |
| `PUT` | `/api/v1/channels/:channelId/workspace/files/:id` | `{content}` | `{file}` |
| `PATCH` | `/api/v1/channels/:channelId/workspace/files/:id` | `{name}` | `{file}` |
| `DELETE` | `/api/v1/channels/:channelId/workspace/files/:id` | - | `204` |
| `POST` | `/api/v1/channels/:channelId/workspace/mkdir` | `{name,parentId?}` | `201 {file}` |
| `POST` | `/api/v1/channels/:channelId/workspace/files/:id/move` | `{parentId}` | `{file}` |
| `GET` | `/api/v1/workspaces` | - | `{files}` |
| `GET` | `/api/v1/remote/nodes` | - | `{nodes}` |
| `POST` | `/api/v1/remote/nodes` | `{machine_name}` | `201 {node}` with token |
| `DELETE` | `/api/v1/remote/nodes/:id` | - | `{ok:true}` |
| `GET` | `/api/v1/remote/nodes/:nodeId/bindings` | - | `{bindings}` |
| `POST` | `/api/v1/remote/nodes/:nodeId/bindings` | `{channel_id,path,label?}` | `201 {binding}` |
| `DELETE` | `/api/v1/remote/nodes/:nodeId/bindings/:id` | - | `{ok:true}` |
| `GET` | `/api/v1/channels/:channelId/remote-bindings` | - | `{bindings}` |
| `GET` | `/api/v1/remote/nodes/:nodeId/ls?path=` | proxied | remote result |
| `GET` | `/api/v1/remote/nodes/:nodeId/read?path=` | proxied | remote result |
| `GET` | `/api/v1/remote/nodes/:nodeId/status` | - | `{online}` |

workspace 同一用户、频道、父目录下文件名冲突时自动重命名：先保留原名，冲突后依次尝试 `name (1).ext`、`name (2).ext`；目录无扩展名时使用 `name (1)`。DB 唯一约束仍作为最后防线，冲突重试次数耗尽返回 `409`。

```json
POST /api/v1/remote/nodes
{ "machine_name": "mbp-1" }

201
{ "node": { "id": "uuid", "user_id": "u1", "machine_name": "mbp-1", "connection_token": "..." } }
```

## 10. WS 消息类型映射

所有 WebSocket connection 必须使用 per-connection write pump/channel 模式：业务 goroutine 只向 outbound channel 投递消息，单独 write pump 串行调用 `websocket.Conn.Write`，并在 close path 统一关闭 channel 和连接。禁止多个 goroutine 直接并发写同一个 `coder/websocket` connection。

### 10.1 Client WS `/ws`

认证成功后创建 connection id，加入 presence map。服务端 30s 心跳：发送 `{type:"ping"}`，客户端回 `{type:"pong"}`；未响应则关闭。

客户端到服务端：

| type | Payload | 行为 |
|---|---|---|
| `subscribe` | `{channel_id}` | 校验频道存在、成员或 admin，回 `subscribed` |
| `unsubscribe` | `{channel_id}` | 移除订阅，回 `unsubscribed` |
| `ping` | - | 回 `pong` |
| `pong` | - | 标记 alive |
| `typing` | `{channel_id}` | 向同频道其他订阅者广播 `typing` |
| `send_message` | `{channel_id,content,content_type?,reply_to_id?,mentions?,client_message_id?}` | 创建消息，ack/nack，广播 `new_message` |
| `register_commands` | `{commands}` | agent only，注册 slash commands |

`register_commands` 边界：每个 agent 最多注册 100 条命令；内置命令名冲突时跳过并计入 `skipped`，不覆盖 builtin；命令名必须匹配 `^[a-z][a-z0-9_-]{0,31}$`；`description` 最长 200 chars，`params` schema JSON 序列化后最长 16KB，超限命令跳过并在响应中返回原因。

服务端到客户端：

| type | Payload |
|---|---|
| `presence` | `{user_id,display_name,status}` |
| `subscribed` / `unsubscribed` | `{channel_id}` |
| `typing` | `{channel_id,user_id,display_name}` |
| `message_ack` | `{client_message_id,message}` |
| `message_nack` | `{client_message_id,code,message}` |
| `new_message` | `{message}` |
| `message_edited` | `{message}` |
| `message_deleted` | `{message_id,channel_id,deleted_at}` |
| `reaction_update` | `{message_id,channel_id,reactions}` |
| `channel_added` | `{channel}` |
| `channel_removed` | `{channel_id}` |
| `channel_created` | `{channel}` |
| `channel_deleted` | `{channel_id,name}` |
| `channel_updated` | `{channel_id,topic}` |
| `visibility_changed` | `{channel_id,visibility}` |
| `user_joined` / `user_left` | `{channel_id,user_id,display_name,member_count}` |
| `channels_reordered` | `{channel_id,position,group_id}` |
| `group_created` / `group_updated` | `{group}` |
| `group_deleted` | `{group_id,ungrouped_channel_ids}` |
| `channel_groups_reordered` | `{group_id,position}` |
| `commands_registered` | `{registered,skipped}` |
| `commands_updated` | no extra fields |
| `error` | `{message}` |

`send_message` NACK code 必须保留：`NOT_FOUND`、`NOT_MEMBER`、`PERMISSION_DENIED`、`INVALID_CONTENT_TYPE`、`INVALID_COMMAND`、`INTERNAL_ERROR`。

### 10.2 Plugin WS `/ws/plugin`

客户端到服务端：`ping`、`pong`、`response`、`api_request`。服务端到客户端：`pong`、`request`、`event`、`api_response`、`error`。

Plugin WS 不做 server-initiated heartbeat；服务端只在收到 client `ping` 时响应 `pong`，并用读超时/连接关闭感知离线。

```json
// plugin -> server
{ "type": "api_request", "id": "req-1", "data": { "method": "GET", "path": "/api/v1/channels" } }

// server -> plugin
{ "type": "api_response", "id": "req-1", "data": { "status": 200, "body": { "channels": [] } } }

// server -> plugin event bridge
{ "type": "event", "event": "message", "data": { "id": "m1", "channel_id": "ch1" } }
```

Go 中不能使用 Fastify `inject`，需要实现内部 loopback：构造 `httptest.NewRequest`，补 `Authorization: Bearer <apiKey>` 和 JSON body，调用同一个 root handler，读取 recorder 结果后返回。

### 10.3 Remote WS `/ws/remote`

客户端到服务端：`ping`、`pong`、`response`。服务端到客户端：`pong`、`request`、`error`。

Remote WS 不做 server-initiated heartbeat；服务端只响应 remote node 发来的 `ping`，不主动发送 ping，以兼容现有 remote client 行为。

```json
// server -> remote node
{ "type": "request", "id": "req_abcd", "data": { "action": "ls", "path": "/repo" } }

// remote node -> server
{ "type": "response", "id": "req_abcd", "data": { "entries": [{ "name": "README.md", "type": "file" }] } }
```

## 11. DB Schema GORM Models

Go `store.Migrate` 启动时执行 `PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON; PRAGMA busy_timeout=5000;`，再调用 `db.AutoMigrate(...)` 同步 schema。GORM model 是 schema 的事实来源；不再维护手写建表语句。迁移只允许做 `AutoMigrate` 无法表达的兼容补丁、历史数据 backfill 和 SQLite 索引细节。

```go
package store

import "time"

type Channel struct {
    ID         string  `gorm:"primaryKey;size:36"`
    Name       string  `gorm:"not null;unique;size:100"`
    Topic      string  `gorm:"not null;default:'';size:500"`
    Visibility string  `gorm:"not null;default:public;size:20"`
    CreatedAt  int64   `gorm:"not null"`
    CreatedBy  string  `gorm:"not null;size:36;index"`
    Type       string  `gorm:"not null;default:channel;size:20"`
    DeletedAt  *int64  `gorm:"index"`
    Position   string  `gorm:"not null;default:0|aaaaaa;size:50;index"`
    GroupID    *string `gorm:"size:36;index"`
}

type ChannelGroup struct {
    ID        string `gorm:"primaryKey;size:36"`
    Name      string `gorm:"not null;size:100"`
    Position  string `gorm:"not null;size:50;index"`
    CreatedBy string `gorm:"not null;size:36;index"`
    CreatedAt int64  `gorm:"not null"`
}

type User struct {
    ID             string  `gorm:"primaryKey;size:36"`
    DisplayName    string  `gorm:"not null;size:100"`
    Role           string  `gorm:"not null;default:member;size:20"`
    AvatarURL      string  `gorm:"size:500"`
    APIKey         *string `gorm:"uniqueIndex;size:128"`
    CreatedAt      int64   `gorm:"not null"`
    Email          *string `gorm:"uniqueIndex:idx_users_email;size:255"`
    PasswordHash   string  `gorm:"size:255"`
    LastSeenAt     *int64
    RequireMention bool    `gorm:"not null;default:true"`
    OwnerID        *string `gorm:"size:36;index"`
    DeletedAt      *int64  `gorm:"index"`
    Disabled       bool    `gorm:"not null;default:false"`
}

type Message struct {
    ID          string  `gorm:"primaryKey;size:36"`
    ChannelID   string  `gorm:"not null;size:36;index:idx_messages_channel_time,priority:1"`
    SenderID    string  `gorm:"not null;size:36;index"`
    Content     string  `gorm:"not null"`
    ContentType string  `gorm:"not null;default:text;size:20"`
    ReplyToID   *string `gorm:"size:36;index"`
    CreatedAt   int64   `gorm:"not null;index:idx_messages_channel_time,priority:2,sort:desc"`
    EditedAt    *int64
    DeletedAt   *int64  `gorm:"index"`
}

type ChannelMember struct {
    ChannelID  string `gorm:"primaryKey;size:36"`
    UserID     string `gorm:"primaryKey;size:36;index"`
    JoinedAt   int64  `gorm:"not null"`
    LastReadAt *int64
}

type Mention struct {
    ID        string `gorm:"primaryKey;size:36"`
    MessageID string `gorm:"not null;size:36;index"`
    UserID    string `gorm:"not null;size:36;index:idx_mentions_user,priority:1"`
    ChannelID string `gorm:"not null;size:36;index:idx_mentions_user,priority:2"`
}

type Event struct {
    Cursor    int64  `gorm:"primaryKey;autoIncrement"`
    Kind      string `gorm:"not null;size:50;index"`
    ChannelID string `gorm:"not null;size:36;index"`
    Payload   string `gorm:"not null"`
    CreatedAt int64  `gorm:"not null;index"`
}

type UserPermission struct {
    ID         uint    `gorm:"primaryKey;autoIncrement"`
    UserID     string  `gorm:"not null;size:36;index;uniqueIndex:idx_user_permissions_unique,priority:1;index:idx_user_permissions_lookup,priority:1"`
    Permission string  `gorm:"not null;size:100;uniqueIndex:idx_user_permissions_unique,priority:2;index:idx_user_permissions_lookup,priority:2"`
    Scope      string  `gorm:"not null;default:*;size:255;uniqueIndex:idx_user_permissions_unique,priority:3;index:idx_user_permissions_lookup,priority:3"`
    GrantedBy  *string `gorm:"size:36"`
    GrantedAt  int64   `gorm:"not null"`
}

type InviteCode struct {
    Code      string  `gorm:"primaryKey;size:128"`
    CreatedBy string  `gorm:"not null;size:36;index"`
    CreatedAt int64   `gorm:"not null"`
    ExpiresAt *int64  `gorm:"index"`
    UsedBy    *string `gorm:"size:36;index"`
    UsedAt    *int64
    Note      string  `gorm:"size:500"`
}

type MessageReaction struct {
    ID        string `gorm:"primaryKey;size:36"`
    MessageID string `gorm:"not null;size:36;index;uniqueIndex:idx_reactions_unique,priority:1"`
    UserID    string `gorm:"not null;size:36;index;uniqueIndex:idx_reactions_unique,priority:2"`
    Emoji     string `gorm:"not null;size:64;uniqueIndex:idx_reactions_unique,priority:3"`
    CreatedAt int64  `gorm:"not null"`
}

type WorkspaceFile struct {
    ID              string    `gorm:"primaryKey;size:36"`
    UserID          string    `gorm:"not null;size:36;index;uniqueIndex:idx_workspace_files_unique,priority:1"`
    ChannelID       string    `gorm:"not null;size:36;index;uniqueIndex:idx_workspace_files_unique,priority:2"`
    ParentID        *string   `gorm:"size:36;index;uniqueIndex:idx_workspace_files_unique,priority:3"`
    Name            string    `gorm:"not null;size:255;uniqueIndex:idx_workspace_files_unique,priority:4"`
    IsDirectory     bool      `gorm:"not null;default:false"`
    MimeType        string    `gorm:"size:255"`
    SizeBytes       int64     `gorm:"not null;default:0"`
    Source          string    `gorm:"not null;default:upload;size:50"`
    SourceMessageID *string   `gorm:"size:36;index"`
    CreatedAt       time.Time `gorm:"autoCreateTime"`
    UpdatedAt       time.Time `gorm:"autoUpdateTime"`
}

type RemoteNode struct {
    ID              string     `gorm:"primaryKey;size:36"`
    UserID          string     `gorm:"not null;size:36;index"`
    MachineName     string     `gorm:"not null;size:255"`
    ConnectionToken string     `gorm:"not null;uniqueIndex;size:255"`
    LastSeenAt      *time.Time `gorm:"index"`
    CreatedAt       time.Time  `gorm:"autoCreateTime"`
}

type RemoteBinding struct {
    ID        string    `gorm:"primaryKey;size:36"`
    NodeID    string    `gorm:"not null;size:36;index;uniqueIndex:idx_remote_bindings_unique,priority:1"`
    ChannelID string    `gorm:"not null;size:36;index;uniqueIndex:idx_remote_bindings_unique,priority:2"`
    Path      string    `gorm:"not null;size:1000;uniqueIndex:idx_remote_bindings_unique,priority:3"`
    Label     string    `gorm:"size:255"`
    CreatedAt time.Time `gorm:"autoCreateTime"`
}
```

`store.Migrate` 调用顺序固定为依赖表优先：`User`、`ChannelGroup`、`Channel`、`Message`、`ChannelMember`、`Mention`、`Event`、`UserPermission`、`InviteCode`、`MessageReaction`、`WorkspaceFile`、`RemoteNode`、`RemoteBinding`。`AutoMigrate` 负责建表、补列和保持向前兼容；已有列不做破坏性变更、不 drop 表、不 drop 列。

缺列兼容仍必须覆盖老库：`users.email/password_hash/last_seen_at/require_mention/owner_id/deleted_at/disabled`、`channels.type/visibility/deleted_at/position/group_id`、`channel_members.last_read_at`、`messages.deleted_at`。迁移后执行 TS 等价 backfill：默认权限、creator channel permissions、agent owner、重复 DM 清理、DM 成员清理、position backfill。涉及部分索引、SQLite partial unique index 或历史列类型差异时，在 `migrations.go` 中用幂等 compatibility migration 包裹，不能散落到 store 查询路径。

store 查询使用 GORM 链式 API 和事务，例如 `Where(...).First(&user)`、`Find(&messages)`、`Create(&message)`、`Save(&channel)`、`Delete(&reaction)`、`db.Transaction(func(tx *gorm.DB) error { ... })`。禁止用手动 prepared statement 做 CRUD；只有 PRAGMA、不可由 GORM 表达的索引兼容和数据修复允许使用 `Exec`，且必须集中在迁移模块。

## 12. 静态文件 Serve

- `/uploads/*`：从 `UPLOAD_DIR` 直接 serve；启动时 `mkdir -p`。
- `/assets/*` 和其他 client 文件：从 `CLIENT_DIST` serve，默认开发路径为 `packages/client/dist`，Docker 内设置为 `/app/client/dist`。
- SPA fallback：非 `/api/*`、非 `/ws*`、无文件扩展名时返回 `index.html`。
- API/WS not found：返回 `404 {"error":"Not found"}`。
- upload 文件名必须使用随机 UUID + MIME 推断扩展名，禁止使用用户原始文件名写路径。
- workspace 文件数据路径：`WORKSPACE_DIR/{userId}/{channelId}/{fileId}.dat`。

## 13. 错误处理

统一 helper：

```go
func JSONError(w http.ResponseWriter, status int, msg string) {
    WriteJSON(w, status, map[string]string{"error": msg})
}
```

兼容规则：

- validation：`400 {"error":"..."}`。
- auth missing/invalid：`401`。
- permission denied：`403`，权限 middleware 保留 `{error, required_permission, scope}`。
- private channel unauthorized：返回 `404 Channel not found`，避免泄露存在性。
- conflict：`409`。
- upload too large：`413`。
- rate/emoji distinct limit：`429`。
- remote/plugin offline：`503`；timeout：`504`。
- delete message/channel 的幂等行为按 TS 保留：已删除 message 返回 `204`，已删除 channel prehandler 返回 `204`。

所有 handler 内部错误记录日志，对外默认 `500 {"error":"Internal server error"}`，除非 TS server 暴露了具体错误字符串。

## 14. 核心数据流走查

### 14.1 浏览器登录

1. `POST /api/v1/auth/login` decode email/password。
2. email lowercase trim，查 `users.email`。
3. bcrypt compare `password_hash`。
4. 检查 `deleted_at/disabled`。
5. 签 JWT `{userId,email}`，7 天过期。
6. 设置 `collab_token` HttpOnly cookie，返回 safe user。

### 14.2 REST 发送消息

1. Auth middleware 写入 current user。
2. `RequirePermission("message.send", "channel:{id}")`。
3. 校验频道存在；private channel 用 `canAccessChannel`；发送者必须是成员。
4. 校验 content 非空，`content_type in text|image`。
5. `store.CreateMessage` 使用 GORM transaction：`Create(&Message)`，解析 `<@id>` 和 `@displayName`，再 `Create` mentions、`Create` message event 和 mention events。
6. Hub 广播 `{type:"new_message",message}`。
7. 如果 image content 指向 `/uploads/*`，异步/非关键地复制到 workspace `attachments`。复制逻辑需要先确保 `attachments` 目录存在；目标文件名使用消息 id + 原上传扩展名，若冲突按 workspace `(1)(2)` 规则重命名；复制、建目录或 DB 记录失败只记录日志并静默吞掉，不能影响消息发送响应。
8. 返回 `201 {message}`。

### 14.3 WS 发送消息

1. `/ws` handshake 鉴权。
2. 收到 `send_message`，校验 channel、membership、permission。
3. `content_type` 支持 `text|image|command`；command 必须 mentions 非空且 content JSON 含 `command` 和 `params`。
4. 创建消息。
5. 对发送方回 `message_ack`；向订阅频道广播 `new_message`。如果请求带 `client_message_id`，广播时排除发送 socket，避免乐观 UI 重复。
6. 失败回 `message_nack`，不能关闭连接。

### 14.4 SSE 推送与补发

1. `GET /api/v1/stream` 认证。
2. 写响应头并 flush 后立即发送 `:connected\n\n` 注释帧。
3. `Last-Event-ID` 存在则从该 cursor 后补发，否则从当前 latest cursor 开始。
4. client 注册为 `ready=false`，先 backfill。
5. 查询 `getEventsSinceWithChanges(cursor, 100, userChannelIds, channelChangeKinds)`。
6. channel change events 做相关性判断并刷新 channelIds；普通事件跳过 sender 自己，但推进 cursor。
7. backfill 完成后 `ready=true` 并 drain-until-stable。
8. 新事件由 `signalNewEvents` 同时唤醒 poll waiters 和 SSE clients。
9. 15s heartbeat 写 `event: heartbeat`，并更新 `last_seen_at`。
10. 每 60s 重新查询一次当前用户可访问 channel list，刷新 SSE client 的 `channelIds`，覆盖权限、加入/退出频道和 public/private 变化未产生可见事件的边界。

### 14.5 Remote Explorer

1. 用户创建 node，DB 写 `remote_nodes`，返回 `connection_token`。
2. remote node 用 token 连接 `/ws/remote`，`RemoteNodeManager.Register(nodeID)`。
3. 浏览器请求 `/api/v1/remote/nodes/:id/ls?path=/x`。
4. handler 校验 node ownership 和 online 状态。
5. manager 发 `{type:"request",id,data:{action:"ls",path}}`，等待 `response`，10s timeout。
6. remote error 映射到 TS 状态码：`path_not_allowed=403`、`file_not_found=404`、`file_too_large=413`、timeout `504`、offline `503`。

## 15. 测试策略

测试分层：

- `store` unit：迁移、CRUD、事务、LexoRank、DM 去重、权限 backfill。
- `api` integration：`httptest.Server` + GORM sqlite in-memory DB + 临时 upload/workspace 目录。
- `ws` integration：`coder/websocket.Dial` 测 `/ws`、`/ws/plugin`、`/ws/remote`。
- compatibility golden：关键 TS response JSON 建 golden 文件，Go 输出字段不得少。
- coverage：`go test ./... -race -coverprofile=coverage.out`，再运行 `scripts/lib/coverage` 生成关键未覆盖报告。

测试隔离使用 GORM sqlite in-memory：`gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})`，每个测试 server 独立执行 `AutoMigrate` 和 seed/backfill，避免污染真实 `data/collab.db`。

Go 测试示例：

```go
func TestCreateMessageBroadcastsAndPersistsEvent(t *testing.T) {
    srv := testutil.NewServer(t)
    token := srv.LoginAs(t, "admin-jianjun")
    ws := srv.DialWS(t, "/ws", token)
    defer ws.Close(websocket.StatusNormalClosure, "")

    ch := srv.CreateChannel(t, token, map[string]any{"name": "go-test"})
    srv.WSWriteJSON(t, ws, map[string]any{"type": "subscribe", "channel_id": ch.ID})
    srv.WSReadType(t, ws, "subscribed")

    res := srv.JSON(t, "POST", "/api/v1/channels/"+ch.ID+"/messages", token, map[string]any{
        "content": "hello <@agent-pegasus>",
        "mentions": []string{"agent-pegasus"},
    })
    require.Equal(t, http.StatusCreated, res.Code)

    var body struct{ Message model.Message `json:"message"` }
    require.NoError(t, json.Unmarshal(res.Body.Bytes(), &body))
    assert.Equal(t, []string{"agent-pegasus"}, body.Message.Mentions)

    got := srv.WSReadType(t, ws, "new_message")
    assert.Equal(t, body.Message.ID, got["message"].(map[string]any)["id"])

    events := srv.Store.EventsSince(t, 0, 10, []string{ch.ID})
    assert.Contains(t, eventKinds(events), "message")
    assert.Contains(t, eventKinds(events), "mention")
}
```

```go
func TestPluginWSAPIRequestUsesBearerAuth(t *testing.T) {
    srv := testutil.NewServer(t)
    apiKey := srv.CreateAgent(t, "agent-ci").APIKey

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    c, _, err := websocket.Dial(ctx, srv.WSURL("/ws/plugin"), &websocket.DialOptions{
        HTTPHeader: http.Header{"Authorization": []string{"Bearer " + apiKey}},
    })
    require.NoError(t, err)
    defer c.Close(websocket.StatusNormalClosure, "")

    writeJSON(t, c, map[string]any{
        "type": "api_request",
        "id": "req-1",
        "data": map[string]any{"method": "GET", "path": "/api/v1/channels"},
    })

    msg := readJSON(t, c)
    require.Equal(t, "api_response", msg["type"])
    data := msg["data"].(map[string]any)
    assert.EqualValues(t, 200, data["status"])
}
```

```go
func TestSSEReplaysFromLastEventID(t *testing.T) {
    srv := testutil.NewServer(t)
    token := srv.LoginAs(t, "admin-jianjun")
    ch := srv.CreateChannel(t, token, map[string]any{"name": "sse"})

    first := srv.PostMessage(t, token, ch.ID, "first")
    cursor := srv.Store.CursorForMessage(t, first.ID)
    second := srv.PostMessage(t, token, ch.ID, "second")

    req, _ := http.NewRequest("GET", srv.URL("/api/v1/stream"), nil)
    req.Header.Set("Cookie", token.CookieHeader())
    req.Header.Set("Last-Event-ID", strconv.FormatInt(cursor, 10))
    resp, err := srv.Client().Do(req)
    require.NoError(t, err)
    defer resp.Body.Close()

    ev := readSSE(t, resp.Body, "message")
    assert.Contains(t, ev.Data, second.ID)
}
```

## 16. Docker 多阶段构建

```dockerfile
# syntax=docker/dockerfile:1.7

FROM node:22-bookworm AS client-builder
WORKDIR /src
COPY package.json package-lock.json ./
COPY packages/client/package.json packages/client/package.json
RUN npm ci
COPY packages/client packages/client
RUN npm run --workspace packages/client build

FROM golang:1.22-bookworm AS go-builder
WORKDIR /src/packages/server-go
RUN apt-get update && apt-get install -y --no-install-recommends gcc libc6-dev sqlite3 libsqlite3-dev \
  && rm -rf /var/lib/apt/lists/*
COPY packages/server-go/go.mod packages/server-go/go.sum ./
RUN go mod download
COPY packages/server-go ./
RUN CGO_ENABLED=1 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/collab ./cmd/collab

FROM debian:bookworm-slim AS runtime
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates sqlite3 libsqlite3-0 \
  && rm -rf /var/lib/apt/lists/* \
  && useradd -r -u 10001 -g root collab
WORKDIR /app
COPY --from=go-builder /out/collab /app/collab
COPY --from=client-builder /src/packages/client/dist /app/client/dist
RUN mkdir -p /app/data/uploads /app/data/workspaces && chown -R 10001:0 /app/data
USER 10001
ENV HOST=0.0.0.0 PORT=4900 DATABASE_PATH=/app/data/collab.db UPLOAD_DIR=/app/data/uploads WORKSPACE_DIR=/app/data/workspaces CLIENT_DIST=/app/client/dist
EXPOSE 4900
ENTRYPOINT ["/app/collab"]
```

`gorm.io/driver/sqlite` 依赖 CGO SQLite driver，因此 builder 和 runtime 都必须包含 SQLite C 运行依赖。若后续要求 scratch/distroless，需要静态链接验证和额外 CA/zoneinfo 处理。

## 17. Task Breakdown

### Phase 1 — Skeleton + Store + Auth（3 人日）

- 搭建 `packages/server-go`、`go.mod`、Makefile、Dockerfile 初版。
- 实现 config、logger、JSON helpers、middleware chain、graceful shutdown。
- 实现 GORM sqlite open、`AutoMigrate`、seed，包含 schema 兼容和 backfill。
- 实现 auth：JWT cookie、API key、dev bypass、permission middleware。
- 覆盖测试：migration、seed、login/register/logout、permission check。

### Phase 2a — REST 核心 CRUD（3 人日）

- 实现 auth/users/channels/messages/DM 核心 CRUD。
- 实现 login/register/logout、users/me、channels list/detail/create/update/delete、join/leave、members、read state。
- 实现 DM 创建/list、message create/edit/delete/list/search。
- 实现 LexoRank 基础 channel reorder、DM channel 规则、message mention parsing。
- 覆盖测试：auth、users、channels、DM、messages 和权限过滤。

### Phase 2b — REST 扩展功能对等（3 人日）

- 实现 admin/agents/workspace/remote/upload/reactions/commands/channel-groups。
- 实现 admin 用户与权限管理、agent CRUD/API key/permissions。
- 实现 workspace 文件、upload、remote REST proxy、reactions aggregation、slash commands、channel groups。
- 实现 channel groups reorder、workspace move/rename、remote binding、command list/register 边界。
- 实现静态文件 serve 和 SPA fallback。
- 覆盖测试：TS 现有 route 测试场景迁移到 Go，重点覆盖 admin 业务错误、workspace 文件名冲突、upload 限制和 remote 错误映射。

### Phase 3 — Realtime：WS + SSE + Poll（4 人日）

- 实现 `/ws` client hub、presence、channel subscriptions、message ack/nack、slash command registration。
- 实现 `/ws/plugin` manager、event bridge。
- 实现 `/ws/remote` manager、pending request、timeout/offline 映射。
- 实现 `/api/v1/poll` waiters 和 `/api/v1/stream` replay/heartbeat。
- 覆盖测试：多设备、permission WS、plugin comm、remote explorer、SSE replay、token rotation。

### Phase 3b — Plugin WS internal API loopback（1 人日）

- 实现 Plugin `api_request` 到 root handler 的 loopback，补齐 Bearer auth、JSON body、headers/body/status 透传。
- 覆盖测试：plugin 通过 loopback 调用 channels/messages/workspace/admin denied 场景，确认与真实 HTTP handler 行为一致。

### Phase 4 — Compatibility Hardening + Release（3 人日）

- 将 TS 集成测试关键断言整理为 Go golden/compat test。
- 增加 `-race`、coverage helper、CI target，覆盖率门槛 ≥85%。
- 压测 SQLite WAL busy_timeout、WS 心跳、SSE 大量 backfill。
- 完成 Docker image、部署文档、切换步骤文档、回滚步骤。
- 用现有数据库副本做 dry-run，确认 frontend 零改动验收。

总估算：17 人日，不含线上灰度观察和 bug buffer。

## 18. 风险与开放问题

| 风险 / 问题 | 影响 | 处理建议 |
|---|---|---|
| `net/http` ServeMux path variable 与 method routing 细节 | 路由冲突可能导致状态码差异 | 建立 route table 测试，逐条校验 method/path/status |
| SQLite CGO 部署 | 镜像/runtime 依赖复杂于纯 Go | 保持 debian slim runtime，CI 构建 linux/amd64 实测 |
| 事件过滤差异 | SSE/Poll/Plugin 漏消息或泄露 private channel | 以 TS `getEventsSinceWithChanges` 语义做集成测试 |
| WS 并发写 | coder/websocket 不允许多 goroutine 无序写 | 每个 connection 建 write pump/channel，所有发送串行化 |
| Go internal API loopback 替代 Fastify inject | Plugin `api_request` 可能和真实 HTTP 行为不一致 | loopback 调用同一个 root handler，保留 headers/body/status |
| multipart parsing | 大文件必须早停且返回 413 | 使用 `http.MaxBytesReader` + streaming copy |
| workspace DB 与文件数据一致性 | 写 DB 成功但文件失败或反向 | 先写临时文件再 rename，DB 事务失败则清理文件 |
| schema migration 顺序 | 新老库缺列或 FK 依赖导致启动失败 | `AutoMigrate` 调用顺序固定，兼容补丁集中在迁移模块，迁移幂等测试 |
| Admin 权限语义历史演进 | 部分旧设计说 admin 不参与业务权限，新代码已允许 admin bypass | Go 以当前 TS server 为准：admin 在 permission middleware 和部分 channel access 中 bypass |
| Cookie secure 判断 | 本地/生产 cookie 行为差异影响登录 | 复制 TS：`HOST` 非 localhost/127.0.0.1 且非 development 时加 `Secure` |
| Go server 与 TS server 并行写同一 SQLite | 切换期双写会有 WAL/事件顺序风险 | 灰度时单 writer；回滚前停止 Go 进程，直接启动 TS server |

开放问题：

1. Go server 是否需要保留 TS 的 legacy seed agents（飞马/野马/战马/烈马）及随机 API key 日志输出？建议保留以兼容旧 dev 数据。
2. Docker build 是否由 monorepo root 执行？上述 Dockerfile 假设 root context。
3. 是否需要在 Phase 4 加入真实前端 Playwright smoke test，验证“前端零改动”？建议加入登录、频道、消息、upload、workspace、remote 状态页 smoke。
