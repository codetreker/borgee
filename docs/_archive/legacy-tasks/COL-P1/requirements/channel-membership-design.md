# 频道成员管理 — 技术设计文档

日期：2026-04-19 | 状态：Draft | 作者：飞马（架构）
PRD：`docs/requirements/channel-membership.md`（已审批）

---

## 背景与问题

Collab 当前所有频道对所有用户可见，`channel_members` 表只用于 unread 追踪和 WS 订阅，不用于可见性控制。新用户通过 `addUserToDefaultChannel` 只加入 `#general`，但 `listChannels`（未登录）返回全部频道，`listChannelsWithUnread`（已登录）通过 `INNER JOIN channel_members` 过滤——**已有一定的成员关系基础，但缺少可见性语义**。

核心诉求：
1. 支持公开/私有频道，私有频道仅成员可见
2. 频道创建者和 Admin 可管理成员（添加/移除）
3. 新用户自动加入所有公开频道，不加入私有频道

## 目标（可验证的验收标准）

1. 创建频道时可选择 `public` / `private`
2. 私有频道仅在成员的侧边栏显示；非成员无法看到频道名称、消息、成员列表
3. `#general` 无法设为私有（UI 禁用 + API 拦截）
4. 频道可见性可切换：公开→私有保留已有成员；私有→公开所有用户自动加入
5. 新用户注册后自动加入所有公开频道
6. 新建公开频道时，所有现有用户自动加入
7. Agent 与人类用户规则一致——不自动加入私有频道
8. Admin 可看到和管理所有频道（包括私有频道）
9. 已有频道 migration 后默认 `public`，无数据丢失
10. 成员变更通过 WebSocket 实时推送，无需刷新

## 方案设计

### 数据模型变更

#### channels 表加 `visibility` 字段

```sql
ALTER TABLE channels ADD COLUMN visibility TEXT DEFAULT 'public';
```

取值：`'public'` | `'private'`，默认 `'public'`。

**Migration 策略**：在 `db.ts` 的 `initSchema` 中追加迁移逻辑（与现有 migration 模式一致）：

```typescript
// Migration: add visibility column to channels
const channelCols = db.prepare("PRAGMA table_info(channels)").all() as { name: string }[];
if (!channelCols.some((c) => c.name === 'visibility')) {
  db.exec("ALTER TABLE channels ADD COLUMN visibility TEXT DEFAULT 'public'");
}
```

所有已有频道自动获得 `visibility = 'public'`，行为不变。

#### 类型更新

`types.ts`（服务端 + 客户端）：

```typescript
export interface Channel {
  id: string;
  name: string;
  topic: string;
  type?: 'channel' | 'dm';
  visibility?: 'public' | 'private';  // 新增
  created_at: number;
  created_by: string;
}
```

### API 设计

#### 1. 创建频道 — `POST /api/v1/channels`

**变更**：Body 新增 `visibility` 字段。创建公开频道时自动添加所有用户为成员。

Request:
```json
{
  "name": "secret-project",
  "topic": "机密项目讨论",
  "visibility": "private",
  "member_ids": ["user-1", "user-2"]
}
```

Response (201):
```json
{
  "channel": {
    "id": "uuid",
    "name": "secret-project",
    "topic": "机密项目讨论",
    "type": "channel",
    "visibility": "private",
    "created_at": 1713567600000,
    "created_by": "admin-jianjun"
  }
}
```

逻辑：
- `visibility` 可选，默认 `'public'`
- 值必须为 `'public'` 或 `'private'`，否则 400
- 公开频道：创建后自动将所有用户（`SELECT id FROM users`）加入 `channel_members`
- 私有频道：只加创建者 + `member_ids` 中指定的用户
- 创建者始终加入

#### 2. 更新频道 — `PUT /api/v1/channels/:channelId`

**变更**：Body 新增 `visibility` 字段，支持可见性切换。

Request:
```json
{
  "visibility": "private"
}
```

Response (200):
```json
{
  "channel": { "...": "更新后的频道对象" }
}
```

逻辑：
- `#general` 不可设为 `private` → 403 `Cannot make #general private`
- 公开→私有：保留已有 `channel_members`，不做变更
- 私有→公开：将所有用户添加到 `channel_members`（`INSERT OR IGNORE`）
- 可见性变更后广播 `visibility_changed` 事件
- 权限：频道创建者或 Admin 可操作

