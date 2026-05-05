# COL-B23: 聊天记录分页加载 — Task Breakdown

日期：2026-04-22

> **现状**：API 已支持 `before`/`limit`/`after` cursor 分页，前端已有 `hasMore` 状态、`loadOlderMessages` action、滚动检测和位置保持逻辑。缺少：初始加载 limit 调大到 100、"已到最早消息"提示、新消息浮动按钮。

---

## Task 1: API 分页完善

**目标**：调整默认 limit 至 100（PRD 要求），最大 200；确保 `hasMore` 语义正确。

### 改动文件

| 文件 | 改动内容 | 预估行数 |
|------|----------|----------|
| `packages/server/src/routes/messages.ts:19` | 默认 limit 50→100，max 100→200 | ~2 行 |
| `packages/client/src/context/AppContext.tsx:451` | `fetchMessages` 初始 limit 50→100 | ~1 行 |

### 验证方式
- `curl /api/v1/channels/:id/messages` 默认返回 ≤100 条
- `curl ...?limit=200` 返回 ≤200 条
- `curl ...?limit=999` 被截断为 200
- `has_more` 在消息数 > limit 时为 true，否则 false

### 依赖
- 无，可独立开发

---

## Task 2: 前端滚动加载完善

**目标**：加载更早消息时显示 spinner；没有更多消息时显示"已到最早消息"提示；确保滚动位置不跳。

### 改动文件

| 文件 | 改动内容 | 预估行数 |
|------|----------|----------|
| `packages/client/src/components/MessageList.tsx:132-145` | `!hasMore && messages.length > 0` 时渲染"已到最早消息"提示 | ~5 行 |
| `packages/client/src/index.css` | `.no-more-messages` 样式（居中灰字） | ~8 行 |

### 验证方式
- 进入有 >100 条消息的频道，向上滚动：显示 spinner → 加载完 → 位置不跳
- 滚动到最顶端所有消息加载完毕后，显示"已到最早消息"
- 进入消息 <100 条的频道，不显示 load-more 按钮，顶部直接显示"已到最早消息"

### 依赖
- Task 1（初始 limit 改为 100 后才能正确触发 hasMore）

---

## Task 3: 新消息浮动按钮

**目标**：用户在看历史消息时，有新消息到达→显示"↓ 新消息"浮动按钮；点击滚动到底部。

### 改动文件

| 文件 | 改动内容 | 预估行数 |
|------|----------|----------|
| `packages/client/src/components/MessageList.tsx` | 新增 `showNewMsgBtn` state；在 `allMessages.length` 变化时判断 `isAtBottom`；渲染浮动按钮 | ~25 行 |
| `packages/client/src/index.css` | `.new-message-btn` 浮动按钮样式（sticky bottom、动画） | ~20 行 |

### 验证方式
- 在频道中往上滚动 >50px → 其他用户发新消息 → 出现浮动按钮
- 点击按钮 → 平滑滚动到最新消息 → 按钮消失
- 用户本来就在底部 → 新消息直接自动滚动，不出现按钮
- 切换频道后按钮状态重置

### 依赖
- Task 2（共享 `isAtBottom` 逻辑和 MessageList 组件）

---

## 总结

| Task | 预估改动 | 依赖 |
|------|----------|------|
| T1: API 分页完善 | ~3 行 | 无 |
| T2: 前端"已到最早"提示 | ~13 行 | T1 |
| T3: 新消息浮动按钮 | ~45 行 | T2 |
| **合计** | **~61 行** | |
