# CHN-8 content lock — notification pref dropdown 文案 + DOM (战马D v0)

> 战马D · 2026-04-30 · client SPA NotificationPrefDropdown 文案 + DOM
> byte-identical 锁. **关联**: spec `chn-8-spec.md` + stance + acceptance.
> **承袭锚**: CHN-7 #550 mute UX (`静音` / `已静音`) + CHN-6 #544
> PinThreshold 双向锁模式 + DL-4 push gateway pref skip best-effort.

## §1 NotificationPrefDropdown DOM (byte-identical)

```tsx
<select
  className="notif-pref-dropdown"
  data-testid="notification-pref-dropdown"
  value={pref}
  onChange={(e) => onChange(e.target.value as NotifPref)}
>
  <option value="all">所有消息</option>
  <option value="mention">仅@提及</option>
  <option value="none">不打扰</option>
</select>
```

**字面锁** (vitest 反向 grep 守):
- option value `all` / `mention` / `none` byte-identical (跟 server 三态)
- 文案 `所有消息` 4 字 (option all)
- 文案 `仅@提及` 4 字 (option mention, 含 `@` 符号)
- 文案 `不打扰` 3 字 (option none)
- `data-testid="notification-pref-dropdown"` byte-identical

## §2 反约束 — 同义词 reject

NotificationPrefDropdown + 任何 notif pref 相关 UI 字面反向 reject:
- `subscribe` / `unsubscribe` (English) — 反 reject
- `follow` / `unfollow` — 反 reject
- `snooze` — 反 reject (snooze 是临时暂停语义, 跟 pref 三态拆死)
- `订阅` / `取消订阅` (Chinese subscribe) — 反 reject
- `关注` / `取消关注` (Chinese follow) — 反 reject
- `dnd` / `disturb` / `quiet` — 反 reject (CHN-7 mute 已用同义词反向)

## §3 const 三向锁 (server + client + bitmap byte-identical)

| 端 | 字面 |
|---|---|
| server (Go) | `const NotifPrefShift = 2` / `const NotifPrefMask = 3` / `const NotifPrefAll = 0` / `NotifPrefMention = 1` / `NotifPrefNone = 2` |
| client (TS) | `export const NOTIF_PREF_SHIFT = 2;` / `NOTIF_PREF_MASK = 3;` / `NOTIF_PREF_ALL = 0;` / `NOTIF_PREF_MENTION = 1;` / `NOTIF_PREF_NONE = 2;` |
| bitmap | `(collapsed >> 2) & 3` 字面拆 bits 2-3 |

**反约束**:
- 改一处 = 改三处 (server const + client const + bitmap 计算式).
- 反向 reject `(collapsed >> 2) & 3 == 3` (3 reserved/invalid → 跟 server
  SetNotifPref 入参 reject 同源).
- pref ∈ {0, 1, 2}, 字面 byte-identical 跟 NotifPrefAll/Mention/None.

## §4 字符串映射 byte-identical (server + client)

| pref (int) | server const | client const | API string | UI 文案 |
|---|---|---|---|---|
| 0 | NotifPrefAll | NOTIF_PREF_ALL | `"all"` | `所有消息` |
| 1 | NotifPrefMention | NOTIF_PREF_MENTION | `"mention"` | `仅@提及` |
| 2 | NotifPrefNone | NOTIF_PREF_NONE | `"不打扰"` 文案上 — API string 仍 `"none"` | `不打扰` |
| 3 | (reserved) | (reserved) | (反 400 invalid_value) | — |

## §5 toast 文案 byte-identical

| 触发 | toast 文案 |
|---|---|
| 设置成功 | (无 toast — UI dropdown 本身视觉反馈) |
| 设置失败 | `通知偏好设置失败` byte-identical (跟 mute `静音失败` 同模式 — 操作 + 失败 拼接) |
