# DL-3 — 阈值哨 + cold archive offload (≤80 行)

> 落地: PR feat/dl-3 · DL3.1 (ThresholdMonitor 4 metric) + DL3.2 (EventsArchiveOffloader cold archive) + DL3.3 closure
> 蓝图锚: data-layer.md §5 阈值哨 (db_size / wal_pending / write_lock / row_count)
> 设计沿用: [`dl-3-spec.md`](../../implementation/modules/dl-3-spec.md) §0 ① DL-1+DL-2 byte-identical + ② 0 schema 改 + 4 阈值哨 SSOT + auto cold archive + ③ 0 endpoint 改 + admin god-mode 永不挂

## 1. 文件清单

| 文件 | 行 | 角色 |
|---|---|---|
| `internal/datalayer/events_threshold.go` | 244 | ThresholdMonitor + DBThreshold + 4 metric collector (db_size_mb / wal_pending_pages / write_lock_wait_ms / events_row_count) + level enum (OK/Warn/Critical) |
| `internal/datalayer/events_archive_offloader.go` | 165 | EventsArchiveOffloader.RunOnce (ATTACH archive_<yyyy-mm>.db + INSERT SELECT + DELETE WHERE created_at<cutoff + EventBus audit "events.archive_offload") |
| `internal/server/server.go` 扩 | +6 | NewThresholdMonitor(...).Start(s.ctx) wire (sweeper 同精神承袭) |
| `internal/datalayer/events_threshold_test.go` | 196 | 9 unit (DefaultThresholds 字面 + Classify 边界 + level.String + RunOnce 4 levels + Collect err skip + StartStop ctx-aware + ZeroInterval + RowCount roundtrip + DBSize/WAL non-negative + noopCollector) |
| `internal/datalayer/events_archive_offloader_test.go` | 175 | 4 unit (BelowThreshold no-op + OffloadsExpired full path + NoBus OK + DefaultsApplied) |

## 2. 4 阈值常量 (蓝图 §5 byte-identical)

| metric | WARN | CRITICAL | 来源 |
|---|---|---|---|
| db_size_mb | 5000 | 10000 | PRAGMA page_count*page_size/MB |
| wal_pending_pages | 1000 | 5000 | PRAGMA wal_checkpoint(PASSIVE).log_size |
| write_lock_wait_ms | 100 | 1000 | v1 noop placeholder (单写 SQLite 无 contention) |
| events_row_count | 1_000_000 | 10_000_000 | SELECT COUNT(*) FROM channel_events |

`DefaultThresholds()` SSOT 单源, 反 inline 字面漂.

## 3. cold archive offload 触发流程

1. `RunOnce(ctx)` 读 `SELECT COUNT(*) FROM channel_events`
2. 行数 < threshold (default 1M) → no-op
3. ≥ threshold → ATTACH `archive_<yyyy-mm>.db` AS arch + CREATE TABLE IF NOT EXISTS arch.channel_events
4. transaction: INSERT SELECT WHERE created_at < cutoff (default now-30d) + DELETE 同事务 (rollback on err)
5. DETACH archive (SQLite 限制: ATTACH/DETACH 不能在 tx 内)
6. EventBus.Publish("events.archive_offload", payload) — 走 DL-2 cold consumer 必落 audit

## 4. 行为不变量 byte-identical 锚

| 字面 | baseline | 当前 | 锚 |
|---|---|---|---|
| DL-1 4 interface signature | DL-1 #609 | byte-identical ✅ | EventBus / Repository / PresenceStore / Storage 0 改 |
| DL-2 EventStore + RetentionSweeper | DL-2 #615 | byte-identical ✅ | 仅 Publish 调用方加, store/retention 不动 |
| 0 endpoint URL 改 | byte-identical | byte-identical ✅ | server.go 仅加 ThresholdMonitor.Start |
| 0 schema 改 (复用 DL-2 表) | byte-identical | byte-identical ✅ | 0 migration v 号 + registry.go 不动 |
| admin god-mode 不挂 events 阈值 (ADM-0 §1.3) | byte-identical | byte-identical ✅ | 仅 slog stdout 输出, 0 /admin-api/threshold |

## 5. 跨 milestone byte-identical 锁链

- DL-1 #609 4 interface (EventBus byte-identical)
- DL-2 #615 EventStore + retention sweeper + must_persist_kinds (offloader audit 走 cold consumer)
- 蓝图 §5 阈值哨 4 metric 字面 (db_size/wal_pending/write_lock/row_count)
- ADM-0 §1.3 admin god-mode 红线 (events 阈值域永不挂 admin endpoint)
- ctx-aware Start(ctx) (反 goroutine leak, 跟 #608/#614/#615 一致)
- post-#615 haystack gate Func=50/Pkg=70/Total=85
- 0-endpoint-改 wrapper 决策树**变体** (跟 INFRA-3/4 / REFACTOR-1/2 / NAMING-1 / DL-2 同源)

## 6. Tests + verify

- `go build -tags sqlite_fts5 ./...` ✅
- `go test -tags sqlite_fts5 -timeout=300s ./...` 全包 PASS ✅
- haystack gate TOTAL 85.6% / Pkg datalayer 89.0% / 0 func<50% (events_threshold collectors 全测) ✅

## 7. 反向 grep 守门

- DL-1+DL-2 interface 不破: `git diff origin/main -- internal/datalayer/{eventbus,repository,presence,storage,events_store,events_retention,must_persist_kinds}.go` signature 0 改
- 0 schema 改: `ls migrations/ | grep -cE 'dl_3|threshold|offload'` 0 hit
- 4 阈值 enum SSOT: `grep -cE 'DefaultThresholds' events_threshold.go` ==1
- audit kind: `grep -cE '"events\.archive_offload"' events_archive_offloader.go` ==1
- admin god-mode 0 hit: `grep -rE '/admin-api/.*threshold|/admin-api/.*archive' packages/server-go/` 0 hit
- 0 endpoint: `git diff origin/main -- internal/server/server.go | grep -cE '\\+.*HandleFunc|\\+.*Handle\\('` 0 hit

## 8. 留账 (透明)

- EventBus 切 NATS/Redis (蓝图 §4.C.11) 留 v2+ 阈值哨触发人工决策切
- SQLite → PG/CockroachDB (蓝图 §4.C.10) 留 v2+
- Storage 切对象存储 (蓝图 §4.B.8) 留 v2+ — archive_offloader 当前单机磁盘
- Prometheus/Datadog metrics export 留 v2+ /metrics endpoint (admin god-mode 永不挂)
- events_archive 跨 db UNION ALL 查询留 v3+ admin 必要时手动 attach
- HB-2 v0(D) Borgee Helper SQLite consumer 已落 #617 (`packages/borgee-helper/`); 阈值哨 (`host_grants` 表) wire 留 v1.x
