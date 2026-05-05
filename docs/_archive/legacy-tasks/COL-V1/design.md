# Collab v1 — 技术设计文档

日期：2026-04-18 | 状态：Draft | 作者：飞马（架构师）

---

## 背景与问题

Discord 在中国大陆连接极不稳定，团队（1 人类 + 4 AI agent）日常沟通受严重影响。需要一个自部署、中国可访问的轻量团队聊天平台，并让 OpenClaw agent 原生接入。

详见 [PRD](../PRD.md)。

## 目标

完成后可验证的验收标准：

1. 建军从中国浏览器打开 Collab，页面 < 5s 加载完成
2. 在频道中发送消息，所有在线成员 < 1s 收到
3. AI agent（飞马/野马/战马/烈马）通过 OpenClaw plugin 在 Collab 频道正常收发消息
4. 手机浏览器可正常使用所有功能
5. 刷新页面后消息历史完整保留

## 方案设计

### 整体架构

```
┌─────────────────────────────────────────────────────────┐
│                    Cloudflare                            │
│  ┌──────────────┐  ┌──────────────┐                     │
│  │ CF Access    │  │ CF Tunnel    │                     │
│  │ (认证)       │  │ (暴露服务)    │                     │
│  └──────┬───────┘  └──────┬───────┘                     │
└─────────┼─────────────────┼─────────────────────────────┘
          │                 │
          ▼                 ▼
┌─────────────────────────────────────────────────────────┐
│                  oc-apps (宿主机)                         │
│                                                          │
│  ┌──────────────────────────────────────────────────┐   │
│  │              Collab Server (Node.js)               │   │
│  │                                                    │   │
│  │  ┌────────────┐  ┌─────────────┐  ┌───────────┐  │   │
│  │  │ HTTP/REST  │  │  WebSocket  │  │  Static   │  │   │
│  │  │ API        │  │  Server     │  │  Files    │  │   │
│  │  │ (Fastify)  │  │  (ws)       │  │  (React)  │  │   │
│  │  └─────┬──────┘  └──────┬──────┘  └───────────┘  │   │
│  │        │                │                          │   │
│  │        ▼                ▼                          │   │
│  │  ┌──────────────────────────────────┐             │   │
│  │  │         SQLite (data.db)          │             │   │
│  │  │  channels | messages | users      │             │   │
│  │  └──────────────────────────────────┘             │   │
│  └──────────────────────────────────────────────────┘   │
│                                                          │
│  ┌─────────────────────────────┐                        │
│  │  OpenClaw (各 agent 容器)     │                        │
│  │  ┌───────────────────────┐  │                        │
│  │  │  Collab Channel Plugin │  │                        │
│  │  │  (HTTP poll → REST)    │  │                        │
│  │  └───────────────────────┘  │                        │
│  └─────────────────────────────┘                        │
└─────────────────────────────────────────────────────────┘

Browser (建军手机/电脑)
  │
  ├── HTTPS → 页面加载 (Static Files)
  ├── HTTPS → REST API (发消息/拉历史/管频道)
  └── WSS   → WebSocket (实时消息推送)
```

**两个组件**：
1. **Collab Server** — 单进程 Node.js 应用，同时 serve 前端静态文件、REST API 和 WebSocket
2. **Collab Channel Plugin** — OpenClaw 插件（独立 npm 包），参照 `qa-channel` 模式，通过 HTTP API 连接 Collab Server

### 数据模型

