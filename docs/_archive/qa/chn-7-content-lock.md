# CHN-7 content lock — mute UI 文案 + DOM (战马D v0)

> 战马D · 2026-04-30 · client SPA MuteButton / MutedChannelIndicator +
> 文案 byte-identical 锁. **关联**: spec `chn-7-spec.md` + stance
> `chn-7-stance-checklist.md` + acceptance `acceptance-templates/chn-7.md`.
> **承袭**: CHN-6 PinThreshold 双向锁模式 (server const + client const
> byte-identical, 改一处 = 改两处).

## §1 MuteButton DOM (byte-identical)

```tsx
<button
  className="btn btn-sm btn-mute"
  data-action={muted ? "unmute" : "mute"}
  onClick={handleClick}
>
  {muted ? "取消静音" : "静音"}
</button>
```

**字面锁** (vitest 反向 grep 守):
- `静音` 2 字 (未 mute) byte-identical
- `取消静音` 4 字 (已 mute) byte-identical
- `data-action="mute"` (未 mute → 用户点 mute)
- `data-action="unmute"` (已 mute → 用户点 unmute)

## §2 MutedChannelIndicator DOM (byte-identical)

```tsx
{muted && (
  <span
    className="muted-channel-indicator"
    data-testid="muted-channel-indicator"
    title="已静音"
  >🔕 已静音</span>
)}
```

**字面锁**:
- `已静音` 3 字 byte-identical
- `🔕` emoji byte-identical (跟 CHN-1.3 silent badge `🔕 silent` 同源
  emoji 复用 — 但文案不同: silent 用 `silent` 英, mute 用 `已静音` 中)
- `data-testid="muted-channel-indicator"` byte-identical
- 不静音状态 — 整个 `<span>` 不渲染 (return null)

## §3 反约束 — 同义词 reject

MuteButton + MutedChannelIndicator + 任何 mute 相关 UI 字面反向 reject:
- `mute` (English) — 反 reject (data-action 字面除外, 跟 user-visible
  text 拆死, vitest 检查 `<button>...mute</button>` 文本节点 0 hit)
- `silence` — 反 reject
- `dnd` (do not disturb 缩写) — 反 reject
- `disturb` — 反 reject
- `quiet` — 反 reject
- `屏蔽` — 反 reject
- `关闭通知` — 反 reject
- `勿扰` — 反 reject (DnD 中文)

## §4 const 双向锁 (server + client byte-identical)

| 端 | 字面 |
|---|---|
| server (Go) | `const MuteBit = 2` (api pkg, 字面单源) |
| client (TS) | `export const MUTE_BIT = 2;` (lib/mute.ts, 字面单源) |

**反约束**: 改一处 = 改两处. vitest + go test 双向编译期检查.
collapsed bitmap: bit 0 (=1) = 折叠态 (CHN-3 既有), bit 1 (=2) = 静音态
(CHN-7 新增). IsMuted(collapsed) 谓词调用 collapsed & MuteBit != 0;
client filter `(channel.collapsed ?? 0) & MUTE_BIT` byte-identical 跟
server IsMuted 同源.

## §5 toast 文案 byte-identical

| 触发 | toast 文案 |
|---|---|
| 静音成功 | (无 toast — UI indicator 出现即视觉反馈) |
| 取消静音成功 | (无 toast — UI indicator 消失即反馈) |
| 失败 | `静音失败` / `取消静音失败` byte-identical (跟 archive `归档失败` 同模式 — 操作 + 失败 二字) |
