# P5: 聊天 UX 增强 — 技术设计

日期：2026-04-21 | 状态：Draft

## 1. 背景

本文档是 [P5 聊天 UX 增强 PRD](../requirements/chat-ux-enhancements.md) 的技术设计。P1 权限系统上线后，Collab 聊天基础功能已完整，现在需要增强聊天交互体验。

本设计覆盖 4 个功能：
1. **Typing Indicator** — 正在输入提示
2. **Emoji Picker** — 表情选择器
3. **Message Reactions** — 消息反应
4. **消息已送达标记** — 发送确认

### 现有架构概览

- **服务端**：Fastify + `@fastify/websocket`，SQLite (better-sqlite3)，WebSocket 在 `ws.ts` 中管理客户端连接（`clients` Map<WebSocket, WsClient>）、频道订阅和广播
- **客户端**：React SPA，`AppContext` (useReducer) 全局状态管理，`useWebSocket` hook 管理 WS 连接和消息分发
- **WS 协议**：客户端发 `subscribe`/`unsubscribe`/`ping`/`send_message`，服务端推送 `new_message`/`presence`/`channel_*` 等事件
- **广播函数**：`broadcastToChannel(channelId, payload, excludeWs?)`、`broadcastToUser(userId, payload)`、`broadcastToAll(payload)`
- **注意**：`broadcastToChannel` 将新增 `excludeWs` 参数，用于排除发送者自己（typing、new_message 场景）
- **数据库**：SQLite，migration 采用 `initSchema()` 中的 `ALTER TABLE ADD COLUMN` 模式（检查列是否存在后添加）

---

## 2. F1: Typing Indicator（正在输入提示）

### 2.1 WS 事件协议

**客户端 → 服务端：**
```json
{
  "type": "typing",
  "channel_id": "<channelId>"
}
```

**服务端 → 频道其他成员（广播）：**
```json
{
  "type": "typing",
  "channel_id": "<channelId>",
  "user_id": "<userId>",
  "display_name": "<displayName>"
}
```

服务端收到 `typing` 后，向同频道的**其他**已订阅客户端广播（排除发送者自身的 WebSocket 连接）。纯内存操作，不做持久化、不写 events 表。

### 2.2 服务端实现（`ws.ts`）

在 `socket.on('message')` switch 中新增 `typing` case：

```typescript
case 'typing': {
  if (!msg.channel_id) break;
  if (!client.subscribedChannels.has(msg.channel_id)) break;
  const data = JSON.stringify({
    type: 'typing',
    channel_id: msg.channel_id,
    user_id: userId,
    display_name: user.display_name,
  });
  for (const c of clients.values()) {
    if (c !== client && c.subscribedChannels.has(msg.channel_id) && c.ws.readyState === 1) {
      c.ws.send(data);
    }
  }
  break;
}
```

### 2.3 客户端节流（`MessageInput.tsx`）

在 `handleChange` 中触发 typing 事件，使用 `useRef` 实现 2 秒节流：

```typescript
const lastTypingSent = useRef(0);

const emitTyping = useCallback(() => {
  const now = Date.now();
  if (now - lastTypingSent.current < 2000) return;
  lastTypingSent.current = now;
  sendWsMessage({ type: 'typing', channel_id: channelId });
}, [channelId, sendWsMessage]);
```

`useWebSocket` hook 需要暴露 `sendWsMessage(payload)` 方法供组件调用。通过 `AppContext` 传递或单独创建 `WebSocketContext`。

### 2.4 前端状态管理（`AppContext`）

AppState 新增：
```typescript
typingUsers: Map<string, Map<string, { displayName: string; expiresAt: number }>>;
// channelId -> Map<userId, { displayName, expiresAt }>
```

新增 reducer actions：
- `SET_TYPING { channelId, userId, displayName }` — 设置/刷新 typing 状态（expiresAt = Date.now() + 3000）
- `CLEAR_EXPIRED_TYPING` — 清除所有 expiresAt < Date.now() 的条目

在 `AppProvider` 中设置 1 秒周期 `setInterval` 调用 `dispatch({ type: 'CLEAR_EXPIRED_TYPING' })`。

