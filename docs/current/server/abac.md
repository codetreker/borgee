# AP-3 — abac cross-org owner-only gate (server 单一来源)

> **Canonical pointer.** Canonical helper at
> `packages/server-go/internal/auth/abac.go::HasCapability`. Schema in
> `packages/server-go/internal/migrations/ap_3_1_user_permissions_org.go`
> (v=29). Endpoint reach is unchanged (改 = 改 abac.go 一处, AP-1 shared helper
> same approach — endpoint 0 行改).

## Why

AP-1 #493 closes single-org ABAC (`HasCapability(ctx, perm, scope) bool`
+ 14-const capability whitelist + agent strict no-wildcard). AP-1 留作
之一 — cross-org enforcement — was deferred to AP-3 per
`auth-permissions.md` §5. AP-3 (战马C v0) adds **one** gate layer to
the existing shared helper without touching any endpoint code:
grantee `user.org_id` ≠ resource `org_id` ⇒ `HasCapability` returns
`false` immediately, regardless of explicit grants or `(*,*)` wildcards.

## 原则 (ap-3-spec.md §0)

- **① cross-org owner-only enforcement** — agent / user 跨 org 调
  channel/artifact 路径直接 false. 复用 `channel.org_id` + CV-1 设计 ①
  "artifact 归属 channel" + CM-3 #208 既有不变量 (artifact 跟 channel
  同 org).
- **② `user_permissions.org_id TEXT NULL` (兼容 AP-1)** — NULL = 历史数据
  / inheritance, 跟 user.org_id NULL = 历史数据 同精神 (跟 AP-1.1
  expires_at NULL = 永久 ALTER ADD COLUMN NULL 模式同源, 现网行为零变).
- **③ grep 检查 cross-org bypass 0 hit** — 跟 AP-1 #493 5 grep negative checks
  同模式守 future 脱节.

## Schema (v=29)

`ALTER TABLE user_permissions ADD COLUMN org_id TEXT` (nullable; not FK
to organizations — 跟 user.org_id 同精神, 业务校验 server 层) +
`CREATE INDEX idx_user_permissions_org_id ON user_permissions(org_id)
WHERE org_id IS NOT NULL` (sparse, 跟 AP-1.1 expires_at sparse 同模式).

Migration is forward-only via the framework's `schema_migrations` gate.
Existing rows preserve `org_id = NULL` (历史 inheritance).

## Gate semantics (`HasCapability`)

```
HasCapability(ctx, permission, scope) → bool
  user := UserFromContext(ctx)
  if user == nil → false

  // AP-3 cross-org gate (NEW) — 高于 wildcard 短路.
  if resourceOrgID, ok := resolveScopeOrgID(store, scope); ok {
    if user.OrgID != "" && user.OrgID != resourceOrgID → false
  }

  // AP-1 path (unchanged) — wildcard / explicit lookup.
  for p := range ListUserPermissions(user.ID) {
    if !isAgent && p == ("*","*") → true
    if p.permission == permission && (p.scope == "*" || p.scope == scope) → true
  }
  return false
```

`resolveScopeOrgID` parses scope strings:

- `"*"` → `("", false)` — wildcard, no resource bound, skip org gate.
- `"channel:<id>"` → `(channel.org_id, true)` if found and non-empty;
  else `("", false)`.
- `"artifact:<id>"` → resolves to `artifacts.channel_id` →
  `channel.org_id` (CV-1 设计 ① + CM-3 既有 invariant).
- Unknown prefix → `("", false)` — forward-compat to v2+ scope 层级
  扩展, skip org gate.

## NULL compatibility (设计 ⑥)

The gate enforces only when **both** sides have non-empty `org_id`:

- `user.OrgID == ""` (历史 AP-1 user) → skip gate, fall through to
  AP-1 path.
- `channel.OrgID == ""` (历史 AP-1 channel) → skip gate.
- Either side NULL/empty → AP-1 现网行为零变.

This matches the AP-1.1 expires_at + AP-3 org_id NULL = 历史
精神 across the whole milestone family.

## Error code (字面单一来源)

```go
const ErrCodeCrossOrgDenied = "abac.cross_org_denied"
```

`HasCapability` returns `false`; the calling endpoint returns its
existing 403 path (CV-1.2 commit handler / etc). The cross-org error
code is intended for future endpoint-level error response refinement
(handler can substring-match this const for cross-org explanation
text); for v0 the 403 path is unchanged from AP-1 既有 403 (改 =
改 abac.go 一处).

