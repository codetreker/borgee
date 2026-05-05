## B10 消息编辑与删除 — Task Breakdown

### Task 1: 后端 — DB migration + 编辑 API + WS 广播

- **文件**: `packages/server/src/db.ts`、`routes/messages.ts`、`queries.ts`、`ws.ts`
- **内容**:
  1. migration: `ALTER TABLE messages ADD COLUMN deleted_at INTEGER`（edited_at 已存在）
  2. `PUT /api/v1/messages/:messageId` handler
  3. 权限检查：`sender_id === currentUser.id`；Admin 不能编辑别人的消息；已删除消息不能编辑
  4. 更新 `content` + `edited_at`
  5. 返回完整 message 对象（含 `edited_at`、`deleted_at`）
  6. `broadcastToChannel` 推送 `message_edited` 事件
- **依赖**: 无
- **验收标准**:
  - [ ] 编辑自己的消息 → 200，response 包含 `edited_at`
  - [ ] 编辑别人的消息 → 403
  - [ ] 编辑已删除消息 → 400
  - [ ] 内容为空 → 400
  - [ ] WS 广播 `message_edited`

### Task 2: 后端 — 删除 API + WS 广播 + 列表过滤

- **文件**: `routes/messages.ts`、`queries.ts`
- **内容**:
  1. `DELETE /api/v1/messages/:messageId` handler
  2. 权限检查：自己的消息 || admin
  3. 软删除：`UPDATE messages SET deleted_at = ? WHERE id = ?`
  4. 返回 200 + `{ id, channel_id, deleted_at }`（不返回 204，前端需要 deleted_at 时间戳）
  5. `broadcastToChannel` 推送 `message_deleted` 事件（payload 含 `deleted_at`）
  6. GET messages 列表：`deleted_at` 不为 null 的消息 `content` 替换为空字符串
- **依赖**: Task 1（共用 migration）
- **验收标准**:
  - [ ] 删除自己的消息 → 200，response 含 `deleted_at`
  - [ ] Admin 删除任何消息 → 200
  - [ ] 非 Admin 删除别人消息 → 403
  - [ ] 已删除消息再次删除 → 200（幂等）
  - [ ] 消息列表中已删除消息 content 为空字符串，`deleted_at` 字段保留
  - [ ] WS `message_deleted` payload 含 `deleted_at`

### Task 3: 前端 — Message 类型补全 + 历史消息渲染适配

- **文件**: `packages/client/src/context/AppContext.tsx`、`types.ts`（或 Message 类型定义处）
- **内容**:
  1. `Message` 类型增加 `deleted_at?: number` 和 `edited_at?: number` 字段
  2. `EDIT_MESSAGE` reducer：更新 `content` + `edited_at`
  3. `DELETE_MESSAGE` reducer：设置 `deleted_at`，清空 `content`
  4. bootstrap / 历史消息加载时，后端返回的 `deleted_at`、`edited_at` 正确映射到前端状态
  5. `useWebSocket` 新增 `message_edited` 和 `message_deleted` handler
- **依赖**: 无（可与 Task 1 并行）
- **验收标准**:
  - [ ] Message 类型包含 `deleted_at` 和 `edited_at`
  - [ ] 历史加载后，已删除消息正确显示"此消息已删除"
  - [ ] 历史加载后，已编辑消息显示"(已编辑)"
  - [ ] WS `message_edited` / `message_deleted` 正确更新本地状态

### Task 4: 前端 — 消息操作栏 + Inline 编辑

- **文件**: `packages/client/src/components/MessageItem.tsx`、`api.ts`、`index.css`
- **内容**:
  1. hover 消息显示操作栏（编辑 ✏️ + 删除 🗑️，和 ReactionBar ➕ 同行）
  2. 编辑和删除按钮仅在自己的消息上显示；Admin 在所有消息上显示删除
  3. 点击 ✏️ 进入 inline 编辑模式（`<textarea>`）
  4. Enter 保存（调用 PUT API），Esc 取消
  5. 编辑后显示"(已编辑)"标记 + `edited_at` 时间（如 "已编辑于 14:30"）
  6. 已删除消息显示"此消息已删除"（灰色斜体），不显示操作栏
