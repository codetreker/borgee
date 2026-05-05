# COL-B26 / B27: 频道拖动排序 + 自定义分组 — 技术设计

日期：2026-04-25 | 状态：Final

## 0. 权限模型

- **用户（User）**：拥有 channel / group / agent，对自己创建的资源有完整操作权限
- **管理员（Admin）**：系统管理员，管理整个系统；同时也是普通用户，可使用所有用户功能
- Admin **不参与**业务层权限判断——API 权限校验只看 `created_by`，不检查 admin 角色

## 1. 背景与目标

Collab sidebar 频道列表当前按最近活跃时间排序，不支持手动排序和分组。本设计实现：

- **B26**：频道 owner 可拖动排序，排序全局生效（所有用户看到相同顺序）
- **B27**：频道 owner 可创建分组（Category），将自己的频道归入分组

核心约束：排序和分组是**频道属性**，由频道 owner（created_by）管理，非个人偏好。

## 2. UI 设计稿

→ [频道排序与分组 UI 线框图](../../ui/channel-sort-groups.md)

## 3. 设计决策

| # | 决策 | 选项 | 结论 | 理由 |
|---|------|------|------|------|
| D1 | 排序存储方式 | (a) integer position (b) fractional indexing (c) linked list | **(b) fractional indexing** | 拖拽只需更新 1 行，无需批量 reindex；用 lexorank 字符串实现 |
| D2 | 分组归属 | (a) channels 加 group_id FK (b) 中间表 | **(a) channels 加 group_id** | 频道最多属于 1 个分组，1:N 关系用 FK 最简单 |
| D3 | 分组排序 | 复用 fractional indexing | 与频道排序同机制 | 一致性 |
| D4 | 前端拖拽库 | (a) dnd-kit (b) react-beautiful-dnd (c) 原生 HTML5 DnD | **(a) @dnd-kit/core** | 活跃维护、支持 touch、支持多列表拖拽、tree-friendly |
| D5 | 权限模型 | 复用 created_by 字段判断 owner | 不新增权限类型 | owner = created_by，简单清晰 |
| D6 | 折叠状态存储 | localStorage per-user | 不入库 | 唯一的个人偏好，无需同步 |
| D7 | 新频道默认位置 | 追加到所属分组末尾（lexorank = 该分组最大 position 之后） | — | 统一规则：有 position 就按 position 排序，不回退到活跃时间 |
| D8 | 未分组频道展示 | group_id = NULL，显示在所有分组上方 | — | 符合 PRD |

## 4. DB Schema

### 4.1 channels 表变更 — Phase 1（B26 排序）

```sql
-- Phase 1: 仅 position 列
ALTER TABLE channels ADD COLUMN position TEXT DEFAULT '0|aaaaaa';
CREATE INDEX idx_channels_position ON channels(position);
```

### 4.1.1 channels 表变更 — Phase 3（B27 分组）

```sql
-- Phase 3: 加 group_id FK（依赖 channel_groups 表先建好）
ALTER TABLE channels ADD COLUMN group_id TEXT REFERENCES channel_groups(id) ON DELETE SET NULL;
CREATE INDEX idx_channels_group ON channels(group_id);
```

- `group_id`：NULL = 未分组；SET NULL on group delete → 频道自动回到未分组
- `position`：lexorank 字符串，字典序 = 显示顺序

### 4.2 channel_groups 新表

```sql
CREATE TABLE IF NOT EXISTS channel_groups (
  id          TEXT PRIMARY KEY,
  name        TEXT NOT NULL,
  position    TEXT NOT NULL,          -- lexorank，分组间排序
  created_by  TEXT NOT NULL REFERENCES users(id),
  created_at  INTEGER NOT NULL
);

CREATE INDEX idx_channel_groups_position ON channel_groups(position);
```

### 4.3 Lexorank 策略

使用 `bucket|rank` 格式（如 `0|aaaaaa`）。插入两项之间时取中间值。当间距不足时触发局部 rebalance（同一分组内 ≤50 项重排）。初始种子：首个频道 `0|hzzzzz`（中位），后续插入取中值。

## 5. API 设计

### 5.1 频道排序

```
PUT /api/v1/channels/reorder
```

