# DL-2 — events 双流 + retention (≤80 行)

> 落地: PR feat/dl-2 · DL2.1 (schema v=46/47) + DL2.2 (EventStore + retention sweeper + cold consumer) + DL2.3 closure
> 蓝图锚: data-layer.md §2.7 / §3.4 必落清单 / §4.A.4 ULID
> 设计沿用: [`dl-2-spec.md`](../../implementation/modules/dl-2-spec.md) §0 ① DL-1 byte-identical + ② 双流 enum SSOT + ③ 0 user-facing 改

## 1. 文件清单

| 文件 | 行 | 角色 |
|---|---|---|
| `internal/migrations/channel_events.go` | 53 | v=46 channel_events 表 + 2 idx |
| `internal/migrations/global_events.go` | 56 | v=47 global_events 表 + 2 idx |
| `internal/datalayer/must_persist_kinds.go` | 67 | 4 类必落 prefix SSOT + RetentionDaysForKind |
| `internal/datalayer/events_store.go` | 110 | sqliteEventStore (PersistChannel/Global + ULID lex_id + sync.Mutex) |
| `internal/datalayer/events_retention.go` | 116 | EventsRetentionSweeper (Start/Done/RunOnce, ctx-aware) |
| `internal/datalayer/v1_sqlite.go` 扩 | +30 | inProcessEventBus.store + NewInProcessEventBusWithStore + cold-stream 异步 INSERT |
| `internal/server/server.go` 扩 | +5 | NewEventsRetentionSweeper(...).Start(s.ctx) wire |
| `internal/datalayer/events_test.go` | 220 | 9 unit tests (覆 IsMustPersistKind / RetentionDaysForKind / IsChannelScopedKind / PersistChannel/Global / EventBus 双流 hot+cold deterministic / sweeper RunOnce + 必落 NULL retention 不删 / Start/Stop ctx-aware + ZeroInterval) |

## 2. 蓝图 §3.4 必落清单 4 类

| Prefix | 例子 kind | 隐私契约 |
|---|---|---|
| `perm.` | perm.grant / perm.revoke | 权限授予/撤销永不删 |
| `impersonate.` | impersonate.start / impersonate.end | 模拟会话审计永不删 |
| `agent.state` | agent.state | agent 上下线永不删 |
| `admin.force_` | admin.force_delete / admin.force_disable | admin 强删/禁用永不删 |

`IsMustPersistKind(kind)` SSOT, `RetentionDaysForKind` 返 -1 sentinel = 永不 reap.

## 3. retention 阈值 (per-kind default)

| kind 范围 | 默认 retention_days | 来源 |
|---|---|---|
| must-persist 4 类 | -1 (永久) | 蓝图 §3.4 隐私契约 |
| `channel.*` / `message.*` | 30 | 蓝图 §4 retention |
| `agent_task.*` / `artifact.*` | 60 | 蓝图 §4 retention |
| 其他 | 90 | 蓝图 §4 default |

row-level `retention_days` 列覆盖 default (NULL = use kind default).

## 4. 行为不变量 byte-identical 锚

| 字面 | baseline | 当前 | 锚 |
|---|---|---|---|
| EventBus.Publish/Subscribe signature | DL-1 #609 | byte-identical ✅ | NewInProcessEventBus backward-compat 不动 |
| 既有 EventBus caller | byte-identical | byte-identical ✅ | DL-2 是 additive (新加 store 可选字段) |
| 0 endpoint URL 改 | byte-identical | byte-identical ✅ | server.go 仅加 sweeper Start, routes 0 改 |
| 0 schema column 改 (既有表) | byte-identical | byte-identical ✅ | 仅加 channel_events + global_events 新表 |
| 0 migration v 号字面改 (≤45) | byte-identical | byte-identical ✅ | v=46/v=47 顺位扩, 不动既有 |

## 5. 跨 milestone byte-identical 锁链

- DL-1 #609 4 interface (EventBus 不破)
- reasons.IsValid #496 / AP-4-enum #591 / NAMING-1 #614 enum SSOT (mustPersistKinds 单源)
- AL-7 #533 + HB-5 audit retention sweeper 模式 (events_retention sweeper 同精神)
- ULID lex_id 蓝图 §4.A.1+§4.A.4 (channel_events + global_events 主键 + cursor)
- ctx-aware Start(ctx) 反 goroutine leak (跟 #608 + #614 一致)
- post-#614 haystack gate Func=50/Pkg=70/Total=85 (跟 TEST-FIX-3-COV 一致)
- 0-endpoint-改 wrapper 决策树**变体** (跟 INFRA-3/4 / CV-15 / TEST-FIX-3 / REFACTOR-1/2 / NAMING-1 同源)

## 6. Tests + verify

- `go build -tags sqlite_fts5 ./...` ✅
- `go test -tags sqlite_fts5 -timeout=300s ./...` 24 包全 PASS ✅ (含 9 新 events_test PASS, deterministic via sync.WaitGroup)
- post-NAMING-1 haystack gate TOTAL 85.6%, no func<50%, no pkg<70% ✅

## 7. 反向 grep 守门

- DL-1 signature 不破: `git diff origin/main -- eventbus.go | grep -E '^-.*Publish|^-.*Subscribe'` 0 hit
- channel_events + global_events 单表: `ls migrations/ | grep -cE 'channel_events|global_events'` ==2
- mustPersistKinds 单源: `grep -cE 'MustPersistKindPrefixes|IsMustPersistKind' must_persist_kinds.go` ≥1 hit
- EventsRetentionSweeper 单源: `grep -cE 'func .*EventsRetentionSweeper' events_retention.go` ≥1
- 双流分离: `grep -E 'INSERT INTO channel_events|INSERT INTO global_events' events_store.go` ==2 hit (cold) + hot stream chan Event 不动
- 0 endpoint URL: `git diff origin/main -- server.go | grep -cE '\\+.*HandleFunc|\\+.*Handle\\('` 0 hit (仅 sweeper.Start 加)

## 8. 留账 (透明)

- DL-3 阈值哨 (DB size / WAL checkpoint / write lock wait 监控) 留 DL-3 单 milestone
- EventBus 切 NATS/Redis 留 DL-3 阈值哨触发再启
- HB-2 v0(D) Borgee Helper SQLite consumer 已落 #617 (`packages/borgee-helper/internal/grants/sqlite_consumer.go`)
- session_resume_hint 表 (蓝图 §2.7) 留 DL-5+
- events fanout 接 RT-3 留后续 PR
- per-user events feed / inbox 留 DL-5+
- events FTS 搜索 留 v3+
