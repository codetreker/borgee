# Acceptance Template — AP-3: cross-org owner-only 强制 wrapper milestone

> Spec: `docs/implementation/modules/ap-3-spec.md` (战马C v0, d69b617)
> 蓝图: `auth-permissions.md` §5 cross-org 留账 + `channel-model.md` §1.4 主权 + CM-3 #208 cross-org 资源归属
> 前置: AP-1 #493 HasCapability SSOT + capabilities.go 14 const ✅ + AP-1.1 #493 user_permissions.expires_at ALTER ADD COLUMN NULL 模式 ✅ + CM-3 #208 cross-org 资源归属 ✅ + ADM-0 §1.3 admin god-mode 红线
> Owner: 战马C (主战) + 飞马 (spec) + 烈马 (验收)

## 验收清单

### AP-3.1 schema migration v=29 + cross-org error const

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 1.1 `ALTER TABLE user_permissions ADD COLUMN org_id TEXT` (NULL nullable, 跟 AP-1.1 expires_at 同模式 — 不挂 default / NOT NULL / FK organizations(id)); INSERT 接 NULL legacy 行 + INSERT with org_id 显式行 | unit | 战马C / 烈马 | `internal/migrations/ap_3_1_user_permissions_org_test.go::TestAP31_AddsOrgIDColumn` + `TestAP31_LegacyRowsNullPreserved` |
| 1.2 `CREATE INDEX idx_user_permissions_org_id ON user_permissions(org_id) WHERE org_id IS NOT NULL` sparse index (跟 expires_at 同模式) | unit | 战马C / 烈马 | `TestAP31_HasOrgIDIndex` (sqlite_master 反向 + WHERE 子句 byte-identical 锚) |
| 1.3 `auth.ErrCodeCrossOrgDenied = "abac.cross_org_denied"` const 字面单源 (跟 AP-1 capabilities.go const 同模式) | unit | 战马C / 烈马 | `internal/auth/abac_test.go::TestAP32_ErrCodeCrossOrgDeniedConst` (字面 byte-identical) |
| 1.4 idempotent re-run guard (AP-1.1 expires_at ALTER 同模式, schema_migrations 框架守) | unit | 战马C / 烈马 | `TestAP31_Idempotent` |

### AP-3.2 abac.HasCapability 加 org gate

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 2.1 cross-org user `HasCapability` reject — user.org_id ≠ resource.org_id → false (即使有 wildcard `(*,*)` 也 reject; cross-org 闸高于 wildcard 短路) | unit | 战马C / 烈马 | `abac_test.go::TestAP32_CrossOrgUser_Rejected` + `TestAP32_CrossOrg_WildcardDoesNotShortCircuit` |
| 2.2 same-org user `HasCapability` 接受 — user.org_id == resource.org_id + permission match → true (跟 AP-1 既有 wildcard / explicit 路径完全兼容) | unit | 战马C / 烈马 | `TestAP32_SameOrgUser_PermissionGranted` |
| 2.3 cross-org agent `HasCapability` reject — agent.org_id ≠ resource.org_id → false (BPP-1 #304 agent runtime org sandbox 同源, AP-1 立场 ④ "agent 不享 wildcard" 精神继续守) | unit | 战马C / 烈马 | `TestAP32_CrossOrgAgent_Rejected` |
| 2.4 NULL org_id 兼容 (legacy AP-1 行) — user.org_id NULL 或 resource.org_id NULL 走 legacy 路径 (跟 AP-1 现网 ABAC 行为零变, NULL = inheritance, 立场 ⑥) | unit | 战马C / 烈马 | `TestAP32_LegacyNullOrgID_FallsThroughToAP1` |
| 2.5 admin god-mode 不入 — `admin/*` cookie path 走 `/admin-api/*` 单独 mw, 不调 HasCapability (反向 grep `admin.*HasCapability.*\.org` count==0, 立场 ⑤) | unit + reverse grep | 战马C / 烈马 | `TestAP32_AdminGodMode_NotInThisPath` (反向 grep filepath.Walk count==0) |

### AP-3.3 e2e + closure

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 3.1 server-side full-flow integration: org-A user grant `write_artifact channel:<ch-A>` → POST /artifacts/:id/commits org-A artifact OK 200 + 同 capability 调 org-B artifact 拒 403 + error body `abac.cross_org_denied` 字面 | http e2e | 战马C / 烈马 | `internal/api/ap_3_3_cross_org_integration_test.go::TestAP33_CrossOrg_FullFlow` (org-A grant + org-A 200 + org-B 403 + error code byte-identical + agent path 同断) |
| 3.2 反向 grep CI lint 等价单测 (5 grep 锚, 立场 ③ + ④ + ⑤ + ⑦) | unit | 战马C / 烈马 | `abac_test.go::TestAP32_ReverseGrep_NoCrossOrgBypass` (filepath.Walk 扫 internal/api/ count==0 含 5 pattern) |
| 3.3 closure: registry §3 REG-AP3-001..N + acceptance + PROGRESS [x] AP-3 + docs/current sync (server/auth.md §cross-org + blueprint auth-permissions.md §5 字面对齐) | docs | 战马C / 烈马 | registry + PROGRESS + 4 件套全闭 |

## 不在本轮范围 (spec §4)

- v2 cross-org grant request UI (留 ADM-3+, server-side enforce + 错码已就位)
- AP-1.bis expires_at 业务化 sweeper (留 ADM-0+, 跟 AP-3 解耦)
- ABAC condition (time/ip/etc) (留 v2+)
- multi-org user (同一 user 横跨 2+ org) v3+
- cross-org admin god-mode (走 ADM-3+ `/admin-api/*`, 不入此路径)

## 退出条件

- AP-3.1 1.1-1.4 (schema ALTER + index + 错码 const + idempotent) ✅
- AP-3.2 2.1-2.5 (org gate cross / same / agent / NULL / admin) ✅
- AP-3.3 3.1-3.3 (e2e 路径 + 反向 grep + closure) ✅
- 现网回归不破: 全套 server unit 全 PASS (cross-org 仅 enforce, 单 org 路径零变)
- REG-AP3-001..N 落 registry + 5 反约束 grep 全 count==0
- 4 件套全闭 (spec ✅ + stance ✅ + acceptance ✅ + content-lock 不需要 server-only)

## 更新日志

- 2026-04-29 — 战马C v0 acceptance template (4 件套第二件): 3 段实施 (1.1-1.4 / 2.1-2.5 / 3.1-3.3) + 5 不在范围 + 退出条件 6 项. 联签 AP-3.1/.2/.3 三段同 branch 同 PR (一 milestone 一 PR 协议默认 1 PR, 跟 CV-2 v2 #517 / AL-5 d2622ec 同模式).
