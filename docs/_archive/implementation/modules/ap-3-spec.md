# AP-3 spec brief — cross-org owner-only 强制 (Phase 5+ 续作)

> 战马C · 2026-04-29 · ≤80 行 spec lock (4 件套之一; AP-1 #493 留账之一 wrapper milestone)
> **蓝图锚**: [`auth-permissions.md`](../../blueprint/current/auth-permissions.md) §1.2 (Scope 层级 v1 三层 — `*` / `channel:<id>` / `artifact:<id>`) + §1.3 主入口 + [`channel-model.md`](../../blueprint/current/channel-model.md) §1.4 (channel.created_by = owner 主权列) + [`auth-permissions.md`](../../blueprint/current/auth-permissions.md) §5 与现状的差距 ("cross-org 强制 — AP-3 后续 milestone")
> **关联**: AP-1 #493 ABAC HasCapability SSOT + capabilities.go 14 const ✅ + AP-1.1 #493 user_permissions.expires_at 列 ✅ + CHN-1 #286 channel-org membership + CM-3 #208 cross-org 资源归属 + ADM-0 §1.3 admin god-mode 红线
> **命名**: AP-1 已落 (单组织内 ABAC + capability 白名单 + 严格 403); AP-3 接 AP-1 留账 cross-org 边界 (跨 organization 不发 capability, 反向: agent 无 cross-org permission); AP-2 名占给 AP-1.bis 留账 (expires_at 业务化 v2+, ADM-0 sweeper)

> ⚠️ AP-3 是 **wrapper milestone** (跟 AL-5 #cv-2-v2 wrapper 同模式) — 复用既有 AP-1 HasCapability SSOT + capabilities.go + CM-3 cross-org 资源归属, 仅补 cross-org enforcement 路径, **不裂新组件**, 不另起 ABAC 状态机.

## 0. 关键约束 (3 条立场, 蓝图字面承袭)

1. **cross-org owner-only 强制** (蓝图 `auth-permissions.md` §5 + `channel-model.md` §1.4 + CM-3 #208 字面承袭): cross-org capability check 必走 `org_id` 同源闸 — `agent.org_id` / `user.org_id` ≠ resource.org_id (channel / artifact 所属 org) → `HasCapability` 直返 false (跟 AP-1 立场 ② SSOT 同源 helper 加 1 层 org gate); 反约束: 不开 cross-org grant 路径 (跟 AP-1 立场 ③ wildcard 收窄精神同, agent 无 cross-org permission); admin god-mode 走 `/admin-api/*` 单独 mw 不入此路径 (ADM-0 §1.3 红线)
2. **user_permissions 加 org_id scope 字段** (跟 AP-1 schema 兼容, ALTER ADD COLUMN NULL 模式同 AP-1.1 #493 expires_at): `user_permissions.org_id TEXT NULL` — NULL = legacy 行 (跟 user.org_id 同 inheritance, 不破 AP-1 现状 ABAC 行为); cross-org check 路径 grant 时显式写 `org_id = grantee.org_id`; sparse index `idx_user_permissions_org_id WHERE org_id IS NOT NULL` (跟 expires_at 同模式); 反约束: schema 不挂 FK org_id → organizations(id) (跟 user.org_id 同精神 — 跨表 FK 业务校验 server 层做, 蓝图 §5 留账)
3. **反约束 grep cross-org bypass 0 hit** (跟 AP-1 #493 5 grep 反约束同模式守 future drift): 反向 grep `cross.org.*bypass|skip.*org.*check|admin.*HasCapability.*\.org` 在 `internal/api/` count==0; agent runtime 路径走 `agent.org_id` gate (BPP-1 #304 既有 org sandbox 同源, 反向 grep `agent.*cross.*org.*permission` 0 hit, BPP-1 envelope whitelist 不裂)

## 1. 拆段实施 (AP-3.1 / 3.2 / 3.3, ≤3 PR 同 branch 叠 commit, 一 milestone 一 PR 协议下默认 1 PR)

| 段 | 范围 | 闭锁 | owner |
|---|---|---|---|
| **AP-3.1** schema migration v=N + cross-org enforce const | `internal/migrations/ap_3_1_user_permissions_org_id.go` (ALTER ADD COLUMN `user_permissions.org_id TEXT NULL` + `idx_user_permissions_org_id WHERE org_id IS NOT NULL` sparse index, 跟 AP-1.1 expires_at 同模式 ALTER); `internal/auth/capabilities.go` 加 const `ErrCodeCrossOrgDenied = "abac.cross_org_denied"` 错码字面 (跟 AP-1 既有 const 单源同模式); 5 unit (TestAP31_AddsOrgIDColumn + RejectsNonOrgRow + Idempotent + LegacyRowsNullPreserved + IndexExists) | 待 PR (战马C) | 战马C / 烈马 |
| **AP-3.2** server `abac.HasCapability` 加 org check + endpoint enforce | `internal/auth/abac.go::HasCapability` 加 1 层 org gate — 取 grantee `user.org_id` (cached on user struct); 取 resource org (channel / artifact 所属 channel → channel.org_id, 蓝图 channel-model §1 既有); cross-org → false; v0 stance: org gate 仅当 grantee 跟 resource 都有 org_id 时 enforce, NULL 行走 legacy 路径 (兼容 AP-1 现网); endpoint 受影响: `/api/v1/channels/:id/*` + `/api/v1/artifacts/:id/*` + `/api/v1/messages` (走 channel.org_id) — 全走既有 HasCapability 单源, **不改 endpoint 代码** (跟 AP-1 立场 ② SSOT 同源, 改 = 改 abac.go 一处); 6 unit + 反向 grep | 待 PR (战马C) | 战马C / 烈马 |
| **AP-3.3** e2e + closure | server-side full-flow integration: org-A user grant capability `write_artifact channel:<id>` → org-A artifact OK + org-B 同 capability artifact reject 403 + `abac.cross_org_denied` 错码字面; agent path 同 (agent in org-A 调 org-B artifact endpoint → 403); admin god-mode 不入 (反向 grep `admin.*HasCapability.*org` count==0); registry §3 REG-AP3-001..N + acceptance + PROGRESS [x] AP-3 + docs/current sync (server/auth.md §cross-org + blueprint auth-permissions.md §5 cross-org 字面承袭) | 待 PR (战马C) | 战马C / 烈马 |

## 2. 留账边界 (不接 v2+)

- v2 cross-org grant request UI (留 ADM-3+) — AP-3 仅 server-side enforce + 错码; cross-org 显式授权 UI 走 admin god-mode (ADM-3 cross-org admin recover 同精神)
- AP-1.bis expires_at 业务化 (留 ADM-0 sweeper v2+) — 跟 AP-3 解耦, expires_at sweep 是 cron 路径
- ABAC condition (e.g. `time-of-day`/`ip-range`) v2+ — 蓝图 §5 留账, AP-3 仅 org gate 一层
- multi-org user (同一 user 横跨 2+ org) v3+ — v1 假设 user.org_id 单值 (CM-1 #184 字面)
- cross-org admin god-mode 路径 (走 ADM-3+ 既有 `/admin-api/*` cross-org 强制, 不入此 milestone)

## 3. 反查 grep 锚 (5 反约束, count==0)

```bash
# 1) cross-org bypass — 反向 ad-hoc skip path
git grep -nE 'cross.org.*bypass|skip.*org.*check|bypass.*org_id' \
  packages/server-go/internal/api/  # 0 hit
# 2) admin HasCapability 走 ABAC (反 admin god-mode 入业务 ABAC)
git grep -nE 'admin.*HasCapability.*\.org|HasCapability\(.*admin_' \
  packages/server-go/internal/api/  # 0 hit (admin 走 /admin-api 单独 mw)
# 3) agent cross-org permission 路径 (BPP-1 #304 既有 org sandbox 同源)
git grep -nE 'agent.*cross.*org.*permission|agent.*org_id.*ignore' \
  packages/server-go/internal/  # 0 hit
# 4) user_permissions FK org_id (反, 不挂 FK, 蓝图 §5 业务校验 server 层)
git grep -nE 'user_permissions.*FOREIGN KEY.*organizations' \
  packages/server-go/internal/migrations/  # 0 hit (跟 user.org_id 同精神)
# 5) cross-org 错码字面单源 (反 hardcode error string)
git grep -nE '"abac\.cross_org_denied"' packages/server-go/internal/  # ≥1 hit (capabilities.go const) + 0 hit hardcode in handler
```

## 4. 不在范围

- v2 cross-org grant UI (ADM-3+)
- AP-1.bis expires_at 业务化 sweeper (留 ADM-0+)
- ABAC condition (time/ip/etc) (留 v2+)
- multi-org user v3+
- cross-org admin god-mode (走 `/admin-api/*` ADM-3+)

## 5. 跨 milestone byte-identical 锁

- 跟 AP-1 #493 HasCapability SSOT + capabilities.go 14 const 同源 (改 = 改 abac.go + capabilities.go 两处, AP-3 仅加 org gate + 1 错码 const)
- 跟 AP-1.1 #493 user_permissions ALTER ADD COLUMN NULL 模式 (改 = 改 schema 一处)
- 跟 CM-3 #208 cross-org 资源归属 + CHN-1 #286 channel-org membership 同源 (改 = 改 channel-org gate 一处, AP-3 复用)
- 跟 ADM-0 §1.3 红线 admin god-mode 不入业务路径同源 (cross-org admin recover 走 ADM-3+ `/admin-api/*`)
- 跟 BPP-1 #304 agent runtime org sandbox 同源 (agent.org_id gate, 反向 grep `agent.*cross.*org` 0 hit)