新增事件广播：
```json
{
  "type": "visibility_changed",
  "channel_id": "uuid",
  "visibility": "private"
}
```

并新增 EventKind：`'visibility_changed'`。

#### 3. 列出频道 — `GET /api/v1/channels`

**变更**：过滤私有频道的可见性。

当前逻辑（已登录 `listChannelsWithUnread`）通过 `INNER JOIN channel_members` 过滤，**已经只返回用户加入的频道**。只需要确认：
- Admin 用户：额外返回所有频道（包括未加入的私有频道），并标记 `is_member: false`
- 未登录 `listChannels`：只返回 `visibility = 'public'` 的频道

修改 `listChannels` 查询：
```sql
SELECT c.*, ...
FROM channels c
LEFT JOIN channel_members cm ON cm.channel_id = c.id
WHERE (c.type = 'channel' OR c.type IS NULL)
  AND (c.visibility = 'public' OR c.visibility IS NULL)  -- 新增
GROUP BY c.id
ORDER BY ...
```

Admin 列表新增查询 `listAllChannelsForAdmin`：
```sql
SELECT c.*,
       COUNT(cm.user_id) AS member_count,
       ...
       EXISTS(SELECT 1 FROM channel_members WHERE channel_id = c.id AND user_id = ?) AS is_member
FROM channels c
LEFT JOIN channel_members cm ON cm.channel_id = c.id
WHERE (c.type = 'channel' OR c.type IS NULL)
GROUP BY c.id
ORDER BY ...
```

路由层逻辑：
```typescript
if (request.currentUser?.role === 'admin') {
  // 返回所有频道，包含 is_member 标记
  channels = Q.listAllChannelsForAdmin(db, request.currentUser.id);
} else if (request.currentUser) {
  // 只返回用户是成员的频道（现有逻辑不变）
  channels = Q.listChannelsWithUnread(db, request.currentUser.id);
} else {
  // 未登录：只返回公开频道
  channels = Q.listChannels(db);
}
```

#### 4. 频道详情 — `GET /api/v1/channels/:channelId`

**变更**：私有频道访问控制。

逻辑：
- 公开频道：任何已认证用户可访问
- 私有频道：只有成员或 Admin 可访问，否则 404（不泄漏频道存在）

```typescript
const channel = Q.getChannel(db, channelId);
if (!channel) return reply.status(404).send({ error: 'Channel not found' });

if (channel.visibility === 'private') {
  const userId = request.currentUser?.id;
  const user = userId ? Q.getUserById(db, userId) : null;
  if (!userId || (!Q.isChannelMember(db, channelId, userId) && user?.role !== 'admin')) {
    return reply.status(404).send({ error: 'Channel not found' });
  }
}
```

#### 5. 获取消息 — `GET /api/v1/channels/:channelId/messages`

**变更**：私有频道访问控制（同上模式）。

在现有路由中，读取消息前检查：
- 私有频道 → 必须是成员或 Admin，否则 404

#### 6. 搜索消息 — `GET /api/v1/channels/:channelId/messages/search`

**变更**：同消息端点，加相同的私有频道访问控制。

#### 7. 频道成员列表 — `GET /api/v1/channels/:channelId/members`

**变更**：私有频道访问控制。

- 私有频道：仅成员或 Admin 可查看，否则 404

#### 8. 添加成员 — `POST /api/v1/channels/:channelId/members`

**无结构性变更**。现有权限逻辑（创建者或 Admin）已满足需求。

新增：添加成员后，向被添加用户广播频道信息（以便侧边栏实时更新）。

需要新增一个全局广播机制（或向特定用户的所有 WS 连接发送）：
```json
{
  "type": "channel_added",
  "channel": { "完整频道对象" }
}
```

#### 9. 移除成员 — `DELETE /api/v1/channels/:channelId/members/:userId`

**无结构性变更**。现有逻辑已正确。

新增：向被移除用户广播：
```json
{
  "type": "channel_removed",
  "channel_id": "uuid"
}
```

#### 10. 加入公开频道（自助） — `POST /api/v1/channels/:channelId/join`（新增）

当前 `POST /api/v1/channels/:channelId/members` 需要创建者/Admin 权限。公开频道应允许用户自行加入。

Request:
```
POST /api/v1/channels/:channelId/join
```
（无 Body，使用当前登录用户）

Response (200):
```json
{ "ok": true }
```

逻辑：
- 频道必须是公开的 → 否则 403
- DM 频道不可加入 → 403
- 用户已是成员 → 200（幂等）
- 加入后广播 `user_joined` 事件

