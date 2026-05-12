# DL-3 — threshold monitor + cold archive offload (≤80 行)

> Landed in PR feat/dl-3: DL3.1 (ThresholdMonitor 4 metrics) + DL3.2 (EventsArchiveOffloader cold archive) + DL3.3 closure
> Blueprint source: data-layer.md §5 threshold monitor (db_size / wal_pending / write_lock / row_count)
> Design source: [`dl-3-spec.md`](../../implementation/modules/dl-3-spec.md) §0 ① DL-1+DL-2 unchanged + ② no schema changes + four threshold metrics as a source + auto cold archive + ③ no endpoint changes + no admin endpoint

## 1. 文件清单

| 文件 | 行 | 角色 |
|---|---|---|
| `internal/datalayer/events_threshold.go` | 244 | ThresholdMonitor + DBThreshold + 4 metric collector (db_size_mb / wal_pending_pages / write_lock_wait_ms / events_row_count) + level enum (OK/Warn/Critical) |
| `internal/datalayer/events_archive_offloader.go` | 165 | EventsArchiveOffloader.RunOnce (ATTACH archive_<yyyy-mm>.db + INSERT SELECT + DELETE WHERE created_at<cutoff + EventBus audit "events.archive_offload") |
| `internal/server/server.go` 扩 | +6 | NewThresholdMonitor(...).Start(s.ctx) wired, following the sweeper startup pattern |
| `internal/datalayer/events_threshold_test.go` | 196 | 9 unit (DefaultThresholds 字面 + Classify 边界 + level.String + RunOnce 4 levels + Collect err skip + StartStop ctx-aware + ZeroInterval + RowCount roundtrip + DBSize/WAL non-negative + noopCollector) |
| `internal/datalayer/events_archive_offloader_test.go` | 175 | 4 unit (BelowThreshold no-op + OffloadsExpired full path + NoBus OK + DefaultsApplied) |

## 2. Four Threshold Constants (蓝图 §5 byte-identical)

| metric | WARN | CRITICAL | 来源 |
|---|---|---|---|
| db_size_mb | 5000 | 10000 | PRAGMA page_count*page_size/MB |
| wal_pending_pages | 1000 | 5000 | PRAGMA wal_checkpoint(PASSIVE).log_size |
| write_lock_wait_ms | 100 | 1000 | v1 noop placeholder; single-writer SQLite has no contention here |
| events_row_count | 1_000_000 | 10_000_000 | SELECT COUNT(*) FROM channel_events |

`DefaultThresholds()` is the shared definition; inline literal drift intentionally fails the check.

## 3. cold archive offload Trigger Flow

1. `RunOnce(ctx)` 读 `SELECT COUNT(*) FROM channel_events`
2. 行数 < threshold (default 1M) → no-op
3. ≥ threshold → ATTACH `archive_<yyyy-mm>.db` AS arch + CREATE TABLE IF NOT EXISTS arch.channel_events
4. transaction: INSERT SELECT WHERE created_at < cutoff (default now-30d) + DELETE 同事务 (rollback on err)
5. DETACH archive (SQLite 限制: ATTACH/DETACH 不能在 tx 内)
6. EventBus.Publish("events.archive_offload", payload) — goes through the DL-2 cold consumer so audit persistence is required

## 4. 行为不变量 byte-identical 反查

| 字面 | baseline | 当前 | 反查 |
|---|---|---|---|
| DL-1 4 interface signature | DL-1 #609 | unchanged ✅ | EventBus / Repository / PresenceStore / Storage 0 改 |
| DL-2 EventStore + RetentionSweeper | DL-2 #615 | unchanged ✅ | only Publish callers were added; store/retention is unchanged |
| 0 endpoint URL 改 | unchanged | unchanged ✅ | server.go 仅加 ThresholdMonitor.Start |
| 0 schema 改 (复用 DL-2 表) | unchanged | unchanged ✅ | 0 migration v 号 + registry.go 不动 |
| admin path does not expose event thresholds (ADM-0 §1.3) | unchanged | unchanged ✅ | slog stdout only, 0 /admin-api/threshold |

## 5. 跨 milestone byte-identical 守护链

- DL-1 #609 4 interface (EventBus unchanged)
- DL-2 #615 EventStore + retention sweeper + must_persist_kinds (offloader audit 走 cold consumer)
- 蓝图 §5 阈值哨 4 metric 字面 (db_size/wal_pending/write_lock/row_count)
- ADM-0 §1.3 admin path isolation: event threshold controls are not exposed as admin endpoints
- ctx-aware Start(ctx), matching #608/#614/#615, to avoid goroutine leaks
- post-#615 haystack gate Func=50/Pkg=70/Total=85
- 0-endpoint-改 wrapper 决策树**变体** (跟 INFRA-3/4 / REFACTOR-1/2 / NAMING-1 / DL-2 同源)

## 6. Tests + verify

- `go build -tags sqlite_fts5 ./...` ✅
- `go test -tags sqlite_fts5 -timeout=300s ./...` 全包 PASS ✅
- haystack gate TOTAL 85.6% / Pkg datalayer 89.0% / 0 func<50% (events_threshold collectors 全测) ✅

## 7. grep 守门

- DL-1+DL-2 interface 不破: `git diff origin/main -- internal/datalayer/{eventbus,repository,presence,storage,events_store,events_retention,must_persist_kinds}.go` signature 0 改
- 0 schema 改: `ls migrations/ | grep -cE 'dl_3|threshold|offload'` 0 hit
- 4 阈值 enum 单一来源: `grep -cE 'DefaultThresholds' events_threshold.go` ==1
- audit kind: `grep -cE '"events\.archive_offload"' events_archive_offloader.go` ==1
- admin endpoint 0 hit: `grep -rE '/admin-api/.*threshold|/admin-api/.*archive' packages/server-go/` 0 hit
- 0 endpoint: `git diff origin/main -- internal/server/server.go | grep -cE '\\+.*HandleFunc|\\+.*Handle\\('` 0 hit

## 8. Known Follow-Ups

- EventBus migration to NATS/Redis (蓝图 §4.C.11) is left for v2+ and should be triggered by an explicit threshold decision
- SQLite → PG/CockroachDB (蓝图 §4.C.10) is left for v2+
- Storage migration to object storage (蓝图 §4.B.8) is left for v2+; archive_offloader currently uses local disk
- Prometheus/Datadog metrics export is left for v2+ via a /metrics endpoint; it is not an admin endpoint
- events_archive cross-db UNION ALL queries are left for v3+; admins can manually attach archives when needed
- HB-2 v0(D) Borgee Helper SQLite consumer 已落 #617 (`packages/borgee-helper/`); 阈值哨 (`host_grants` 表) wire 留 v1.x
