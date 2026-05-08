# Acceptance Template — DL-2 (events 双流 + retention sweeper)

> Spec brief `dl-2-spec.md` (飞马 v0). Owner: 战马C 实施 / 飞马 review / 烈马 验收.
>
> **DL-2 范围**: 接 DL-1 #609 4 interface 抽象之 EventBus 真落地 — `channel_events` (per-channel) + `global_events` (全局) 双 SQLite 表 v=46 ALTER + EventBus consumer 真接 + retention sweeper deterministic (固定 retention enum: `permanent` / `90d` / `30d` / `7d`). 立场承袭 DL-1 #609 + AL-7 RetentionSweeper #533 + HB-5 HeartbeatRetentionSweeper #607 既有 ctx-aware shutdown + post-#612 haystack gate. **0 endpoint 行为改 + 0 schema 字面 retention**.

## 验收清单

### §1 schema 验收 (v=46 + retention enum + drift test)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 1.1 schema migration v=46 — `internal/migrations/event_streams.go` ALTER 创 `channel_events` (id ULID PK / channel_id / kind / payload_json / created_at / retention TEXT NOT NULL) + `global_events` (id ULID PK / kind / payload_json / created_at / retention TEXT NOT NULL); Version: 46 byte-identical 不复用 | unit | `internal/migrations/event_streams_test.go::TestEventStreamsCreates2Tables` (PRAGMA verify) + `_VersionIs46` + `_Idempotent` 3 unit PASS |
| 1.2 retention enum 4-dict byte-identical (`permanent` / `90d` / `30d` / `7d`) — const SSOT 在 `internal/datalayer/retention.go::RetentionEnum`; 反向 grep retention 字面值 散落 production body 0 hit (除 const 单源 + _test) | grep + unit | `TestRetentionEnum_4DictByteIdentical` (字面 + len exact 反第 5 词污染) + reverse grep `"permanent"\|"90d"\|"30d"\|"7d"` body 0 hit (除 retention.go SSOT) |
| 1.3 drift test — Version sequence 严守严格递增 (45→46 不跳号), retention enum 跨层 byte-identical (server-go const ↔ blueprint data-layer.md §2.7 字面同源) | unit + grep | `TestRegistryStrictlyIncreasing` PASS + blueprint anchor verify ≥1 hit |

