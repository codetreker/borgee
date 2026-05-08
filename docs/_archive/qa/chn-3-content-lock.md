# CHN-3 sidebar 拖拽 reorder 文案锁 (野马 G3.x demo 预备)

> **状态**: v0 (野马, 2026-04-29)
> **目的**: CHN-3.x client UI 实施前锁 sidebar 拖拽 reorder + group 折叠 + pin 入口 + DM 反约束文案 + DOM 字面 — 跟 AL-3 #305 / DM-2 #314 / AL-4 #321 / CHN-2 #354 / CV-2 #355 / CV-3 #370 / CV-4 #380 / CHN-4 #382 同模式 (用户感知签字 + byte-identical), 防 CHN-3 实施时 DM 行漏拖拽 handle / 失败 toast 漂移 / pin 字面同义词污染.
> **关联**: 野马 stance #366 7 立场 + 飞马 spec #371 §1 CHN-3.3 字面 (toast 文案 byte-identical) + 烈马 acceptance #376 §3 验收锚 + CHN-2 #354/#364 立场 ④ DM 不参与分组 6 源同根 + CHN-4 #382 ⑤ DM 永不含 workspace tab 7 源 byte-identical.
> **#338 cross-grep 反模式遵守**: 既有实施 `Sidebar.tsx` (#288) + ChannelGroupComponent (#288) 字面已稳定 (`"分组"` / `"删除分组"` / `"频道不会被删除"` 不动), CHN-3 仅加个人偏好层字面, 不改作者侧字面 (跟 #366 立场 ① 物理拆死同源).

---

## 1. 6 处文案 + DOM 字面锁

