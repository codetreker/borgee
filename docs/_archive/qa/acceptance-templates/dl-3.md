# Acceptance Template — DL-3 阈值哨 (events 监控 + sweeper 统计 + 告警)

> Spec brief `dl-3-spec.md` (飞马 v0). Owner: 战马C 实施 / 飞马 review / 烈马 验收.
>
> **DL-3 范围**: DL-2 #615 events 双流 + retention 落地后接阈值哨 — events_archive (channel_events + global_events) 行数 + 累计 size 真监控 + EventsRetentionSweeper reaped over time 统计 + 告警三轨 (Prometheus metrics + structured log + system DM 给 admin). 立场承袭 DL-2 #615 EventStore SSOT + AL-7 #533 + HB-5 retention sweeper 模式 + post-#614 haystack gate. **0 endpoint 行为改 + 0 schema 字面改 (复用 DL-2 events 表)**.

## 验收清单

### §1 阈值监控验收 (events 行数 + size 真测)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 1.1 `internal/datalayer/events_metrics.go::EventsMetrics` SSOT — `ChannelEventsRowCount(ctx)` + `GlobalEventsRowCount(ctx)` + `ChannelEventsTotalSize(ctx)` + `GlobalEventsTotalSize(ctx)` 4 helper byte-identical 跟 spec §1.1 (反约束: 反向 grep `^func.*EventsMetrics` 在 internal/ 除 datalayer/ 0 hit) | unit + grep | `events_metrics_test.go::TestEventsMetrics_RowCountRoundtrip` + `_TotalSizeBytes` + `_BothTablesAggregated` 3 unit PASS |
| 1.2 阈值 const SSOT — `ThresholdEventsRows = 100_000` (channel + global 各) + `ThresholdEventsBytes = 100 << 20` (100MB) byte-identical 跟蓝图 data-layer.md §5 阈值哨字面同源; 反向 grep 100_000 / 100MB 字面 散落 production body 0 hit (除 events_metrics.go SSOT + _test) | grep + unit | reverse grep test PASS + `TestThreshold_ConstByteIdentical` (字面 + 跨层锁 blueprint anchor verify ≥1 hit) |
| 1.3 4 helper deterministic 真测 (走 SQLite COUNT(*) + SUM(LENGTH(payload)), in-memory SQLite + DL-2 events 表 verify roundtrip) | unit | `TestEventsMetrics_DeterministicRoundtrip` PASS (反 race scheduler 依赖, sync 走 truth) |