- **依赖**: Task 1, Task 3
- **验收标准**:
  - [ ] hover 显示操作栏，权限正确
  - [ ] inline 编辑 + Enter 保存 + Esc 取消
  - [ ] 编辑后显示"(已编辑)" + 编辑时间
  - [ ] 已删除消息显示灰色斜体"此消息已删除"
  - [ ] WS 实时同步编辑和删除

### Task 5: 前端 — 删除确认弹窗

- **文件**: `packages/client/src/components/MessageItem.tsx`
- **内容**:
  1. 点击 🗑️ 弹出确认对话框"确定删除这条消息？"
  2. 确认后调用 DELETE API
  3. 删除后本地立即更新（通过 `DELETE_MESSAGE` dispatch）
- **依赖**: Task 2, Task 3, Task 4
- **验收标准**:
  - [ ] 确认弹窗正常弹出
  - [ ] 确认后消息变为"此消息已删除"
  - [ ] 取消后无操作
  - [ ] Admin 可删除任何消息

### Task 6: SSE 推送 + Agent REST 权限

- **文件**: SSE 相关文件（`routes/sse.ts` 或 plugin 路由）
- **内容**:
  1. `message_edited` 和 `message_deleted` 事件通过 SSE 推送给 Agent
  2. Agent 可通过 REST API 编辑/删除自己的消息（复用 T1/T2 的权限检查，与人类相同）
  3. 编辑/删除发生时，**不**往 messages 表插入 `sender_id = 'system'` 的行（避免 FK 约束冲突）
  4. 改为在 SSE 事件 payload 中附加 `system_message` 字段，供 Agent 获取上下文通知：
     - 编辑事件：`system_message: "用户 A 编辑了消息"`
     - 删除事件：`system_message: "用户 A 删除了一条消息"`
  5. WS 广播（面向人类客户端）不包含 `system_message`（前端自行根据事件类型渲染）
- **依赖**: Task 1, Task 2
- **验收标准**:
  - [ ] Agent 通过 SSE 收到 `message_edited` / `message_deleted` 事件
  - [ ] SSE 事件 payload 含 `system_message` 字段
  - [ ] **不**在 messages 表中插入 system 消息行
  - [ ] Agent 可通过 PUT/DELETE 编辑/删除自己的消息

---

## 依赖关系图

```
Task 1 (编辑 API + migration) ──┬── Task 4 (操作栏 + inline 编辑)
                                │                │
Task 2 (删除 API) ──────────────┤   Task 5 (删除确认弹窗)
          │                     │
Task 3 (类型 + reducer + WS) ──┘
          │
Task 6 (SSE + Agent) ← 依赖 Task 1, Task 2
```

## Review 修正说明

| # | 问题 (HIGH) | 修正 |
|---|-------------|------|
| 1 | DELETE API 返回 204 无 body，前端拿不到 `deleted_at` | T2 改为返回 200 + `{ id, channel_id, deleted_at }`；WS payload 也含 `deleted_at` |
| 2 | PRD Agent 行为要求注入 `sender_id = 'system'` 的 DB 消息行，违反 FK 约束 | T6 改为不插入 DB 行，通过 SSE 事件 payload 的 `system_message` 字段传递上下文通知 |
| 3 | T6（SSE/Agent）缺少对 T1（编辑 API）的依赖 | T6 显式依赖 T1 和 T2 |
| 4 | `deleted_at` 未贯穿到客户端 Message 类型和 bootstrap 加载 | 新增 T3 专门处理类型定义、reducer、历史消息渲染适配 |
| 5 | PRD 要求"编辑后显示编辑时间"但原 tasks 仅写"(已编辑)"标记 | T4 补充：显示 `edited_at` 时间（如"已编辑于 14:30"） |
| 6 | Agent 双重投递：SSE 事件 + DB system 消息导致重复通知 | 统一为 SSE 事件投递，DB 不插入 system 消息行；Agent "注入 context" 通过 SSE payload `system_message` 字段实现 |

## 总结

- **后端改动**: T1（编辑 API + migration）、T2（删除 API）、T6（SSE 推送）
- **前端改动**: T3（类型 + reducer）、T4（操作栏 + 编辑 + 编辑时间）、T5（删除确认）
- **关键路径**: T1 → T4 → T5（后端先行，前端跟进）
- **并行可能**: T3 可与 T1/T2 并行；T6 在 T1+T2 完成后开始