**请求：**
```json
{
  "channel_id": "ch_abc",
  "after_id": "ch_def",
  "group_id": "grp_123"
}
```

- `after_id`：插入到该频道之后；`null` = 插入到目标分组/未分组区域的最前面
- `group_id`：目标分组；`null` = 未分组

**响应 200：**
```json
{
  "channel": { "id": "ch_abc", "position": "0|hzzzzm", "group_id": "grp_123" }
}
```

**权限：** 当前用户必须是 `channel_id` 的 created_by（owner）。否则 403。

**校验：**
- `after_id` 非空时必须存在，否则返回 404
- `group_id` 非空时必须存在，否则返回 404

**逻辑：**
1. 验证 channel 存在且 created_by = currentUser.id
2. 校验 after_id / group_id 存在性
3. `BEGIN IMMEDIATE` 事务（read→compute→write 原子化，防止并发 lexorank 冲突）
4. 查找 after_id 的 position 和下一项的 position
5. 计算中间 lexorank
6. UPDATE channels SET position = ?, group_id = ? WHERE id = ?
7. COMMIT
8. 广播 WS 事件

### 5.2 分组 CRUD

#### 创建分组

```
POST /api/v1/channel-groups
```

**请求：**
```json
{ "name": "工程" }
```

**校验：** name trim 后非空且 ≤ 50 字符，否则 400。
```json
{
  "group": { "id": "grp_abc", "name": "工程", "position": "0|zzzzzz", "created_by": "user_1", "created_at": 1714012800000 }
}
```

**权限：** 已认证用户（任何 channel owner 都可以创建分组）。

#### 更新分组（重命名）

```
PUT /api/v1/channel-groups/:groupId
```

**请求：**
```json
{ "name": "工程团队" }
```

**响应 200：**
```json
{ "group": { "id": "grp_abc", "name": "工程团队", ... } }
```

**权限：** 分组 created_by = currentUser.id。否则 403。

#### 分组排序

```
PUT /api/v1/channel-groups/reorder
```

**请求：**
```json
{
  "group_id": "grp_abc",
  "after_id": "grp_def"
}
```

- `after_id`：null = 移到所有分组最前面

**响应 200：**
```json
{ "group": { "id": "grp_abc", "position": "0|hzzzzm", ... } }
```

**权限：** 分组 created_by = currentUser.id。否则 403。每个用户只能拖动自己创建的分组；所有用户看到的 sidebar 分组按 position 排序（只读视图）。

> **已知限制（v1）**：分组排序权限绑定创建者，无法实现"全局统一排序"。如果用户 A 和 B 各创建了分组，A 无法调整 B 的分组顺序。v1 接受此限制。

```
DELETE /api/v1/channel-groups/:groupId
```

**响应 200：**
```json
{ "ok": true, "ungrouped_channel_ids": ["ch_1", "ch_2"] }
```

**权限：** 分组 created_by = currentUser.id。否则 403。

**逻辑：**
1. UPDATE channels SET group_id = NULL WHERE group_id = ?
2. DELETE FROM channel_groups WHERE id = ?
3. 广播 WS 事件

### 5.3 频道列表 API 变更

现有 `GET /api/v1/channels` 响应新增字段：

```json
{
  "channels": [
    { "id": "ch_abc", "name": "general", "position": "0|aaaaaa", "group_id": null, ... }
  ],
  "groups": [
    { "id": "grp_abc", "name": "工程", "position": "0|hzzzzz", "created_by": "user_1" }
  ]
}
```

前端按 group_id 分桶、按 position 字典序排列。

## 6. WS 事件

### 6.1 频道排序变更

```json
{
  "type": "channels_reordered",
  "channel_id": "ch_abc",
  "position": "0|hzzzzm",
  "group_id": "grp_123"
}
```

广播给所有在线用户（broadcastToAll）。

### 6.2 分组事件

```json
{ "type": "group_created", "group": { "id": "grp_abc", "name": "工程", "position": "0|zzzzzz", "created_by": "user_1" } }
```

```json
{ "type": "group_updated", "group": { "id": "grp_abc", "name": "工程团队", ... } }
```

```json
{ "type": "group_reordered", "group_id": "grp_abc", "position": "0|hzzzzm" }
```