### §2 server 验收 (EventBus consumer + retention sweeper deterministic)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 2.1 EventBus consumer 真接 SQLite — `internal/datalayer/eventbus.go::SQLiteEventBus` 实现 DL-1 #609 EventBus interface (Publish 写 channel_events 或 global_events 按 scope; Subscribe 走 in-process map fanout 跟 v1 单机立场承袭, 反 NATS/Redis dep) | unit + grep | `eventbus_test.go::TestSQLiteEventBus_PublishWritesRow` + `_SubscribeFanout_InProcessMap` 2 unit PASS + 反向 grep `redis\|nats\|kafka` 0 hit |
| 2.2 retention sweeper deterministic — `internal/datalayer/retention_sweeper.go::EventRetentionSweeper.Start(ctx)` 走 ctx.Done() (跟 AL-7 #533 + HB-5 #607 + TEST-FIX-2 #608 ctx-aware shutdown 立场承袭, 反 context.Background() leak); 4 retention 各自 DELETE WHERE created_at < now() - retention.Duration() (permanent 不 sweep) | unit | `retention_sweeper_test.go::TestRetentionSweeper_PermanentSkipped` + `_90dExpired` + `_30dExpired` + `_7dExpired` + `_CtxCancelExits` + `_TickIdempotent` 6 unit PASS (deterministic 0 race scheduler 依赖, 跟 TEST-FIX-3 #610 hub_heartbeat_test sync.WaitGroup + done chan 同模式) |
| 2.3 反向断言 0 endpoint 行为改 — DL-2 是 server 内部 EventBus 实现, 不动既有 endpoint shape; `git diff main -- packages/server-go/internal/api/` 0 行 (除 EventBus DI wire-up server.go 1-2 行) | git diff | `git diff main --shortstat packages/server-go/internal/api/` ≤2 行 |
| 2.4 EventBus consumer 跟 RT-1 #290 cursor opaque + DM-3 #508 mention push 路径不污染 (DL-2 仅 events table 写, 不动 cursor / WS push 通道; 反向 grep `cursor\|WS push\|hub.Broadcast` 在 datalayer/eventbus.go body count==0 除注释) | grep | reverse grep test PASS |

### §3 closure 验收 (REG + cov gate + 跨 milestone 锁)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 3.1 既有全包 unit + e2e + vitest 全绿不破 (Wrapper 立场, EventBus 接 SQLite 不动 endpoint) | full test | `go test -tags sqlite_fts5 -timeout=300s ./...` 24+ packages 全 PASS + e2e + vitest 全 PASS |
| 3.2 post-#612 haystack gate 三轨过 — Func=50 / Pkg=70 / Total=85 (cov gate 立场承袭 #612, internal/datalayer/ 新代码 cov ≥50% per func) | CI verify | go-test-cov SUCCESS + TOTAL ≥85% no func<50% no pkg<70% |
| 3.3 反平行 EventBus / 反 admin god-mode bypass — 反向 grep `func.*EventBus.*Publish\|func.*EventBus.*Subscribe` 在 internal/ 除 datalayer/ + _test count==0 (单源 SQLiteEventBus) + admin god-mode 反向 grep `admin.*event.*publish\|/admin-api.*events` admin*.go 0 hit (ADM-0 §1.3 红线) | CI grep | reverse grep tests PASS |

## REG-DL2-* 占号 (initial ⚪)

- REG-DL2-001 🟢 schema migration v=46 (channel_events + global_events 双表 ALTER + retention TEXT NOT NULL) + drift test (Version 严格递增 45→46) + idempotent
- REG-DL2-002 🟢 retention enum 4-dict byte-identical (`permanent` / `90d` / `30d` / `7d`) + const SSOT in internal/datalayer/retention.go + 跨层锁 (blueprint data-layer.md §2.7 字面同源)
- REG-DL2-003 🟢 SQLiteEventBus 真接 SQLite (Publish 写 events 表 + Subscribe in-process map fanout, 反 NATS/Redis dep — 跟 v1 单机立场承袭)
- REG-DL2-004 🟢 EventRetentionSweeper.Start(ctx) ctx-aware shutdown (跟 AL-7 + HB-5 + TEST-FIX-2 立场承袭) + 4 retention 各自 sweep + permanent 不 sweep + deterministic 0 race scheduler 依赖 (sync.WaitGroup + done chan)
- REG-DL2-005 🟢 0 endpoint 行为改 + 既有 24+ packages unit + e2e + vitest 全绿不破 + post-#612 haystack gate 三轨过 (Func=50/Pkg=70/Total=85)
- REG-DL2-006 🟢 反平行 EventBus + 反 admin god-mode bypass (ADM-0 §1.3) + 跨十一 milestone const SSOT 锁链承袭 (BPP-2 + REFACTOR-REASONS + DM-9 + CHN-15 + AP-4-enum + DL-1 + REFACTOR-1 + REFACTOR-2 + NAMING-1 + DL-2)

## 退出条件

- §1 (3) + §2 (4) + §3 (3) 全绿 — 一票否决
- migration v=46 + drift test PASS (Version 严格递增 + retention enum byte-identical)
- SQLiteEventBus 真接 SQLite + EventRetentionSweeper deterministic ctx-aware
- 全包 unit + e2e + vitest 全绿不破 + post-#612 haystack gate 全过
- 0 endpoint 行为改 + 0 schema 字面 retention 漂
- 反平行 EventBus + 反 admin god-mode bypass
- 登记 REG-DL2-001..006

## 更新日志

| 日期 | 作者 | 变化 |
|---|---|---|
| 2026-05-01 | 烈马 | v0 — acceptance template 草稿 (3 选 1 验收框架 + REG-DL2-001..006 6 行占号 ⚪). 立场承袭 DL-1 #609 (EventBus interface 抽象源) + AL-7 #533 + HB-5 #607 + TEST-FIX-2 #608 (ctx-aware shutdown) + post-#612 haystack gate (Func=50/Pkg=70/Total=85) + post-#613 REFACTOR-2 + post-#614 NAMING-1 锁链. 关键: schema v=46 + retention 4-dict const SSOT + EventBus 真接 SQLite + retention sweeper deterministic 0 race scheduler 依赖. 跨十一 milestone const SSOT 锁链 (BPP-2 + REFACTOR-REASONS + DM-9 + CHN-15 + AP-4-enum + DL-1 + REFACTOR-1 + REFACTOR-2 + NAMING-1 + DL-2). |

| 2026-05-01 | 战马C | flip — REG-DL2-001..006 6 ⚪→🟢 实施验收 PASS. 实测: schema v=46 channel_events + v=47 global_events 双表分立 (lex_id ULID PK + 2 idx 各) + EventStore (sqliteEventStore PersistChannel/Global + sync.Mutex 串行) + EventsRetentionSweeper (ctx-aware Start/Done/RunOnce, 跟 AL-7/HB-5 同模式) + mustPersistKinds 4 类 SSOT (perm/impersonate/agent.state/admin.force_) + EventBus cold consumer 异步 INSERT (hot byte-identical 不破) + server.go wire NewEventsRetentionSweeper(...).Start(s.ctx) + 9 unit tests deterministic via sync.WaitGroup. 24 包 test 全 PASS, post-NAMING-1 haystack gate TOTAL 85.6% no func<50% no pkg<70%. DL-1 interface signature byte-identical (Publish/Subscribe 0 改, NewInProcessEventBus backward-compat). 0 endpoint URL 改 / 0 schema column drift / Version v=46/v=47 串行不撞. 留账透明: DL-3 阈值哨 / EventBus 切 NATS/Redis / HB-2 v0(D) / session_resume_hint / events 接 RT-3 / per-user feed / FTS — 全留各自 milestone. |
