# Acceptance Template — CHN-3: channel 分组个人微调层 (collapsed + position + pin)

> 蓝图: `channel-model.md` §1.4 (作者定义大局 + 个人偏好微调 — "不允许个人改 group 名") + §3.4 (差距 — 缺个人折叠/排序, 蓝图建议表 `user_channel_layout(user_id, channel_id, collapsed, position)`)
> Spec: `docs/implementation/modules/chn-3-spec.md` (飞马 #371, 3 立场 + 3 拆段 + 11 grep 反查)
> Stance: `docs/qa/chn-3-stance-checklist.md` (野马 #366, 7 立场 + 7 行黑名单 grep + 验收挂钩三段对齐)
> 拆 PR (拟): **CHN-3.1** schema migration v=19 (`user_channel_layout` 表) + **CHN-3.2** server `GET/PUT /me/layout` API + DM 校验 + admin god-mode 白名单 + **CHN-3.3** client SPA 拖拽 reorder + 折叠 + pin 入口
> Owner: 战马B 实施 / 烈马 验收

## 验收清单

### §1 schema (CHN-3.1) — user_channel_layout 数据契约

> 锚: 飞马 #371 spec §1 CHN-3.1 + 野马 #366 立场 ① + ② + ③ 字面 + AL-3.1 #310 / CV-2.1 #359 schema 三轴模板

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 1.1 表 schema 三轴: `user_id` TEXT NOT NULL FK users + `channel_id` TEXT NOT NULL FK channels + `collapsed` INTEGER NOT NULL DEFAULT 0 (BOOL 0/1) + `position` REAL NOT NULL + `created_at` + `updated_at`; PRIMARY KEY (user_id, channel_id) 复合主键 | migration drift test | 战马A / 烈马 | ✅ #410 (0cde6f9) — `internal/migrations/chn_3_1_user_channel_layout_test.go::TestCHN31_CreatesUserChannelLayoutTable` PASS |
| 1.2 INDEX `idx_user_channel_layout_user_id` (本人 GET 热路径) | migration drift test | 战马A / 烈马 | ✅ #410 — `chn_3_1_user_channel_layout_test.go::TestCHN31_HasUserIDIndex` PASS |
| 1.3 PRIMARY KEY (user_id, channel_id) 反向 — 同 (user_id, channel_id) 二次 INSERT reject | migration drift test | 战马A / 烈马 | ✅ #410 — `chn_3_1_user_channel_layout_test.go::TestCHN31_PKEnforcesUniqueRowPerUserChannel` PASS (重复对 reject, 跨 user/channel 独立行允许) |
| 1.4 migration v=18 → v=19 串行号 + idempotent rerun no-op; `registry.go` v=19 字面锁 (sequencing 链字面延续 14/15/16/17/18/19) | migration drift test | 战马A / 烈马 | ✅ #410 — `chn_3_1_user_channel_layout_test.go::TestCHN31_Idempotent` + `registry.go` chn31UserChannelLayout 字面锁 |
| 1.5 反约束 — 表无 `hidden` / `muted` / `pinned` / `is_pinned` / `group_id` 列 (野马 #366 立场 ② hide/mute 留 v3+ + ③ pin 走 position 不裂 BOOL); pragma 反向断言 column list 不含上述列名 | migration drift test + grep | 飞马 / 烈马 | ✅ #410 — `chn_3_1_user_channel_layout_test.go::TestCHN31_NoDomainBleed` (8 列反向: hidden/muted/is_hidden/is_muted/pinned/is_pinned/group_id/cursor/sort_index/order_index) PASS |

### §2 server API (CHN-3.2) — GET/PUT /me/layout + DM 校验 + admin 白名单

> 锚: 飞马 #371 spec §1 CHN-3.2 + 野马 #366 立场 ④ DM 反约束 + ⑤ admin 白名单不含 + ⑦ 作者删 group lazy 清理

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 2.1 `GET /api/v1/me/layout` 返本人 row list — joined channels.name/type 业务数据 (跟 CHN-1 #286 ACL 同源, 仅本人 user_id 行) | unit + e2e | 战马A / 烈马 | ✅ #412 — `internal/api/chn_3_2_layout_test.go::TestCHN32_GetEmpty` + `TestCHN32_PutBatchUpsertAndGet` + `TestCHN32_PerUserIsolation` (本人写本人读, 跨用户独立) PASS |
| 2.2 `PUT /api/v1/me/layout` body `[{channel_id, collapsed, position}]` 批量 upsert; 前置校验所有 channel 是 channel member, else 403 | unit + e2e | 战马A / 烈马 | ✅ #412 — `chn_3_2_layout_test.go::TestCHN32_NonMemberReject` (非成员 channel_id → 403) + `TestCHN32_PutBatchUpsertAndGet` (ON CONFLICT upsert atomic) PASS |
| 2.3 反约束 DM — channel.type='dm' 含 → HTTP 400 错码 `layout.dm_not_grouped` 字面锁 byte-identical (跟 CHN-2 #354 立场 ⑤ + #357 立场 ③ + #366 立场 ④ 三源) | unit + grep | 战马A / 烈马 | ✅ #412 — `chn_3_2_layout_test.go::TestCHN32_DMReject` 字面 byte-identical PASS; `grep -n "layout.dm_not_grouped" packages/server-go/internal/api/layout.go` count≥1 (5 源 byte-identical: #357/#353/#366/#402/server const) |
| 2.4 反约束 admin god-mode 白名单不含 user_channel_layout — admin token GET admin endpoint 不返 layout 行 (跟 ADM-0 §1.3 红线 + AL-3 #303 ⑦ god-mode 字段白名单同模式) | unit + grep | 飞马 / 烈马 | ✅ #412 — `chn_3_2_layout_test.go::TestCHN32_AdminAPINotMounted` (/admin-api/v1/users/:id/layout 返 404/401, 不挂 admin mux) + `grep -nE 'admin.*user_channel_layout\|godmode.*layout' packages/server-go/internal/api/admin*.go` count==0 |
| 2.5 反约束 旁路 — 不开 `POST /me/layout/pin/:channel_id` 旁路 endpoint (走 PUT 单源, position 客户端算 MIN-1.0) | grep | 飞马 / 烈马 | ✅ #412 — `grep -nE 'POST.*\/me\/layout\/pin\|PATCH.*\/me\/layout\/pin' packages/server-go/internal/api/` count==0 (仅 GET/PUT 双 endpoint) |
| 2.6 作者删 group 路径**不级联**写 layout (lazy GC 走独立 cron 90d, 不阻塞 CHN-1 既有删 group endpoint) | unit | 战马A / 烈马 | ✅ #410 + #412 — schema 无 ON DELETE CASCADE (REG-CHN3-002 反向断言 group_id 列 0 hit); LayoutHandler 不订阅 channel.delete event (反向 grep 锚) |
| 2.7 反约束 push fanout 不查偏好 — WS frame 不混 position / collapsed (排序是 client 端事, 立场 ⑥) | grep | 飞马 / 烈马 | ✅ #410 + #412 — `grep -nE 'WSEnvelope.*position\|push.*frame.*\bposition\b\|fanout.*user_channel_layout' packages/server-go/internal/ws/` count==0 |

### §3 client SPA (CHN-3.3) — 拖拽 reorder + 折叠 + pin 入口

> 锚: 飞马 #371 spec §1 CHN-3.3 + 野马 #366 立场 ④ DM 行无拖拽 handle (跟 #364 byte-identical)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 3.1 `<Sidebar.tsx>` channel 行加拖拽 handle (`@dnd-kit/sortable` 复用 #288 既有 ChannelGroupComponent 同 lib); 拖拽完成立即 `PUT /me/layout` (debounce 200ms) | e2e | 战马A / 烈马 | ✅ #415 (bad0112) + #422 — `packages/e2e/tests/chn-3-3-sidebar-reorder.spec.ts::① drag handle DOM byte-identical + aria-label + ⋮⋮ icon visible` PASS; `chn-3-content-lock.test.ts` 11 cases (vitest) PASS; `grep -n 'data-sortable-handle' packages/client/src/components/SortableChannelItem.tsx` count≥1 |
| 3.2 group header 加 ▼/▶ 折叠按钮 — 点击切换状态后 `PUT /me/layout` 写 collapsed | e2e | 战马A / 烈马 | ✅ #415 — `GroupHeader.tsx` 加 `data-collapsed={collapsed ? "true" : "false"}` + aria-label `"折叠分组"` + ▶/▼ 二态 icon 切换 (vitest content-lock case "② group folding DOM byte-identical" PASS) |
| 3.3 行右键 / 长按菜单 "置顶" / "取消置顶" — 置顶 = `position = MIN(已有 position) - 1.0` PUT (单调小数, 不裂 pinned BOOL 列) | e2e | 战马A / 烈马 | ✅ #415 + #422 + #425 (drift fix `<menu>` byte-identical) — `chn-3-3-sidebar-reorder.spec.ts::③ right-click pin menu shows "置顶" / "取消置顶" + role="menu" + data-context` (PUT /me/layout 命中 + position < 0 单调小数验) PASS |
| 3.4 反约束 DM — `[data-kind="dm"]` 行 **无**拖拽 handle + **无** "置顶" 菜单 (野马 #366 立场 ④ + #364 同源 DOM 反断) | e2e DOM 反断 | 战马A / 烈马 | ✅ #415 + #422 — `chn-3-3-sidebar-reorder.spec.ts::⑤ DM row 反约束: no drag handle + no pin menu (5 源 byte-identical)` PASS; DM 行渲染走 `Sidebar.MergedDmList` 独立路径, ChannelList/SortableChannelItem 不见 DM (omit-not-disable) |
| 3.5 拖拽失败 toast 文案锁 byte-identical — `"侧栏顺序保存失败, 请重试"` (跟 #371 spec §1 CHN-3.3 字面同源, 反向 grep 同义词漂移) | e2e | 战马A / 烈马 | ✅ #415 — `useUserLayout.ts` const `LAYOUT_SAVE_TOAST = '侧栏顺序保存失败, 请重试'` 5 源 byte-identical (#371/#376/#402/#412 server const/本 const) + chn-3-content-lock.test.ts case "④ failure toast 字面 byte-identical 5 源" PASS |

### §4 反向 grep / e2e 兜底 (跨 CHN-3.x 反约束 — 跟野马 #366 §2 黑名单 byte-identical)

> 锚: 飞马 #371 spec §3 11 行 + 野马 #366 §2 黑名单 7 行 byte-identical 同源

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 4.1 立场 ① 物理拆死 — `grep -nE 'ALTER TABLE channels.*ADD.*collapsed\|ALTER TABLE channel_groups.*ADD.*position.*user' packages/server-go/internal/migrations/` count==0 | CI grep | 飞马 / 烈马 | _(每 CHN-3.* PR 必跑)_ |
| 4.2 立场 ② 个人不改 group 结构 — `grep -nE 'PATCH /me/groups/.*name\|POST /me/groups\|DELETE /me/groups' packages/server-go/internal/api/` count==0 | CI grep | 飞马 / 烈马 | _(每 CHN-3.2 PR 必跑)_ |
| 4.3 立场 ③ pin 走 position — `grep -nE 'pinned\s+BOOL\|pinned\s+INTEGER\|is_pinned\s+BOOL' packages/server-go/internal/migrations/` count==0 | CI grep | 飞马 / 烈马 | _(每 CHN-3.1 PR 必跑)_ |
| 4.4 立场 ④ DM 不进个人分组 — `grep -nE 'user_channel_layout.*type.*dm\|INSERT.*user_channel_layout.*WHERE.*type=.dm' packages/server-go/internal/api/` count==0 (反向: server 校验 reject 而非允许) + e2e DOM `[data-kind="dm"] [data-sortable-handle]` count==0 (跟 #364 同源) | CI grep + e2e | 飞马 / 烈马 | _(每 CHN-3.* PR 必跑)_ |
| 4.5 立场 ⑤ admin god-mode 不入 — `grep -rnE 'admin.*user_channel_layout\|godmode.*layout' packages/server-go/internal/api/admin*.go` count==0 | CI grep | 飞马 / 烈马 | _(每 CHN-3.2 PR 必跑)_ |
| 4.6 立场 ⑥ ordering client 端 — `grep -nE 'WSEnvelope.*position\|push.*frame.*\bposition\b\|fanout.*user_channel_layout' packages/server-go/internal/ws/` count==0 | CI grep | 飞马 / 烈马 | _(每 CHN-3.* PR 必跑)_ |
| 4.7 立场 ⑦ lazy 清理不级联 — `grep -nE 'cascade.*delete.*user_channel_layout\|FOREIGN KEY.*group_id.*ON DELETE CASCADE' packages/server-go/internal/migrations/` count==0 | CI grep | 飞马 / 烈马 | _(每 CHN-3.1 PR 必跑)_ |

## 边界 (跟其他 milestone 关系)

| Milestone | 关系 | 字面承袭 |
|---|---|---|
| CHN-1 ✅ | 作者侧 channel_groups CRUD #288 已就位, CHN-3 完全不动 | channels / channel_groups schema byte-identical 不破 |
| CHN-2 #357/#354 | DM 拆死字面承袭三源 (server 校验 + DOM 反断) | `data-kind="dm"` 行无拖拽 handle (#364 同源) |
| CV-2/3/4 | 非冲突, CHN-3 是 channel-level UX, artifact/anchor/iterate 是 artifact-level | 无 |
| CHN-4 #374 | CHN-3 偏好仅作用于 sidebar, **不渗透** workspace tab / artifact / anchor 视图 (CHN-4 立场 ③ 字面承袭) | 反向断言 WorkspaceTab / AnchorSidebar 不读 user_channel_layout |
| RT-1 / CM-4 | push fanout 不查偏好 (立场 ⑥) — 排序 client 端事 | hub.cursors 单调发号不混 layout 字段 |
| ADM-0 | admin god-mode 白名单不含 user_channel_layout (§1.3 红线) | 跟 AL-3 #303 ⑦ 同模式 |

## 退出条件

- §1 schema 5 项 + §2 server 7 项 + §3 client 5 项 + §4 反向 grep 7 项**全绿** (一票否决)
- 反查锚 §4.1-4.7 (跟野马 #366 §2 黑名单 byte-identical) 每 PR 必跑 0 命中
- 登记 `docs/qa/regression-registry.md` REG-CHN3-001..024 (5 schema + 7 server + 5 client + 7 反向 grep)
- v=14-19 sequencing 字面延续 (CV-2.1 ✅ / DM-2.1 ✅ / AL-4.1 v=16 / CV-3.1 v=17 / CV-4.1 v=18 / **CHN-3.1 v=19**)
- 个人偏好不挂 WS frame 反约束守住 (多设备实时同步留 v3+, 立场 ③ 字面)
