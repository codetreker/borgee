# ADM-3 v1 — multi-source audit 合并查询 (≤80 行)

> 落地: PR feat/adm-3 · ADM3.1 (server query helper + admin endpoint) + ADM3.2 (client admin UI) + ADM3.3 closure
> 蓝图出处: admin-model.md §1.4 来源透明 (人/agent/admin/混合)
> 设计沿用: [`adm-3-spec.md`](../../implementation/modules/adm-3-spec.md) v1 §0 ① DL-1+DL-2 + ADM-2/ADM-3 #586 audit_events byte-identical + ② 0 schema 改 + 4 source enum 单一来源 + ③ 0 user-rail / admin god-mode 路径独立

## 1. 文件清单

| 文件 | 行 | 角色 |
|---|---|---|
| `internal/api/admin_audit_query.go` | 260 | 4 source enum 单一来源 + AdminAuditMultiSourceHandler + MultiSourceAuditQuery UNION ALL + queryAuditEvents + queryAgentEvents + sortByTSDesc |
| `internal/api/admin_audit_query_test.go` | 250 | 9 server unit (4 source byte-identical + 4 source UNION + filter + InvalidSource + TimeRange + InvalidTimeRange + OrderTSDesc + UserCookieRejected + UnauthRejected + LimitClamp + HostBridgePlaceholder) |
| `internal/server/server.go` 扩 | +5 | NewAdminAuditMultiSourceHandler.RegisterAdminRoutes(s.mux, adminMw) wire |
| `internal/api/agent_log_filter_test.go` 改 | +1 allow | AL-8 既有 reverse-grep `/admin-api/v1/audit/<not-log>` 加 `/multi-source` 单一白名单 (spec §0 ② 授权端点) |
| `packages/client/src/admin/api.ts` 扩 | +35 | AUDIT_SOURCES 4-tuple + AuditSource type + MultiSourceAuditRow + fetchMultiSourceAudit |
| `packages/client/src/admin/pages/MultiSourceAuditPage.tsx` | 150 | 4 source badge + filter dropdown + table view + DOM 出处 |
| `packages/client/src/admin/AdminApp.tsx` 扩 | +3 | nav 加 `/admin/audit-multi-source` route |
| `packages/client/src/__tests__/MultiSourceAuditPage.test.tsx` | 130 | 7 vitest (AUDIT_SOURCES byte-identical + DOM 出处 + per-source row + SOURCE_LABEL + empty state + error alert + filter triggers fetch + 反同义词 reject) |

## 2. 4 source enum 单一来源 (蓝图 §1.4 byte-identical)

| source const | 字面 | 数据源 |
|---|---|---|
| `AuditSourceServer` | `"server"` | audit_events (action 非 plugin_*) — ADM-2 #484 admin actions |
| `AuditSourcePlugin` | `"plugin"` | audit_events (action plugin_* prefix) — BPP-8 #532 lifecycle |
| `AuditSourceHostBridge` | `"host_bridge"` | HB-1 audit 表 (placeholder, 0 行) — 留 HB-1 后续 PR 真接 |
| `AuditSourceAgent` | `"agent"` | DL-2 #615 channel_events + global_events UNION ALL |

`AuditSources` slice 4-elem ordering 单一来源 (改 = 改 server const + client `AUDIT_SOURCES` + i18n `SOURCE_LABEL` 三处).

## 3. UNION ALL 跨 4 源查询流程

1. ?source filter validate (4 enum 单一例外 → 400 `audit.source_invalid`)
2. ?since/?until ms epoch reject negative/non-int → 400 `audit.time_range_invalid`
3. include(server || plugin) → queryAuditEvents (audit_events SELECT + WHERE created_at + LIMIT)
4. project source by `action[:7] == "plugin_"` (BPP-8 enum prefix)
5. include(agent) → queryAgentEvents (channel_events UNION ALL global_events 内查 + WHERE/LIMIT)
6. include(host_bridge) → 0 行 placeholder (HB-1 表未落 v1)
7. sortByTSDesc (insertion-sort, 跨源 newest-first)
8. trim to LIMIT (per-source LIMIT 可能漏 sparse 源, 走 merge-then-trim)
9. response: `{sources: [...4...], rows: [...]}`

## 4. 行为不变量 byte-identical 反查

| 字面 | baseline | 当前 | 反查 |
|---|---|---|---|
| ADM-2 既有 /admin-api/v1/audit-log | byte-identical | byte-identical ✅ | 0 改 |
| audit_events 表 schema | ADM-3 #586 RENAME | byte-identical ✅ | 0 ALTER, 0 column add |
| DL-2 channel_events/global_events | DL-2 #615 | byte-identical ✅ | 仅 SELECT 消费 |
| user-rail 0 audit/multi-source 脱节 | n/a | 0 hit ✅ | ADM-0 §1.3 红线 |
| admin god-mode 路径独立 | byte-identical | byte-identical ✅ | 走 adminMw (admin cookie 路径分叉) |

## 5. 跨 milestone byte-identical 守护链

- ADM-2 #484 admin_actions/audit_events schema + AdminFromContext + adminMw 复用
- ADM-3 #586 RENAME audit_events 表 + alias view backward compat
- BPP-8 #532 plugin lifecycle action `plugin_*` prefix (DB CHECK enum)
- DL-2 #615 channel_events + global_events 双流 + mustPersistKinds (agent kind 走 cold consumer)
- reasons.IsValid #496 / NAMING-1 #614 / DL-2 mustPersistKinds enum 单一来源 同模式
- ADM-0 §1.3 admin god-mode 红线 (反 user-rail 脱节)
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
- admin god-mode 路径独立: `grep /api/v1/audit/multi-source packages/server-go/internal/api/` 0 hit
- UNION ALL 跨 4 源: `grep -c "UNION ALL" admin_audit_query.go` ≥1 hit + `grep -E "audit_events|channel_events|global_events" admin_audit_query.go` ≥3 hit
- admin auth 复用: `grep -E "AdminFromContext|adminMw" admin_audit_query.go` ≥2 hit
- DL-2 mustPersistKinds 不破: `git diff origin/main -- must_persist_kinds.go` 0 行
- AUDIT_SOURCES 跨层锁定: server const + client AUDIT_SOURCES + SOURCE_LABEL 三处一致

## 8. 遗留 (透明)

- HB-1 audit 表未落 v1 — host_bridge source 走 placeholder 0 行 (留 HB-1 后续真接)
- 跨 source 反向追溯链 (agent action → host_bridge syscall trace) 留 v3+
- audit FTS 搜索 留 v3+ (本 v1 走 LIKE 简单 filter)
- audit retention 跨 source 统一 留 各 source 既有阈值 (DL-2 retention sweeper / ADM-3 audit_events forward-only)
- user-rail audit feed (per-user 隐私视图) **永不挂** (ADM-0 §1.3 红线 + 蓝图 §3.4 必落 kind concern 不同)
- audit_events external export (Splunk/Datadog) 留 v2+
- audit FTS / pagination cursor / sort by relevance 全留 v3+
