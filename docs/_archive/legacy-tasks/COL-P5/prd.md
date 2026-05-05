# P5: 聊天 UX 增强 — PRD

日期：2026-04-21 | 状态：Approved

## 背景

P1 权限系统上线后，Collab 聊天基础功能已完整。现在需要增强聊天体验，对标 Discord/Slack 的基本交互。

本 PRD 覆盖 4 个功能，按优先级排序：Typing Indicator → Emoji Picker → Message Reactions → 消息已送达标记。

## 目标用户

- Collab 平台的所有聊天用户（人类用户）
- Agent 用户作为消息接收方受益，但不主动触发 typing 等前端交互

## 功能 1: Typing Indicator（正在输入提示）

**用户故事**：作为用户，我想看到谁正在输入消息，这样知道有人在回复我。

**需求**：

- 用户开始输入时，通过 WS 发送 typing 事件（带 channelId + userId）
- 同频道其他成员底部显示"xxx 正在输入…"
- 3 秒无新输入自动消失
- 多人同时输入显示"xxx, yyy 正在输入…"
- 超过 3 人时显示"多人正在输入…"
- Agent 不发 typing 事件（Agent 通过 API/Plugin 发消息，不经过输入框）
- 节流：客户端每 2 秒最多发送一次 typing 事件，避免 WS 洪泛

**验收标准**：

- [ ] 用户输入时频道底部显示 typing indicator
- [ ] 3 秒超时自动消失
- [ ] 多人 typing 正确显示（≤3 人列名，>3 人显示"多人正在输入…"）
- [ ] typing 事件有节流，不会高频发送

## 功能 2: Emoji Picker（表情选择器）

**用户故事**：作为用户，我想在消息里插入 emoji。

**需求**：

- 输入框旁边加 emoji 按钮（😊 图标），点击弹出 emoji picker
- 支持搜索 emoji（按关键词过滤）
- 点击 emoji 插入到输入框光标位置
- 支持常用 emoji 快捷面板（基于用户使用频率，本地存储）
- Emoji 分类浏览（表情、手势、动物、食物等标准分类）
- 支持键盘导航（方向键 + Enter 选择）
- 点击 picker 外部区域自动关闭

**验收标准**：

- [ ] 点击按钮弹出 emoji picker
- [ ] 可搜索 emoji
- [ ] 选中后插入到输入框光标位置
- [ ] 常用 emoji 面板正确显示

## 功能 3: Message Reactions（消息反应）

**用户故事**：作为用户，我想对消息加 emoji 反应，不用打字就能表达态度。

**需求**：

- 鼠标悬浮消息时显示 reaction 按钮（➕ 图标）
- 点击弹出 emoji picker 选择反应（复用功能 2 的 picker 组件）
- 显示在消息底部（emoji + 计数）
- 点击已有 reaction 可以 +1 或取消自己的
- 悬浮 reaction 气泡显示参与者列表
- 存储：新建 `message_reactions` 表

  | 字段 | 类型 | 说明 |
  |---|---|---|
  | id | UUID | 主键 |
  | message_id | UUID | 关联消息 |
  | user_id | UUID | 反应用户 |
  | emoji | VARCHAR(32) | emoji 字符 |
  | created_at | TIMESTAMP | 创建时间 |

  唯一约束：`(message_id, user_id, emoji)`

- WS 实时广播 reaction 变更（`reaction:add` / `reaction:remove` 事件）
- API 端点：
  - `PUT /api/messages/{id}/reactions/{emoji}` — 添加 reaction
  - `DELETE /api/messages/{id}/reactions/{emoji}` — 移除 reaction
  - `GET /api/messages/{id}/reactions` — 获取 reaction 列表

**验收标准**：

- [ ] 可以给消息加 emoji reaction
- [ ] 显示 emoji + 计数
- [ ] 可以 +1 或取消自己的 reaction
- [ ] 实时同步到其他用户
- [ ] 同一用户对同一消息同一 emoji 不能重复添加

## 功能 4: 消息已送达标记

**用户故事**：作为用户，我想知道消息是否成功送达服务器。

**需求**：

- 发送中：显示 ⏳（pending）
- 已送达服务器：显示 ✓
- 发送失败：显示 ❌ + 重试按钮
- 实现方式：
  - 客户端发送消息时生成临时 ID（client_message_id），状态置为 pending
  - 服务器确认后通过 WS 返回 ack（携带 client_message_id + server_message_id）
  - 客户端收到 ack 后更新状态为 ✓
  - 超时（如 10 秒无 ack）或网络错误时标记为 ❌
- 重试按钮点击后重新发送原始消息内容
- v1 不做"已读"标记（复杂度太高）

**验收标准**：

- [ ] 发送中显示 pending 状态（⏳）
- [ ] 服务器确认后显示 ✓
- [ ] 失败显示 ❌ 可重试
- [ ] 重试后消息正常发送和确认

## 不在 v1 范围

- 已读回执（✓✓）— 需要追踪每个用户的阅读状态，复杂度太高，v2
- 自定义 emoji — v2
- GIF 搜索 — v2
- 消息编辑/删除 — 单独 PRD
- Emoji 快捷输入（`:emoji_name:` 语法）— v2

## 优先级

1. Typing Indicator — 最基础的聊天交互信号
2. Emoji Picker — Reactions 的前置依赖（picker 组件复用）
3. Message Reactions — 依赖 Emoji Picker，需要后端存储
4. 消息已送达标记 — 独立功能，优先级最低但用户价值明确

## 成功指标

- Typing indicator 延迟 < 500ms（从输入到对方看到）
- Emoji picker 打开时间 < 200ms
- Reaction 操作端到端延迟 < 1s
- 消息送达确认率 > 99.9%

## 开放问题

1. Emoji picker 是否使用第三方库（如 emoji-mart）还是自研？— 建议用成熟库，减少工作量
2. Reaction 数量是否需要限制（如每条消息最多 20 种不同 emoji）？
3. 消息送达标记的超时时间定多少合适？（建议 10 秒）