```json
{ "type": "group_deleted", "group_id": "grp_abc", "ungrouped_channel_ids": ["ch_1", "ch_2"] }
```

所有分组事件均 broadcastToAll（排序/分组对所有用户可见）。

## 7. 前端实现

### 7.1 拖拽库集成

安装 `@dnd-kit/core` + `@dnd-kit/sortable` + `@dnd-kit/utilities`。

### 7.2 Sidebar 改造

**数据流：**
1. `GET /api/v1/channels` 返回 channels（含 position、group_id）和 groups
2. 前端按 group_id 分桶，每桶按 position 排序
3. 未分组频道（group_id = null）显示在最上方
4. WS 事件实时更新 state

**组件结构：**
```
Sidebar
├── UngroupedChannels          // group_id = null 的频道
│   └── SortableChannelItem[]  // owner 的频道可拖拽
├── ChannelGroup[]             // 按 group.position 排序
│   ├── GroupHeader            // 折叠箭头 + 名称 + 右键菜单（owner only）
│   └── SortableChannelItem[]
└── DmList                     // 不变
```

**权限判断：**
- `channel.created_by === currentUser.id` → 显示拖动手柄（≡）
- `group.created_by === currentUser.id` → 显示分组右键管理菜单
- 非 owner → 只读视图，无手柄、无管理入口

**拖拽行为：**
- `DndContext` 包裹整个频道列表区域
- `SortableContext` per group（含未分组）
- onDragEnd 时调用 `PUT /api/v1/channels/reorder`，乐观更新 local state
- 分组 header 也是 sortable item（仅 owner 可拖）

### 7.3 折叠状态

```typescript
const STORAGE_KEY = 'collab:collapsed-groups';
// localStorage: { [groupId]: boolean }
```

折叠/展开纯本地，不走网络。

### 7.4 创建分组弹窗

复用现有 modal 组件。入口：sidebar 顶部 `[+]` 下拉菜单对所有用户可见，包含"创建频道"选项；"创建分组"选项仅对有 channel owner 身份的用户显示。

### 7.5 分组右键菜单

仅 `group.created_by === currentUser.id` 时渲染。选项：重命名（inline edit）、删除（确认弹窗）。

## 8. 否决方案

| 方案 | 否决理由 |
|------|----------|
| 每用户独立排序（per-user position） | PRD 明确要求排序是频道属性，全局一致 |
| integer position + 批量 reindex | 每次拖拽 O(N) 更新，高并发下冲突多 |
| react-beautiful-dnd | 已停止维护（2022 archived），不支持 React 18 strict mode |
| 原生 HTML5 DnD | 不支持 touch、无动画、开发成本高 |
| 分组嵌套 | PRD 明确排除，增加复杂度无收益 |
| 用 events 表 replay 排序 | 过度设计，直接存 position 更简单 |

## 9. 测试策略

### 9.1 后端测试

| 场景 | 类型 | 覆盖 |
|------|------|------|
| 频道排序 API — owner 成功排序 | integration | position 更新、group_id 更新 |
| 频道排序 API — 非 owner 被拒绝 | integration | 403 |
| 分组 CRUD — 创建/重命名/删除 | integration | 全流程 |
| 分组删除 — 频道自动回到未分组 | integration | group_id → NULL |
| Lexorank 中值计算 | unit | 边界值、极端间距 |
| Lexorank rebalance | unit | 间距不足时重排 |
| WS 广播 — 排序变更广播给所有人 | integration | broadcastToAll |
| 并发排序 — 两个 owner 同时拖拽 | integration | 无冲突（各自更新自己的频道） |

### 9.2 前端测试

| 场景 | 类型 |
|------|------|
| Owner 频道显示拖动手柄，非 owner 不显示 | component |
| 拖拽排序触发 API 调用 + 乐观更新 | component |
| 分组折叠/展开 + localStorage 持久化 | component |
| 创建分组弹窗流程 | component |
| WS 事件更新 sidebar 排序 | integration |

### 9.3 E2E 测试

- Owner 拖拽频道 → 刷新 → 顺序不变
- Owner 拖入分组 → 其他用户 sidebar 实时更新
- 非 owner 无法拖拽
- 删除分组 → 频道回到未分组