### 2.5 `useWebSocket` 处理 typing 事件

在 `handleMessage` switch 中新增：
```typescript
case 'typing': {
  const channelId = data.channel_id as string;
  const userId = data.user_id as string;
  const displayName = data.display_name as string;
  dispatch({ type: 'SET_TYPING', channelId, userId, displayName });
  break;
}
```

### 2.6 前端组件：`TypingIndicator`

新建 `packages/client/src/components/TypingIndicator.tsx`，放在 `MessageList` 的 `<div ref={bottomRef} />` 之前：

```tsx
function TypingIndicator({ channelId }: { channelId: string }) {
  const { state } = useAppContext();
  const typingMap = state.typingUsers.get(channelId);
  if (!typingMap || typingMap.size === 0) return null;

  const names = [...typingMap.values()].map(t => t.displayName);

  let text: string;
  if (names.length <= 3) {
    text = `${names.join(', ')} 正在输入…`;
  } else {
    text = '多人正在输入…';
  }

  return <div className="typing-indicator"><span className="typing-dots" />{text}</div>;
}
```

---

## 3. F2: Emoji Picker（表情选择器）

### 3.1 技术选型

使用 `@emoji-mart/react` + `@emoji-mart/data`（PRD 建议使用成熟库）。

优势：
- 内置搜索、分类浏览、常用面板、键盘导航
- 常用 emoji 自动基于 `localStorage` 维护，无需额外开发
- 支持 i18n（可配置中文 locale）

### 3.2 安装依赖

```bash
cd packages/client && npm install @emoji-mart/react @emoji-mart/data emoji-mart
```

### 3.3 输入框组件改造（`MessageInput.tsx`）

在 `message-input-row` 中，upload 按钮和 textarea 之间添加 emoji 按钮：

```tsx
const [emojiPickerOpen, setEmojiPickerOpen] = useState(false);
const emojiPickerRef = useRef<HTMLDivElement>(null);
const emojiBtnRef = useRef<HTMLButtonElement>(null);

<button
  ref={emojiBtnRef}
  className="icon-btn emoji-btn"
  onClick={() => setEmojiPickerOpen(v => !v)}
  title="选择表情"
>
  😊
</button>

{emojiPickerOpen && (
  <div className="emoji-picker-popover" ref={emojiPickerRef}>
    <Picker
      data={data}
      onEmojiSelect={(emoji: { native: string }) => {
        insertEmojiAtCursor(emoji.native);
        setEmojiPickerOpen(false);
        textareaRef.current?.focus();
      }}
      locale="zh"
      previewPosition="none"
    />
  </div>
)}
```

### 3.4 光标位置插入

```typescript
const insertEmojiAtCursor = (emoji: string) => {
  const ta = textareaRef.current;
  if (!ta) return;
  const start = ta.selectionStart;
  const end = ta.selectionEnd;
  const newText = text.slice(0, start) + emoji + text.slice(end);
  setText(newText);
  requestAnimationFrame(() => {
    const pos = start + emoji.length;
    ta.setSelectionRange(pos, pos);
  });
};
```

### 3.5 点击外部关闭

```typescript
useEffect(() => {
  if (!emojiPickerOpen) return;
  const handler = (e: MouseEvent) => {
    if (emojiPickerRef.current?.contains(e.target as Node)) return;
    if (emojiBtnRef.current?.contains(e.target as Node)) return;
    setEmojiPickerOpen(false);
  };
  document.addEventListener('mousedown', handler);
  return () => document.removeEventListener('mousedown', handler);
}, [emojiPickerOpen]);
```

### 3.6 样式定位

`.emoji-picker-popover` 使用 `position: absolute`，相对于 `.message-input-container`（需设为 `position: relative`），向上弹出（`bottom: 100%`）。

---

## 4. F3: Message Reactions（消息反应）

### 4.1 数据库 Schema

在 `db.ts` 的 `initSchema` migration 中添加：