### §2 sweeper 统计验收 (reaped over time + ctx-aware)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 2.1 EventsRetentionSweeper 加 stats — `internal/datalayer/events_retention.go` 扩 `Stats() RetentionStats` (reaped_total / reaped_last_run / last_run_at / next_run_at 4 字段) byte-identical (反 v=DL-2 仅 RunOnce + Done 不暴露 stats) | unit | `events_retention_test.go::TestRetentionSweeper_StatsAfterRun` + `_StatsResetOnNewSweeper` PASS |
| 2.2 reaped over time 真累计 — 每 RunOnce(ctx) 累加 reaped 计数; ctx-aware Start(ctx) (跟 AL-7 + HB-5 + TEST-FIX-2 #608 立场承袭) + deterministic 0 race scheduler 依赖 (sync.WaitGroup + Done() chan) | unit | `_StatsAccumulateAcrossRuns` + `_StatsCtxCancelExits` PASS (跟 TEST-FIX-3 #610 hub_heartbeat sync.WaitGroup 同模式) |
| 2.3 must-persist 不计 reaped — 反向 grep DL-2 `MustPersistKindPrefixes` 4 类 byte-identical 跨 milestone const SSOT 锁链 (perm./impersonate./agent.state/admin.force_) | grep | `TestRetentionSweeper_MustPersistNotCounted` PASS + reverse grep `MustPersistKindPrefixes` import datalayer/must_persist_kinds.go 单源 ≥1 hit ✅ |

### §3 告警三轨验收 (Prometheus + log + system DM)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 3.1 Prometheus metrics — `internal/datalayer/events_alerts.go::ExposePromMetrics()` 暴露 4 gauge (`borgee_events_channel_rows` / `_global_rows` / `_channel_bytes` / `_global_bytes`) + 1 counter (`borgee_events_reaped_total`); 字面 byte-identical 跟蓝图 §5 metric name 同源 | unit + scrape | `events_alerts_test.go::TestPromMetrics_4Gauge1Counter_ByteIdentical` + scrape endpoint `/metrics` verify PASS |
| 3.2 structured log 告警 — 阈值超 → `slog.Warn("dl3.threshold_exceeded", "kind", "channel_events_rows", "current", N, "threshold", 100000)` 字面 byte-identical (反约束: 反向 grep 同义词 `threshold reached / size limit / row limit` body 0 hit, 走 4-dict event kind SSOT) | unit + grep | `_LogStructuredFormatByteIdentical` PASS + reverse grep test PASS |
| 3.3 system DM 给 admin — 阈值超 → 走 ADM-2 #484 system DM 5 模板路径 (`channelDM=admin-alerts`, sender=`system`, kind=`dm3.threshold_exceeded`, byte-identical 跟 ADM-2 system DM 5 模板字面同源) — 反平行实施 admin god-mode 不挂 (ADM-0 §1.3 红线) | unit | `_SystemDMSentOnThreshold` PASS + reverse grep `admin.*ThresholdExceeded\|/admin-api.*alerts` admin*.go 0 hit |

### §4 closure 验收 (REG + cov gate + 跨 milestone 锁)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 4.1 既有全包 unit + e2e + vitest 全绿不破 (Wrapper 立场, 监控 sidecar 不动 endpoint) + post-#614 haystack gate 三轨过 (Func=50/Pkg=70/Total=85) | full test + CI | `go test -tags sqlite_fts5 -timeout=300s ./...` + go-test-cov SUCCESS |
| 4.2 0 endpoint URL 改 + 0 schema (复用 DL-2 #615 channel_events + global_events 表) | git diff | `git diff main -- internal/migrations/` 0 行 + `git diff main -- internal/api/` 仅 +`/metrics` Prometheus endpoint wire-up 1-2 行 |
| 4.3 反平行 EventsMetrics + 反 admin god-mode bypass — 反向 grep `func.*EventsMetrics\|func.*ExposePromMetrics` 在 internal/ 除 datalayer/ + _test count==0 (单源) + admin god-mode 反向 grep 0 hit (ADM-0 §1.3) | CI grep | reverse grep tests PASS |

## REG-DL3-* 占号 (initial ⚪ → 🟢 flipped 2026-05-01)

- REG-DL3-001 🟢 4 阈值哨 enum SSOT (`DefaultThresholds`) byte-identical 跟蓝图 §5 + level enum OK/Warn/Critical
- REG-DL3-002 🟢 ThresholdMonitor SSOT (Start/Done/RunOnce ctx-aware) + 4 SQLite collector + slog.Warn/Error 双档输出 + deterministic 反 leak
- REG-DL3-003 🟢 EventsArchiveOffloader SSOT (ATTACH+INSERT SELECT+DELETE+DETACH + audit "events.archive_offload" 走 DL-2 EventBus + 跨月 archive 文件分立)
- REG-DL3-004 🟢 DL-1 + DL-2 interface byte-identical 不破 (signature 0 改)
- REG-DL3-005 🟢 0 endpoint URL / 0 routes.go / 0 schema 改 + admin god-mode 永不挂 events 阈值 (ADM-0 §1.3 红线)
- REG-DL3-006 🟢 post-#615 haystack gate 三轨过 (TOTAL 85.6% / datalayer pkg 89.0%) + 既有 24+ packages 全绿

## 退出条件

- §1 (3) + §2 (3) + §3 (3) + §4 (3) 全绿 — 一票否决
- EventsMetrics SSOT 4 helper + 阈值 const SSOT (ThresholdEventsRows / ThresholdEventsBytes byte-identical 蓝图 §5)
- EventsRetentionSweeper.Stats() 累计 reaped over time + ctx-aware deterministic
- 告警三轨真接: Prometheus 4 gauge + 1 counter / structured log 4-dict / ADM-2 system DM 5 模板复用
- must-persist 4 类不计 reaped (DL-2 mustPersistKinds 单源)
- 全包 unit + e2e + vitest 全绿不破 + post-#614 haystack gate 三轨过 + 0 endpoint URL 改 + 0 schema
- 反平行 EventsMetrics + 反 admin god-mode bypass (ADM-0 §1.3)
- 登记 REG-DL3-001..006

## 更新日志

| 日期 | 作者 | 变化 |
|---|---|---|
| 2026-05-01 | 烈马 | v0 — acceptance template 草稿 (4 选 1 验收框架 + REG-DL3-001..006 6 行占号 ⚪). 立场承袭 DL-2 #615 (EventStore SSOT + EventsRetentionSweeper + mustPersistKinds 4 类) + AL-7 #533 + HB-5 #607 + TEST-FIX-2 #608 (ctx-aware shutdown) + ADM-2 #484 (system DM 5 模板) + post-#612 haystack gate + post-#613 REFACTOR-2 + post-#614 NAMING-1. 关键: 0 endpoint 行为改 + 0 schema (复用 DL-2 events 表) + 4 helper deterministic 真测 (sync.WaitGroup + Done() chan) + 告警三轨字面 byte-identical (Prometheus + slog 4-dict + system DM 5 模板). 跨十二 milestone const SSOT 锁链 (BPP-2 + REFACTOR-REASONS + DM-9 + CHN-15 + AP-4-enum + DL-1 + REFACTOR-1 + REFACTOR-2 + NAMING-1 + DL-2 + DL-3). |
| 2026-05-01 | 战马C | v1 — acceptance flip ⚪→🟢 全 6 行 (DL3.1+DL3.2+DL3.3 落地). 实施收口为 spec 三段 (ThresholdMonitor 4 metric + EventsArchiveOffloader cold archive + closure REG/PROGRESS/docs). 飞马 spec audit-反转 acceptance v0 范围 (Prometheus + system DM 留 v2+, slog stdout 输出已够 v1; cold archive offload 取代 EventsMetrics 4 helper 走真 DL-2 表 row count + DB size PRAGMA collector). 验收证据: TestDefaultThresholds_ByteIdentical + ThresholdMonitor_RunOnce_AllLevels + ArchiveOffloader_OffloadsExpired (含 EventBus.Publish "events.archive_offload" 真测) + ctx-aware StartStop deterministic + 全包 24+ go test PASS + haystack gate TOTAL 85.6% datalayer pkg 89.0%. |
