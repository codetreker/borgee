# ADM-3 v1 — multi-source audit query (≤80 行)

> Landed in PR feat/adm-3: ADM3.1 (server query helper + admin endpoint) + ADM3.2 (client admin UI) + ADM3.3 closure
> Blueprint source: admin-model.md §1.4 source transparency across human / agent / admin / mixed events
> Design source: [`adm-3-spec.md`](../../implementation/modules/adm-3-spec.md) v1 §0 ① DL-1+DL-2 + ADM-2/ADM-3 #586 audit_events byte-identical + ② no schema changes + four-source enum single source + ③ no user-rail endpoint / admin path isolation

## 1. 文件清单

| 文件 | 行 | 角色 |
|---|---|---|
| `internal/api/admin_audit_query.go` | 260 | four-source enum single source + AdminAuditMultiSourceHandler + MultiSourceAuditQuery UNION ALL + queryAuditEvents + queryAgentEvents + sortByTSDesc |
| `internal/api/admin_audit_query_test.go` | 250 | 9 server unit (4 source byte-identical + 4 source UNION + filter + InvalidSource + TimeRange + InvalidTimeRange + OrderTSDesc + UserCookieRejected + UnauthRejected + LimitClamp + HostBridgePlaceholder) |
| `internal/server/server.go` 扩 | +5 | NewAdminAuditMultiSourceHandler.RegisterAdminRoutes(s.mux, adminMw) wired |
| `internal/api/agent_log_filter_test.go` 改 | +1 allow | AL-8 reverse-grep for `/admin-api/v1/audit/<not-log>` adds `/multi-source` as the single allow-list exception (spec §0 ② authorized endpoint) |
| `packages/client/src/admin/api.ts` 扩 | +35 | AUDIT_SOURCES 4-tuple + AuditSource type + MultiSourceAuditRow + fetchMultiSourceAudit |
| `packages/client/src/admin/pages/MultiSourceAuditPage.tsx` | 150 | four source badges + filter dropdown + table view + DOM source markers |
| `packages/client/src/admin/AdminApp.tsx` 扩 | +3 | nav 加 `/admin/audit-multi-source` route |
| `packages/client/src/__tests__/MultiSourceAuditPage.test.tsx` | 130 | 7 vitest (AUDIT_SOURCES byte-identical + DOM 出处 + per-source row + SOURCE_LABEL + empty state + error alert + filter triggers fetch + 反同义词 reject) |

## 2. Four-Source Enum Single Source (蓝图 §1.4 byte-identical)

| source const | Literal | Data Source |
|---|---|---|
| `AuditSourceServer` | `"server"` | audit_events (action 非 plugin_*) — ADM-2 #484 admin actions |
| `AuditSourcePlugin` | `"plugin"` | audit_events (action plugin_* prefix) — BPP-8 #532 lifecycle |
| `AuditSourceHostBridge` | `"host_bridge"` | HB-1 audit table placeholder, currently returning 0 rows; real wiring is left for a later HB-1 PR |
| `AuditSourceAgent` | `"agent"` | DL-2 #615 channel_events + global_events UNION ALL |

`AuditSources` slice ordering is the single source for the four entries; any change must update the server const, client `AUDIT_SOURCES`, and i18n `SOURCE_LABEL` together.

## 3. UNION ALL 跨 4 源查询流程

1. ?source filter validate (4 enum 单一例外 → 400 `audit.source_invalid`)
2. ?since/?until ms epoch reject negative/non-int → 400 `audit.time_range_invalid`
3. include(server || plugin) → queryAuditEvents (audit_events SELECT + WHERE created_at + LIMIT)
4. project source by `action[:7] == "plugin_"` (BPP-8 enum prefix)
5. include(agent) → queryAgentEvents (channel_events UNION ALL global_events 内查 + WHERE/LIMIT)
6. include(host_bridge) → 0-row placeholder (HB-1 table is not present in v1)
7. sortByTSDesc (insertion-sort, 跨源 newest-first)
8. trim to LIMIT (per-source LIMIT can miss sparse sources, so rows are merged before trimming)
9. response: `{sources: [...4...], rows: [...]}`

## 4. 行为不变量 byte-identical 反查

| 字面 | baseline | 当前 | 反查 |
|---|---|---|---|
| ADM-2 既有 /admin-api/v1/audit-log | byte-identical | byte-identical ✅ | 0 改 |
| audit_events 表 schema | ADM-3 #586 RENAME | byte-identical ✅ | 0 ALTER, 0 column add |
| DL-2 channel_events/global_events | DL-2 #615 | byte-identical ✅ | 仅 SELECT 消费 |
| user-rail has no audit/multi-source route | n/a | 0 hit ✅ | ADM-0 §1.3 isolation rule |
| admin path is isolated | byte-identical | byte-identical ✅ | uses adminMw with the admin cookie path |

## 5. 跨 milestone byte-identical 守护链

- ADM-2 #484 admin_actions/audit_events schema + AdminFromContext + adminMw 复用
- ADM-3 #586 RENAME audit_events 表 + alias view backward compat
- BPP-8 #532 plugin lifecycle action `plugin_*` prefix (DB CHECK enum)
- DL-2 #615 channel_events + global_events 双流 + mustPersistKinds (agent kind 走 cold consumer)
- reasons.IsValid #496 / NAMING-1 #614 / DL-2 mustPersistKinds enum 单一来源 同模式
- ADM-0 §1.3 admin path isolation (prevents user-rail coupling)
- post-#618 haystack gate Func=50/Pkg=70/Total=85 (跟 TEST-FIX-3-COV 一致)
- AL-8 既有 reverse-grep 测试加 `/multi-source` 白名单单一例外

## 6. Tests + verify

- `go build -tags sqlite_fts5 ./...` ✅
- `go test -tags sqlite_fts5 -timeout=300s ./...` 25+ packages 全 PASS ✅
- `pnpm exec vitest run` 99 file 655 tests 全 PASS ✅
- haystack gate TOTAL 85.6% / 0 func<50% / exit 0 ✅

## 7. grep 守门

- 4 source const 单一来源: `grep AuditSource{Server,Plugin,HostBridge,Agent} admin_audit_query.go` ==4 hit
- 0 schema 改: `git diff origin/main -- migrations/` 0 行
- admin path isolation: `grep /api/v1/audit/multi-source packages/server-go/internal/api/` 0 hit
- UNION ALL 跨 4 源: `grep -c "UNION ALL" admin_audit_query.go` ≥1 hit + `grep -E "audit_events|channel_events|global_events" admin_audit_query.go` ≥3 hit
- admin auth 复用: `grep -E "AdminFromContext|adminMw" admin_audit_query.go` ≥2 hit
- DL-2 mustPersistKinds 不破: `git diff origin/main -- must_persist_kinds.go` 0 行
- AUDIT_SOURCES 跨层锁定: server const + client AUDIT_SOURCES + SOURCE_LABEL 三处一致

## 8. Known Follow-Ups

- HB-1 audit table is not present in v1, so the host_bridge source remains a 0-row placeholder until a later HB-1 wiring PR
- Cross-source traceability (agent action → host_bridge syscall trace) is left for v3+
- audit FTS search is left for v3+; v1 uses simple LIKE filtering
- unified audit retention across sources is deferred; each source keeps its existing threshold policy (DL-2 retention sweeper / ADM-3 audit_events forward-only)
- user-rail audit feed (per-user privacy view) must never be mounted or exposed on the user API/user-rail under ADM-0 §1.3 and blueprint §3.4
- audit_events external export (Splunk/Datadog) is left for v2+
- audit FTS / pagination cursor / relevance sorting are left for v3+