```sql
CREATE TABLE IF NOT EXISTS message_reactions (
  id          TEXT PRIMARY KEY,
  message_id  TEXT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
  user_id     TEXT NOT NULL REFERENCES users(id),
  emoji       TEXT NOT NULL,
  created_at  INTEGER NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_reactions_unique
  ON message_reactions(message_id, user_id, emoji);

CREATE INDEX IF NOT EXISTS idx_reactions_message
  ON message_reactions(message_id);
```

使用 `CREATE TABLE IF NOT EXISTS` + `CREATE INDEX IF NOT EXISTS`，与现有 migration 模式一致（无需检查列存在性，因为是新表）。

### 4.2 查询层（`queries.ts` 新增函数）

```typescript
// 添加 reaction（幂等，INSERT OR IGNORE）
export function addReaction(
  db: Database.Database, messageId: string, userId: string, emoji: string
): void

// 移除 reaction
export function removeReaction(
  db: Database.Database, messageId: string, userId: string, emoji: string
): boolean

// 聚合查询单条消息的 reactions
export function getReactionsByMessageId(
  db: Database.Database, messageId: string
): { emoji: string; count: number; user_ids: string[] }[]

// 批量查询多条消息的 reactions（避免 N+1）
export function getReactionsForMessages(
  db: Database.Database, messageIds: string[]
): Map<string, { emoji: string; count: number; user_ids: string[] }[]>
```

`getReactionsForMessages` 实现：
```sql
SELECT message_id, emoji, GROUP_CONCAT(user_id) AS user_ids, COUNT(*) AS count
FROM message_reactions
WHERE message_id IN (?, ?, ...)
GROUP BY message_id, emoji
ORDER BY MIN(created_at) ASC
```

### 4.3 API 端点（新建 `routes/reactions.ts`）

在 `index.ts` 中 `registerReactionRoutes(app)` 注册。

| 方法 | 路径 | 权限 | 说明 |
|------|------|------|------|
| `PUT` | `/api/v1/messages/:messageId/reactions` | 需登录 + 频道成员 | 添加 reaction（幂等），body: `{ emoji: "👍" }` |
| `DELETE` | `/api/v1/messages/:messageId/reactions` | 需登录 + 频道成员 | 移除自己的 reaction，body: `{ emoji: "👍" }` |

> **注意**：emoji 通过 request body 传递，不放 URL path，避免 ZWJ 序列等特殊字符的 URL 编码问题。每条消息限 20 种不同 emoji，超出返回 429。
| `GET` | `/api/v1/messages/:messageId/reactions` | 需登录 + 频道成员 | 获取 reaction 聚合列表 |

PUT/DELETE handler 流程：
1. 验证 messageId 对应的消息存在
2. 验证当前用户是消息所在频道的成员
3. 执行数据库操作
4. 查询该消息的最新 reactions 聚合
5. `broadcastToChannel(channelId, { type: 'reaction_update', message_id, channel_id, reactions })`

### 4.4 WS 事件

**服务端 → 频道成员：**
```json
{
  "type": "reaction_update",
  "message_id": "<messageId>",
  "channel_id": "<channelId>",
  "reactions": [
    { "emoji": "👍", "count": 3, "user_ids": ["u1", "u2", "u3"] },
    { "emoji": "❤️", "count": 1, "user_ids": ["u4"] }
  ]
}
```

采用**全量推送**模式（推送该消息的所有 reactions），而非增量 add/remove。原因：
- 客户端同步逻辑简单（直接替换）
- 单条消息 reactions 数据量小（几十个 emoji 最多几 KB）
- 避免客户端因消息乱序导致状态不一致

### 4.5 扩展消息列表 API

在 `queries.ts` 的 `getMessages` 返回后，附加调用 `getReactionsForMessages` 为每条消息附带 reactions 数据。在 `routes/messages.ts` 的 GET handler 中处理：

```typescript
const { messages, has_more } = Q.getMessages(db, channelId, before, limit, after);
const messageIds = messages.map(m => m.id);
const reactionsMap = Q.getReactionsForMessages(db, messageIds);
const messagesWithReactions = messages.map(m => ({
  ...m,
  reactions: reactionsMap.get(m.id) ?? [],
}));
return { messages: messagesWithReactions, has_more };
```