#### 11. 离开频道（自助） — `POST /api/v1/channels/:channelId/leave`（新增）

Request:
```
POST /api/v1/channels/:channelId/leave
```

Response (200):
```json
{ "ok": true }
```

逻辑：
- `#general` 不可离开 → 403
- DM 频道不可离开 → 403
- 私有频道也可离开（离开后看不到频道）
- 广播 `user_left` 事件

#### 12. 新用户注册 — 修改 `addUserToDefaultChannel`

重命名为 `addUserToPublicChannels`，改为将新用户加入**所有公开频道**：

```typescript
export function addUserToPublicChannels(db: Database.Database, userId: string): void {
  const now = Date.now();
  const publicChannels = db.prepare(
    "SELECT id FROM channels WHERE (type = 'channel' OR type IS NULL) AND (visibility = 'public' OR visibility IS NULL)"
  ).all() as { id: string }[];

  const stmt = db.prepare(
    'INSERT OR IGNORE INTO channel_members (channel_id, user_id, joined_at, last_read_at) VALUES (?, ?, ?, ?)'
  );
  for (const ch of publicChannels) {
    stmt.run(ch.id, userId, now, now);
  }
}
```

在 `admin.ts` 的 create user 路由和 `seed.ts` 中替换调用。

### 前端变更

#### 1. 类型更新（`types.ts`）

```typescript
export interface Channel {
  // ...existing fields...
  visibility?: 'public' | 'private';  // 新增
}
```

#### 2. 创建频道 UI（`Sidebar.tsx`）

在创建频道表单中新增可见性选择：

```tsx
<div className="visibility-toggle">
  <label>
    <input
      type="radio"
      name="visibility"
      value="public"
      checked={visibility === 'public'}
      onChange={() => setVisibility('public')}
    />
    🌐 公开 — 所有人可见
  </label>
  <label>
    <input
      type="radio"
      name="visibility"
      value="private"
      checked={visibility === 'private'}
      onChange={() => setVisibility('private')}
    />
    🔒 私有 — 仅邀请成员可见
  </label>
</div>
```

当选择 `private` 时，显示成员选择列表（可选，不选则只有创建者自己）。
`createChannel` API 调用传递 `visibility` 参数。

#### 3. 侧边栏频道列表（`Sidebar.tsx`）

- 私有频道显示 🔒 图标代替 `#`：
```tsx
<span className="channel-hash">
  {channel.visibility === 'private' ? '🔒' : '#'}
</span>
```

#### 4. 频道设置/可见性切换（`ChannelMembersModal.tsx` 扩展）

在现有成员管理弹窗顶部，增加频道可见性设置区域：

- 显示当前可见性状态
- 频道创建者或 Admin 可切换
- `#general` 禁用切换按钮
- 切换时弹出确认对话框：
  - 公开→私有：「将频道设为私有？已有成员将保留，新用户不会自动加入。」
  - 私有→公开：「将频道设为公开？所有用户将自动加入此频道。」

#### 5. 频道头部（`ChannelView.tsx`）

- 私有频道标题旁显示 🔒 图标

#### 6. API 客户端（`api.ts`）

新增/修改：
```typescript
export async function createChannel(
  name: string,
  topic?: string,
  memberIds?: string[],
  visibility?: 'public' | 'private',  // 新增
): Promise<Channel> { ... }

export async function updateChannelVisibility(
  channelId: string,
  visibility: 'public' | 'private',
): Promise<Channel> { ... }

export async function joinChannel(channelId: string): Promise<void> { ... }

export async function leaveChannel(channelId: string): Promise<void> { ... }
```

#### 7. WebSocket 事件处理（`useWebSocket.ts`）

新增事件处理：
- `channel_added`：将频道加入侧边栏
- `channel_removed`：从侧边栏移除频道
- `visibility_changed`：更新频道可见性状态

#### 8. AppContext 状态管理

新增 Action 类型：
```typescript
| { type: 'REMOVE_CHANNEL'; channelId: string }
| { type: 'UPDATE_CHANNEL'; channelId: string; updates: Partial<Channel> }
```

### 权限逻辑

#### 读写权限矩阵

