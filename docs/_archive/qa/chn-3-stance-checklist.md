# CHN-3 立场反查表 (channel 分组个人微调层)

> **状态**: v0 (野马, 2026-04-29)
> **目的**: CHN-3 实施 PR 直接吃此表为 acceptance; 飞马 spec brief / 烈马 acceptance template / 战马拆段实施 review 拿此表反查立场漂移。一句话立场 + §X.Y 锚 + 反约束 (X 是, Y 不是) + v0/v1。
> **关联**: `channel-model.md` §1.4 (作者定义 + 个人微调) + §3.4 (差距 — 缺个人折叠/排序, 蓝图建议表 `user_channel_layout(user_id, channel_id, collapsed, position)`); 既有 `channel_groups` 表 (`channels.group_id` FK 已落) + `ChannelGroupComponent.tsx` 作者侧 group CRUD (#288 CHN-1.3 已就位); CHN-2 #354 立场 ⑤ DM 不参与个人分组同源.
> **依赖**: CHN-1 ✅ closed (channels + channel_groups schema + 作者侧 group CRUD); CHN-2 in-flight (#357 spec / #353 acceptance / #354 文案锁) — DM 拆死前提.
> **#338 cross-grep 反模式遵守**: 既有 `channel_groups` 表 (作者侧, schema 已落) + `ChannelGroupComponent.tsx` 字面 "分组" / "删除分组" / "频道不会被删除"; CHN-3 不动作者侧, 仅加个人偏好层, 字面跟既有 cross-grep 不冲突.

---

## 1. CHN-3 立场反查表 (channel 分组个人微调)

| # | 立场锚 | 一句话立场 | 反约束 (X 是, Y 不是) | v0 / v1 |
|---|--------|----------|----------------------|---------|
| ① | channel-model §1.4 + §3.4 | **作者侧 group 结构** (创建/改名/分配 channel 到 group) **跟个人偏好层物理拆死** — 个人改自己的, 不污染他人 | **是** 作者侧 `channel_groups` 表 (#288 已落) 全员同步看; 个人偏好表 `user_channel_layout(user_id, channel_id_or_group_id, collapsed BOOL, position INT)` 仅本用户 GET; **不是** 个人能改 group 名 (蓝图 §1.4 字面 "不允许个人**改 group 名**"); **不是** 个人能重新分组 (channel 跨 group 移动是作者权, 蓝图 §1.4 字面) | v0: 个人偏好独立表; v1 同, 不并入 channels 表 (避免读放大) |
| ② | channel-model §1.4 + §3.4 | **个人偏好仅 2 个维度** — group 折叠状态 (collapsed) + 侧边栏内排序 (position) | **是** 个人控 (a) 哪些 group 折叠 (b) group 内 channel 顺序 + group 之间顺序; **不是** 个人改 group 名 / 个人加 group / 个人删 group (这三项是作者权, 蓝图 §1.4); **不是** 个人 hide/mute channel (mute 是 notification 维度走 Phase 5+, hide 蓝图无, 留 v3+); 偏好缺失 = fallback 作者顺序 (蓝图 §1.4 "保留协作心智") | v0: collapsed + position 两列; v1 同, 不加 hidden / muted 列 |
| ③ | channel-model §1.4 + concept-model §1.3 (协作可以扩权不行) | **pin (置顶) = 个人偏好层第一类公民, 不是新概念** — pin 实现 = 把 channel 在个人 position 排到最前 (position 单调小数), **不另起 pinned BOOL 列** | **是** pin 走 position 排序 (置顶 = position = MIN(position) - 1.0); **不是** `pinned BOOL` 独立列 (避免双源排序: 先按 pin 再按 position 的复杂度); **不是** 全员可见 pin (是个人偏好); **不是** 跨 channel pin 数量限制 (个人想 pin 几个就几个, UI 上自己负责) | v0: pin = position 单调小数; v1 同, UI 加 pin icon 但底层无 pinned 列 |
| ④ | channel-model §3.4 + CHN-2 #354 ⑤ + DM-2 | **个人分组只对 channel (`type='private'/'public'`) 生效, DM 永不参与分组** | **是** `user_channel_layout` row WHERE channel.type IN ('private','public'); **不是** DM (CHN-2 #354 ⑤ 反约束: DM 永远 2 人 + 独立 "私信" 区, 蓝图 §1.2); server INSERT/UPDATE 路径校验 channel.type != 'dm', else 400 "DM 不参与分组"; client UI DM 行无拖拽 hover handle | v0/v1 同 — DM 不参与永久锁 (跟 CHN-2 #357 spec ③ "DM 不参与分组" 字面一致) |
| ⑤ | channel-model §1.4 + ADM-0 §1.3 红线 | **admin 不入 channel** → admin 不存个人偏好 (admin god-mode SPA 看作者侧 group 结构, 不看任意 user 的个人偏好) | **是** `user_channel_layout` 仅业务用户写 + 仅本人读; admin god-mode endpoint 白名单**不含** `user_channel_layout` (跟 ADM-0 §1.3 红线 + AL-3 #303 ⑦ god-mode 字段白名单同模式); **不是** admin 可代写个人偏好 (CHN-1 owner 改名同模式, admin 不入业务路径) | v0/v1 同 — admin 永久不入 |
| ⑥ | channel-model §1.4 + concept-model §4 (mention 路由按 sender_id 不抄 owner) | **个人偏好不进 push fanout** — channel 内 message 推送用作者侧 channel.position 排序, 不按个人偏好排 | **是** WS push fanout (CM-4 / RT-1 路径) 不查 `user_channel_layout`; client 端拉到 message 后**渲染时**走个人偏好排序 (ordering 是 client 端事, 不上服务器); **不是** server 按用户算 fanout (会爆 N×M 复杂度); **不是** push frame 含 position 字段 (走 GET /channels/:id 拉 + GET /me/layout 拉, 二次拉合并) | v0: client 端 ordering; v1 同, 加 client 端 IndexedDB 缓存 layout 减网络 |
| ⑦ | channel-model §1.4 + 反向兜底 | **作者侧 group 删除** (#288 已就位 ChannelGroupComponent "删除分组「{name}」吗？分组内的频道不会被删除") **触发个人偏好级联清理** — 残留无效 group_id 的 layout row 走 lazy delete 不阻塞 | **是** 作者删 group → 该 group 在所有用户的 `user_channel_layout` 标 `group_id_invalid` (lazy, 90d GC, 跟 CHN-1 ⑤ soft delete 同精神); UI 渲染时 group_id 失效 → fallback 默认顺序 (蓝图 §1.4); **不是** 同步 cascade 删 (会卡作者删 group 路径); **不是** 阻止作者删 group (协作场作者权重于个人, 蓝图 §1.4 "作者控制大局") | v0: lazy 清理, 90d GC; v1 加 owner 端 "已清理 N 行" 通知 |

---

## 2. 黑名单 grep — CHN-3 实施 PR merge 后跑, 全部预期 0 命中

```bash
# 立场 ① — 个人偏好不应混入作者侧 channels / channel_groups 表 (物理拆死)
grep -rnE "ALTER TABLE channels.*ADD.*collapsed|ALTER TABLE channel_groups.*ADD.*position.*user" packages/server-go/internal/migrations/ | grep -v _test.go
# 立场 ② — 个人不应改 group 名 / 创 group / 删 group (作者权)
grep -rnE "PATCH /me/groups/.*name|POST /me/groups|DELETE /me/groups" packages/server-go/internal/api/ | grep -v _test.go
# 立场 ③ — 不应有 pinned BOOL 独立列 (走 position 单调小数)
grep -rnE "pinned\s+BOOL|pinned\s+INTEGER|is_pinned\s+BOOL" packages/server-go/internal/migrations/ | grep -v _test.go
# 立场 ④ — DM 不参与分组 (server 端反约束)
grep -rnE "user_channel_layout.*type.*dm|INSERT.*user_channel_layout.*WHERE.*type=.dm" packages/server-go/internal/api/ | grep -v _test.go
# 立场 ⑤ — admin god-mode 不含 user_channel_layout
grep -rnE "admin.*user_channel_layout|godmode.*layout" packages/server-go/internal/api/admin*.go | grep -v _test.go
# 立场 ⑥ — push frame 不含 position (排序是 client 端事)
grep -rnE "WSEnvelope.*position|push.*frame.*\\bposition\\b|fanout.*user_channel_layout" packages/server-go/internal/ws/ | grep -v _test.go
# 反向 (字面承袭): 个人 hide/mute 列 (留 v3+)
grep -rnE "user_channel_layout.*hidden|user_channel_layout.*muted" packages/server-go/internal/migrations/ | grep -v _test.go
```

---

## 3. 不在 CHN-3 范围 (避免 PR 膨胀)

- ❌ 个人 hide/mute channel (Phase 5+ notification 模块); ❌ 跨 channel pin 上限 (UI 自负责)
- ❌ 个人改 group 名 / 个人加 group / 个人删 group (蓝图 §1.4 字面 — 作者权)
- ❌ DM 进个人分组 (CHN-2 #354 ⑤ + 立场 ④ 永久锁)
- ❌ admin god-mode 看用户个人偏好 (ADM-0 §1.3 红线 + 立场 ⑤ 永久锁)
- ❌ server 端按用户 fanout 排序 (立场 ⑥ — 排序是 client 端事)
- ❌ 个人偏好 push WS frame (走 GET /me/layout 拉)
- ❌ 拖拽到 group 之外 (channel-group 关系是作者权, 个人 reorder 仅 group 内 + group 之间)

---

## 4. 验收挂钩

- CHN-3.1 schema PR: 立场 ①②③ — `user_channel_layout` 表 (`user_id PK part / channel_id_or_group_id PK part / collapsed BOOL / position REAL`) + 反向 grep 立场 ① / ③ 黑名单 0 命中
- CHN-3.2 server PR: 立场 ④⑤⑥⑦ — server 端 `GET/PUT /me/layout` endpoint + DM 反约束 (立场 ④) + admin god-mode 白名单不含 (立场 ⑤) + push frame 不混 position (立场 ⑥) + 作者删 group lazy 清理 (立场 ⑦)
- CHN-3.3 client PR: 拖拽 reorder UI (`@dnd-kit/sortable` 复用 #288 既有) + group 折叠状态本地 cache + DM 行无拖拽 handle (立场 ④ DOM 反断言 — DM `data-kind="dm"` 行无 `[data-sortable-handle]` attr 跟 #364 同源)
- CHN-3 entry 闸: 立场 ①-⑦ 全锚 + §2 黑名单 grep 全 0 + 跟 CHN-2 #354/#353 DM 反约束 byte-identical 不漂

---

## 5. 更新日志

| 日期 | 作者 | 变化 |
|------|------|------|
| 2026-04-29 | 野马 | v0, 7 立场 (作者vs个人物理拆死 / 偏好仅 collapsed+position 两维 / pin 走 position 不裂列 / DM 不参与 / admin 不入 / fanout 不混偏好 / 作者删 group lazy 清理) + 7 行反向 grep + 7 项不在范围 + 验收挂钩三段对齐. 跟既有 cross-grep #338 反模式: `channel_groups` 表 + `ChannelGroupComponent.tsx` 作者侧字面已稳定, CHN-3 不动作者侧字面, 仅加个人偏好层物理拆死 |
