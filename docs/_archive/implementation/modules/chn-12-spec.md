# CHN-12 spec brief — channel sort manual reorder (战马D v0)

> Phase 6 channel sidebar 手动拖拽排序闭环 — `PUT /api/v1/me/layout`
> 既有 batch upsert endpoint (CHN-3.2 #357) byte-identical 不动. 本 milestone
> 收尾: client SPA `ChannelDragHandle.tsx` + dnd-kit `<DndContext>` wiring +
> `position` 单调小数 client-side 算法 + 文案 byte-identical. **0 server
> production code** (跟 CHN-11 #563 / DM-6 #556 / CV-12 #545 系列 0-server
> 模式承袭).

## §0 立场 (3 + 3 边界)

- **①** 0 schema 改 (复用 user_channel_layout.position 既有 REAL 列). 反向 grep
  `migrations/chn_12_\d+\|ALTER user_channel_layout` 在 internal/migrations/
  0 hit.
- **②** **0 server production code** — `PUT /api/v1/me/layout` 既有 batch
  upsert (CHN-3.2 #357 / CHN-3.3 #415 单调小数 acceptance §2.4) byte-identical
  不动 (反向 grep `chn_12` 在 internal/api/*.go 非 _test.go production 0 hit).
  git diff main..HEAD -- 'packages/server-go/**/*.go' ':!**/*_test.go' 0 行.
- **③** 文案 byte-identical 锁: drag handle aria-label `拖拽排序` 4 字 +
  `调整顺序` 4 字 (空态 hint) byte-identical; 同义词反向 reject
  (`drag/sort/reorder/move/移动/排序/拖动/抓握`) 在 user-visible Chinese 0 hit.

边界:
- **④** 既有 server endpoint byte-identical — CHN-3.2 #357 既有 PUT /me/layout
  + DM reject `layout.dm_not_grouped` + non-member 403 + 文案锁
  `侧栏顺序保存失败, 请重试` 全套不动 (反向 grep `chn_12` 在 layout.go::
  handlePutMyLayout block 0 hit).
- **⑤** AL-1a reason 锁链不漂 — CHN-12 不引入新 reason (反向 grep
  `chn12.*reason\|reorder.*reason` 0 hit, 锁链停在 HB-6 #19).
  audit 5 字段链不延伸 — reorder 是 per-user UI preference, 不 audit
  (跟 CHN-7 mute / CHN-8 notif-pref 同精神, 立场 ⑥).
- **⑥** AST 锁链延伸第 20 处 forbidden 3 token (`pendingReorder /
  reorderQueue / deadLetterReorder`) 在 internal/api 0 hit.

## §1 拆段

**CHN-12.1 — schema**: 0 行 (复用 user_channel_layout.position).

**CHN-12.2 — server**: **0 行 production** — 既有 PUT /api/v1/me/layout
(CHN-3.2 #357) byte-identical 不动. 反向 grep守门 test
(`chn_12_no_server_prod_test.go::TestCHN121_NoServerProductionCode`).

**CHN-12.3 — client**:
- `lib/dnd_position.ts::computeReorderPosition(prev, next)` — 单调小数算法
  byte-identical 跟 CHN-3.3 #415 同源 (prev/next 取相邻 position 中点;
  prev=null → next-1.0; next=null → prev+1.0; 都 null → 1.0).
- `components/ChannelDragHandle.tsx` — drag handle 图标 + aria-label `拖拽排序`.
- `components/ChannelList.tsx` — `<DndContext>` `<SortableContext>` wiring +
  `onDragEnd` → computeReorderPosition → `setMyLayout([{channel_id, position}])`
  既有 lib/api wrapper.
- 既有 `lib/api.ts::setMyLayout` thin wrapper for PUT /me/layout 既有 (无新 wrapper).

**CHN-12.4 — closure**: REG-CHN12-001..006 6 🟢. AST 锁链延伸第 20 处.

## §2 反约束 grep 锚

- 0 schema: `migrations/chn_12_\d+\|ALTER user_channel_layout` 0 hit.
- 0 server prod: `git diff main..HEAD -- 'packages/server-go/**/*.go'
  ':!**/*_test.go'` 0 行 + 反向 grep `chn_12` 在 internal/api/*.go 非
  _test.go 0 hit.
- 既有 endpoint byte-identical: handlePutMyLayout block 内反向 grep
  `chn_12` 0 hit.
- 同义词反向 (user-visible Chinese): `移动/排序/拖动/抓握` 在 client/src/ user-
  visible Chinese text 0 hit (drag handle aria-label 锁定 `拖拽排序` /
  empty-state hint `调整顺序`).
- AL-1a reason 锁链不漂: `chn12.*reason\|reorder.*reason` 0 hit.
- AST 锁链延伸第 20 处 forbidden 3 token 0 hit.

## §3 不在范围

- group reorder (CHN-9 group ordering 已分别 milestone, 本 milestone 仅
  channel-within-section level).
- bulk reorder API (留 v3 — server PUT /me/layout 已 batch 单源).
- pinned section drag-into-pinned (留 v3 — pinned 仅 CHN-6 toggle path).
- DM reorder (永不 — DM 不参与分组立场 ④ byte-identical 跟 CHN-3.2 #357).
- 跨用户共享 sort (永不 — per-user preference 立场 ②).
- admin god-mode reorder override (永远不挂 ADM-0 §1.3 红线 — sort 是
  per-user preference).
- audit log entry per reorder (留 v3 — 跟 CHN-7 mute / CHN-8 notif-pref
  立场 ⑥ "per-user preference 不入 admin_actions" 同精神).