```sql
-- 频道
CREATE TABLE channels (
  id          TEXT PRIMARY KEY,          -- UUID
  name        TEXT NOT NULL UNIQUE,      -- #general, #project-collab
  topic       TEXT DEFAULT '',           -- 频道描述
  created_at  INTEGER NOT NULL,          -- Unix timestamp (ms)
  created_by  TEXT NOT NULL              -- user id
);

-- 用户（人类 + agent）
CREATE TABLE users (
  id           TEXT PRIMARY KEY,         -- UUID
  display_name TEXT NOT NULL,            -- "建军", "飞马", "战马"
  role         TEXT DEFAULT 'member',    -- "admin" | "member" | "agent"
  avatar_url   TEXT,
  api_key      TEXT UNIQUE,              -- agent 认证用，人类为 NULL
  created_at   INTEGER NOT NULL
);

-- 消息
CREATE TABLE messages (
  id          TEXT PRIMARY KEY,          -- UUID
  channel_id  TEXT NOT NULL REFERENCES channels(id),
  sender_id   TEXT NOT NULL REFERENCES users(id),
  content     TEXT NOT NULL,             -- Markdown 文本或图片 URL
  content_type TEXT DEFAULT 'text',      -- "text" | "image"
  reply_to_id TEXT REFERENCES messages(id),
  created_at  INTEGER NOT NULL,          -- Unix timestamp (ms)
  edited_at   INTEGER
);

CREATE INDEX idx_messages_channel_time ON messages(channel_id, created_at DESC);
CREATE INDEX idx_messages_sender ON messages(sender_id);

-- 频道成员（记录谁在哪个频道）
CREATE TABLE channel_members (
  channel_id  TEXT NOT NULL REFERENCES channels(id),
  user_id     TEXT NOT NULL REFERENCES users(id),
  joined_at   INTEGER NOT NULL,
  PRIMARY KEY (channel_id, user_id)
);

-- Mention 记录（用于高亮和未来通知）
CREATE TABLE mentions (
  id          TEXT PRIMARY KEY,
  message_id  TEXT NOT NULL REFERENCES messages(id),
  user_id     TEXT NOT NULL REFERENCES users(id),
  channel_id  TEXT NOT NULL REFERENCES channels(id)
);

CREATE INDEX idx_mentions_user ON mentions(user_id, channel_id);

-- 事件游标（用于 plugin 长轮询）
CREATE TABLE events (
  cursor      INTEGER PRIMARY KEY AUTOINCREMENT,
  kind        TEXT NOT NULL,             -- "message" | "message_edited" | "message_deleted"
  channel_id  TEXT NOT NULL,
  payload     TEXT NOT NULL,             -- JSON
  created_at  INTEGER NOT NULL
);
```

### API 设计

#### REST API

**认证**：
- 浏览器用户：Cloudflare Access JWT（从 `Cf-Access-Jwt-Assertion` header 提取）
- Agent：`Authorization: Bearer <api_key>`

##### 频道

```
GET /api/v1/channels
Response: { channels: [{ id, name, topic, created_at, member_count }] }

POST /api/v1/channels
Body: { name: string, topic?: string }
Response: { channel: { id, name, topic, created_at } }
```

##### 消息

```
GET /api/v1/channels/:channelId/messages?before=<cursor>&limit=50
Response: { messages: [{ id, channel_id, sender_id, sender_name, content, content_type, reply_to_id, created_at, mentions: [user_id] }], has_more: boolean }

POST /api/v1/channels/:channelId/messages
Body: { content: string, content_type?: "text"|"image", reply_to_id?: string, mentions?: string[] }
Response: { message: { id, ... } }
```

##### 图片上传

```
POST /api/v1/upload
Content-Type: multipart/form-data
Body: file (image/*)
Response: { url: "/uploads/{uuid}.{ext}", content_type: "image/png" }
```

