# COL-B23: 聊天记录分页加载 — 技术设计

日期：2026-04-22 | 状态：Draft

## 1. 概述

进入频道只加载最近 100 条消息，向上滚动时增量加载历史。API 支持 cursor 分页。

## 2. API 改造

### 2.1 GET /api/v1/channels/:channelId/messages

现有 API 返回全部消息。改为支持分页：

**参数**：
- `limit`：每页条数，默认 100，最大 200
- `before`：cursor，返回该 ID 之前的消息

**响应**：
```json
{
  "messages": [...],
  "hasMore": true
}
```

`hasMore` = 实际返回数 === limit（还有更多）。

### 2.2 SQL

```sql
SELECT * FROM messages
WHERE channel_id = ?
  AND deleted_at IS NULL
  AND (? IS NULL OR id < ?)  -- before cursor
ORDER BY id DESC
LIMIT ?
```

返回后前端反转顺序（DESC 查询保证性能，前端 reverse 显示）。

## 3. 前端改造

### 3.1 状态

```typescript
interface MessageState {
  messages: Message[];
  hasMore: boolean;
  loadingMore: boolean;
  initialLoaded: boolean;
}
```

### 3.2 初始加载

进入频道 → `GET /messages?limit=100` → 设 `initialLoaded=true` → 滚动到底部。

### 3.3 向上滚动加载

监听 `scrollTop` 接近 0：

```typescript
const handleScroll = () => {
  if (scrollRef.scrollTop < 100 && hasMore && !loadingMore) {
    loadMore();
  }
};

async function loadMore() {
  setLoadingMore(true);
  const oldestId = messages[0].id;
  const res = await api.getMessages(channelId, { limit: 50, before: oldestId });
  // 保持滚动位置
  const prevHeight = scrollRef.scrollHeight;
  setMessages(prev => [...res.messages, ...prev]);
  setHasMore(res.hasMore);
  // 恢复滚动位置
  nextTick(() => {
    scrollRef.scrollTop = scrollRef.scrollHeight - prevHeight;
  });
  setLoadingMore(false);
}
```

**关键**：加载后恢复滚动位置，用 `scrollHeight` 差值计算。

### 3.4 Loading 指示器

滚动到顶部加载时，显示 spinner：

```tsx
{loadingMore && <div className="loading-more">加载中...</div>}
{!hasMore && messages.length > 0 && <div className="no-more">已到最早消息</div>}
```

### 3.5 新消息浮动按钮

如果用户滚动位置不在底部，新消息来时显示浮动按钮：

```typescript
const isAtBottom = scrollRef.scrollHeight - scrollRef.scrollTop - scrollRef.clientHeight < 50;

if (!isAtBottom && newMessage) {
  setShowNewMessageButton(true);
}
```

点击按钮 → `scrollToBottom()` + `setShowNewMessageButton(false)`。

## 4. 改动文件

### Server
| 文件 | 改动 |
|------|------|
| `src/routes/messages.ts` | GET 加 limit + before 参数 |
| `src/queries.ts` | 分页查询 |

### Client
| 文件 | 改动 |
|------|------|
| `components/ChannelView.tsx` | 分页状态 + 滚动监听 + loadMore |
| `components/NewMessageButton.tsx` | 新建：浮动按钮 |
| `lib/api.ts` | getMessages 加分页参数 |
| `index.css` | loading spinner + 浮动按钮样式 |

## 5. Task Breakdown

### T1: API 分页
- messages route 加 limit + before
- 分页查询 + hasMore
- 测试

### T2: 前端分页加载
- 初始 100 条
- 向上滚动触发 loadMore
- 滚动位置保持
- Loading 指示器 + "已到最早"

### T3: 新消息浮动按钮
- 检测不在底部
- 浮动按钮 UI
- 点击滚动到底

## 6. 验收标准

- [ ] 进入频道只加载 100 条
- [ ] 向上滚动加载更多，位置不跳
- [ ] 没有更多时显示"已到最早"
- [ ] 新消息浮动按钮正常
- [ ] 加载速度 < 1s