| 操作 | 公开频道 | 私有频道（成员） | 私有频道（非成员） | 私有频道（Admin 非成员） |
|------|----------|-----------------|-------------------|------------------------|
| 看到频道存在 | ✅ 所有人 | ✅ | ❌ | ✅ |
| 读消息 | ✅ 成员 | ✅ | ❌ | ✅ |
| 发消息 | ✅ 成员 | ✅ | ❌ | ❌（需先加入） |
| 自行加入 | ✅ | N/A | ❌ | ❌（用管理权加入） |
| 自行离开 | ✅（#general 除外） | ✅ | N/A | N/A |
| 添加成员 | 创建者/Admin | 创建者/Admin | N/A | ✅ |
| 移除成员 | 创建者/Admin | 创建者/Admin | N/A | ✅ |
| 改可见性 | 创建者/Admin | 创建者/Admin | N/A | ✅ |
| 改名/话题 | 创建者/Admin | 创建者/Admin | N/A | ✅ |

**注意**：Admin 可以看到私有频道但不能直接发消息（需要先把自己加为成员），这避免了 Admin 无意间在私有频道发言。

#### 访问控制实现位置

抽取一个 helper 函数，在所有需要的路由中调用：

```typescript
// queries.ts 新增
export function canAccessChannel(
  db: Database.Database,
  channelId: string,
  userId: string,
): boolean {
  const channel = getChannel(db, channelId);
  if (!channel) return false;
  if (channel.visibility !== 'private') return true;
  if (isChannelMember(db, channelId, userId)) return true;
  const user = getUserById(db, userId);
  return user?.role === 'admin';
}
```

### WebSocket 变更

#### 向特定用户广播

当前 `broadcastToChannel` 只向订阅了特定频道的客户端广播。成员添加/移除需要向**特定用户的所有连接**广播（因为用户可能还没订阅该频道）。

新增 `broadcastToUser` 函数（`ws.ts`）：

```typescript
export function broadcastToUser(userId: string, payload: unknown): void {
  const data = JSON.stringify(payload);
  for (const client of clients.values()) {
    if (client.userId === userId && client.ws.readyState === 1) {
      client.ws.send(data);
    }
  }
}
```

#### WS 订阅检查增强

现有 `subscribe` 消息处理已检查 `isChannelMember`。对于 Admin 访问私有频道，需要额外允许：

```typescript
case 'subscribe': {
  // ...existing checks...
  const isMember = Q.isChannelMember(db, msg.channel_id, userId);
  const isAdmin = user.role === 'admin';
  if (!isMember && !isAdmin) {
    socket.send(JSON.stringify({ type: 'error', message: 'Not a member of this channel' }));
    break;
  }
  // ...
}
```

### Plugin 影响（Poll 端点）

`POST /api/v1/poll` **无需修改**。

原因：Poll 路由已通过 `channel_members` 表过滤用户可见的频道：
```typescript
const userChannelIds = db.prepare(
  "SELECT channel_id FROM channel_members WHERE user_id = ?"
).all(user.id);
```

只要 `channel_members` 表正确维护（私有频道只有成员条目），Poll 自动只推送用户有权限的频道事件。

## 备选方案

### 方案 B：用 `type` 字段复用（放弃）

将 `type` 从 `'channel' | 'dm'` 扩展为 `'public' | 'private' | 'dm'`。

**放弃原因**：`type` 表示频道的结构性类别（普通频道 vs DM），`visibility` 表示访问控制维度——两个正交概念不应合并。且现有代码大量使用 `type === 'dm'` 过滤，混入 public/private 会引入大量改动和 bug 风险。

### 方案 C：独立权限表（放弃）

新建 `channel_permissions` 表，支持细粒度权限（read/write/manage）。

**放弃原因**：v1 不需要角色分级（PRD 明确排除），引入权限表增加复杂度，且与 v2 的 owner/moderator/member 分级可能冲突。当前 `visibility` 字段 + `channel_members` 已足够。

## 测试策略

### 后端单测

1. **Migration 测试**：验证 `visibility` 列正确添加，已有频道默认 `'public'`
2. **创建频道**：
   - 创建 public 频道 → 所有用户自动加入
   - 创建 private 频道 → 只有创建者和指定成员加入
   - 不传 visibility → 默认 public
3. **可见性切换**：
   - public → private → 成员不变
   - private → public → 所有用户加入
   - #general 设为 private → 403
4. **访问控制**：
   - 非成员访问私有频道详情 → 404
   - 非成员读取私有频道消息 → 404
   - 非成员向私有频道发消息 → 403
   - Admin 非成员可访问私有频道
