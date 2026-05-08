# Acceptance Template — ADM-3 (audit_events query API + admin 视图 v2)

> Spec brief `adm-3-spec.md` (飞马 v0). Owner: 战马待派 实施 / 飞马 review / 烈马 验收. v0 元数据 RENAME PR #586 已 merge (REG-ADM3-001..006 全 🟢), 本 v1 batch 接 audit_events query API + admin 视图.
>
> **ADM-3 v1 范围**: 接 v0 admin_actions → audit_events RENAME + view alias 落地后, 真接 audit-events query 路径 — `GET /admin-api/v1/audit-events` (admin 互可见, 蓝图 §1.4 红线 3) + `GET /me/audit-events` (user 只见自己 target_user_id 行 byte-identical 跟 ADM-2 #484 既有立场承袭). **0 schema 改 + 0 新表** (复用 v0 audit_events 表 + view alias backward compat).

## 验收清单

### §1 行为不变量 (audit forward-only + cross-actor 单表 + ADM-2 既有路径不破)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 1.1 audit_events 单表跨所有 actor type (admin / system / plugin lifecycle / 未来类型) — `actor_kind` 字段值走 const 不 hardcode (跟 ADM-2 #484 + BPP-8 #532 三处单测锁 byte-identical, 改 = 改三处) | unit + grep | `TestADM3_ActorKindConstByteIdentical` (LifecycleSystemActor + AdminActor + audit_events.actor_kind 三处) PASS |
| 1.2 forward-only 反向断 — 反向 grep `DELETE FROM audit_events\|UPDATE audit_events` 在 production 路径 (除 migration / retention sweeper) 0 hit (跟 ADM-2.1 / AP-2 / BPP-4 / BPP-7 / BPP-8 / AL-7 audit forward-only 锁链同精神跨七 milestone) | CI grep | reverse grep test PASS |
| 1.3 ADM-2 #484 既有路径 byte-identical 不破 — `InsertAdminAction` + `GetAdminActionsByTarget` 函数名留 (compat 期不删), SQL 改写 audit_events 直接, 既有调用方 0 改 | unit | `TestADM3_InsertAdminActionStillWorks` (函数签名 + 写入 audit_events 直接) + `TestADM3_BPP8LifecycleStillWorks` (5 method 路径不破) PASS |

### §2 数据契约 (0 schema 改 + view alias backward compat + audit_events 9 字段 byte-identical)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 2.1 0 schema 改 — `git diff main -- internal/migrations/` 0 行 (复用 v0 v=43 ALTER + view alias) | git diff | 0 行 ✅ |
| 2.2 view `admin_actions` deprecated 标注但不删 (留 Phase 5+ deprecation announcement) — `CREATE VIEW admin_actions AS SELECT * FROM audit_events` v0 backward compat 不动 | inspect | view 字面 byte-identical 跟 v0 PR #586 锁 + reverse grep `INSERT INTO admin_actions` 在非 migration 路径 0 hit (RENAME 后写都走 audit_events table) |
| 2.3 audit_events 9 字段 byte-identical 跟蓝图 admin-model.md §1.4 同源 (`id / actor_id / actor_kind / action / target_user_id / target_type / payload_json / created_at / metadata`) — 跨层锁 server const ↔ blueprint 字面同源 | grep + unit | `TestADM3_9FieldsSchemaByteIdentical` (PRAGMA verify) + blueprint anchor verify ≥1 hit |

### §3 E2E (admin-api/v1/audit-events query + /me/audit-events ACL)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 3.1 `GET /admin-api/v1/audit-events` 真接 — admin 互可见全 audit_events 行 (蓝图 §1.4 红线 3 + 走 /admin-api/* 单独 mw 跟 ADM-0 §1.3 admin god-mode 红线立场承袭, 反约束: user 走 /api/* 不见 admin 行) | unit + integration | `TestADM3_AdminAuditEventsQuery_Happy` + `TestADM3_AdminAuditEventsQuery_UserRejected` (user 走 /admin-api/* → 403) PASS |
| 3.2 `GET /me/audit-events` user-scoped — 只见自己 target_user_id 行 (跟 ADM-2 既有 /me/admin-actions 立场承袭 byte-identical, 反 cross-user leak) | unit + integration | `TestADM3_MeAuditEventsQuery_OwnerOnly` + `TestADM3_MeAuditEventsQuery_NoCrossUserLeak` PASS |
| 3.3 Playwright e2e 真测 — admin login → /admin/audit-events 看全行 + user login → /me/audit-events 看自己行 + 反 cross-user (user A 不见 user B target 行) | E2E | `packages/e2e/tests/adm-3-audit-events.spec.ts` 3 case PASS (Playwright `--timeout=30000`) |

### §4 closure (REG + cov gate + 跨 milestone 锁)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 4.1 既有全包 unit + e2e + vitest 全绿不破 + post-#614 haystack gate 三轨过 (Func=50/Pkg=70/Total=85) | full test + CI | `go test -tags sqlite_fts5 -timeout=300s ./...` + go-test-cov SUCCESS |
| 4.2 反平行 audit query / 反 admin god-mode bypass user-scoped — 反向 grep `func.*GetAuditEventsForAdmin` 在 internal/ 除 internal/admin/ 0 hit (admin path 单源走 /admin-api/* 单独 mw 不污染 /api/* user path) | CI grep | reverse grep test PASS |
| 4.3 4 件套全闭: spec brief + stance + acceptance + content-lock (audit_events 9 字段 + actor_kind const + 错码字面 byte-identical) | inspect | 文件存在 verify ≥3 件 |

## REG-ADM3-* (v0 #586 RENAME 已 🟢 / v1 multi-source 🟢 flipped 2026-05-01)

- REG-ADM3-001..006 🟢 (v0 PR #586 merged) — admin_actions → audit_events RENAME + view alias backward compat + actor_kind 三处单测锁

**v1 multi-source audit 合并** (本 milestone PR 翻 🟢):
- REG-ADM3-007 🟢 4 source enum SSOT (`AuditSourceServer/Plugin/HostBridge/Agent` byte-identical "server"/"plugin"/"host_bridge"/"agent") + AuditSources ordering 单源 (server const + client AUDIT_SOURCES + i18n SOURCE_LABEL 三处锁)
- REG-ADM3-008 🟢 `GET /admin-api/v1/audit/multi-source` admin-rail UNION ALL 4 源 (audit_events server/plugin 走 plugin_* prefix 区分 + DL-2 channel_events/global_events agent + host_bridge placeholder 0 行 留 HB-1 follow-up) + admin god-mode 路径独立 (反 user-rail 漂 ADM-0 §1.3 红线) + post-#618 haystack gate 三轨过 (TOTAL 85.6%) + 9 unit + 7 vitest 全 PASS

## 退出条件

- §1 (3) + §2 (3) + §3 (3) + §4 (3) 全绿 — 一票否决
- 0 schema 改 (复用 v0 audit_events 表 + view alias backward compat)
- audit_events 9 字段 byte-identical 跨层锁 (server const ↔ blueprint §1.4 同源)
- audit forward-only 立场承袭跨七 milestone (ADM-2.1 / AP-2 / BPP-4 / BPP-7 / BPP-8 / AL-7 / ADM-3)
- ADM-2 既有 InsertAdminAction / GetAdminActionsByTarget 函数名留 + BPP-8 LifecycleAuditor 路径不破
- /admin-api/v1/audit-events admin 互可见 (走 /admin-api/* 单独 mw, 反 user 走 /api/*)
- /me/audit-events user-scoped owner-only (反 cross-user leak)
- 全包 unit + e2e + vitest 全绿不破 + post-#614 haystack gate 三轨过
- 反平行 admin query + 反 admin god-mode user-scoped bypass
- 登记 REG-ADM3-007..008 (v0 001..006 已 🟢)

## 更新日志

| 日期 | 作者 | 变化 |
|---|---|---|
| 2026-04-30 | 烈马 | v0 — RENAME + view alias acceptance (REG-ADM3-001..006 全 🟢, PR #586 merged). |
| 2026-05-01 | 烈马 | v1 — 扩 4 段验收覆盖 audit_events query API + admin 视图. REG-ADM3-007..008 ⚪ 占号. 立场承袭 ADM-2 #484 system DM 5 模板 + audit forward-only 锁链跨七 milestone (ADM-2.1 + AP-2 + BPP-4 + BPP-7 + BPP-8 + AL-7 + ADM-3) + ADM-0 §1.3 admin god-mode 红线 (admin 走 /admin-api/* 单独 mw) + post-#614 haystack gate + actor_kind 三处单测锁 byte-identical. |
| 2026-05-01 | 战马C | v2 — multi-source audit 合并查询落地 (飞马 spec brief v1 重写后实施). REG-ADM3-007..008 🟢 翻牌. 实施 scope: server-side `internal/api/admin_audit_query.go` (4 source enum SSOT + UNION ALL helper + admin endpoint) + `internal/server/server.go` +5 行 wire + `packages/client/src/admin/api.ts` AUDIT_SOURCES + fetchMultiSourceAudit + `packages/client/src/admin/pages/MultiSourceAuditPage.tsx` (4 source badge + filter + DOM 锚) + 9 server unit + 7 vitest. Bypass: 改 AL-8 既有 reverse-grep test 加 `/admin-api/v1/audit/multi-source` 白名单单一例外 (spec §0 立场 ② 授权端点). 反向 grep 8 锚: 4 source const ==4 hit / 0 schema 改 / admin god-mode 路径独立 / UNION ALL 跨表 / admin auth 复用 / DL-2 mustPersistKinds 不破 / i18n SOURCE_LABEL 4 key / haystack gate TOTAL 85.6% datalayer pkg 89.0%. host_bridge HB-1 audit 表未落 v1, 走 placeholder 0 行 (留 HB-1 follow-up). |
