# CHN-6 content lock — pin/unpin UI 文案 + DOM (战马D v0)

> 战马D · 2026-04-30 · client SPA PinButton / PinnedChannelsSection +
> 文案 byte-identical 锁. **关联**: spec `chn-6-spec.md` + stance
> `chn-6-stance-checklist.md` + acceptance `acceptance-templates/chn-6.md`.

## §1 PinButton DOM (byte-identical)

```tsx
<button
  className="btn btn-sm btn-pin"
  data-action={pinned ? "unpin" : "pin"}
  onClick={handleClick}
>
  {pinned ? "取消置顶" : "置顶"}
</button>
```

**字面锁** (vitest 反向 grep 守):
- `置顶` 1 字 (未 pin 状态) byte-identical
- `取消置顶` 3 字 (已 pin 状态) byte-identical
- `data-action="pin"` (未 pin → 用户点 pin)
- `data-action="unpin"` (已 pin → 用户点 unpin)
- 反向 reject 同义词 (`收藏 / 标星 / star / favorite / top / 顶置 / 钉住`)

## §2 PinnedChannelsSection DOM (byte-identical)

```tsx
<section className="pinned-channels-section" data-testid="pinned-channels-section">
  <header className="pinned-channels-header">已置顶频道</header>
  <ul className="pinned-channels-list">
    {pinned.map(ch => (
      <li key={ch.id} className="pinned-channel-item" data-pinned="true">
        <span className="channel-name">#{ch.name}</span>
      </li>
    ))}
  </ul>
</section>
```

**字面锁**:
- header 字面 `已置顶频道` 4 字 byte-identical
- `data-testid="pinned-channels-section"` byte-identical
- `data-pinned="true"` 行级 byte-identical
- empty state — 无 pin 时整个 section **不渲染** (return null)
- filter 表达式 `channel.position < POSITION_PIN_THRESHOLD` byte-identical
  跟 server PinThreshold 双向锁

## §3 反约束 — 同义词 reject

PinButton + PinnedChannelsSection + 任何 pin 相关 UI 字面反向 reject:
- `收藏` (favorite Chinese) — 反 reject
- `标星` (star Chinese) — 反 reject
- `star` (English) — 反 reject
- `favorite` (English) — 反 reject
- `top` 单字英 — 反 reject (跟 `顶置` 区分; 我们只用 `置顶` 不用 `顶置`)
- `顶置` (反义词) — 反 reject (中文同义但字序差, 我们用 `置顶`)
- `钉住` (pin Chinese alt) — 反 reject (我们用 `置顶`)

## §4 const 双向锁 (server + client byte-identical)

| 端 | 字面 |
|---|---|
| server (Go) | `const PinThreshold = 0.0` (auth pkg / store pkg, 字面单源) |
| client (TS) | `export const POSITION_PIN_THRESHOLD = 0;` (lib/pin.ts, 字面单源) |

**反约束**: 改一处 = 改两处. vitest + go test 双向编译期检查.
client 端 filter `channel.position < POSITION_PIN_THRESHOLD` byte-
identical; server 端 IsPinned(position) 谓词调用 PinThreshold.
