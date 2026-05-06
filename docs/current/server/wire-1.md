# WIRE-1 — wire-up 死代码 3 处真接 (≤80 行)

> 落地: PR feat/wire-1 · W1.1 DL-2 cold consumer + W1.2 DL-3 offloader + wire-3 RT-3 AgentTaskNotifier + W1.3 closure
> Spec 锚: [`wire-1-spec.md`](../../implementation/modules/wire-1-spec.md) §0 ① 3 处 wire-up 真接 + ② 0 schema/endpoint 改 + ③ ctx-aware 反 leak
> Trigger: G4.audit independent dev audit (zhanma-c) P0 — closure docs 字面"已 wire"但 production 0 callsite

## 1. 文件清单

| 文件 | 行 | 角色 |
|---|---|---|
| `internal/datalayer/factory.go` | -3/+5 | NewInProcessEventBusWithStore 真接 + logger 参数 + delete NewInProcessEventBus dead code |
| `internal/datalayer/v1_sqlite.go` | -3 | NewInProcessEventBus 删除 (post-WIRE-1 已无 callsite) |
| `internal/datalayer/events_archive_offloader.go` | +50 | Start(ctx) ticker driver + Done() chan + sync.Once + runOnceLog (跟 ThresholdMonitor 同精神 ctx-aware shutdown) + interval 参数 |
| `internal/server/server.go` | +13 | NewEventsArchiveOffloader.Start(s.ctx) + AgentTaskNotifier wire + SetPushFanout + channelMemberFetcherAdapter |
| `internal/bpp/task_lifecycle_handler.go` | +50 | ChannelMemberFetcher + AgentTaskPushNotifier interface + SetPushFanout + fanoutPush per-member (反 self-push agent 自身 + 空 user_id) |
| `internal/datalayer/factory_wire_test.go` | 130 | 4 wire unit (ColdConsumer_Wired + GlobalRoute_Wired + Start_TickerLoop + Start_ZeroInterval + RunOnceLog_DBError + RunOnceLog_Triggered) |
| `internal/bpp/task_lifecycle_wire_test.go` | 130 | 4 wire unit (TaskStarted_PushFanoutPerMember + TaskFinished_IdleFanout + NilFanout_NoOp + MembersErr_Skipped) |
| `internal/server/adapters_test.go` 扩 | +18 | TestChannelMemberFetcherAdapter_ListUserIDs (cov) |
| `internal/datalayer/events_archive_offloader_test.go` 扩 | +1 sig | NewEventsArchiveOffloader 加 interval=0 参数 |
| `internal/datalayer/datalayer_test.go` 扩 | +1 sig | NewDataLayer 加 logger=nil 参数 |

## 2. 3 处 wire-up 真接

### wire-1: DL-2 cold consumer
- factory.go: `EventBus: NewInProcessEventBusWithStore(NewSQLiteEventStore(s.DB(), logger))` (替 hot-only)
- 真验: `dl.EventBus.Publish` → 1s poll → channel_events / global_events INSERT count ≥ 1 (cold goroutine deterministic)

### wire-2: DL-3 EventsArchiveOffloader
- offloader.Start(ctx) ticker driver 加 (sync.Once + Done() chan + ctx-aware shutdown 跟 EventsRetentionSweeper 同模式)
- server.go: `NewEventsArchiveOffloader(s.store.DB(), s.dl.EventBus, s.logger, "", 0, 0, time.Hour).Start(s.ctx)` 跟 ThresholdMonitor 旁同精神

### wire-3: RT-3 AgentTaskNotifier
- TaskLifecycleHandler.SetPushFanout(members, notifier) → fanoutPush 调 notifier.NotifyAgentTask per channel member
- nil-safe: members 或 notifier 任一 nil → 跳 (反 panic, 反 leak); 反 self-push agent 自身 + 空 user_id
- server.go: 加 channelMemberFetcherAdapter 桥 store.ListChannelMembers → bpp.ChannelMemberFetcher

## 3. 行为不变量 byte-identical 锚

| 字面 | baseline | 当前 | 锚 |
|---|---|---|---|
| DL-1+DL-2+DL-3+DL-4 interface signature | byte-identical | byte-identical ✅ | 仅 NewEventsArchiveOffloader 加 interval 参数, NewDataLayer 加 logger 参数 (callsite 跟随) |
| 0 endpoint URL 改 | byte-identical | byte-identical ✅ | server.go 仅 +Start / +SetPushFanout, 0 HandleFunc |
| 0 schema 改 | byte-identical | byte-identical ✅ | migrations/ 0 行 |
| admin god-mode 不挂 wire 路径 (ADM-0 §1.3) | 0 hit | 0 hit ✅ | 反向 grep `admin.*EventsArchiveOffloader\|admin.*AgentTaskNotifier\|/admin-api/.*offload` 0 hit |
| ctx-aware 反 leak | byte-identical | ✅ | Start(s.ctx) 跨 RetentionSweeper / ThresholdMonitor / EventsArchiveOffloader 3 处 + sync.Once + Done() chan |

## 4. 跨 milestone byte-identical 锁链

- DL-2 #615 EventStore + EventsRetentionSweeper byte-identical 不破
- DL-3 #618 ThresholdMonitor / EventsArchiveOffloader 字面 byte-identical (仅加 Start/Done/runOnceLog ctx-aware)
- DL-4 #485 AgentTaskNotifier nil-safe 模式承袭
- RT-3 #616 TaskLifecycleHandler 字面 byte-identical (SetPushFanout 是 setter 加, BPP-3 既有 wire 模式不破)
- TEST-FIX-2 #608 ctx-aware shutdown 立场承袭 (反 goroutine leak)
- ADM-0 §1.3 admin god-mode 红线 (反 user-rail 漂)
- post-#621 haystack gate Func=50/Pkg=70/Total=85 (TEST-FIX-3-COV 立场承袭)

## 5. Tests + verify

- `go build -tags sqlite_fts5 ./...` ✅
- `go test -tags sqlite_fts5 -timeout=300s ./...` 25+ packages 全 PASS ✅
- haystack gate TOTAL 85.7% / datalayer 91.4% / bpp 93.7% / 0 func<50% / exit 0 ✅

## 6. 反向 grep 守门 (spec §2 6 锚)

- DL-2 cold consumer: `grep -cE 'NewInProcessEventBusWithStore' factory.go` ==1 + `func NewInProcessEventBus()` 0 hit (已删)
- DL-3 offloader 真启: `grep -cE 'EventsArchiveOffloader.*Start\(' server.go` ==1
- AgentTaskNotifier 真接: `grep -cE 'NotifyAgentTask' task_lifecycle_handler.go` ≥1 + `SetPushFanout` server.go ≥1
- 0 endpoint URL: `git diff -- server.go | grep -cE '^\+.*HandleFunc'` 0 hit
- 0 schema: `git diff -- migrations/` 0 行 + `grep -cE '^\+\s*Version:'` 0 hit
- ctx-aware: `grep -cE 'Start\(s\.ctx\)' server.go` ≥3 hit (Retention + Threshold + Offloader)

## 7. 留账 (透明)

- events 接 RT-3 fanout 上游 hook (DL-2 cold → RT-3 hub.PushFrame 桥) 留 v1.x follow-up
- HB-2 v0(D) Borgee Helper SQLite consumer 已落 #617 (`packages/borgee-helper/internal/grants/sqlite_consumer.go`); 阈值哨 wire 留 v1.x
- ADM-3 v1 host_bridge placeholder 真接 留 ADM-3.bis (HB-1 audit 表 v1 未落)
