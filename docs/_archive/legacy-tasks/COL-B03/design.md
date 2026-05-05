# COL-B03: 公开频道预览 + 加入 — 技术设计

日期：2026-04-21 | 状态：Draft

## 1. 背景

PRD：`docs/tasks/COL-B03/prd.md`。公开频道对未加入用户不可见，需要支持预览 + 一键加入。

## 2. API 变更

### 2.1 频道列表 API 调整

现有 `GET /api/v1/channels` 只返回用户已加入的频道。需要同时返回未加入的公开频道。

**方案**：返回所有公开频道 + 用户已加入的私有频道，附带 `is_member: boolean` 字段（已有）。

修改 `queries.ts` 的 `getChannelsForUser`：
```sql
-- 已加入的频道（含私有）
SELECT c.*, 1 AS is_member FROM channels c
JOIN channel_members cm ON c.id = cm.channel_id
WHERE cm.user_id = ? AND c.deleted_at IS NULL

UNION

-- 未加入的公开频道
SELECT c.*, 0 AS is_member FROM channels c
WHERE c.visibility = 'public' AND c.deleted_at IS NULL
AND c.id NOT IN (SELECT channel_id FROM channel_members WHERE user_id = ?)
```

### 2.2 频道预览 API（新建）

```
GET /api/v1/channels/:channelId/preview
Response: { messages: Message[], channel: Channel }

权限：
- 公开频道：任何登录用户可预览
- 私有频道：404
```

返回最近 24h 的消息（限 50 条）。

### 2.3 加入频道 API

已有：`POST /api/v1/channels/:channelId/members` body: `{ userId }`

v1 公开频道自动通过——用户可以把自己加入公开频道。需确认后端不阻止"用户自己加入公开频道"。

## 3. 前端设计

### 3.1 侧边栏

- 已加入频道：正常显示（当前样式）
- 未加入的公开频道：灰色/半透明 + 小标签"预览"
- 排序：已加入在上，未加入在下（分组）

### 3.2 预览模式

点击未加入的公开频道：
- 加载 `GET /channels/:id/preview`（24h 消息）
- 消息列表只读显示
- 输入框替换为"加入频道"按钮（居中，primary 样式）
- 顶部 banner："你正在预览 #频道名"

### 3.3 加入流程

1. 点击"加入频道"
2. 调用 `POST /channels/:id/members` body: `{ userId: currentUser.id }`
3. 成功 → 刷新频道列表 → 频道变为已加入 → 输入框恢复 → 加载完整消息历史
4. WS subscribe 该频道

### 3.4 AppContext 变更

- 频道列表已有 `is_member` 字段
- 新增 `JOIN_CHANNEL` action：将频道的 `is_member` 设为 true
- `ChannelView` 根据 `is_member` 决定显示输入框还是"加入频道"按钮

## 4. Task Breakdown

### T1: 后端 — 频道列表返回未加入的公开频道

**改动文件**：`queries.ts`、`routes/channels.ts`

**内容**：修改 getChannelsForUser，UNION 未加入的公开频道

**验收标准**：
- [ ] API 返回已加入 + 未加入的公开频道
- [ ] is_member 正确标记
- [ ] 私有频道不返回给非成员

### T2: 后端 — 频道预览 API

**改动文件**：`routes/channels.ts`、`queries.ts`

**内容**：GET /channels/:id/preview，返回 24h 消息

**验收标准**：
- [ ] 公开频道可预览
- [ ] 私有频道返回 404
- [ ] 最多返回 50 条 24h 内消息

### T3: 后端 — 用户自加入公开频道

**改动文件**：`routes/channels.ts`

**内容**：确认 POST members API 允许用户自己加入公开频道

**验收标准**：
- [ ] 用户可以把自己加入公开频道
- [ ] 私有频道自加入被拒

### T4: 前端 — 侧边栏显示未加入频道

**改动文件**：侧边栏组件、`index.css`

**内容**：未加入公开频道灰色显示 + "预览"标签

**验收标准**：
- [ ] 未加入频道可见且样式区分
- [ ] 已加入和未加入分组

### T5: 前端 — 预览模式 + 加入按钮

**改动文件**：`ChannelView.tsx`、`MessageInput.tsx`、`AppContext.tsx`

**内容**：
1. 非成员点击频道 → 加载预览 API
2. 只读消息列表 + "加入频道"按钮
3. 加入后刷新 + WS subscribe

**验收标准**：
- [ ] 预览模式显示 24h 消息
- [ ] 无法在预览模式发消息
- [ ] 加入后可正常使用

## 5. 不在范围

- 审批流程
- 频道搜索/发现页
