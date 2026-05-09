# HostGrantsPanel — HB-3.3 弹窗 UX

> **单一来源 pointer.** Component
> `packages/client/src/components/HostGrantsPanel.tsx` + tests
> `packages/client/src/__tests__/HostGrantsPanel.test.tsx`. Server side
> in `docs/current/server/api/host-grants.md`.

## Why

HB-3 ships the host授权弹窗 UX (蓝图 host-bridge.md §1.3 字面: "DevAgent
想读你的代码目录 ~/code … [拒绝][仅这一次][始终允许]"). The component
is invoked when an agent needs runtime access to a host resource
(filesystem read / network egress) and the daemon has no matching
active grant.

## DOM ↔ DB enum 双向锁定 (content-lock §1.①+§1.②)

| Button label | data-action          | data-hb3-button | DB ttl_kind |
|--------------|----------------------|-----------------|-------------|
| 拒绝         | `deny`               | `danger`        | (no row written) |
| 仅这一次     | `grant_one_shot`     | `primary`       | `one_shot`  |
| 始终允许     | `grant_always`       | `primary`       | `always`    |

`data-action` literal maps to ttl_kind enum byte-identical:
`grant_one_shot` ↔ `one_shot`, `grant_always` ↔ `always`. Changing
either side requires touching three places (this doc + content-lock
§1.① + the React component + the DB CHECK constraint = effectively
**改 = 改三处单测锁定**).

## actionLabel 4-enum (蓝图 §1.3 弹窗 UX 模板)

actionLabel maps grant_type → 中文动词字面. Used in popup title +
body. **改 = 改两处** (this doc + `HostGrantsPanel.tsx::actionLabel`):

| grant_type   | actionLabel (中文动词) | DB CHECK enum |
|--------------|-----------------------|---------------|
| `install`    | 安装                  | `install`     |
| `exec`       | 执行                  | `exec`        |
| `filesystem` | 读取                  | `filesystem`  |
| `network`    | 访问                  | `network`     |

## Title + body templates (字面跟蓝图 §1.3)

```
title: "{agentName} 想{actionLabel}你的{scopeLabel}"
body:  "原因: {agentName} 配置中的「{capabilityLabel}」能力\n      需要{actionLabel}{scopeLabel}"
```

## Props

```ts
interface HostGrantsPanelProps {
  agentName: string;            // e.g. "DevAgent"
  grantType: 'install' | 'exec' | 'filesystem' | 'network';
  scopeLabel: string;           // e.g. "代码目录 ~/code"
  capabilityLabel: string;      // e.g. "代码 review"
  onDecide: (action: 'deny' | 'grant_one_shot' | 'grant_always') => void;
}
```

`onDecide` is called once with the literal data-action string. The
caller is responsible for translating to the server REST call:

```
deny             → no API call (popup closes, daemon stays denied)
grant_one_shot   → POST /api/v1/host-grants {grant_type, scope, ttl_kind: "one_shot"}
grant_always     → POST /api/v1/host-grants {grant_type, scope, ttl_kind: "always"}
```

## API contract reference

POST/GET/DELETE endpoints documented in
[`docs/current/server/api/host-grants.md`](../server/api/host-grants.md).
Owner-only ACL (出处 #360 同模式) — caller writes own grants;
admin god-mode 不入路径 (用户主权, ADM-0 §1.3 红线).

## Reverse-grep守门 (content-lock §2)

Forbidden synonyms (≥10 同义词grep 检查 0 hit, CI lint 守):

- 拒绝 同义词: 否决 / 不允许 / reject / deny() / 拒绝授权 → 0 hit
- 仅这一次 同义词: 单次 / 临时 / once / transient → 0 hit (注: "仅这一次"
  含 "一次" 子串故 substring grep 仅按 attr 集查 enum 单一来源)
- 始终允许 同义词: 永久 / 长期 / forever / permanent / persistent → 0 hit
- data-action 仅三值: `deny | grant_one_shot | grant_always`
- data-hb3-button 仅二值: `danger | primary`

Test file `HostGrantsPanel.test.tsx` (5 vitest cases) enforces all
above + `onDecide` 三值回调 + actionLabel 4-enum byte-identical.

## Adding a new grant_type

1. Update server-side `host_grants.go::hostGrantTypeWhitelist` map +
   migration CHECK constraint (forward-only ALTER).
2. Update `HostGrantsPanel.tsx::actionLabel` (中文动词).
3. Update content-lock §1 + spec §1 + this doc byte-identical.
4. Add a vitest case to `HostGrantsPanel.test.tsx::actionLabel 4-enum`
   covering the new enum literal.
5. CI lint catches 脱节 via reflect (`TestHB31_GrantTypeEnumReject`)
   + DOM data-action enumeration.
