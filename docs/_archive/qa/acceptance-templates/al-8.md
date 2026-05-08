# Acceptance Template — AL-8: audit log query API filter 扩

> 蓝图 admin-model.md §1.4 audit log + ADM-2.2 #484 既有 path + AL-7 #533 archived_at 列读视互补. Spec `al-8-spec.md` (战马D v0). Stance `al-8-stance-checklist.md` (战马D v0). 不需 content-lock — admin-rail API 无 client UI v1 (admin dashboard 留 v3). **0 schema / 0 新 endpoint** — 仅 ADM-2.2 既有 GET /admin-api/v1/audit-log + AL-7.1 archived_at 列复用. Owner: 战马D 实施 / 飞马 review / 烈马 验收.

## 验收清单

### §1 AL-8.1 — schema 0 行 + AL-8.2 — server filter 扩

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 1.1 0 schema 改 反向断言 — 反向 grep `ALTER TABLE admin_actions\|CREATE INDEX.*audit_log\|migrations/al_8_` 在 internal/migrations/ 0 hit + registry.go 字面 byte-identical 不动 (al71AdminActionsArchivedAt 后无新条目) | grep + Idempotent | 战马D / 飞马 / 烈马 | `internal/api/al_8_audit_log_filter_test.go::TestAL81_NoSchemaChange` (filepath.Walk migrations/ 反向 0 hit) |
| 1.2 既有 GET /admin-api/v1/audit-log 路径单源 — 反向 grep `audit-log/query\|audit-log/search\|/admin-api/v1/audit/` 在 internal/api/ 0 hit (除 ADM-2.2 既有单源) + ADM-2.2 既有 unit (TestADM22_GetAdminAuditLog_FullVisibility) 不破 | grep + 既有 unit | 战马D / 飞马 / 烈马 | `TestAL81_NoNewEndpoint` (反向 grep 0 hit) + ADM-2.2 既有 unit 全 PASS |
| 1.3 archived 三态 — `?archived=active` (默认) / `?archived=archived` / `?archived=all` 三视图; spec 外值 → 400 `audit_log.archived_view_invalid` byte-identical | unit (4 sub-case) | 战马D / 烈马 | `TestAL82_ArchivedView_ActiveDefault` (3 active + 2 archived → 3 行) + `_ArchivedView_ArchivedOnly` (3 active + 2 archived → 2 行) + `_ArchivedView_All` (5 行) + `_ArchivedView_RejectsUnknown` (`?archived=foo` → 400 错码字面) |
| 1.4 since/until 区间 — int64 ms epoch; negative / non-int → 400 `audit_log.time_range_invalid`; since>until → 400 `audit_log.time_range_inverted` | unit (4 sub-case) | 战马D / 烈马 | `TestAL82_TimeRange_HappyPath` (since=T-7d/until=T → 区间内行) + `_TimeRange_RejectsNegative` + `_TimeRange_RejectsNonInt` + `_TimeRange_RejectsInverted` (since=T/until=T-7d → 400) |
| 1.5 actions 多值 query — `?action=a&action=b` 走 IN 子句; 单值 `?action=foo` ADM-2.2 backward-compat byte-identical 不破 | unit (3 sub-case) | 战马D / 烈马 | `TestAL82_Actions_MultiValue` (?action=delete_channel&action=suspend_user → 2 类) + `_Actions_SingleValue_Backcompat` (跟 ADM-2.2 既有 unit byte-identical) + `_Actions_EmptyDefault` (无 ?action 参数 = 全 6 行) |

### §2 AL-8.2 — admin-rail only + AST scan + 立场反断

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 2.1 admin-rail only 立场 ② — user cookie 调 GET /admin-api/v1/audit-log?archived=archived → 401 (admin.RequireAdmin middleware 兜底); 反向 grep `/api/v1/.*audit-log` user-rail handler 0 hit + ADM2Handler.RegisterUserRoutes 不挂 audit-log 路径 (反向 grep `RegisterUserRoutes.*audit-log` 0 hit) | unit + grep | 战马D / 烈马 | `TestAL82_RejectsUserRail` (user cookie → 401) + `TestAL81_NoUserRailAuditLog` (反向 grep 0 hit) |
| 2.2 AL-1a reason 锁链 AL-8 = 第 16 处 — 复用 reasons.Unknown 字面 byte-identical 跟 AL-7 SweeperReason 同源 (AL-7 #15 承袭不漂); AL-8 不另起 reason 字典 | const + grep | 战马D / 烈马 | `TestAL82_ReasonChain_NotExpanded` (反向 grep `runtime_recovered\|al8_specific_reason\|16th.*reason\|audit_query_reason` 在 internal/ 0 hit) |
| 2.3 AST 锁链延伸第 8 处 — forbidden 3 token (`pendingAuditQuery / auditQueryRetryQueue / deadLetterAuditQuery`) 在 internal/auth + internal/api production *.go 0 hit (跟 BPP-4/5/6/7/8 + HB-3 v2 + AL-7 同模式) | AST scan | 飞马 / 烈马 | `TestAL83_NoAuditQueryQueue` (AST ident scan 3 forbidden 0 hit) |

## 边界

- ADM-2.2 #484 既有 GET /admin-api/v1/audit-log + 3-filter (actor_id/action/target_user_id) byte-identical 不动 / AL-7 #533 archived_at 列 + sparse idx 反向同源 / ADM-0 §1.3 红线 admin-rail only / AL-1a reason 锁链 AL-8 = 第 16 处 (AL-7 #15 承袭) / AST 锁链延伸第 8 处 (BPP-4+5+6+7+8 + HB-3 v2 + AL-7 + AL-8) / audit 5 字段链第 8 处 (ADM-2.1+AP-2+BPP-4+BPP-7+BPP-8+HB-3 v2+AL-7+AL-8) / parseLimit helper default 100/max 500 复用 ADM-2.2 同源 / 0 schema / 0 新 endpoint / 0 client UI

## 退出条件

- §1 (5) + §2 (3) 全绿 — 一票否决
- 0 schema / 0 新 endpoint / 0 新 path (registry.go + mux.Handle 字面不动)
- ADM-2.2 既有 unit (TestADM22_GetAdminAuditLog_*) 不破
- AL-1a reason 锁链 AL-8 = 第 16 处
- audit 5 字段链第 8 处 (ADM-2.1+AP-2+BPP-4+BPP-7+BPP-8+HB-3 v2+AL-7+AL-8)
- AST 锁链延伸第 8 处
- admin-rail only (反向 grep user-rail 0 hit)
- 登记 REG-AL8-001..006