约束：单文件 ≤10MB，只允许 image/* MIME type。存储在本地磁盘 `/opt/collab/data/uploads/`，通过 Collab Server 静态 serve。前端支持粘贴（Ctrl+V）和拖拽上传。

##### 用户

```
GET /api/v1/users
Response: { users: [{ id, display_name, role, avatar_url }] }

GET /api/v1/users/me
Response: { user: { id, display_name, role } }
```

##### Plugin 长轮询

```
POST /api/v1/poll
Body: { api_key: string, cursor: number, timeout_ms?: number }
Response: { cursor: number, events: [{ cursor, kind, channel_id, payload }] }
```

轮询机制：服务端收到请求后，如果没有新事件，hold 住连接最多 `timeout_ms`（默认 30000ms），有新事件立即返回。超时无事件时返回 `200` + 空 events 数组 + 当前 cursor（不返回 4xx）。

#### WebSocket 协议

连接：`wss://collab.codetrek.work/ws?token=<cf_access_jwt>`

##### 客户端 → 服务端

```json
// 订阅频道
{ "type": "subscribe", "channel_id": "xxx" }

// 取消订阅
{ "type": "unsubscribe", "channel_id": "xxx" }

// 发送消息（也可以走 REST）
{ "type": "send_message", "channel_id": "xxx", "content": "hello", "content_type": "text", "mentions": ["user_id_1"] }

// 心跳
{ "type": "ping" }
```

##### 服务端 → 客户端

```json
// 新消息
{ "type": "new_message", "message": { id, channel_id, sender_id, sender_name, content, content_type, created_at, mentions } }

// 心跳响应
{ "type": "pong" }

// 错误
{ "type": "error", "message": "reason" }
```

### 前端架构

**技术栈**：React 18 + Vite + TypeScript

**关键约束**：
- 所有依赖必须打包，不使用外部 CDN
- 不引入 Google Fonts，使用系统字体栈
- Markdown 渲染用 `marked` + `highlight.js`（和 RFR 一致）+ `DOMPurify`（XSS 防护）

```
src/
├── App.tsx                    # 主布局
├── components/
│   ├── Sidebar.tsx            # 频道列表侧边栏
│   ├── ChannelView.tsx        # 消息列表 + 输入框
│   ├── MessageList.tsx        # 消息列表（虚拟滚动）
│   ├── MessageItem.tsx        # 单条消息渲染
│   ├── MessageInput.tsx       # 消息输入框 + mention 选择器
│   ├── MentionPicker.tsx      # @mention 下拉选择
│   ├── ConnectionStatus.tsx   # 连接状态 banner
│   └── MobileLayout.tsx       # 移动端适配布局
├── hooks/
│   ├── useWebSocket.ts        # WebSocket 连接管理 + 自动重连
│   ├── useMessages.ts         # 消息列表状态
│   ├── useChannels.ts         # 频道列表状态
│   └── useAuth.ts             # 认证状态
├── lib/
│   ├── api.ts                 # REST API 客户端
│   ├── ws.ts                  # WebSocket 客户端（重连逻辑）
│   └── markdown.ts            # Markdown 渲染
└── types.ts                   # 共享类型定义
```

**UI 细节**：
- @mention 高亮：`@用户名` 渲染为蓝色背景 + 白色文字（类 Discord 风格）
- 加载状态：消息列表用 skeleton loading，频道切换用 spinner
- 图片上传：支持粘贴（Ctrl+V）和拖拽，上传中显示进度条

**状态管理**：React Context + useReducer，不上 Redux（项目规模不需要）。

**重连策略**：
1. WebSocket 断开后立即尝试重连
2. 使用指数退避：1s → 2s → 4s → 8s → 16s → 30s（封顶）
3. 重连成功后，用最后一条消息的 `created_at` 调 REST API 拉取错过的消息
4. 顶部显示连接状态 banner：🟢 在线 / 🔴 断连 / 🟡 重连中

### OpenClaw Channel Plugin

基于 `qa-channel` 模式，完全独立的 npm 包。

**文件结构**：

```
extensions/collab/
├── openclaw.plugin.json       # 插件描述
├── package.json
├── src/
│   ├── channel.ts             # createChatChannelPlugin() 注册
│   ├── accounts.ts            # 账户解析
│   ├── config-schema.ts       # 配置 schema
│   ├── gateway.ts             # 长轮询消息接收
│   ├── inbound.ts             # 消息 → agent session 派发
│   ├── outbound.ts            # agent 回复 → Collab server
│   ├── api-client.ts          # Collab REST API 客户端
│   ├── setup.ts               # 配置向导
│   ├── status.ts              # 状态检查
│   ├── runtime.ts             # 运行时注入
│   ├── runtime-api.ts         # 类型导出
│   └── types.ts               # 类型定义
└── tsconfig.json
```

**消息流**：

```
[人类在浏览器发消息]
  → WebSocket → Collab Server → 写入 SQLite + 插入 events 表
  → Plugin gateway 长轮询 /api/v1/poll 获取新 event
  → inbound.ts → dispatchInboundReplyWithBase() → agent session
  → agent 回复 → outbound.ts → POST /api/v1/channels/:id/messages
  → Collab Server → 写入 SQLite → WebSocket 广播给浏览器
```

**Plugin 配置** (openclaw.yaml):

```yaml
channels:
  collab:
    accounts:
      default:
        baseUrl: "http://localhost:4900"
        apiKey: "col_xxxxx"
        botUserId: "agent-pegasus"
        botDisplayName: "飞马"
```

**Target 格式**：
- 频道消息：`channel:<channel_id>`
- DM（v2）：`dm:<user_id>`

**Cursor 持久化**：Plugin 将最新 cursor 写入本地文件（`~/.openclaw/collab-cursor.json`），进程重启后从上次 cursor 恢复。即使 cursor 丢失，OpenClaw 的 `dispatchInboundReplyWithBase` 通过 MessageSid 去重，不会重复处理。

### 部署架构

和 RFR 一致，用 docker-compose 部署：

1. **Collab Server** Docker 容器，端口 4900
2. **Caddy** 反代 localhost:4900 → collab.codetrek.work
3. **Cloudflare Tunnel** 暴露服务
4. **Cloudflare Access** 保护访问（OTP 认证）
5. **域名**：`collab.codetrek.work`

```yaml
# docker-compose.yml
version: "3.8"
services:
  collab:
    build: .
    restart: always
    ports:
      - "4900:4900"
    volumes:
      - ./data:/app/data    # SQLite DB + uploads 持久化
    environment:
      - PORT=4900
      - DATABASE_PATH=/app/data/collab.db
      - UPLOAD_DIR=/app/data/uploads
```

```dockerfile
# Dockerfile
FROM node:22-slim
WORKDIR /app
COPY package*.json ./
RUN npm ci --production
COPY dist/ ./dist/
COPY public/ ./public/
EXPOSE 4900
CMD ["node", "dist/server.js"]
```

### 错误处理与重连

| 场景 | 处理 |
|------|------|
| WebSocket 断连 | 指数退避重连，重连后拉取缺失消息 |
| REST API 超时 | 3 次重试，间隔 1s/2s/4s |
| Plugin 轮询失败 | 指数退避，从上次 cursor 恢复 |
| SQLite 写入失败 | 返回 500，客户端提示重试 |
| Cloudflare Access token 过期 | 前端检测 401，引导重新登录 |

## 备选方案

### 为什么不用 Cloudflare Workers + Durable Objects

- **优点**：边缘部署、自动扩缩、天然支持 WebSocket（via Durable Objects）
- **不选的原因**：
  - Durable Objects 的 WebSocket API 和标准不同，学习曲线高
  - SQLite on Workers（D1）有限制（单 DB 最大 10GB、无触发器）
  - 调试困难（本地 miniflare vs 远程行为不一致）
  - 我们的流量极小（5 个用户），VPS 方案成本更低、调试更简单
  - **将来可以迁移**——前后端分离，后端 API 不变，只需要换部署层

### 为什么不用 PostgreSQL

- SQLite 够用：单机部署、5 个用户、消息量可预见
- 零运维：不需要额外的数据库进程
- 性能足够：better-sqlite3 在写密集场景下也能 10k+ ops/s
- **迁移路径清晰**：如果将来需要，API 层不变，只换 ORM/查询层

### 为什么不用 Socket.IO

- `ws` 更轻量，没有不需要的 fallback（我们只需要 WebSocket，不需要 polling fallback）
- 依赖更少
- Plugin 端只需要 HTTP polling，不需要 Socket.IO 客户端

## 测试策略

**覆盖率目标**：新代码 ≥85%，关键路径（DB 操作、消息广播、认证）100%。

**图片消息约束**：v1 只支持 URL 引用（不做上传）。图片加载失败显示 broken image placeholder + 原始 URL 文本 fallback。

**分页实现**：`has_more` 用 `fetch limit+1` 策略判断，避免 off-by-one。单测覆盖恰好整数倍和边界场景。

### 单元测试

| 模块 | 测试重点 |
|------|----------|
| API 路由 | 参数校验、权限检查、错误响应 |
| WebSocket 消息处理 | 消息格式解析、广播逻辑 |
| 数据库操作 | CRUD、分页、索引命中 |
| Markdown 渲染 | XSS 过滤（DOMPurify）、`<script>` 标签转义为纯文本、格式正确性 |
| Plugin API client | 请求构造、错误处理 |

### 集成测试

| 场景 | 验证点 |
|------|--------|
| 发送消息端到端 | REST 发消息 → DB 持久化 → WebSocket 广播 → 其他客户端收到 |
| 长轮询 | 无消息时 hold → 有消息时立即返回 → cursor 正确递增 |
| 断连重连 | WebSocket 断开 → 重连 → 用 REST 补齐缺失消息 |
| Plugin 消息流 | 浏览器发消息 → Plugin 收到 → agent 回复 → 浏览器显示 |

### E2E 验收

由 QA（烈马）执行：
1. 浏览器打开 Collab，看到频道列表
2. 进入频道，发送文字消息、Markdown 消息、图片 URL
3. 另一个浏览器 tab 实时收到消息
4. 手机浏览器打开同一页面，功能一致
5. 断网 → 显示断连状态 → 恢复网络 → 自动重连 → 补齐消息
6. Agent 通过 Plugin 发消息，浏览器实时显示
7. 无/错 API key 访问 → 401 拒绝；CF Access JWT 过期 → 跳转重认证（不白屏）
8. 发送 `<script>alert(1)</script>` 消息 → 转义为纯文本显示，不执行
9. 图片 URL 消息内联显示；图片加载失败 → 显示 broken image placeholder + 原始 URL
10. 建军从中国网络验证可访问性（由建军手动确认，非自动化，QA 报告中注明）

## Task Breakdown

按依赖顺序排列。总估时：~40-50h。

| ID | 任务 | 依赖 | 估时 | 说明 |
|----|------|------|------|------|
| COL-T01 | 项目脚手架 | — | 2h | Node.js + Fastify + Vite + React 初始化，monorepo 结构（server + client + plugin） |
| COL-T02 | 数据库 schema + 基础 CRUD | T01 | 3h | SQLite 初始化、迁移、channels/messages/users 表操作封装 |
| COL-T02b | 初始 seed 脚本 | T02 | 1h | 创建 #general 频道、管理员用户、agent 用户（含 API key） |
| COL-T02c | 骨架部署验证 | T01 | 2h | 先部署最小服务到 oc-apps，CF Tunnel + Access，让建军从中国测 HTTPS + WebSocket 连通性 |
| COL-T03 | REST API — 频道 | T02 | 2h | GET/POST /channels，认证中间件 |
| COL-T04 | REST API — 消息 | T02 | 3h | GET/POST /messages，分页查询，mention 解析 |
| COL-T04b | 图片上传 API + 前端 | T02 | 3h | POST /upload，本地存储，粘贴/拖拽上传 UI |
| COL-T05 | REST API — 用户 + 认证 | T02 | 2h | CF Access JWT 解析、API key 认证、/users/me |
| COL-T06 | WebSocket 服务 | T02 | 4h | 连接管理、subscribe/unsubscribe、消息广播、心跳 |
| COL-T07 | 长轮询 API | T02 | 3h | events 表、cursor 机制、hold-until-new-event |
| COL-T08 | 前端 — 频道侧边栏 | T03 | 3h | 频道列表、切换、创建频道 UI |
| COL-T09 | 前端 — 消息列表 | T04, T06 | 3h | 消息渲染（Markdown + 图片）、overflow-y:auto 滚动、分页加载历史（不做虚拟滚动） |
| COL-T10 | 前端 — 消息输入 + @mention | T04, T05 | 4h | 输入框、@mention picker、发送逻辑 |
| COL-T11 | 前端 — WebSocket 集成 | T06 | 3h | 连接管理、重连、状态 banner、消息实时更新 |
| COL-T12 | 前端 — 响应式布局 | T08, T09 | 3h | 移动端适配、频道列表折叠、触摸优化 |
| COL-T13 | OpenClaw Plugin 骨架 | T07 | 3h | plugin.json、channel.ts、accounts.ts、config-schema.ts |
| COL-T14 | Plugin — Gateway + Inbound | T07, T13 | 4h | 长轮询、消息派发到 agent session |
| COL-T15 | Plugin — Outbound | T04, T13 | 2h | agent 回复发送到 Collab |
| COL-T16 | 部署 | T01-T15 | 4h | systemd service、Caddy 配置、Cloudflare Tunnel + Access |
| COL-T17 | E2E 测试 + 修复 | T16 | 4h | 全链路验证 + bug 修复 |

**关键路径**：T01 → T02 → **T02c（骨架部署验证，确认中国可访问）** → T04/T06 → T09/T11 → T16 → T17

**v1 前端不做的 UI**（数据库字段保留，前端不实现）：
- 消息引用（reply_to_id）
- 消息编辑/删除
- 在线状态（presence）

## 风险与开放问题

| 风险 | 影响 | 缓解 |
|------|------|------|
| Cloudflare 在中国偶尔不稳定 | 建军体验下降 | 监控连通性，不行就迁国内机器 |
| OpenClaw plugin 安装方式 | 各 agent 容器需要安装 plugin | 先在飞马容器测通，再推全团队 |
| SQLite 并发写 | 多 agent 同时写消息可能锁冲突 | better-sqlite3 用 WAL 模式，5 个用户完全不是问题 |
| CF Access 在中国 | OTP 邮件可能延迟 | 用建军常用邮箱，或考虑 token 直接认证 |

**开放问题**：
1. Plugin 安装到各 agent 容器的具体步骤——需要测试后文档化
2. 初始频道列表——从 Discord 迁移哪些频道过来，建军定

## 参考资料

- [Collab PRD](../PRD.md)
- [OpenClaw qa-channel plugin 源码](/workspace/openclaw/extensions/qa-channel/src/)
- [OpenClaw channel plugin SDK](/workspace/openclaw/src/plugin-sdk/channel-core.ts)
- [Ready for Review 部署参考](/workspace/ready-for-review/)
- [Fastify 文档](https://fastify.dev)
- [ws 库](https://github.com/websockets/ws)
- [better-sqlite3](https://github.com/WiseLibs/better-sqlite3)
