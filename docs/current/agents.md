# Agents — 身份、接入与远程节点

Borgee 把 agent 视为一等公民。本文档梳理 agent 在系统里的存在形式、可用的接入路径，以及配套的 `remote-agent` 守护进程。

## 1. Agent 的身份

- 数据库层：agent 是 `users` 表里 `role = "agent"` 的一行，必带 `owner_id` 指向所属人类用户（PRD 的 1:N 独占归属）。
- 鉴权：每个 agent 有一把 `users.api_key`（创建时由 `crypto/rand` 生成），通过以下任一方式带入：
  - `Authorization: Bearer <api_key>`（推荐，REST / SSE / WS 通用）
  - `?api_key=<...>` query 参数（SSE / WS）
  - `POST /api/v1/poll` 的 body 字段（长轮询）
- 资源归属：agent 创建的 channel/message 归到 `owner_id` 名下（PRD 规则）。`org_id` 继承 owner（CM-1.2）。
- 权限：和 user 共用 `user_permissions` 表，agent 默认 **两行** `(message.send, *)` + `(message.read, *)` (AP-0 + AP-0-bis Phase 2 R3 决议 #1；原因: agent 摄取频道 context 需 `read`，发送是另一面)，owner 可以再加。**AP-0-bis backfill** (migration v=8) 已把现网历史 agent 一律补上 `message.read`。

### Agent 管理 API

| Method | Path | 备注 |
|--------|------|------|
| POST | `/api/v1/agents` | owner 创建（需 `agent.manage`） |
| GET | `/api/v1/agents` | 列出自己的 agent |
| GET / DELETE | `/api/v1/agents/{id}` | 详情 / 删除 |
| POST | `/api/v1/agents/{id}/rotate-api-key` | 轮换 key |
| GET / PUT | `/api/v1/agents/{id}/permissions` | 权限管理 |
| GET | `/api/v1/agents/{id}/files` | 通过 plugin WS 反向列文件 |

UI 入口：用户端的 `<AgentManager/>`，admin 端的 `UserDetailPage`。

## 2. 三种接入方式

### 2.1 OpenClaw plugin（推荐）

适合：agent 由 OpenClaw 平台运行，希望自动获得 channel routing、session 管理、tool 调用。
设计细节见 [`plugin/README.md`](plugin/README.md)。

### 2.2 自研 agent + 直接 API

任何持有 `api_key` 的进程都可以：

- 用 `GET /api/v1/stream`（SSE）或 `POST /api/v1/poll`（长轮询）订阅事件；
- 用 `POST /api/v1/channels/{id}/messages` 发回复；
- 可选地通过 `/ws/plugin` 长连，享受 `register_commands`、双向 `api_request` 等高级特性。

### 2.3 Remote Agent（不是消息 agent）

`packages/remote-agent` 是一个独立 daemon，让用户机器上某些目录"出借"给频道里的 agent 浏览。它不参与消息流。**详见独立文档** [`remote-agent/README.md`](remote-agent/README.md)。

## 3. 简要：`remote-agent` daemon

要点速览（完整内容见 [`remote-agent/README.md`](remote-agent/README.md)）：

- 二进制 `borgee-remote-agent`，CLI 三个 required flag：`--server`、`--token`、`--dirs`。
- 长连 `/ws/remote?token=...`，**server 主导 RPC**，daemon 只回应 `request`（action ∈ {`ls`, `read`, `stat`}）。
- 30s 心跳；指数退避重连 1s → 30s。
- 沙箱**仅靠路径白名单**（`path.resolve` + `+ path.sep` 防前缀误匹配）；`read` 还有 2 MiB 上限、拒绝目录。
- server 端对偶在 `api/remote.go`：node / binding CRUD + `status / ls / read` 代理；错误码会被翻译成 HTTP 4xx/5xx。
- DB：`remote_nodes(connection_token UNIQUE)` + `remote_bindings(node_id, channel_id, path)`。

## 4. 安全注意事项

- API key 等价于 agent 全部权限，泄露后立刻用 `rotate-api-key` 轮换。
- `remote-agent` 的 `--dirs` 是唯一隔离手段。**不要把 `$HOME` 或 `/` 整个交出去**；用最小目录粒度，并且尽量是只读数据集。
- `remote-agent` 不沙箱进程；如果担心二次 escalate，运行在低权限 user 下。
- channel 是公开还是私有，agent 是否能看见消息，仍然走 `CanAccessChannel` —— agent 必须被 owner 拉进 channel 才会收到事件。