| # | 场景 | 字面锁 (byte-identical) | 反约束 |
|---|------|-----|------|
| ① | **拖拽 handle DOM** (channel 行 hover 出现) | DOM: `<button class="sortable-handle" data-sortable-handle="" aria-label="拖拽调整顺序">⋮⋮</button>` byte-identical (icon ⋮⋮ 锁 + aria-label 中文 a11y); 跟 `@dnd-kit/sortable` 复用 #288 既有 ChannelGroupComponent 同 lib (跟 #371 spec §1 CHN-3.3 同源) | ❌ 不准 "Drag" / "拖动" / "排序" / "移动" 同义词 (中文 byte-identical 锁); ❌ DM 行不准渲染 handle (`[data-kind="dm"] [data-sortable-handle]` count==0, 跟 #366 立场 ④ + #364 patch + #371 立场 ② + #376 §3.4 + #382 ⑤ **5 源 byte-identical** 同根); ❌ 不准 `aria-label` 缺失 (a11y 永久锁) |
| ② | **group 折叠按钮** (group header) | DOM: `<button class="group-toggle" data-collapsed="{true|false}" aria-label="折叠分组">{▶|▼}</button>` byte-identical (icon ▶ 折叠 / ▼ 展开 + `data-collapsed` 二态锁); 点击切换 PUT `/me/layout` 写 collapsed (跟 #371 spec §1 CHN-3.3 + #376 §3.2 同源) | ❌ 不准 "Collapse/Expand" / "收起/展开" / "▲" 同义词; ❌ data-collapsed 缺失 (e2e 反断必有); ❌ 折叠状态进 push frame (跟 #366 立场 ⑥ + #371 立场 ③ "ordering 是 client 端事不进 fanout" 同源 — 走 GET/PUT /me/layout 拉) |
| ③ | **右键菜单 pin / unpin** (channel 行右键 / 长按) | DOM: `<menu role="menu" data-context="channel-pin"><button>{置顶\|取消置顶}</button></menu>` byte-identical (`"置顶"` / `"取消置顶"` 中文 2 字面锁); 点击 → `position = MIN(已有 position) - 1.0` PUT (单调小数, 跟 #371 立场 ② + #366 立场 ③ "pin = position 不裂 pinned BOOL" 同源) | ❌ 不准 "Pin/Unpin" / "固定/取消固定" / "Stick" 同义词; ❌ DM 行不准弹此菜单 (跟 #366 立场 ④ + ① 同源); ❌ 不准走 `POST /me/layout/pin/:channel_id` 旁路 endpoint (走 PUT 单源, #366 立场 ②/③ + #371 §3 grep `POST.*\/me\/layout\/pin` 0 hit 反约束); ❌ pin 不裂 `pinned BOOL` 列 (跟 #371 §3 grep `pinned\s+BOOL` 0 hit 同源) |
| ④ | **拖拽失败 toast 文案** (PUT /me/layout 失败) | toast 字面: `"侧栏顺序保存失败, 请重试"` byte-identical 跟 **#371 spec §1 CHN-3.3 + #376 §3.5 三源 byte-identical** (改 = 改三处); debounce 200ms 后 PUT, 失败 toast 1.5s 自动消 (跟 CV-3 #370 ③ "已复制" toast 1.5s 同精神) | ❌ 不准 "保存失败" / "Save failed" / "请稍后重试" / "网络错误" 同义词漂移; ❌ 不准 toast 持续 >3s (UX 噪声); ❌ 不准 toast 显示 raw error.message (隐私 + UX, 跟 #305 ③ error 文案模板锁同精神) |
| ⑤ | **DM 行反约束 — 无拖拽 handle + 无右键 pin 菜单** (跟 CHN-2 立场 6+ 源同根) | DM 视图 (`channel.type==='dm'`) 行 DOM 反约束: 无 `[data-sortable-handle]` + 无 `[data-context="channel-pin"]` 右键菜单; 跟 **#366 立场 ④ + #364 patch + #371 立场 ② + #376 §3.4 + #382 ⑤ 五源 byte-identical** | ❌ 不准 DM 行任何路径下出现拖拽 handle (defense-in-depth: omit 不 disable); ❌ 不准右键 DM 弹 "置顶" 菜单; ❌ 不准 DM 进 user_channel_layout 表 (server 端 #371 错码 `layout.dm_not_grouped` 兜底, 跟 #357 spec ③ + #354 ⑤ 同源) |
| ⑥ | **group 状态恢复** (页面重载后偏好持久) | 进 SPA 时 `GET /me/layout` 拉本人偏好 → 应用 `data-collapsed` + position 排序; 偏好缺失 → fallback 作者侧 `channel_groups.position` 顺序 (跟 #366 立场 ② "偏好缺失 = fallback 作者顺序" 同源); 不挂 push frame (跟 #366 ⑥ + #371 立场 ③ 同源) | ❌ 不准 `LayoutChangedFrame` push frame (RT-1 4 frame 已锁, 跟 #366 ⑥ + #371 §2 + #382 ⑤ 同源); ❌ 不准 client 端 IndexedDB 缓存优先于 GET /me/layout (避免多设备状态错位 — v3+ 才考虑离线缓存, 跟 #366 立场 ⑥ "v1 加 IndexedDB 缓存" 留账); ❌ 不准 fallback 用 `channel_groups.created_at` 时序 (用 position 显式排序) |

---

## 2. 反向 grep — CHN-3.x PR merge 后跑, 全部预期 0 命中 (除标 ≥1)

```bash
# ① 拖拽 handle aria-label byte-identical (a11y 永久锁, 预期 ≥1)
grep -rnE 'aria-label=["'"'"']拖拽调整顺序["'"'"']' packages/client/src/components/Sidebar.tsx 2>/dev/null | grep -v _test  # 预期 ≥1
# ① 拖拽 handle 同义词漂移防御
grep -rnE "['\"](Drag|拖动|排序|移动|Move)['\"]" packages/client/src/components/Sidebar.tsx 2>/dev/null | grep -v _test
# ② group 折叠 icon + data-collapsed 二态锁 (预期 ≥1)
grep -rnE 'data-collapsed=["'"'"'](true|false)["'"'"']' packages/client/src/components/Sidebar.tsx 2>/dev/null | grep -v _test  # 预期 ≥1
grep -rnE "['\"](Collapse|Expand|收起|展开)['\"]" packages/client/src/components/Sidebar.tsx 2>/dev/null | grep -v _test
# ③ pin 菜单 同义词漂移
grep -rnE "['\"](Pin|Unpin|固定|取消固定|Stick)['\"]" packages/client/src/components/Sidebar.tsx 2>/dev/null | grep -v _test
# ③ "置顶"/"取消置顶" 字面锁 (预期 ≥2 — 双菜单项)
grep -rnE '["'"'"']置顶["'"'"']|["'"'"']取消置顶["'"'"']' packages/client/src/components/Sidebar.tsx 2>/dev/null | grep -v _test  # 预期 ≥2
# ④ 失败 toast 字面 byte-identical (预期 ≥1, 跟 #371 spec §1 CHN-3.3 同源)
grep -rnE "['\"]侧栏顺序保存失败, 请重试['\"]" packages/client/src/ 2>/dev/null | grep -v _test  # 预期 ≥1
# ④ 失败 toast 同义词漂移
grep -rnE "['\"](保存失败|Save failed|请稍后重试|网络错误)['\"]" packages/client/src/components/Sidebar.tsx 2>/dev/null | grep -v _test
# ⑤ DM 行反约束 — 无拖拽 handle + 无 pin 菜单 (跟 #364 byte-identical 同源)
grep -rnE 'data-kind=["'"'"']dm["'"'"'].*data-sortable-handle|dm.*data-sortable' packages/client/src/components/ 2>/dev/null | grep -v _test
grep -rnE 'data-kind=["'"'"']dm["'"'"'].*data-context=["'"'"']channel-pin["'"'"']|dm.*channel-pin' packages/client/src/components/ 2>/dev/null | grep -v _test
# ⑥ 不挂 push frame (RT-1 4 frame 已锁)
grep -rnE 'LayoutChangedFrame|UserChannelLayoutChanged' packages/server-go/internal/ws/ 2>/dev/null | grep -v _test.go
```

---

## 3. 验收挂钩 (CHN-3.x PR 必带)

- ① 拖拽 handle e2e: hover channel 行 → DOM `data-sortable-handle` count≥1 + aria-label 字面 byte-identical + ⋮⋮ icon 锁
- ② group 折叠 e2e: 点击 ▶/▼ 切换 → `data-collapsed` 二态切换 + PUT /me/layout 写 collapsed
- ③ 右键 pin e2e: 右键 channel 行 → 菜单 "置顶"/"取消置顶" 字面 byte-identical + 反向 grep 同义词漂移 0 hit + 反向断言无 `pinned BOOL` 列
- ④ 拖拽失败 toast e2e: 模拟 PUT 失败 → toast `"侧栏顺序保存失败, 请重试"` byte-identical 字面 1.5s 显示 (跟 #371 + #376 §3.5 三源 byte-identical 同源)
- ⑤ DM 反约束 e2e: DM 行 hover → DOM `[data-kind="dm"] [data-sortable-handle]` count==0 + 右键 DM 行 → 菜单不含 "置顶" (跟 5 源 byte-identical 同根)
- ⑥ 偏好恢复 e2e: SPA reload → GET /me/layout → 状态恢复 (拖拽顺序 + 折叠状态) + 偏好缺失 fallback 作者顺序 + 反向断言无 push frame 触发
- G3.x demo 截屏 1 张归档: `docs/qa/screenshots/g3.x-chn3-sidebar-reorder.png` (跟 #391 §1 截屏路径锁 byte-identical 同源 — 验拖拽 handle + 折叠状态 + DM 行无 handle)

---

## 4. 不在范围

- ❌ 拖拽 handle 键盘快捷键 (a11y 增强留 v3+)
- ❌ 多选拖拽 (一行拖拽 v1 够用, 留 v3+)
- ❌ 跨 group 拖拽 (channel-group 关系是作者权, 跟 #366 立场 ② + #371 立场 ① 同源)
- ❌ DM 行加拖拽 / 加 pin (蓝图 §1.2 + #366 立场 ④ + 5 源 byte-identical 永久锁)
- ❌ 多设备实时同步偏好 (留 v3+, 跟 #366 立场 ⑦ "v1 加 IndexedDB 缓存留账" 同源)
- ❌ pin 上限 / 限制 (UI 自负责, 跟 #366 立场 ③ "个人 pin 数量不限" 同源)
- ❌ admin SPA 看用户偏好 (admin 不入业务路径, ADM-0 §1.3 红线 + #366 立场 ⑤)
- ❌ pinned BOOL 独立列 (蓝图无, 跟 #366 立场 ③ "pin 走 position 单调小数" + #371 §3 grep `pinned\s+BOOL` 0 hit 同源)

---

## 5. 更新日志

| 日期 | 作者 | 变化 |
|------|------|------|
| 2026-04-29 | 野马 | v0, 6 处文案锁 (拖拽 handle ⋮⋮ aria-label "拖拽调整顺序" + group 折叠 ▶/▼ data-collapsed 二态 + 右键 "置顶"/"取消置顶" + 失败 toast "侧栏顺序保存失败, 请重试" byte-identical 跟 #371 + #376 三源 + DM 反约束跟 #366/#364/#371/#376/#382 五源 byte-identical + 偏好恢复 GET 拉不挂 push frame) + 11 行反向 grep (含 5 预期 ≥1 + 6 反约束) + G3.x demo 截屏 1 张归档. #338 cross-grep 反模式遵守: 既有 Sidebar.tsx (#288) 作者侧字面已稳定不动, CHN-3 仅加个人偏好层字面, 跟既有 byte-identical 引用 |
