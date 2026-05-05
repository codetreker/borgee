# COL-B10: 消息编辑与删除 — 技术设计

日期：2026-04-21 | 状态：Draft

## 1. 背景

参考 PRD：`docs/tasks/COL-B10/prd.md`。用户需要编辑和删除已发送的消息。

## 2. 数据库变更

messages 表已有 `edited_at` 字段。需要新增：
- `deleted_at INTEGER` — 软删除时间戳（migrate 自动 ALTER TABLE ADD COLUMN）

```sql
-- 在 db.ts migrate() 中添加
ALTER TABLE messages ADD COLUMN deleted_at INTEGER
```

## 3. API 设计

### 3.1 编辑消息

```
PUT /api/v1/messages/:messageId
Body: { content: string }
Response: 200 { message }

权限：
- 只能编辑自己的消息（sender_id === currentUser.id）
- Admin 不能编辑别人的消息
- 已删除的消息不能编辑
```

### 3.2 删除消息

```
DELETE /api/v1/messages/:messageId
Response: 204

权限：
- 可以删除自己的消息
- Admin 可以删除任何人的消息
- 已删除的消息返回 204（幂等）
```

### 3.3 消息列表 API 调整

GET messages 返回时：
- `deleted_at` 不为 null 的消息，`content` 替换为空字符串（不泄露原始内容）
- 前端根据 `deleted_at` 显示"此消息已删除"

## 4. WS 事件

### 4.1 消息编辑

```json
{
  "type": "message_edited",
  "message": {
    "id": "...",
    "channel_id": "...",
    "content": "new content",
    "edited_at": 1234567890
  }
}
```

通过 `broadcastToChannel(channelId, payload)` 推送。

### 4.2 消息删除

```json
{
  "type": "message_deleted",
  "message_id": "...",
  "channel_id": "..."
}
```

通过 `broadcastToChannel(channelId, payload)` 推送。

## 5. 前端设计

### 5.1 消息操作栏

鼠标悬浮消息时，右上角显示操作栏（和 ReactionBar 的 ➕ 按钮同一行）：
- 😊 Reaction（已有）
- ✏️ 编辑（仅自己的消息）
- 🗑️ 删除（自己的消息 + Admin 所有消息）

### 5.2 Inline 编辑

- 点击 ✏️ 后，消息内容变为 `<textarea>`（和 MessageInput 类似）
- Enter 保存，Esc 取消
- 保存时调用 PUT API
- 编辑后消息显示 "(已编辑)" 标记

### 5.3 删除确认

- 点击 🗑️ 后弹出确认对话框
- 确认后调用 DELETE API
- 删除后消息显示 "此消息已删除"（灰色斜体）

### 5.4 AppContext 新增 Actions

```typescript
| { type: 'EDIT_MESSAGE'; channelId: string; messageId: string; content: string; editedAt: number }
| { type: 'DELETE_MESSAGE'; channelId: string; messageId: string }
```

### 5.5 useWebSocket 新增 handler

```typescript
case 'message_edited': { dispatch({ type: 'EDIT_MESSAGE', ... }); break; }
case 'message_deleted': { dispatch({ type: 'DELETE_MESSAGE', ... }); break; }
```

## 6. SSE/Plugin 推送

编辑和删除事件也通过 SSE 推送给 Agent（和 `new_message` 类似）：
- `message_edited` 事件
- `message_deleted` 事件

Agent 可通过 REST API 编辑/删除自己的消息（和人类相同权限检查）。

## 7. 错误处理

| 场景 | HTTP 状态码 | 错误信息 |
|------|------------|---------|
| 消息不存在 | 404 | "Message not found" |
| 非自己的消息（编辑） | 403 | "Can only edit your own messages" |
| 非自己且非 Admin（删除） | 403 | "Permission denied" |
| 已删除消息编辑 | 400 | "Cannot edit deleted message" |
| 内容为空（编辑） | 400 | "Content is required" |

## 8. Task Breakdown

### T1: 后端 — 编辑 API + WS 广播

**改动文件**：`routes/messages.ts`、`queries.ts`、`ws.ts`

**内容**：
1. PUT /api/v1/messages/:messageId handler
2. 权限检查（sender_id === currentUser.id）
3. 更新 content + edited_at
4. broadcastToChannel `message_edited`

**验收标准**：
- [ ] 编辑自己的消息 → 200
- [ ] 编辑别人的消息 → 403
- [ ] WS 广播 message_edited

### T2: 后端 — 删除 API + WS 广播

**改动文件**：`routes/messages.ts`、`queries.ts`、`db.ts`（migration）

**内容**：
1. DELETE /api/v1/messages/:messageId handler
2. 权限检查（自己 || admin）
3. 软删除（UPDATE deleted_at）
4. broadcastToChannel `message_deleted`
5. messages 列表 API 过滤已删除消息内容

**验收标准**：
- [ ] 删除自己的消息 → 204
- [ ] Admin 删除任何消息 → 204
- [ ] 非 Admin 删除别人消息 → 403
- [ ] 消息列表中已删除消息 content 为空

### T3: 前端 — 消息操作栏 + Inline 编辑

**改动文件**：`MessageItem.tsx`、`AppContext.tsx`、`useWebSocket.ts`、`api.ts`、`index.css`

**内容**：
1. hover 消息显示操作栏（编辑 + 删除按钮）
2. inline 编辑模式（textarea + Enter/Esc）
3. 调用 PUT API
4. EDIT_MESSAGE reducer
5. useWebSocket 处理 message_edited

**验收标准**：
- [ ] hover 显示操作栏
- [ ] inline 编辑 + Enter 保存
- [ ] "(已编辑)" 标记
- [ ] WS 实时同步

### T4: 前端 — 删除确认 + 删除状态显示

**改动文件**：`MessageItem.tsx`、`AppContext.tsx`、`useWebSocket.ts`

**内容**：
1. 删除确认对话框
2. 调用 DELETE API
3. DELETE_MESSAGE reducer
4. 已删除消息显示"此消息已删除"
5. useWebSocket 处理 message_deleted

**验收标准**：
- [ ] 确认弹窗
- [ ] 删除后显示"此消息已删除"
- [ ] WS 实时同步
- [ ] Admin 可删除任何消息

### T5: SSE/Plugin 推送

**改动文件**：SSE 相关文件

**内容**：
1. message_edited + message_deleted 事件通过 SSE 推送
2. Agent 可通过 REST API 编辑/删除自己的消息

**验收标准**：
- [ ] Agent 收到编辑/删除事件
- [ ] Agent 可编辑/删除自己的消息

## 9. 不在范围

- 编辑历史查看（v2）
- 批量删除
- 消息撤回时间限制
