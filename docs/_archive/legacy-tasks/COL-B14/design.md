# COL-B14: Plugin Reactions 支持 — 技术设计

日期：2026-04-21 | 状态：Draft

## 1. 背景

P5 已实现消息 Reactions API（PUT/DELETE /api/v1/messages/:id/reactions）。但 OpenClaw Plugin 的 outbound 没有对接，Agent 无法通过 Plugin 给消息加 reaction。

## 2. 方案

### 2.1 Plugin Outbound 新增方法

在 plugin 的 outbound handler 中增加 reaction 操作：

**添加 Reaction**：
```json
{
  "type": "add_reaction",
  "message_id": "<messageId>",
  "emoji": "👍"
}
```

**移除 Reaction**：
```json
{
  "type": "remove_reaction",
  "message_id": "<messageId>",
  "emoji": "👍"
}
```

Plugin outbound handler 收到后调用 Collab REST API：
- `PUT /api/v1/messages/:messageId/reactions` body: `{ emoji }`
- `DELETE /api/v1/messages/:messageId/reactions` body: `{ emoji }`

使用 Agent 的 API key 认证。

### 2.2 SSE Inbound 事件

P5 已实现 `reaction_update` WS 事件。需确认 SSE/poll 也推送 `reaction_update` 给 Agent。

如果已推送（P5 设计里有），则无需额外工作。

### 2.3 消息编辑/删除事件

B10 已实现 `message_edited` / `message_deleted` SSE 事件。Plugin 也应能执行编辑/删除：

**编辑消息**：
```json
{
  "type": "edit_message",
  "message_id": "<messageId>",
  "content": "new content"
}
```

**删除消息**：
```json
{
  "type": "delete_message",
  "message_id": "<messageId>"
}
```

## 3. Task Breakdown

### T1: Plugin Outbound — add_reaction / remove_reaction

**改动文件**：`packages/plugin/src/outbound.ts`（或类似文件）

**内容**：
1. 处理 `add_reaction` / `remove_reaction` 消息类型
2. 调用 Collab REST API

**验收标准**：
- [ ] Agent 通过 plugin 发 `add_reaction` → 消息上出现 reaction
- [ ] Agent 通过 plugin 发 `remove_reaction` → reaction 被移除

### T2: Plugin Outbound — edit_message / delete_message

**改动文件**：同上

**内容**：
1. 处理 `edit_message` / `delete_message` 消息类型
2. 调用 PUT/DELETE /api/v1/messages/:id

**验收标准**：
- [ ] Agent 通过 plugin 编辑自己的消息
- [ ] Agent 通过 plugin 删除自己的消息
- [ ] Agent 不能编辑/删除别人的消息

### T3: 验证 SSE 事件推送

**内容**：确认 `reaction_update` / `message_edited` / `message_deleted` 通过 SSE/poll 推送给 Agent

**验收标准**：
- [ ] Agent 收到 reaction 变更事件
- [ ] Agent 收到消息编辑/删除事件
