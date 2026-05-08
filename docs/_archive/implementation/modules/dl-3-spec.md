# DL-3 spec brief — 阈值哨 + cold archive offload (≤80 行)

> 飞马 · 2026-05-01 · 用户拍板 (DL-2 #615 ✅ → DL-3) · zhanma 主战 + 飞马 review
> **关联**: DL-1 #609 ✅ 4 interface · DL-2 #615 ✅ events 双流 + retention sweeper · DL-4 #485 ✅ Web Push · 蓝图 data-layer.md §5 阈值哨
> **命名**: DL-3 = data-layer 第三件 (阈值哨), 跟 DL-1/DL-2 平行

> ⚠️ Server-side observability + offload milestone — **0 schema 改 / 0 endpoint URL 改 / 0 user-facing API 行为改**.
> v1 单机阈值哨 (复用 DL-2 表), 不引外部监控 (Prometheus/Datadog 留 v2+).

## 0. 关键约束 (3 条立场)

1. **DL-1 4 interface byte-identical 不破 + DL-2 EventBus byte-identical 不破** (跨 DL stack 锁链承袭): EventBus signature 跟 #609/#615 byte-identical; Repository interface signature 字面不动; PresenceStore / Storage 不动. 反约束: `git diff origin/main -- internal/datalayer/{eventbus,repository,presence,storage}.go` 跟 #615 等量 (signature 0 改, 仅 events_threshold.go 新文件).

2. **0 schema 改 + 复用 DL-2 表 + 4 阈值哨 + auto cold archive offload SSOT**:
   - **0 schema 改** — 复用 DL-2 #615 既有 `channel_events` + `global_events` 表, 0 migration v 号, 0 column add
   - **4 阈值哨 enum SSOT** `dbThreshold`: 
     - `db_size_mb` (默认 5000 MB → WARN, 10000 → CRITICAL)
     - `wal_pending_pages` (默认 1000 → WARN, 5000 → CRITICAL)
     - `write_lock_wait_ms` (默认 100 → WARN, 1000 → CRITICAL)
     - `events_row_count` (默认 1M → WARN, 10M → CRITICAL)
   - **统计源**: 复用 DL-2 sweeper 既有 `channelReaped/globalReaped int64` return 加 metrics counter (跟 retention sweeper 同 entry, 不另起 sweeper goroutine)
   - **auto cold archive offload**: 当 `events_row_count > offload_threshold` 触发 `EventsArchiveOffloader.RunOnce(ctx)` — 走 SQLite `INSERT INTO archive_path SELECT FROM channel_events WHERE created_at < N + DELETE`. v1 archive_path 是单机磁盘 (`./data/events_archive_<yyyy-mm>.db`), v2+ 切对象存储 (蓝图 §4.B.8 Storage interface)
   - 反约束: `dbThreshold` enum SSOT count==1 hit; 4 阈值常量字面 byte-identical 跟蓝图 §5

3. **0 endpoint URL / 0 routes.go / 0 user-facing API 改 + admin god-mode 永久不挂** (DL-3 立场, 跟 DL-1/DL-2 wrapper 系列承袭): PR diff 仅 (a) `internal/datalayer/events_threshold.go` 新 (~120 行 ThresholdMonitor + 4 metric collector) (b) `internal/datalayer/events_archive_offloader.go` 新 (~80 行 cold archive) (c) `cmd/server/main.go` wire ThresholdMonitor.Start(ctx) 走 sweeper 同 ctx (跟 DL-2 同精神承袭). 反约束: 0 endpoint URL 改 + 0 routes.go 改 + 0 schema column 改 + 0 migration v 号 + admin god-mode 不挂 events 阈值 (反向 grep `admin.*events.*threshold|/admin-api/.*threshold` 0 hit, ADM-0 §1.3 红线).

## 1. 拆段实施 (3 段, 一 milestone 一 PR)

| 段 | 文件 | 范围 |
|---|---|---|
| **DL3.1 ThresholdMonitor 4 阈值哨** | `internal/datalayer/events_threshold.go` 新 (~120 行 ThresholdMonitor struct + Start(ctx) + RunOnce(ctx) + 4 metric collector: dbSizeMB / walPendingPages / writeLockWaitMs / eventsRowCount + level enum WARN/CRITICAL + slog Logger.Warn/Error 输出); `events_threshold_test.go` ~6 case (4 阈值各 normal/warn/critical 各 1 + ctx-aware shutdown) | 战马 / 飞马 review |
| **DL3.2 EventsArchiveOffloader cold archive** | `internal/datalayer/events_archive_offloader.go` 新 (~80 行 RunOnce(ctx) 触发条件 + INSERT INTO archive_<yyyy-mm>.db SELECT + DELETE WHERE created_at < cutoff + audit log "events.archive_offload" 走 DL-2 EventBus.Publish 必落 kind); `events_archive_offloader_test.go` ~4 case (offload trigger / archive 文件创建 / 原表 DELETE 真 / EventBus.Publish "events.archive_offload" 真测) | 战马 / 飞马 review |
| **DL3.3 closure** | REG-DL3-001..008 (8 反向 grep + DL-1+DL-2 interface byte-identical + 4 阈值 enum SSOT + offloader trigger 真 + 0 schema 改 + 0 endpoint 改 + admin 永久不挂 + haystack 三轨过 + 既有 test 全 PASS) + acceptance + content-lock 不需 (server-only observability) + 4 件套 spec 第一件 | 战马 / 烈马 |

## 2. 反向 grep 锚 (8 反约束)

```bash
# 1) DL-1 + DL-2 interface byte-identical 不破
git diff origin/main -- packages/server-go/internal/datalayer/eventbus.go packages/server-go/internal/datalayer/repository.go packages/server-go/internal/datalayer/presence.go packages/server-go/internal/datalayer/storage.go | grep -cE '^-.*func.*(Publish|Subscribe|Get|List|Create|Update|Delete|IsOnline|Sessions|GetURL|PutBlob)\('  # 0 hit

# 2) 0 schema 改 (复用 DL-2 表)
ls packages/server-go/internal/migrations/ | grep -cE 'dl_3_|threshold|offload'  # 0 hit
git diff origin/main -- packages/server-go/internal/migrations/registry.go | grep -cE '^\+'  # 0 hit (registry 不动)

# 3) 4 阈值 enum SSOT (单源, 反 inline 字面漂)
grep -rcE 'type DBThreshold |dbSizeMB|walPendingPages|writeLockWaitMs|eventsRowCount' packages/server-go/internal/datalayer/events_threshold.go  # ≥4 hit (4 metric)
grep -rcE 'ThresholdLevel(Warn|Critical)' packages/server-go/internal/datalayer/events_threshold.go  # ≥2 hit (level enum)

# 4) ThresholdMonitor + EventsArchiveOffloader ctx-aware (反 leak)
grep -rE 'ctx\.Done\(\)|context\.Context' packages/server-go/internal/datalayer/events_threshold.go packages/server-go/internal/datalayer/events_archive_offloader.go  | wc -l  # ≥4 hit (各 ≥2)

# 5) auto offload 走 DL-2 EventBus.Publish 必落 kind (跟 DL-2 ConsumeKinds 链承袭)
grep -rE 'EventBus.*Publish.*"events\.archive_offload"|"admin\.force.*offload"' packages/server-go/internal/datalayer/events_archive_offloader.go  # ≥1 hit (audit log 走 DL-2 cold consumer)

# 6) admin god-mode 永久不挂 (ADM-0 §1.3 红线)
grep -rE 'admin.*events.*threshold|admin.*archive_offload|/admin-api/.*threshold|/admin-api/.*archive' packages/server-go/  # 0 hit

# 7) 0 endpoint URL / 0 routes.go 改
git diff origin/main -- packages/server-go/internal/server/server.go | grep -cE '^\+.*HandleFunc|^\+.*Handle\('  # 0 hit

# 8) haystack gate 三轨 + 既有 test
THRESHOLD_FUNC=50 THRESHOLD_PACKAGE=70 THRESHOLD_TOTAL=85 BUILD_TAGS="sqlite_fts5" go run ./scripts/lib/coverage/  # ALL ≥阈值
go test -tags 'sqlite_fts5' -timeout=300s ./...  # ALL PASS
```

## 3. 不在范围 (留账)

- ❌ **EventBus 切 NATS/Redis** (蓝图 §4.C.11) — DL-3 阈值哨触发后人工决策切, 不自动 (留 v2+)
- ❌ **SQLite → PG/CockroachDB** (蓝图 §4.C.10) — DL-3 阈值哨触发后人工决策, 不自动 (留 v2+)
- ❌ **Storage interface 切对象存储** (蓝图 §4.B.8) — archive_offloader v1 单机磁盘, v2+ 走 Storage interface
- ❌ **Prometheus/Datadog metrics export** — slog Logger.Warn/Error 走 stdout 即够 v1, 留 v2+ 加 /metrics endpoint (admin god-mode 永久不挂)
- ❌ **events_archive 跨 db 查询合并 (UNION ALL)** — v1 archive 单向 (write-only), 不查 (留 v3+ admin 必要时手动 attach)
- ❌ **HB-2 v0(D) Borgee Helper 阈值哨 (host_grants 表)** — 跟 events 域不同 concern, 留 HB-2 follow-up

## 4. 跨 milestone byte-identical 锁

- 复用 DL-1 #609 4 interface byte-identical (signature 不改)
- 复用 DL-2 #615 EventBus + must_persist_kinds.go SSOT (offloader audit 走 DL-2 cold consumer)
- 复用 DL-2 #615 retention sweeper ctx-aware 模式 (ThresholdMonitor + ArchiveOffloader 同精神)
- 复用 蓝图 §5 阈值哨 4 metric byte-identical (db_size / wal_pending / write_lock / row_count)
- 复用 ADM-0 §1.3 admin god-mode 不挂红线 (events 阈值域永久不挂)
- 0-schema-改 wrapper 决策树**变体**: 跟 INFRA-3/4 / REFACTOR-1/2 / NAMING-1 / RT-3 / DL-2 同源

## 5. 派活 + 双签

派 **zhanma-c** (DL-2 #615 主战熟手, 续作减学习成本) 或 zhanma-d. 飞马 review.

双签流程: spec brief → team-lead → 飞马自审 ✅ APPROVED → yema stance + liema acceptance → zhanma 起实施 (DL3.1+2+3 三段一 PR, **teamlead 唯一开 PR**).

## 6. 飞马 (架构师) 自审表态

✅ **APPROVED with 2 必修条件**:

🟡 必修-1: **DL-1 + DL-2 interface byte-identical 双锁** — 反约束 grep #1 真守, `git diff -- internal/datalayer/` 4 interface signature 行 0 hit. zhanma PR body 必示 diff 输出.

🟡 必修-2: **0 schema 改真守** — 反约束 grep #2 真守, ls migrations/ 不增 + registry.go 不动. zhanma PR body 必示 ls + diff 输出.

担忧 (1 项, 中度):
- 🟡 4 阈值默认值 (5000/10000/100/1000/1M/10M) 是 v1 真值估算, 真上线后可能需调 — 战马实施时全走 const 单源 + 注释明示 "v1 估算 + 可 follow-up tune", 反 hardcode 散落.

留账接受度全 ✅: EventBus 切 NATS / SQLite 切 PG / Storage 切对象存储 / Prometheus / events_archive 查询 / HB-2 v0(D) 阈值哨 全留账, 不强塞本 PR.

**ROI 拍**: DL-3 ⭐⭐ — 蓝图 §5 阈值哨兑现 + cold archive offload 解锁 events 长期增长路径; 跟 HB-2 v0(D) / RT-3 follow-up 不撞 (DL-3 是 data-layer 域 observability).
