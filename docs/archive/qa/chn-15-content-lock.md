# CHN-15 Content Lock — readonly 文案 + DOM byte-identical 锁 (野马 v0)

> 战马C · 2026-04-30 · CHN-15 channel readonly toggle 文案 + DOM 字面锁
> 跟 CHN-7 mute / CHN-9 visibility / DM-8 bookmark 4 件套同模式.

## §1 ReadonlyToggle 文案锁 (3 字面 byte-identical)

字面 (改 = 改三处: client component + 此 content-lock + 测试文件):

```
readonly.set_toast        → "已设为只读"
readonly.unset_toast      → "已恢复编辑"
readonly.no_send_reject   → "只读频道, 仅创建者可发言"
```

**反向 grep** (count==0): `frozen|locked|lockdown|lock-down|禁言|封禁|冻结|locked_channel|channel_locked` 在 `packages/client/src/` + `packages/server-go/internal/` user-visible 文案.

## §2 ReadonlyToggle + ReadonlyBadge DOM 字面锁

### §2.1 ReadonlyToggle (owner-only render)

```html
<button
  type="button"
  class="readonly-toggle"
  data-testid="readonly-toggle"
  data-readonly="true|false"
  title="{readonly ? '已恢复编辑' : '已设为只读'}"
  aria-pressed="true|false"
>
  {readonly ? '已恢复编辑' : '已设为只读'}
</button>
```

`data-readonly` 双 enum byte-identical 跟 server `IsReadonly(channels.created_by collapsed bit 4)` 1:1 映射.

### §2.2 ReadonlyBadge (readonly=true only render)

```html
<span
  class="readonly-badge"
  data-testid="readonly-badge"
  aria-label="只读频道"
>
  只读
</span>
```

readonly=false → return null.

## §3 错码字面单源 (server const ↔ client toast 双向锁)

`internal/api/chn_15_readonly.go::ChannelErrCodeReadonlyNoSend` const +
`packages/client/src/lib/api.ts::CHANNEL_READONLY_TOAST` map 双向锁:

```ts
export const CHANNEL_READONLY_TOAST: Record<string, string> = {
  'channel.readonly_no_send':  '只读频道, 仅创建者可发言',
};
```

**改 = 改三处** (server const + client map + 此 content-lock).

## §4 跨 PR drift 守

改 3 文案 / 1 错码 / DOM data-* attrs = 改五处:
1. `internal/api/chn_15_readonly.go::ChannelErrCodeReadonlyNoSend` const
2. `packages/client/src/lib/api.ts::CHANNEL_READONLY_TOAST` map (1 字面)
3. `packages/client/src/lib/readonly.ts::READONLY_LABEL` 3 文案 const
4. `components/ReadonlyToggle.tsx` + `ReadonlyBadge.tsx` (DOM data-* + 文案使用)
5. 此 content-lock §1+§2+§3

## §5 admin god-mode 红线

readonly 是 channel-level state, 但仅 channel.created_by 可改. admin-rail 0 endpoint, 反向 grep `/admin-api/.*readonly|admin.*readonly.*PATCH` 在 internal/ count==0. 跟 CHN-7 mute / DM-8 bookmark 同 admin god-mode 不入业务路径同精神.