## 10. Task Breakdown

### Phase 1: 后端基础（B26 排序）

| Task | 描述 | 估时 |
|------|------|------|
| T1 | 实现 lexorank 工具模块 `packages/server/src/lexorank.ts`（mid-value、rebalance） | 2h |
| T2 | DB migration：channels 加 position 列（仅 position，不含 group_id） | 0.5h |
| T2.1 | DB migration：为已有频道生成初始 position（按 created_at 排列） | 1h |
| T3 | 新增 queries：getChannelsByPosition、updateChannelPosition | 1h |
| T4 | `PUT /api/v1/channels/reorder` 路由 + owner 权限校验 | 2h |
| T5 | WS broadcastToAll channels_reordered 事件 | 0.5h |
| T6 | 修改 `GET /api/v1/channels` 返回 position + group_id + groups | 1h |
| T7 | 后端测试（T1–T6） | 2h |

### Phase 2: 前端排序（B26）

| Task | 描述 | 估时 |
|------|------|------|
| T8 | 安装 @dnd-kit，Sidebar 拆分组件（UngroupedChannels、SortableChannelItem） | 2h |
| T9 | 实现拖拽排序交互（DndContext、onDragEnd → API） | 3h |
| T10 | 拖动手柄权限判断（owner only）+ 视觉反馈（拖影、插入线） | 1.5h |
| T11 | WS 事件监听 → 实时更新 sidebar 排序 | 1h |
| T12 | 移动端长按拖动适配 | 1h |
| T13 | 前端测试（T8–T12） | 2h |

### Phase 3: 分组后端（B27）

| Task | 描述 | 估时 |
|------|------|------|
| T14 | 分组 CRUD 路由（POST/PUT/DELETE /api/v1/channel-groups） | 2h |
| T15 | 分组排序路由 `PUT /api/v1/channel-groups/reorder` | 1h |
| T16 | DB migration：创建 channel_groups 表 + channels 加 group_id FK 列 | 1h |
| T17 | channels/reorder 支持跨分组拖拽（更新 group_id） | 1h |
| T18 | WS 分组事件（group_created/updated/reordered/deleted） | 1h |
| T19 | 后端测试（T14–T18） | 2h |

### Phase 4: 分组前端（B27）

| Task | 描述 | 估时 |
|------|------|------|
| T20 | ChannelGroup 组件 + GroupHeader（折叠/展开 + localStorage） | 2h |
| T21 | 创建分组弹窗 + sidebar [+] 下拉菜单 | 1.5h |
| T22 | 分组右键菜单（重命名 inline edit + 删除确认弹窗） | 2h |
| T23 | 跨分组拖拽交互（频道从分组 A → 分组 B） | 2h |
| T24 | 分组 header 拖拽排序 | 1h |
| T25 | WS 分组事件监听 + state 更新 | 1h |
| T26 | 前端测试（T20–T25） | 2h |

### Phase 5: 收尾

| Task | 描述 | 估时 |
|------|------|------|
| T27 | E2E 测试（排序持久化、实时同步、权限） | 3h |
| T28 | 新频道默认 position 分配（分组末尾 lexorank） | 0.5h |

**总估时：≈ 39.5h**

## 11. 风险与缓解

| 风险 | 影响 | 缓解 |
|------|------|------|
| Lexorank 间距耗尽 | 频繁拖拽后无法插入中间值 | 局部 rebalance：当间距 < 阈值时，重排同一分组内所有频道的 position |
| 并发拖拽冲突 | 两个 owner 同时移动频道到相同位置 | reorder 操作使用 `BEGIN IMMEDIATE` 事务（read→compute→write 原子化）；SQLite WAL + busy_timeout 保证写串行化；各 owner 只操作自己的频道，实际冲突概率极低 |
| @dnd-kit bundle 体积 | 增加客户端包大小 | @dnd-kit/core + sortable 约 15KB gzipped，可接受 |
| 大量频道时拖拽性能 | 100+ 频道可能卡顿 | dnd-kit 虚拟化支持；v1 实际频道量 < 50，暂不优化 |
| Migration 兼容性 | 现有频道无 position 值 | T29 migration 脚本为所有现有频道生成均匀分布的 lexorank |
