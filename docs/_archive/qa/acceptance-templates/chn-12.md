# CHN-12 acceptance — channel sort manual reorder

战马D · 2026-04-30 · spec `chn-12-spec.md` + stance.

## §1 schema / server

- §1.1 ✅ 0 schema 改 (复用 user_channel_layout.position REAL).
- §1.2 ✅ 0 server production code — PUT /api/v1/me/layout CHN-3.2 #357
  既有 path byte-identical 不动.
- §1.3 ✅ 既有 layout.invalid_payload + layout.dm_not_grouped + non-member
  403 + 文案锁 `侧栏顺序保存失败, 请重试` byte-identical 不变.

## §2 client ChannelDragHandle + dnd-kit + computeReorderPosition

- §2.1 ✅ ChannelDragHandle aria-label `拖拽排序` 4 字 byte-identical.
- §2.2 ✅ 空态 hint `调整顺序` 4 字 byte-identical.
- §2.3 ✅ computeReorderPosition 单调小数算法 byte-identical 跟 CHN-3.3 #415
  同源 (prev/next 中点 → fallback prev+1.0 / next-1.0 / 1.0).
- §2.4 ✅ ChannelList `<DndContext>` `<SortableContext>` wiring + onDragEnd
  → setMyLayout 既有 wrapper.
- §2.5 ✅ 同义词反向 reject (`移动/排序/拖动/抓握`).
- §2.6 ✅ DM channel 不挂 drag handle (DM 不参与分组反向 grep 锚).

## §3 反约束

- §3.1 ✅ 0 schema.
- §3.2 ✅ 0 server prod.
- §3.3 ✅ 既有 handlePutMyLayout block byte-identical.
- §3.4 ✅ admin god-mode 不挂 (反向 grep admin-api/v1/.../layout 0 hit).
- §3.5 ✅ AL-1a reason 锁链不漂 (停在 HB-6 #19).
- §3.6 ✅ AST 锁链延伸第 20 处.

## §4 测试矩阵

- TestCHN121_NoSchemaChange ✅
- TestCHN121_NoServerProductionCode ✅
- TestCHN121_HandlerByteIdentical ✅
- TestCHN123_NoReorderQueue ✅
- TestCHN123_NoAdminLayoutPath ✅
- ChannelDragHandle.test.tsx 5 vitest ✅
- dnd_position.test.ts unit (computeReorderPosition 5 case) ✅
