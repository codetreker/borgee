# AL-7 立场反查清单 (战马D v0)

> 战马D · 2026-04-30 · 立场 review checklist (跟 BPP-7/BPP-8/HB-3 v2 stance 同模式)
> **目的**: AL-7 三段实施 (7.1 schema migration v=33 / 7.2 sweeper + endpoint / 7.3 closure) PR review 时, 飞马/野马/烈马按此清单逐立场 sign-off, 反向断言代码层守住每条立场.
> **关联**: spec `docs/implementation/modules/al-7-spec.md` (战马D v0 3fa2db0) + acceptance `docs/qa/acceptance-templates/al-7.md` (战马D v0)
> **不需 content-lock** — admin-rail API 无 client UI 这版 (admin dashboard 留 v3); 跟 BPP-3/4/5/6/7/8 server-only 同模式.
> **v=33 sequencing**: CV-6 artifacts_fts FTS5 (#531 in-flight) 占 v=32, AL-7.1 顺位 **v=33**.

## §0 立场总表 (3 立场 + 4 蓝图边界)

| # | 立场 | 蓝图字面 | 反约束 (代码层守门) |
|---|---|---|---|
| ① | retention 走 admin_actions.archived_at 列 + sweeper, **不裂表 / 不真删** — ALTER ADD archived_at NULL (跟 AP-2.1 revoked_at + AP-1.1 expires_at + AP-3.1 org_id 跨四 milestone 同模式); sweeper UPDATE archived_at = now (forward-only 不 DELETE); audit 5 字段链第 7 处 (ADM-2.1+AP-2+BPP-4+BPP-7+BPP-8+HB-3 v2+AL-7) | admin-model.md §3 retention + ADM-2.1 forward-only audit | 反向 grep `audit_archive_table\|audit_history_log\|al7_archive_log` 在 internal/ 0 hit; 反向 grep `DELETE FROM admin_actions` 在 production *.go 0 hit |
| ② | admin override 复用 admin_actions audit — admin_actions CHECK 11 → 12 项加 `'audit_retention_override'` 字面 (12-step rebuild 跟 CV-3.1/CV-2 v2/AP-2/BPP-8 同模式); reason 复用 AL-1a 6-dict (sweeper 写 audit row 走 reasons.Unknown 字面); AL-1a reason 锁链 AL-7 = 第 15 处 | reasons-spec.md (#496 SSOT) + ADM-2.1 audit forward-only | 反向 grep `runtime_recovered\|al7_specific_reason\|7th.*reason\|sdk_reason` 在 internal/ 0 hit; admin_actions enum 加 1 项 (11 → 12) — 反向 reject spec 外值 |
| ③ | admin-rail only — admin 操作必走 audit (ADM-0 §1.3 红线) — POST /admin-api/v1/audit-retention/override admin cookie 路径; user-rail (`/api/v1/...`) **不挂** override 路径 | admin-model.md ADM-0 §1.3 + ADM-2.1 admin 业务必走 audit row | 反向 grep `user.*audit_retention_override\|public.*audit_retention` 在 internal/api/ user-rail handler 0 hit; admin handler 必调 InsertAdminAction |
| ④ (边界) | sweeper 复用 time.Ticker 不开 cron 框架 (跟 AP-2 ExpiresSweeper 同模式) — 1h tick + ctx-aware shutdown; RunOnce 同步入口 testable | AP-2 #525 ExpiresSweeper 立场承袭 + best-effort 立场 | 反向 grep `"github.com/.*cron\|robfig/cron\|gocron"` 在 audit_retention_sweeper.go 0 hit |
| ⑤ (边界) | best-effort 立场承袭 BPP-4/5/6/7/8/HB-3 v2 — sweeper 出错 log.Warn 不 panic / 不 retry queue / 不持久化 deferred | bpp-4.md §0.3 best-effort 立场 | AST scan forbidden tokens `pendingRetentionQueue\|retentionRetryQueue\|deadLetterRetention` 在 internal/ 0 hit (锁链延伸第 7 处) |
| ⑥ (边界) | retention 默认 14d 字面单源 — `RetentionDays = 14` const, 反向 grep hardcode 非 14 字面 0 hit; admin override clamp 1..365 (1d min, 1y max — 反 0 / 负 / 非数 / >365 reject) | spec §0.1 + acceptance §1 字面 | const RetentionDays / RetentionMinDays / RetentionMaxDays 字面单源 |
| ⑦ (边界) | sparse idx 仅扫已 archived 行 (跟 AP-2.1 revoked_at sparse idx 同模式) — `idx_admin_actions_archived_at WHERE archived_at IS NOT NULL`; 现网零开销 (active 行不入 idx) | AP-2.1 #525 sparse idx 立场承袭 | 反向 sqlite_master 验 sparse idx WHERE 子句 byte-identical |

## §1 立场 ① archived_at + 不真删 (AL-7.1+7.2 守)

**反约束清单**:

- [ ] migration v=33 ALTER admin_actions ADD COLUMN archived_at INTEGER (nullable, 跟 AP-2 revoked_at NULL = active 行同精神)
- [ ] sparse idx WHERE archived_at IS NOT NULL (跟 AP-2.1 同模式)
- [ ] sweeper UPDATE archived_at = now (反向 grep `DELETE FROM admin_actions` 在 production 0 hit — forward-only)
- [ ] 不裂表 — 反向 grep `audit_archive_table\|audit_history_log\|al7_archive_log` 0 hit
- [ ] audit 5 字段链第 7 处 — admin_actions 行 (id/actor/target/action/metadata) 字段集不动, archived_at 是 sweeper 软删戳

## §2 立场 ② admin override + reason 6-dict 复用 (AL-7.1+7.2 守)

**反约束清单**:

- [ ] admin_actions CHECK enum 12-step rebuild 11 → 12 项加 'audit_retention_override' 字面
- [ ] AcceptsAuditRetentionOverride 真测 — INSERT admin_actions row with action='audit_retention_override' 通过
- [ ] RejectsUnknownAction — `audit_retention_xxx` 自造名字 reject
- [ ] sweeper revoke audit row 走 reasons.Unknown byte-identical (复用 6-dict)
- [ ] AL-1a reason 锁链 AL-7 = 第 15 处 (改 = 改十五处)

## §3 立场 ③ admin-rail only + ADM-0 红线 (AL-7.2 守)

**反约束清单**:

- [ ] POST /admin-api/v1/audit-retention/override admin cookie middleware 真挂
- [ ] user-rail 不挂 — 反向 grep `audit_retention_override` 在 user-rail handler (非 admin*.go) 0 hit
- [ ] admin handler 必调 InsertAdminAction (admin 操作必走 audit row, ADM-0 §1.3 红线)
- [ ] OverrideRejectsUserRail — user cookie 路径 401 / not-found

## §4 蓝图边界 ④⑤⑥⑦ — 跟 ticker / best-effort / const / sparse idx 不漂

**反约束清单**:

- [ ] sweeper 用 time.Ticker — 反向 grep `cron\|robfig\|gocron` 在 audit_retention_sweeper.go 0 hit
- [ ] AST 锁链延伸第 7 处 — `pendingRetentionQueue\|retentionRetryQueue\|deadLetterRetention` 0 hit
- [ ] RetentionDays = 14 const 单源
- [ ] sparse idx WHERE archived_at IS NOT NULL byte-identical 跟 AP-2.1 同模式

## §5 退出条件

- §1 (5) + §2 (5) + §3 (4) + §4 (4) 全 ✅
- 反向 grep 7 项全 0 hit (不裂表 / 真删 / 不扩 reason / cron / retention queue / user-rail / hardcode)
- AL-1a reason 锁链 AL-7 = 第 15 处
- audit 5 字段链第 7 处 (ADM-2.1 + AP-2 + BPP-4 + BPP-7 + BPP-8 + HB-3 v2 + AL-7)
- AST 锁链延伸第 7 处 (BPP-4/5/6/7/8 + HB-3 v2 + AL-7 forbidden tokens 全 0 hit)
- admin-rail only (ADM-0 §1.3 红线)