5. **成员管理**：
   - 添加成员到私有频道 → 被添加者可见
   - 移除成员 → 被移除者不可见
   - 非创建者非 Admin 添加成员 → 403
6. **新用户**：
   - 新用户自动加入所有 public 频道
   - 新用户不加入任何 private 频道
7. **自助加入/离开**：
   - 用户自行加入 public 频道 → 200
   - 用户自行加入 private 频道 → 403
   - 用户离开 #general → 403
8. **Poll 端点**：
   - 私有频道事件不推送给非成员

### 前端测试（手动）

1. 创建 public 频道 → 所有用户侧边栏可见
2. 创建 private 频道 → 仅成员侧边栏可见，🔒 图标正确
3. 切换可见性 → 确认弹窗 → 侧边栏实时更新
4. 被添加到私有频道 → 侧边栏实时出现新频道
5. 被移除 → 侧边栏实时消失
6. Admin 登录 → 可看到所有频道（包括未加入的私有频道）

## Task Breakdown

按依赖顺序排列，每个任务可独立提交和 review。

| # | 任务 | 依赖 | 估时 |
|---|------|------|------|
| T1 | **数据模型**：`db.ts` 添加 `visibility` 列 migration；`types.ts` 更新 Channel 接口（server + client） | 无 | 30min |
| T2 | **查询层**：`queries.ts` 新增 `canAccessChannel`、`addUserToPublicChannels`（替换 `addUserToDefaultChannel`）、`listAllChannelsForAdmin`；修改 `listChannels` 过滤 public；修改 `createChannel` 接受 visibility 参数 | T1 | 1h |
| T3 | **频道路由**：`routes/channels.ts` 修改创建/更新/详情/成员路由，增加 visibility 参数和访问控制；新增 `join` 和 `leave` 端点 | T2 | 1.5h |
| T4 | **消息路由**：`routes/messages.ts` 添加私有频道访问控制 | T2 | 30min |
| T5 | **WebSocket**：`ws.ts` 新增 `broadcastToUser`；修改 subscribe 支持 Admin 访问私有频道；处理新事件类型 | T2 | 45min |
| T6 | **Seed & Admin**：`seed.ts` 和 `routes/admin.ts` 替换 `addUserToDefaultChannel` 调用为 `addUserToPublicChannels` | T2 | 20min |
| T7 | **前端类型 + API**：客户端 `types.ts` 更新；`api.ts` 新增/修改 API 方法 | T1 | 30min |
| T8 | **前端创建频道 UI**：`Sidebar.tsx` 创建频道表单增加 visibility 选择 | T7 | 45min |
| T9 | **前端频道列表**：侧边栏频道项显示 🔒 图标；频道头部显示锁图标 | T7 | 20min |
| T10 | **前端成员管理**：`ChannelMembersModal.tsx` 增加可见性切换 UI + 确认对话框 | T7 | 45min |
| T11 | **前端 WebSocket 事件**：`useWebSocket.ts` 处理 `channel_added`/`channel_removed`/`visibility_changed`；AppContext 新增 reducer | T7 | 45min |
| T12 | **后端测试**：为 T2-T6 的逻辑编写单测 | T6 | 1.5h |
| T13 | **E2E 验收**：浏览器打开测试所有场景 | T11 | 1h |

**总估时：~9.5h**

建议分 3 个 PR：
- **PR1**（T1-T6）：后端全部变更
- **PR2**（T7-T11）：前端全部变更
- **PR3**（T12-T13）：测试和验收

## 风险与开放问题

| # | 项目 | 影响 | 缓解 |
|---|------|------|------|
| 1 | 公开频道创建时自动加用户数量大 | 用户量大时 INSERT 可能慢 | SQLite WAL + 批量 INSERT，当前团队规模（<20 人）无风险 |
| 2 | 私有→公开切换自动加所有用户 | 同上 | 同上，加 transaction 包裹 |
| 3 | Admin 能看到私有频道但不是成员 | UI 需明确区分"可管理"和"已加入" | 返回 `is_member` 标记，前端区分展示 |
| 4 | 频道创建者离开私有频道后无人可管理 | v1 依赖 Admin 兜底 | Admin 可管理所有频道，v2 引入 channel owner 概念 |
| 5 | WebSocket `broadcastToUser` 需遍历所有连接 | 连接数大时有性能影响 | 当前规模无风险；可后续加 userId→connections 索引 |

---

*本文档待架构 Review 后交付开发。*