### 4.6 前端状态（`AppContext`）

不需要单独的 reactions map。直接在 `Message` 类型上扩展 `reactions` 字段：

```typescript
// types.ts
interface Message {
  // ...existing fields
  reactions?: { emoji: string; count: number; user_ids: string[] }[];
}
```

新增 reducer action：
- `UPDATE_REACTIONS { messageId, channelId, reactions }` — 更新指定消息的 reactions

```typescript
case 'UPDATE_REACTIONS': {
  const msgs = new Map(state.messages);
  const channelMsgs = msgs.get(action.channelId);
  if (!channelMsgs) return state;
  const updated = channelMsgs.map(m =>
    m.id === action.messageId ? { ...m, reactions: action.reactions } : m
  );
  msgs.set(action.channelId, updated);
  return { ...state, messages: msgs };
}
```

### 4.7 `useWebSocket` 处理 `reaction_update`

```typescript
case 'reaction_update': {
  dispatch({
    type: 'UPDATE_REACTIONS',
    messageId: data.message_id as string,
    channelId: data.channel_id as string,
    reactions: data.reactions as { emoji: string; count: number; user_ids: string[] }[],
  });
  break;
}
```

### 4.8 前端组件

**`ReactionBar.tsx`**（新建）：

Props: `{ reactions, messageId, channelId, currentUserId }`

- 渲染每个 reaction 为一个 pill（`<button>`）：emoji + count
- 当前用户已参与的 pill 添加 `.active` class 高亮
- 点击 pill：如果已参与则 DELETE，否则 PUT
- 末尾渲染 ➕ 按钮，点击弹出 emoji picker（复用 F2 的 `@emoji-mart/react` Picker）
- Hover tooltip 显示参与者名单

**`MessageItem.tsx` 改造**：

- 在消息内容下方渲染 `<ReactionBar />`
- hover 消息时显示快捷 reaction 按钮（绝对定位在消息右上角）

---

## 5. F4: 消息已送达标记

### 5.1 `client_message_id` 机制

**扩展 WS `send_message` 协议：**

客户端 → 服务端：
```json
{
  "type": "send_message",
  "channel_id": "<channelId>",
  "content": "hello",
  "client_message_id": "<uuid-v4>"
}
```

服务端 → 发送者（ack）：
```json
{
  "type": "message_ack",
  "client_message_id": "<uuid-v4>",
  "message": { /* 完整 server message */ }
}
```

### 5.2 服务端改动（`ws.ts`）

在现有 `send_message` handler 中，修改发送逻辑：
1. 先发 ack 给发送者
2. 再 broadcastToChannel（排除发送者的 ws）
3. 发送失败时返回 message_nack

将 `socket.send({ type: 'message_sent', message })` 改为：

```typescript
socket.send(JSON.stringify({
  type: 'message_ack',
  client_message_id: msg.client_message_id ?? null,
  message,
}));
```

保持向后兼容：如果没有 `client_message_id`，仍然返回 ack（client_message_id 为 null），不影响旧客户端。

### 5.3 前端状态管理（`AppContext`）

新增 `PendingMessage` 类型和状态：

```typescript
interface PendingMessage {
  clientMessageId: string;
  channelId: string;
  content: string;
  contentType: 'text' | 'image';
  status: 'pending' | 'failed';
  createdAt: number;
  senderName: string;
  senderId: string;
}
```

AppState 新增：
```typescript
pendingMessages: Map<string, PendingMessage[]>; // channelId -> pending messages
```

新增 actions：
- `ADD_PENDING_MESSAGE { message: PendingMessage }` — 添加到对应 channel 的 pending 列表
- `ACK_PENDING_MESSAGE { clientMessageId, channelId, serverMessage }` — 从 pending 列表移除，添加到 messages
- `FAIL_PENDING_MESSAGE { clientMessageId, channelId }` — 标记为 failed
- `REMOVE_PENDING_MESSAGE { clientMessageId, channelId }` — 重试前移除

### 5.4 发送流程改造

`MessageInput.tsx` 的 `handleSend` 改为：

