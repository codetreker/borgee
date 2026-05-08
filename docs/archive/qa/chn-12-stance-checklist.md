# CHN-12 stance checklist (战马D v0)

战马D · 2026-04-30 · 立场守门 (3+3 边界).

## §0 立场 3 项

- [x] **① 0 schema 改** — 复用 user_channel_layout.position REAL 既有列;
  反向 grep `migrations/chn_12_\d+\|ALTER user_channel_layout` 0 hit.
- [x] **② 0 server production code** — PUT /api/v1/me/layout (CHN-3.2 #357
  + CHN-3.3 #415 单调小数 acceptance §2.4) byte-identical 不动 (
  layout.invalid_payload + layout.dm_not_grouped + non-member 403 + 文案锁
  `侧栏顺序保存失败, 请重试`); git diff 0 行 production; 反向 grep
  `chn_12` 在 internal/api/*.go 非 _test.go 0 hit.
- [x] **③ 文案 byte-identical** — drag handle aria-label `拖拽排序` 4 字 +
  空态 hint `调整顺序` 4 字 byte-identical; 同义词反向 reject
  `移动/排序/拖动/抓握` 在 user-visible Chinese 0 hit.

## §0.边界 3 项

- [x] **④ 既有 server endpoint byte-identical** — handlePutMyLayout block
  内反向 grep `chn_12` 0 hit; 既有 5 字段反向锚 (DM reject + non-member
  403 + invalid_payload + 文案锁) byte-identical 不变.
- [x] **⑤ AL-1a reason 锁链不漂** — CHN-12 不引入新 reason (反向 grep
  `chn12.*reason\|reorder.*reason` 0 hit, 锁链停在 HB-6 #19);
  reorder per-user UI preference 不 audit (跟 CHN-7 mute / CHN-8
  notif-pref 立场 ⑥ 同精神).
- [x] **⑥ AST 锁链延伸第 20 处** — forbidden 3 token (`pendingReorder
  / reorderQueue / deadLetterReorder`) 0 hit.

## §1 测试

- [x] REG-CHN12-001 0 schema (`TestCHN121_NoSchemaChange`).
- [x] REG-CHN12-002 0 server prod (`TestCHN121_NoServerProductionCode`
  反向 grep `chn_12` 在 internal/api/*.go 非 _test.go production count==0).
- [x] REG-CHN12-003 既有 handlePutMyLayout block byte-identical
  (`TestCHN121_HandlerByteIdentical` 反向 grep chn_12 在 block 0 hit).
- [x] REG-CHN12-004 AST 锁链延伸第 20 处 (`TestCHN123_NoReorderQueue`).
- [x] REG-CHN12-005 client ChannelDragHandle + dnd-kit wiring + 单调小数
  computeReorderPosition 文案 byte-identical (`拖拽排序` / `调整顺序`)
  + 同义词反向 reject + vitest 5 case.
- [x] REG-CHN12-006 admin god-mode 不挂 reorder (反向 grep
  `admin-api/v[0-9]+/.*layout|admin-api/v[0-9]+/.*reorder` 0 hit).

## §2 反约束 grep 锚

- 0 schema: `migrations/chn_12_\d+|ALTER user_channel_layout` 0 hit.
- 0 server prod: `chn_12` 在 internal/api/*.go 非 _test.go 0 hit.
- 同义词反向 (user-visible): `移动|排序|拖动|抓握` 在 client user-visible
  Chinese text 0 hit (我们用 `拖拽排序/调整顺序`).
- AL-1a reason 锁链不漂: `chn12.*reason|reorder.*reason` 0 hit.
- AST 锁链延伸第 20 处: 3 forbidden token 0 hit.
- admin-rail 不挂: `admin-api/v[0-9]+/.*(layout|reorder)` 0 hit.