Drift between this const and handler hardcoded strings is caught by
reverse grep in tests + CI lint (跟 AP-1 const helper + AP-2 sweeper
const 同模式).

## Reverse grep 反向约束 (5 pattern, count==0)

```bash
git grep -nE 'cross.org.*bypass|skip.*org.*check|bypass.*org_id' \
  packages/server-go/internal/api/   # 0 hit
git grep -nE 'admin.*HasCapability.*\.org|HasCapability\(.*admin_' \
  packages/server-go/internal/api/   # 0 hit (admin god-mode 走
                                     # /admin-api/* 单独 mw, ADM-0 §1.3)
git grep -nE 'agent.*cross.*org.*permission|agent.*org_id.*ignore' \
  packages/server-go/internal/        # 0 hit (BPP-1 #304 既有 org
                                      # sandbox 同源)
git grep -nE 'user_permissions.*FOREIGN KEY.*organizations' \
  packages/server-go/internal/migrations/  # 0 hit (跟 user.org_id 同精神)
git grep -nE '"abac\.cross_org_denied"' packages/server-go/internal/  # ≥1
                                      # hit (auth/abac.go const) + 0 hit
                                      # hardcode in handler
```

CI lint runs equivalent unit tests via `filepath.Walk` (`abac_ap3_test.go::
TestAP32_ReverseGrep_NoCrossOrgBypass` + `TestAP31_ReverseGrep_
NoFKOrganizations`).

## capabilities-enum (AP-4-enum #待定)

`internal/auth/capabilities.go` 14 字面 const 单一来源外, 加 `var ALL = []string{
ReadChannel, WriteChannel, DeleteChannel, ReadArtifact, WriteArtifact,
CommitArtifact, IterateArtifact, RollbackArtifact, MentionUser, ReadDM,
SendDM, ManageMembers, InviteUser, ChangeRole}` ordered slice (顺序
literal match with const 声明) + `func init()` 自动 rebuild Capabilities map
(`for _, c := range ALL { Capabilities[c] = true }`) — 改 capability =
改 ALL **一处**, reflect-lint 单测自动守 ALL ↔ const 双向 ⊂.

Handler 不准直查 `auth.Capabilities[name]` map; 走 `auth.IsValidCapability(
name) bool` shared helper (跟 reasons.IsValid #496 helper 包 same pattern). check 走
`packages/server-go/internal/api/permission_reverse_grep_test.go` 3 个反向
grep 行为 test — `HasCapability("...")` hardcode in api/ count==0 +
`auth.Capabilities[` direct in api/ count==0 + `Capabilities["..."] =`
literal mutate in auth/ count==0 (init 走变量 c). 历史 release-gate.yml
里的 `ap4enum-no-hardcode-capability` yaml grep step 已随 #717 整治删除
(行为 test 替临时字符串 grep).

## 跨 milestone byte-identical 锁定

- AP-1 #493 `HasCapability` shared helper — AP-3 仅扩 helper 内部 (改 = 改
  abac.go 一处, endpoint 0 行改, capabilities.go 14 const 不动).
- AP-4-enum #待定 ALL slice + init + IsValidCapability — 复用 AP-1
  14 const 保持不动, 仅加 wrapper helper (跟 reasons.IsValid
  #496 helper 包 same approach).
- AP-1.1 #493 `user_permissions.expires_at` ALTER ADD COLUMN NULL 模式
  — AP-3.1 schema 同模式, 跟 AP-2.1 #ap-2 `revoked_at` 同模式三连.
- CM-3 #208 cross-org 资源归属 + CHN-1 #286 channel-org membership —
  artifact 走 `channel.org_id` resolution path.
- ADM-0 §1.3 admin god-mode 红线 — admin 不入此路径 (走 `/admin-api/*`
  单独 mw).
- BPP-1 #304 agent runtime org sandbox — agent path 走同 helper path (agent
  is user_id 一种).

## 不在范围

- v2 cross-org grant request UI (留 ADM-3+, server-side enforce + 错码
  已就位).
- ABAC condition (time-of-day / ip-range) v2+.
- multi-org user (单一 user 横跨 2+ org) v3+.
- cross-org admin god-mode (走 ADM-3+ `/admin-api/*` cross-org 强制).