1. 生成 `clientMessageId = crypto.randomUUID()`
2. dispatch `ADD_PENDING_MESSAGE`（立即在 UI 显示 pending 消息）
3. 调用 `sendWsMessage({ type: 'send_message', channel_id, content, client_message_id: clientMessageId })`
4. 设置 10 秒超时 `setTimeout`，到期 dispatch `FAIL_PENDING_MESSAGE`

`useWebSocket` 处理 `message_ack`：
1. dispatch `ACK_PENDING_MESSAGE`（用 serverMessage 替换 pending）
2. 清除对应的超时定时器

### 5.5 `MessageList` 渲染

在渲染消息列表时，将 pending messages 追加到已有 messages 之后：

```typescript
const messages = state.messages.get(channelId) ?? [];
const pending = state.pendingMessages.get(channelId) ?? [];
const allMessages = [...messages, ...pending.map(p => toPseudoMessage(p))];
```

`toPseudoMessage` 将 PendingMessage 转换为 Message 兼容对象（带 `_pending: true` / `_failed: true` 标记）。

### 5.6 `MessageItem` 状态图标

仅对 `sender_id === currentUser.id` 的消息显示：

| 消息类型 | 图标 |
|---------|------|
| `_pending === true` | ⏳ |
| `_failed === true` | ❌ + "重试"按钮 |
| 正常服务端消息 | ✓（轻灰色，不喧宾夺主） |

### 5.7 重试机制

"重试"按钮 onClick：
1. dispatch `REMOVE_PENDING_MESSAGE`
2. 用原始 content 重新执行完整发送流程（新 clientMessageId）

### 5.8 去重处理

`message_ack` 到达后将 pending 替换为 server message。服务端发送顺序：**先发 ack 给发送者，再 broadcastToChannel（排除发送者）**。这样发送者不会收到自己消息的 `new_message` 广播，避免重复。

`ADD_MESSAGE` reducer 需新增 id 去重逻辑：如果 `state.messages` 中已存在相同 `id` 的消息，跳过添加。这是防御性编程，防止 ack 和 new_message 顺序异常时的重复显示。

发送失败时，服务端返回 `message_nack` 事件：
```json
{ "type": "message_nack", "client_message_id": "...", "code": "PERMISSION_DENIED", "message": "..." }
```
客户端收到后立即将对应 pending 消息标记为 failed，无需等 10 秒超时。

### 5.9 HTTP API 保留

保留 HTTP `POST /api/v1/channels/:channelId/messages` 作为 Agent/Plugin 发送通道。人类用户前端改为 WS 发送以支持 ack 机制。

---

## 6. Task Breakdown

### T1: Typing Indicator

| # | 任务 | 文件 | 验收标准 |
|---|------|------|---------|
| T1.1 | 服务端 typing 消息处理 + 广播 | `ws.ts` | 同频道其他客户端收到 typing 事件；发送者不收到 |
| T1.2 | `useWebSocket` 暴露 `sendWsMessage` | `useWebSocket.ts` | 组件可通过该方法发送任意 WS 消息 |
| T1.3 | `MessageInput` typing 事件发送（2s 节流） | `MessageInput.tsx` | 连续输入时每 2 秒最多一次 typing 事件 |
| T1.4 | `AppContext` typing 状态 + 过期清理 | `AppContext.tsx` | 3 秒无新 typing 自动清除 |
| T1.5 | `useWebSocket` 处理 typing 事件 | `useWebSocket.ts` | dispatch SET_TYPING |
| T1.6 | `TypingIndicator` 组件 | 新建 `TypingIndicator.tsx` | ≤3 人列名；>3 人"多人正在输入…" |
| T1.7 | 集成到 `MessageList` + CSS | `MessageList.tsx`, CSS | typing indicator 动画效果 |

### T2: Emoji Picker

