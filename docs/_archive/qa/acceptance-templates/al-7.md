# Acceptance Template — AL-7: audit log retention + archive

> 蓝图 `admin-model.md` §3 retention + ADM-2.1 #484 forward-only audit 终结收尾. Spec `al-7-spec.md` (战马D v0 3fa2db0) + Stance `al-7-stance-checklist.md` (战马D v0). 不需 content-lock — admin-rail API 无 client UI v1 (admin dashboard 留 v3). v=33 sequencing post-CV-6 v=32. Owner: 战马D 实施 / 飞马 review / 烈马 验收.

## 验收清单

### §1 AL-7.1 — schema migration v=33 ALTER admin_actions ADD archived_at + CHECK +1

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 1.1 admin_actions ALTER ADD COLUMN archived_at INTEGER NULL (跟 AP-2.1 revoked_at + AP-1.1 expires_at + AP-3.1 org_id 跨四 milestone 同模式; nullable, NULL = active 行, 跟 AP-2 立场承袭) + sparse idx `idx_admin_actions_archived_at WHERE archived_at IS NOT NULL` (跟 AP-2.1 sparse 同模式) | unit (2 sub-case) | 战马D / 烈马 | `internal/migrations/al_7_1_admin_actions_archived_at_test.go::TestAL71_AddsArchivedAtColumn` (PRAGMA nullable) + `_HasSparseIdx` (sqlite_master WHERE byte-identical) |
| 1.2 admin_actions CHECK enum 12-step rebuild 11 → 12 项加 'audit_retention_override' 字面 (跟 CV-3.1/CV-2 v2/AP-2/BPP-8 12-step 同模式) | unit (3 sub-case) | 战马D / 烈马 | `_AcceptsAuditRetentionOverride` (INSERT 11 legacy + 1 new 全通过) + `_RejectsUnknownAction` (audit_retention_xxx 反约束 reject) + `_VersionIs33` + `_Idempotent` |
| 1.3 立场 ① 不裂表 — 反向 grep `audit_archive_table\|audit_history_log\|al7_archive_log` 在 production 0 hit + sqlite_master 0 forbidden table 实证 | grep + sqlite_master | 战马D / 飞马 / 烈马 | `_NoSeparateArchiveTable` (sqlite_master 反向 3 forbidden table 0 hit) |

### §2 AL-7.2 — RetentionSweeper + admin override endpoint admin-rail

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 2.1 RetentionSweeper 1h ticker + ctx-aware shutdown + RunOnce 同步入口 (跟 AP-2 ExpiresSweeper 同模式 nil-safe) — UPDATE archived_at = now WHERE created_at < (now - RetentionDays*24h) AND archived_at IS NULL; **不真删**, 反向 grep `DELETE FROM admin_actions` 0 hit | unit (4 sub-case + nil-safe) | 战马D / 烈马 | `internal/auth/audit_retention_sweeper_test.go::TestAL72_RunOnceArchivesExpired` (3 expired + 2 fresh → archived count=3) + `_RunOnceSoftArchiveNotRealDelete` (UPDATE 不 DELETE) + `_RunOnceIdempotent` + `_StartCtxShutdown` + `_NilSafeCtor` |
| 2.2 admin override endpoint POST /admin-api/v1/audit-retention/override admin-rail (admin cookie middleware 必经); body retention_days int 1..365 clamp; 写 admin_actions row action='audit_retention_override' (ADM-0 §1.3 红线 — admin 操作必走 audit) | unit (3 sub-case) | 战马D / 烈马 | `internal/api/al_7_audit_retention_override_test.go::TestAL72_OverrideEndpointWritesAudit` + `_OverrideRejectsUserRail` (user-rail 0 挂 — 401 / 404) + `_OverrideClampsRetention` (0/-5/999 → reject 400 / clamp 365) |
| 2.3 立场 ② reason 复用 reasons.Unknown byte-identical (sweeper 不另起 reason 字典, 复用 6-dict; AL-1a 锁链第 15 处) — 字面对比 | unit + grep | 战马D / 烈马 | `_SweeperReason_ByteIdentical` (字面对比 reasons.Unknown 跟 BPP-8 / BPP-7 同源) |

### §3 AL-7.3 — closure + AST 锁链延伸第 7 处

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 3.1 立场 ⑤ AST 锁链延伸第 7 处 — forbidden 3 token (`pendingRetentionQueue / retentionRetryQueue / deadLetterRetention`) 在 internal/ 0 hit (跟 BPP-4/5/6/7/8/HB-3 v2 同模式) | AST scan | 飞马 / 烈马 | `TestAL73_NoRetentionQueueOrCronImport` (AST ident scan internal/auth + internal/api production 0 hit; 反向 grep cron import 0 hit) |
| 3.2 立场 ④ time.Ticker 不引 cron 框架 (跟 AP-2 立场承袭) — 反向 grep `cron\|robfig\|gocron` 在 audit_retention_sweeper.go 0 hit | grep | 战马D / 飞马 / 烈马 | `_NoCronFrameworkImport` (反向 grep import path 0 hit) |

## 边界

- ADM-2.1 #484 admin_actions audit (audit 5 字段链第 7 处) / AP-2 #525 ExpiresSweeper (time.Ticker 同模式) / AP-1.1 expires_at + AP-3.1 org_id (ALTER ADD COLUMN nullable 跨四 milestone) / BPP-4 + BPP-7 + BPP-8 + HB-3 v2 (audit / best-effort 锁链) / AL-1 #492 + REFACTOR-REASONS #496 6-dict (锁链第 15 处) / ADM-0 §1.3 红线 (admin 操作必走 audit row) / AST 锁链延伸第 7 处 (BPP-4+5+6+7+8+HB-3 v2+AL-7)

## 退出条件

- §1 (3) + §2 (3) + §3 (2) 全绿 — 一票否决
- AL-1a reason 锁链 AL-7 = 第 15 处 (HB-3 v2 第 14 链承袭不漂)
- audit 5 字段链第 7 处 (ADM-2.1+AP-2+BPP-4+BPP-7+BPP-8+HB-3 v2+AL-7 跨七 milestone 同精神)
- AST 锁链延伸第 7 处
- admin-rail only (反向 grep user-rail 0 hit)
- 登记 REG-AL7-001..006
