# CHN-12 content lock — ChannelDragHandle + ChannelList + computeReorderPosition (战马D v0)

战马D · 2026-04-30 · client SPA drag handle + dnd-kit wiring + 单调小数算法
byte-identical 锁.

## §1 ChannelDragHandle DOM (byte-identical)

```tsx
<button
  type="button"
  className="channel-drag-handle"
  data-testid={`channel-drag-handle-${channelId}`}
  aria-label="拖拽排序"
  {...listeners}
  {...attributes}
>
  <svg viewBox="0 0 16 16" width="12" height="12">
    <circle cx="6" cy="3" r="1" />
    <circle cx="10" cy="3" r="1" />
    <circle cx="6" cy="8" r="1" />
    <circle cx="10" cy="8" r="1" />
    <circle cx="6" cy="13" r="1" />
    <circle cx="10" cy="13" r="1" />
  </svg>
</button>
```

**字面锁**:
- aria-label `拖拽排序` 4 字 byte-identical
- `data-testid="channel-drag-handle-{channelId}"` 行级锚 byte-identical
- 6-dot grip SVG (3 行 × 2 列 byte-identical)
- `{...listeners} {...attributes}` 来自 `useSortable({id: channelId})` (dnd-kit)

## §2 ChannelList dnd-kit wiring (byte-identical)

```tsx
<DndContext
  sensors={sensors}
  collisionDetection={closestCenter}
  onDragEnd={handleDragEnd}
>
  <SortableContext
    items={channels.map((c) => c.id)}
    strategy={verticalListSortingStrategy}
  >
    {channels.map((c) => <SortableChannelRow key={c.id} channel={c} />)}
  </SortableContext>
</DndContext>
```

**handleDragEnd 算法 byte-identical**:
```ts
function handleDragEnd(event: DragEndEvent) {
  const { active, over } = event;
  if (!over || active.id === over.id) return;
  const oldIdx = channels.findIndex((c) => c.id === active.id);
  const newIdx = channels.findIndex((c) => c.id === over.id);
  const reordered = arrayMove(channels, oldIdx, newIdx);
  // computeReorderPosition: prev/next 中点 → fallback prev+1.0 / next-1.0 / 1.0
  const prev = reordered[newIdx - 1]?.position ?? null;
  const next = reordered[newIdx + 1]?.position ?? null;
  const newPos = computeReorderPosition(prev, next);
  setMyLayout([{ channel_id: active.id as string, position: newPos }]);
}
```

## §3 computeReorderPosition 单调小数算法 byte-identical (跟 CHN-3.3 #415 同源)

```ts
// dnd_position.ts — exported pure function for unit test.
export function computeReorderPosition(
  prev: number | null,
  next: number | null,
): number {
  // 都 null — 列表唯一行 (实际 dnd 不会触发, 但保 fallback).
  if (prev === null && next === null) return 1.0;
  // 拖到末尾 — next null → prev+1.0.
  if (prev !== null && next === null) return prev + 1.0;
  // 拖到首位 — prev null → next-1.0.
  if (prev === null && next !== null) return next - 1.0;
  // 中间插入 — 两邻 position 中点 (REAL 单调小数 byte-identical).
  return (prev! + next!) / 2.0;
}
```

**数学锁** (跟 CHN-3.3 #415 acceptance §2.4 一致):
- 中点策略保 REAL 单调性 (永不撞 unique constraint, server 也无 unique
  on position — REAL 浮点足够分辨力).
- prev=null 边界 → next-1.0 (= 最小已用 position 减 1.0, 跟 CHN-3.3 同).
- next=null 边界 → prev+1.0 (= 最大已用 position 加 1.0, 跟 CHN-3.3 同).

## §4 反约束 — 同义词 reject

ChannelDragHandle + ChannelList 任何 user-visible 文本反向 reject:
- `move` (English) — 反 reject (data-testid + className 例外)
- `sort` (English) — 反 reject
- `reorder` — 反 reject
- `drag` (English) — 反 reject (我们用 `拖拽排序`)
- `移动` — 反 reject
- `排序` — 反 reject (我们用 `拖拽排序` 4 字一体, 不拆)
- `拖动` — 反 reject (我们用 `拖拽`)
- `抓握` — 反 reject

## §5 toast 文案 byte-identical

| 触发 | toast 文案 |
|---|---|
| 拖拽成功 | (无 toast — 视觉上 reorder 完成即反馈) |
| 保存失败 | `侧栏顺序保存失败, 请重试` byte-identical (跟 CHN-3.2 #357
  既有 layoutSaveErrorMsg 同源, server 端 5 源锚链延伸第 6 处) |
| DM 拖入 group | `DM 不参与个人分组` byte-identical (跟 CHN-3.2 既有
  layout.dm_not_grouped 同源 — 但 client 端预防式 disable handle, server
  guard 兜底) |

## §6 DM 反挂锁

DM channel 不渲染 `<ChannelDragHandle>` (UI 层 disable; server 端 layout.
dm_not_grouped 兜底). 反向 grep:
```ts
// SortableChannelRow.tsx:
{channel.type !== 'dm' && <ChannelDragHandle channelId={channel.id} />}
```
