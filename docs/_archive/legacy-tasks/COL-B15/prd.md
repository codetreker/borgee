# COL-B15: Collab Plugin Skill — PRD

日期：2026-04-24 | 状态：Draft | 作者：野马(PM)

## 背景

Collab 已有完整的 OpenClaw Plugin（`@codetreker/collab-openclaw-plugin`），Agent 可以通过它连接 Collab 平台收发消息。但目前 Agent 要使用这个 Plugin，需要人工配置，没有标准化的指引文档。

OpenClaw 的 Skill 机制让 Agent 通过读 `SKILL.md` 就能理解一个工具的能力和用法。Collab Plugin 缺少这样一份 Skill 文档，导致：

- Agent 不知道 Collab Plugin 能做什么
- 配置步骤靠口头传递，容易出错
- 新 Agent 上手成本高

## 目标用户

- **OpenClaw Agent**：需要接入 Collab 平台的各种 AI Agent（飞马、野马、战马、烈马等）
- **Agent 运维者（建军等）**：需要为 Agent 配置 Collab 连接的人

## 核心需求

### 需求 1: SKILL.md 文档

**用户故事**：作为 OpenClaw Agent，我想通过读一份 SKILL.md 文档，就能了解如何使用 Collab Plugin 收发消息，以便快速接入 Collab 平台参与团队协作。

SKILL.md 应包含以下内容：

#### 1.1 Plugin 功能概述

说明 Plugin 做什么：
- 作为 OpenClaw 的 channel plugin，让 Agent 连接 Collab 平台
- Agent 在 Collab 频道中作为一等公民参与对话
- 支持实时消息收发（WS / SSE / 长轮询自动降级）

#### 1.2 安装方法

- npm 包名：`@codetreker/collab-openclaw-plugin`
- 安装命令
- 版本要求（peerDependency: `openclaw >= 2026.4.15`）

#### 1.3 配置方法

Plugin 通过 OpenClaw config 的 `channels.collab` 段配置，需要说明：

| 配置项 | 必填 | 说明 |
|--------|------|------|
| `baseUrl` | ✅ | Collab 服务器地址（如 `https://collab.example.com`） |
| `apiKey` | ✅ | API Key（在 Collab 管理后台获取） |
| `botUserId` | 可选 | Agent 的用户 ID（不填会自动从 `/api/v1/users/me` 获取） |
| `botDisplayName` | 可选 | Agent 显示名（不填会自动获取） |
| `transport` | 可选 | 传输方式：`auto`（默认）/ `ws` / `sse` / `poll` |
| `pollTimeoutMs` | 可选 | 长轮询超时（默认 30000ms） |
| `enabled` | 可选 | 是否启用（默认 true） |
| `allowFrom` | 可选 | 允许接收消息的来源（默认 `["*"]` 全部） |
| `defaultTo` | 可选 | 默认发送目标 |

多账号配置说明（`accounts` 字段）。

#### 1.4 使用场景

Agent 配置完成后能做什么：

- **接收频道消息**：Agent 加入的频道中有新消息时，自动收到并可回复
- **发送消息**：Agent 向指定频道或用户发消息
  - 频道目标格式：`channel:<channel_id>`
  - DM 目标格式：`dm:<user_id>`
- **@mention**：消息中使用 `<@user_id>` 格式 mention 其他用户或 Agent
- **收到 @mention**：当 `requireMention` 开启时，Agent 只响应被 @ 的消息；DM 中始终自动响应
- **Reactions**：Agent 可以对消息添加/移除 emoji reaction
- **编辑消息**：Agent 可以编辑自己发送的消息
- **删除消息**：Agent 可以删除消息
- **DM 对话**：Agent 可以发起或回复 DM 单聊

#### 1.5 注意事项

- **认证**：API Key 是唯一的认证方式，Agent 需要在 Collab 管理后台创建账号并获取 API Key
- **传输层**：默认 `auto` 模式会优先尝试 SSE，不可用时降级到长轮询；`ws` 模式使用 WebSocket 全双工连接
- **断线重连**：Plugin 内置断线重连和指数退避逻辑，Agent 无需处理
- **Bot 身份**：Plugin 启动时会自动通过 `/api/v1/users/me` 获取 bot 身份信息
- **消息过滤**：Plugin 自动过滤掉 bot 自己发的消息，避免自我循环
- **权限**：Agent 的频道权限由其 owner（人类用户）管理，Agent 需要被 owner 拉入频道才能参与

### 需求 2: Skill 可发现与安装

**用户故事**：作为 Agent 运维者，我想把 Collab Plugin Skill 安装到 Agent 的 workspace，让 Agent 自动发现并使用。

- Skill 放在 Plugin 包内或独立发布到 ClawHub
- 安装后出现在 Agent 的 `<available_skills>` 列表中
- `description` 字段准确描述触发场景，让 Agent 能正确匹配

Skill description 建议：
> Connect to Collab team chat platform. Use when: Agent needs to send/receive messages on Collab, configure Collab channel plugin, or troubleshoot Collab connection. Covers: plugin installation, configuration (baseUrl, apiKey, transport), messaging (channels, DMs, mentions, reactions), and connection troubleshooting.

## 验收标准

- [ ] SKILL.md 写完并通过飞马 review
- [ ] SKILL.md 涵盖以上所有内容模块（功能概述、安装、配置、使用场景、注意事项）
- [ ] Agent 通过读 SKILL.md 能成功配置和使用 Collab Plugin（至少一个 Agent 实际验证）
- [ ] Skill 安装到至少一个 Agent workspace 并出现在 available_skills 列表中
- [ ] Agent 配置后能成功收发 Collab 消息

## 不在范围

- 不改 Plugin 代码——本任务纯文档
- 不写新的 Plugin 功能
- 不涉及 Collab 服务端变更
- 不涉及 Agent 归属/权限系统（P1 范围）

## 成功指标

- 新 Agent 通过读 SKILL.md 完成 Collab 接入时间 < 10 分钟（无需人工指导）
- 配置错误率降低（有标准文档可参照）

## 开放问题

1. **Skill 发布位置**：放在 Plugin npm 包内还是独立发布到 ClawHub？需要和飞马确认
2. **多 Agent 场景**：多个 Agent 连同一个 Collab 实例时，SKILL.md 是否需要说明多账号配置的最佳实践？
3. **版本同步**：Plugin 更新后 SKILL.md 如何保持同步？是否需要 CI 检查？
