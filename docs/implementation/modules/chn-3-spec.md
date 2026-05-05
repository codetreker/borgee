# CHN-3 spec brief — channel 分组个人微调层 (collapsed + position + pin)

> 飞马 · 2026-04-29 · ≤80 行 spec lock (实施视角 3 段拆 PR 由战马B 落, 跟 CV-4 并行不抢号)
> **蓝图锚**: [`channel-model.md`](../../blueprint/current/channel-model.md) §1.4 (作者定义大局 + 个人偏好微调) + §3.4 (差距 — 缺个人折叠/排序, 蓝图建议表 `user_channel_layout(user_id, channel_id, collapsed, position)`)
> **关联**: 野马 #366 stance v0 (7 立场 byte-identical 锚, 已 LGTM); CHN-1 ✅ (#276 schema + #286 server + #288 client `ChannelGroupComponent.tsx` 作者侧 group CRUD 已就位); CHN-2 in-flight (#357 spec / #353 acceptance / #354 文案锁 — DM 拆死前提); CV-1/RT-1 不依赖
> **章程闸**: G3.4 协作场骨架 demo 隐含 UX (个人侧栏 reorder + pin demo 可视价值)

> ⚠️ 锚说明: CHN-3 仅加个人偏好物理拆死层, **不动**作者侧 `channels` / `channel_groups` 表 (CHN-1 已落); pin 走 position 单调小数实现, **不裂** `pinned BOOL` 列 (野马 #366 立场 ③ 字面)

## 0. 关键约束 (3 条立场, 蓝图字面 + #366 stance 7 立场承袭)

1. **作者侧 vs 个人偏好物理拆死, 个人偏好仅 collapsed + position 两维** (野马 #366 立场 ① + ②, 蓝图 §1.4 + §3.4 字面): 作者侧 `channel_groups` 表 (#288 已落) 全员同步看; 新建 `user_channel_layout` 仅个人偏好, **不并入 channels 表** (避免读放大); 个人**不能**改 group 名 / 不能加 group / 不能删 group / 不能重新分组 channel (这四项是作者权, 蓝图 §1.4 字面); 偏好缺失 = fallback 作者顺序; **反约束**: 不加 hidden / muted 列 (mute 走 Phase 5+ notification, hide 留 v3+)
2. **pin = 个人偏好层, 走 position 单调小数, 不裂 pinned BOOL** (野马 #366 立场 ③ + 蓝图 §1.4): pin 实现 = `position = MIN(已有 position) - 1.0` (单调小数避免双源排序); 个人 pin 数量不限 (UI 自负责); DM 永不参与分组 (野马 #366 立场 ④ + CHN-2 #354 立场 ⑤ + #357 立场 ③ 三源 byte-identical, server INSERT/UPDATE 校验 channel.type IN ('private','public') else 400 `layout.dm_not_grouped`); **反约束**: 不裂 `pinned BOOL` 独立列 (避免 ORDER BY pinned DESC, position ASC 双源排序)
3. **个人偏好走 GET/PUT 拉取不进 push fanout, ordering 是 client 端事** (野马 #366 立场 ⑤ + ⑥ + ⑦, ADM-0 §1.3 红线): `GET /me/layout` + `PUT /me/layout` (本人写本人读, admin god-mode 白名单**不含** user_channel_layout 跟 ADM-0 §1.3 + AL-3 #303 ⑦ 同模式); WS push fanout (CM-4 / RT-1) **不查** user_channel_layout (避免 N×M 复杂度), client 端拉到 message 后渲染时排序; 作者删 group → 个人 layout 行 lazy 清理 90d GC (跟 CHN-1 ⑤ soft delete 同精神, **不阻塞**作者删 group 路径)

## 1. 拆段实施 (CHN-3.1 / 3.2 / 3.3, ≤ 3 PR)

| 段 | 范围 | 闭锁 | owner |
|---|---|---|---|
| **CHN-3.1** schema migration v=19 | `user_channel_layout` 表 (`user_id` TEXT NOT NULL FK users / `channel_id` TEXT NOT NULL FK channels / `collapsed` INTEGER NOT NULL DEFAULT 0 (BOOL 0/1) / `position` REAL NOT NULL / `created_at` / `updated_at`); PRIMARY KEY (user_id, channel_id) 复合主键 (本人偏好按 user_id + channel_id 唯一); 索引 `idx_user_channel_layout_user_id` (本人 GET 热路径); v=18 (CV-4.1) → v=19 双向 (sequencing 锁字面延续 CV-2.1=14 / DM-2.1=15 / AL-4.1=16 / CV-3.1=17 / CV-4.1=18 / **CHN-3.1=19**); 反约束 column list 反向断言无 hidden / muted / pinned / group_id (野马 #366 立场 ① + ② + ③ 字面) | 待 PR (战马B) | 战马B |
| **CHN-3.2** server `GET/PUT /me/layout` API + DM 校验 + admin god-mode 白名单 | `GET /api/v1/me/layout` 返本人 row list (joined channels.name/type 仅业务数据, 跟 CHN-1 #286 同源 ACL); `PUT /api/v1/me/layout` body `[{channel_id, collapsed, position}]` 批量 upsert (前置校验所有 channel 是 channel member + channel.type IN ('private','public') else 400 `layout.dm_not_grouped`); admin god-mode endpoint 白名单**不返回** user_channel_layout 行 (跟 ADM-0 §1.3 + AL-3 #303 ⑦ 同模式); 作者删 group 路径**不级联**写 layout (lazy GC 走独立 cron 90d, 不阻塞 CHN-1 既有删 group endpoint); **反约束**: 不开 `POST /me/layout/pin/:channel_id` 旁路 (走 PUT 单源, position 客户端算 MIN-1.0) | 待 PR (战马B) | 战马B |
| **CHN-3.3** client SPA 拖拽 reorder + 折叠 + pin 入口 | `<Sidebar.tsx>` channel 行加拖拽 handle (`@dnd-kit/sortable` 复用 #288 既有 ChannelGroupComponent 同 lib); group header 加 ▼/▶ 折叠按钮; 行右键 / 长按菜单 "置顶" / "取消置顶" (置顶 = position 算 MIN-1.0 PUT); DM `[data-kind="dm"]` 行**无**拖拽 handle + 无 "置顶" 菜单 (野马 #366 立场 ④ + #364 反向 DOM 断言同源, e2e DOM `[data-kind="dm"] [data-sortable-handle]` count==0); 拖拽完成立即 PUT (debounce 200ms), 失败 toast `"侧栏顺序保存失败, 请重试"` byte-identical | 待 PR (战马B) | 战马B |

## 2. 与 CHN-1 / CHN-2 / CV-2 / ADM-0 / RT-1 / CM-4 留账冲突点

- **CHN-1 channel_groups 不动** (核心): 作者侧 group CRUD #288 已就位, CHN-3 完全不碰 channel_groups schema / endpoint; 反约束: 不加 `ALTER TABLE channels ADD collapsed` / 不加 `ALTER TABLE channel_groups ADD position user` (野马 #366 黑名单 grep 立场 ① 字面)
- **CHN-2 DM 拆死** (字面承袭三源): DM 永不参与分组 — `user_channel_layout` row WHERE channel.type != 'dm' (server 校验); 跟 CHN-2 #354 立场 ⑤ + #357 立场 ③ + #366 立场 ④ byte-identical; 反约束 client `[data-kind="dm"] [data-sortable-handle]` count==0 (#364 同源)
- **CV-2 锚点对话** (非冲突): CHN-3 是 channel-level UX, anchor 是 artifact-level; 不交叉
- **ADM-0 §1.3 红线** (核心): admin 不入业务路径; admin god-mode endpoint 白名单不含 user_channel_layout (跟 AL-3 #303 ⑦ god-mode 字段白名单同模式); 反约束: 不开 `GET /admin/users/:id/layout` (野马 #366 立场 ⑤)
- **RT-1 / CM-4 push fanout** (核心反约束): WS frame 不混 position / collapsed (排序是 client 端事, 立场 ⑥); 反约束: WSEnvelope / push frame schema 0 hit `position` 字段 (野马 #366 黑名单 grep 立场 ⑥)
- **AL-4 / CV-3 / CV-4 sequencing 锁字面延续**: v=14/15/16/17/18/19 串接 (CV-2.1 ✅ #359 / DM-2.1 ✅ #361 / AL-4.1 待 v=16 / CV-3.1 待 v=17 / CV-4.1 待 v=18 / CHN-3.1 待 v=19)
- **个人偏好不挂 WS frame** (立场 ③): 估计 CHN-3 无 WS push frame — 偏好是本人写本人读, 跨设备同步走 GET pull (本人多设备开页面会重新拉一次 layout, 不实时 push); 若将来 v3+ 需多设备实时同步偏好, 再开 `LayoutChangedFrame` (本 spec 不开, 反约束)

## 3. 反查 grep 锚 (Phase 3 续作 / Phase 4 验收)

```
git grep -nE 'CREATE TABLE.*user_channel_layout'              packages/server-go/internal/migrations/   # ≥ 1 hit (CHN-3.1)
git grep -nE 'GET /api/v1/me/layout|PUT /api/v1/me/layout'    packages/server-go/internal/api/          # ≥ 1 hit (CHN-3.2 endpoint)
git grep -nE 'data-sortable-handle'                           packages/client/src/components/Sidebar.tsx # ≥ 1 hit (CHN-3.3 拖拽 handle)
git grep -nE 'layout\.dm_not_grouped'                         packages/server-go/internal/api/          # ≥ 1 hit (DM 反约束错码)
# 反约束 (野马 #366 黑名单 7 行 byte-identical 同源)
git grep -nE 'ALTER TABLE channels.*ADD.*collapsed|ALTER TABLE channel_groups.*ADD.*position.*user' packages/server-go/internal/migrations/   # 0 hit (立场 ① 物理拆死)
git grep -nE 'PATCH /me/groups/.*name|POST /me/groups|DELETE /me/groups' packages/server-go/internal/api/   # 0 hit (立场 ② 个人不改 group 结构)
git grep -nE 'pinned\s+BOOL|pinned\s+INTEGER|is_pinned\s+BOOL' packages/server-go/internal/migrations/   # 0 hit (立场 ③ pin 走 position)
git grep -nE 'user_channel_layout.*hidden|user_channel_layout.*muted' packages/server-go/internal/migrations/   # 0 hit (留 v3+/Phase 5+)
git grep -nE 'admin.*user_channel_layout|godmode.*layout'     packages/server-go/internal/api/admin*.go   # 0 hit (立场 ⑤ ADM-0 红线)
git grep -nE 'WSEnvelope.*position|push.*frame.*\bposition\b|fanout.*user_channel_layout' packages/server-go/internal/ws/   # 0 hit (立场 ⑥ ordering client 端)
git grep -nE 'cascade.*delete.*user_channel_layout|FOREIGN KEY.*group_id.*ON DELETE CASCADE' packages/server-go/internal/migrations/   # 0 hit (立场 ⑦ lazy 清理不级联)
```

任一 0 hit (除反约束行) → CI fail.

## 4. 不在本轮范围 (反约束 — #366 不在范围 7 项延续)

- ❌ 个人 hide / mute channel (Phase 5+ notification 模块)
- ❌ 跨 channel pin 上限 (UI 自负责, 蓝图无)
- ❌ 个人改 group 名 / 个人加 group / 个人删 group (蓝图 §1.4 字面, 作者权)
- ❌ DM 进个人分组 (CHN-2 #354 ⑤ + 立场 ④ 永久锁)
- ❌ admin god-mode 看用户个人偏好 (ADM-0 §1.3 红线 + 立场 ⑤ 永久锁)
- ❌ server 端按用户 fanout 排序 (立场 ⑥ — 排序是 client 端事)
- ❌ 个人偏好 push WS frame (走 GET /me/layout 拉, 多设备实时同步留 v3+)
- ❌ 拖拽到 group 之外 (channel-group 关系是作者权, 个人 reorder 仅 group 内 + group 之间)

## 5. Test plan (实施 PR 各自带, 此 spec 不带)

- CHN-3.1: migration v=18 → v=19 双向 + PRIMARY KEY (user_id, channel_id) 反向 (重复对 reject) + idx hit + 反约束 column list 反向断言 (无 hidden/muted/pinned/group_id) + 跟既有 CV-1.1/AL-3.1 同模式 idempotent
- CHN-3.2: GET 本人 layout 返业务数据 + PUT batch upsert 200 (DM channel_id 含 → 400 `layout.dm_not_grouped` 字面锁) + 非成员 channel 含 → 403 + admin god-mode 白名单反向断言 (admin token GET /admin/users/:id 不返 layout) + 作者删 group 路径不阻塞 (CHN-1 既有 endpoint 行为不破)
- CHN-3.3: e2e 拖拽 channel reorder PUT 命中 + group ▼/▶ 折叠状态 PUT + 右键置顶 → position MIN-1.0 PUT + DM 行 e2e DOM `[data-kind="dm"] [data-sortable-handle]` count==0 (野马 #366 立场 ④ + #364 同源) + 失败 toast 文案锁 byte-identical