| # | 任务 | 文件 | 验收标准 |
|---|------|------|---------|
| T2.1 | 安装 emoji-mart 依赖 | `package.json` | 无版本冲突 |
| T2.2 | emoji 按钮 + Picker 弹窗 | `MessageInput.tsx` | 点击弹出/关闭 |
| T2.3 | `insertEmojiAtCursor` 实现 | `MessageInput.tsx` | emoji 插入光标位置，光标定位正确 |
| T2.4 | 点击外部关闭 | `MessageInput.tsx` | 点击 picker 外自动关闭 |
| T2.5 | 样式适配（定位、主题） | CSS | 上方弹出，不溢出视口 |

### T3: Message Reactions

| # | 任务 | 文件 | 验收标准 |
|---|------|------|---------|
| T3.1 | `message_reactions` 表 migration | `db.ts` | 表 + 索引创建成功 |
| T3.2 | reaction CRUD 查询函数 | `queries.ts` | 幂等添加、正确删除、批量查询 |
| T3.3 | reaction API 端点 | 新建 `routes/reactions.ts`, `index.ts` | PUT/DELETE/GET 正确工作 |
| T3.4 | API 操作后 WS 广播 | `routes/reactions.ts` | 频道订阅者收到 reaction_update |
| T3.5 | 消息列表 API 附带 reactions | `routes/messages.ts` | 每条 message 包含 reactions 字段 |
| T3.6 | `Message` 类型扩展 + reducer | `types.ts`, `AppContext.tsx` | UPDATE_REACTIONS 正确更新 |
| T3.7 | `useWebSocket` 处理 reaction_update | `useWebSocket.ts` | 实时同步 |
| T3.8 | `ReactionBar` 组件 | 新建 `ReactionBar.tsx` | emoji + 计数 pills，当前用户高亮 |
| T3.9 | `MessageItem` 集成 reaction | `MessageItem.tsx` | hover ➕ 按钮 + 底部 ReactionBar |
| T3.10 | reaction pill hover tooltip | `ReactionBar.tsx` | 显示参与者列表 |
| T3.11 | 点击 pill 切换 reaction | `ReactionBar.tsx` | add/remove 正确切换 |

### T4: 消息已送达标记

| # | 任务 | 文件 | 验收标准 |
|---|------|------|---------|
| T0.1 | `broadcastToChannel` 加 `excludeWs` 参数 | `ws.ts` | typing、new_message 可排除发送者 |
| T0.2 | `sendWsMessage` 暴露给前端（通过 Context 或 hook） | `useWebSocket.ts`, `AppContext.tsx` | 断连时 fail fast，不 queue |
| T0.3 | `ADD_MESSAGE` reducer 加 id 去重 | `AppContext.tsx` | 已存在的 message.id 不重复添加 |
| T4.1 | 服务端 `send_message` 支持 client_message_id + message_ack + message_nack + 排除发送者广播 | `ws.ts` | ack 先于广播，nack 携带 client_message_id |
| T4.2 | `AppContext` pending 状态管理 | `AppContext.tsx`, `types.ts` | pending/ack/fail 状态转换正确 |
| T4.3 | `MessageInput` 改为 WS 发送 | `MessageInput.tsx` | 立即显示 pending 消息 |
| T4.4 | `useWebSocket` 处理 message_ack | `useWebSocket.ts` | pending 替换为 server message |
| T4.5 | 10 秒超时标记失败 | `MessageInput.tsx` / `useWebSocket.ts` | 超时显示 ❌ |
| T4.6 | `MessageItem` 发送状态图标 | `MessageItem.tsx` | ⏳ / ✓ / ❌ 正确显示 |
| T4.7 | 重试按钮功能 | `MessageItem.tsx` | 点击重试后正常发送 |
| T4.8 | 去重验证 | `AppContext.tsx` | ack + new_message 不导致重复 |

---

## 7. 实施顺序

```
T1 (Typing Indicator)
  └─► T2 (Emoji Picker)
        └─► T3 (Message Reactions)  ← 依赖 T2 的 Picker 组件
              └─► T4 (消息已送达标记)  ← 独立，放最后因涉及发送流程改造
```

T1、T2 可以并行开发（无依赖关系），但建议串行以控制 review 粒度。T3 必须在 T2 完成后开始（复用 Picker）。T4 独立于前三项，但因改动发送核心流程，放在最后降低风险。
