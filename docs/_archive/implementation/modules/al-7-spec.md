# AL-7 spec brief — audit log retention + archive (≤80 行)

> 战马D · Phase 6 · ≤80 行 · 蓝图 [`admin-model.md`](../../blueprint/current/admin-model.md) §3 admin_actions retention + ADM-2.1 #484 forward-only audit 终结收尾. 模块锚 [`auth-permissions.md`](auth-permissions.md) §AL-7. 依赖 ADM-2.1 #484 admin_actions + AP-2 #525 sweeper (time.Ticker) + BPP-4 #499 watchdog + BPP-8 #532 lifecycle audit + AL-1 #492 + REFACTOR-REASONS #496 6-dict + ADM-0 §1.3 红线.

## 0. 关键约束 (3 条立场, 蓝图 §3 + ADM-2.1 字面承袭)

1. **retention 走 admin_actions.archived_at 列 + sweeper, 不裂表** — admin_actions 表 ALTER ADD COLUMN `archived_at INTEGER NULL` (跟 AP-2.1 #525 revoked_at + AP-1.1 expires_at + AP-3.1 org_id ALTER ADD COLUMN NULL 跨四 milestone 同模式); retention 14d default 由 sweeper goroutine 周期扫 (1h ticker, 跟 AP-2 ExpiresSweeper 同模式) UPDATE archived_at = now WHERE created_at < (now - 14d) AND archived_at IS NULL; **不裂 audit_archive 表 / 不裂 audit_history 表**. **反约束**: 反向 grep `audit_archive_table\|audit_history_log\|al7_archive_log` 在 internal/ 0 hit; 反向 grep `DELETE FROM admin_actions` 在 production *.go 0 hit (forward-only 立场承袭, 不真删). audit 5 字段 byte-identical 跨链 — AL-7 = **第 7 处** (ADM-2.1 + AP-2 + BPP-4 + BPP-7 + BPP-8 + HB-3 v2 + AL-7).

2. **admin retention override 复用 admin_actions audit** — admin override 调 `Store.InsertAdminAction(actor=<admin_id>, action='audit_retention_override', target=<scope>, metadata=JSON{retention_days, scope})` (复用 ADM-2.1 既有 audit 路径, 不另开 audit table); admin_actions CHECK enum 12-step rebuild +1 条 `'audit_retention_override'` 字面 (跟 CV-3.1 / CV-2 v2 / AP-2 / BPP-8 12-step 同模式). reason 复用 AL-1a 6-dict (sweeper revoke audit 走 `reasons.Unknown` byte-identical, AL-1a reason 锁链 AL-7 = **第 15 处** BPP-2.2/AL-2b/BPP-4/BPP-5/BPP-6/BPP-7/BPP-8/HB-3 v2/AL-7). **反约束**: 反向 grep `runtime_recovered\|al7_specific_reason\|7th.*reason\|sdk_reason` 0 hit.

3. **admin retention override owner=admin only — admin 操作必走 audit (ADM-0 §1.3)** — `POST /admin-api/v1/audit-retention/override` admin-rail (跟 ADM-0 §1.3 字面 — admin 业务操作必走 admin_actions audit row 写入). **反约束**: 反向 grep `user.*audit_retention_override\|public.*audit_retention` 在 user-rail handler 0 hit (admin-only); ADM-0 §1.3 红线: admin 操作不能旁路 audit (admin_actions 行写入是必经路径). AST 锁链延伸第 7 处 — forbidden tokens (`pendingRetentionQueue\|retentionRetryQueue\|deadLetterRetention`) 0 hit (跟 BPP-4/5/6/7/8/HB-3 v2 best-effort 同精神).

## 1. 拆段 (一 milestone 一 PR, 整段一次合 — 跟 BPP-2/3/4/5/6/7/8 + DM-3/4 + CV-4 v2 + HB-3 v2 同源)

| 段 | 文件 | 范围 |
|---|---|---|
| AL-7.1 schema migration v=32 | `internal/migrations/al_7_1_admin_actions_archived_at.go` 新 (ALTER admin_actions ADD COLUMN archived_at INTEGER NULL + sparse idx WHERE archived_at IS NOT NULL + admin_actions CHECK enum 11 → 12 项加 `'audit_retention_override'` 字面 12-step rebuild) + 4 unit (AddsArchivedAtColumn / HasSparseIdx / AcceptsAuditRetentionOverride / RejectsUnknownAction / VersionIs32 / Idempotent) | 0 新表 — 复用 admin_actions 跟 ADM-2.1+AP-2+BPP-8 同模式 |
| AL-7.2 server retention sweeper + admin override endpoint | `internal/auth/audit_retention_sweeper.go` 新 (RetentionSweeper struct + Start ctx-aware 1h ticker + RunOnce 同步入口 testable; UPDATE archived_at = now WHERE created_at < threshold AND archived_at IS NULL; 跟 AP-2 ExpiresSweeper 同模式 nil-safe) + `internal/api/al_7_audit_retention_override.go` 新 (admin-rail POST endpoint + admin_actions audit write 必经; body retention_days int 1..365 clamp) + 7 unit (RunOnceArchivesExpired / SoftArchiveNotRealDelete / Idempotent / StartCtxShutdown / OverrideEndpointWritesAudit / OverrideRejectsUserRail 401 / OverrideClampsRetention) | sweeper 复用 time.Ticker 不开 cron; admin override endpoint admin-rail (跟 ADM-2 既有路径同模式) |
| AL-7.3 closure REG-AL7 + acceptance + PROGRESS [x] | REG-AL7-001..006 + acceptance/al-7.md + PROGRESS update + AST scan 锁链延伸第 7 处 (`pendingRetentionQueue\|retentionRetryQueue\|deadLetterRetention`) 0 hit | best-effort 立场承袭 BPP-4/5/6/7/8/HB-3 v2 锁链延伸第 7 处 |

## 2. 留账边界

- **client admin retention dashboard UI** (留 v3) — admin SPA 看 retention 状态留 follow-up; AL-7 v1 仅 admin override endpoint (admin_actions 直挂)
- **per-user retention 覆盖** (留 v3) — 当前 retention 全局 14d, per-user override 留 v3 跟 ADM-2 联动
- **真物理 GC** (留 v3 — admin_actions 行已 archived 后真物理删除) — 当前 forward-only 仅 archived_at stamp, 跨年 archived 行清理留 v3 sweep
- **retention metric exporter** (留 v3) — Prometheus archived counter 跟 BPP-7/8 §2 留账同源

## 3. 反查 grep 锚 (Phase 6 验收 + AL-7 实施 PR 必跑)

```
git grep -nE 'audit_retention_override' packages/server-go/internal/   # ≥ 1 hit (字面真挂)
git grep -nE 'RetentionSweeper|RunOnce.*audit_retention' packages/server-go/internal/auth/   # ≥ 1 hit (sweeper struct + RunOnce)
# 反约束 (5 条 0 hit)
git grep -nE 'audit_archive_table|audit_history_log|al7_archive_log' packages/server-go/internal/   # 0 hit (复用 admin_actions, §0.1)
git grep -nE 'DELETE FROM admin_actions' packages/server-go/internal/   # 0 hit (forward-only, §0.1)
git grep -nE 'runtime_recovered|al7_specific_reason|7th.*reason' packages/server-go/internal/   # 0 hit (复用 6-dict, §0.2)
git grep -nE 'pendingRetentionQueue|retentionRetryQueue|deadLetterRetention' packages/server-go/internal/   # 0 hit (best-effort 锁链延伸第 7 处, §0.3)
git grep -nE 'user.*audit_retention_override' packages/server-go/internal/api/[^a]*.go   # 0 hit (admin-rail only, §0.3)
git grep -nE '"github.com/.*cron|robfig/cron|gocron"' packages/server-go/internal/auth/audit_retention_sweeper.go   # 0 hit (time.Ticker 不引 cron 框架)
```

## 4. 不在本轮范围 (反约束 deferred)

- ❌ client admin retention dashboard UI (留 v3)
- ❌ per-user retention 覆盖 (留 v3 跟 ADM-2 联动)
- ❌ archived_at 行真物理 GC (留 v3 sweep)
- ❌ Prometheus metrics exporter (留 v3 跟 BPP-7/8 同源)
- ❌ AL-1 6-dict 扩第 7 reason (字典分立)
- ❌ 另开 audit_archive 表 (§0.1 立场, 复用 admin_actions)
- ❌ 真物理 DELETE admin_actions (forward-only 立场承袭)
- ❌ admin god-mode 旁路 audit (ADM-0 §1.3 红线 — admin override 必走 audit row)
