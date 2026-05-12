# WIRE-1 — connect three previously unused integration points (≤80 行)

> Landed in PR feat/wire-1: W1.1 DL-2 cold consumer + W1.2 DL-3 offloader + wire-3 RT-3 AgentTaskNotifier + W1.3 closure
> Spec source: [`wire-1-spec.md`](../../implementation/modules/wire-1-spec.md) §0 ① three integration points wired + ② no schema/endpoint changes + ③ context-aware shutdown to avoid leaks
> Trigger: G4.audit independent dev audit (zhanma-c) P0 — closure docs said the paths were wired, but production had 0 call sites

## 1. 文件清单

| 文件 | 行 | 角色 |
|---|---|---|
| `internal/datalayer/factory.go` | -3/+5 | NewInProcessEventBusWithStore wired + logger parameter + delete NewInProcessEventBus dead code |
| `internal/datalayer/v1_sqlite.go` | -3 | NewInProcessEventBus 删除 (post-WIRE-1 已无 callsite) |
| `internal/datalayer/events_archive_offloader.go` | +50 | Start(ctx) ticker driver + Done() chan + sync.Once + runOnceLog (same context-aware shutdown model as ThresholdMonitor) + interval parameter |
| `internal/server/server.go` | +13 | NewEventsArchiveOffloader.Start(s.ctx) + AgentTaskNotifier wire + SetPushFanout + channelMemberFetcherAdapter |
| `internal/bpp/task_lifecycle_handler.go` | +50 | ChannelMemberFetcher + AgentTaskPushNotifier interface + SetPushFanout + fanoutPush per member, excluding self-push to the agent and empty user_id |
| `internal/datalayer/factory_wire_test.go` | 130 | 4 wire unit (ColdConsumer_Wired + GlobalRoute_Wired + Start_TickerLoop + Start_ZeroInterval + RunOnceLog_DBError + RunOnceLog_Triggered) |
| `internal/bpp/task_lifecycle_wire_test.go` | 130 | 4 wire unit (TaskStarted_PushFanoutPerMember + TaskFinished_IdleFanout + NilFanout_NoOp + MembersErr_Skipped) |
| `internal/server/adapters_test.go` 扩 | +18 | TestChannelMemberFetcherAdapter_ListUserIDs (cov) |
| `internal/datalayer/events_archive_offloader_test.go` 扩 | +1 sig | NewEventsArchiveOffloader adds interval=0 parameter |
| `internal/datalayer/datalayer_test.go` 扩 | +1 sig | NewDataLayer adds logger=nil parameter |

## 2. Three Wired Integration Points

### wire-1: DL-2 cold consumer
- factory.go: `EventBus: NewInProcessEventBusWithStore(NewSQLiteEventStore(s.DB(), logger))` (replaces the hot-only bus)
- Verification: `dl.EventBus.Publish` → 1s poll → channel_events / global_events INSERT count ≥ 1 (cold goroutine deterministic)

### wire-2: DL-3 EventsArchiveOffloader
- offloader.Start(ctx) ticker driver added (sync.Once + Done() chan + context-aware shutdown matching EventsRetentionSweeper)
- server.go: `NewEventsArchiveOffloader(s.store.DB(), s.dl.EventBus, s.logger, "", 0, 0, time.Hour).Start(s.ctx)` runs alongside ThresholdMonitor

### wire-3: RT-3 AgentTaskNotifier
- TaskLifecycleHandler.SetPushFanout(members, notifier) → fanoutPush calls notifier.NotifyAgentTask for each channel member
- nil-safe: if members or notifier is nil, skip the fanout; also skip self-push to the agent and empty user_id
- server.go: adds channelMemberFetcherAdapter to bridge store.ListChannelMembers → bpp.ChannelMemberFetcher

## 3. 行为不变量 byte-identical 反查

| 字面 | baseline | 当前 | 反查 |
|---|---|---|---|
| DL-1+DL-2+DL-3+DL-4 interface signature | byte-identical | byte-identical ✅ | 仅 NewEventsArchiveOffloader 加 interval 参数, NewDataLayer 加 logger 参数 (callsite 跟随) |
| 0 endpoint URL 改 | byte-identical | byte-identical ✅ | server.go 仅 +Start / +SetPushFanout, 0 HandleFunc |
| 0 schema 改 | byte-identical | byte-identical ✅ | migrations/ 0 行 |
| admin path must not expose wire paths (ADM-0 §1.3) | 0 hit | 0 hit ✅ | grep 检查 `admin.*EventsArchiveOffloader\|admin.*AgentTaskNotifier\|/admin-api/.*offload` 0 hit |
| context-aware leak prevention | byte-identical | ✅ | Start(s.ctx) across RetentionSweeper / ThresholdMonitor / EventsArchiveOffloader + sync.Once + Done() chan |

## 4. 跨 milestone byte-identical 守护链

- DL-2 #615 EventStore + EventsRetentionSweeper byte-identical 不破
- DL-3 #618 ThresholdMonitor / EventsArchiveOffloader 字面 byte-identical (仅加 Start/Done/runOnceLog ctx-aware)
- DL-4 #485 AgentTaskNotifier nil-safe 同模式
- RT-3 #616 TaskLifecycleHandler 字面 byte-identical (SetPushFanout 是 setter 加, BPP-3 既有 wire 模式不破)
- TEST-FIX-2 #608 ctx-aware shutdown 同模式 (反 goroutine leak)
- ADM-0 §1.3 admin path isolation (prevents user-rail coupling)
- post-#621 haystack gate Func=50/Pkg=70/Total=85 (跟 TEST-FIX-3-COV 一致)

## 5. Tests + verify

- `go build -tags sqlite_fts5 ./...` ✅
- `go test -tags sqlite_fts5 -timeout=300s ./...` 25+ packages 全 PASS ✅
- haystack gate TOTAL 85.7% / datalayer 91.4% / bpp 93.7% / 0 func<50% / exit 0 ✅

## 6. grep 守门 (spec §2 6 反查)

- DL-2 cold consumer: `grep -cE 'NewInProcessEventBusWithStore' factory.go` ==1 + `func NewInProcessEventBus()` 0 hit (已删)
- DL-3 offloader is started: `grep -cE 'EventsArchiveOffloader.*Start\(' server.go` ==1
- AgentTaskNotifier is wired: `grep -cE 'NotifyAgentTask' task_lifecycle_handler.go` ≥1 + `SetPushFanout` server.go ≥1
- 0 endpoint URL: `git diff -- server.go | grep -cE '^\+.*HandleFunc'` 0 hit
- 0 schema: `git diff -- migrations/` 0 行 + `grep -cE '^\+\s*Version:'` 0 hit
- ctx-aware: `grep -cE 'Start\(s\.ctx\)' server.go` ≥3 hit (Retention + Threshold + Offloader)

## 7. Known Follow-Ups

- events to RT-3 fanout upstream hook (DL-2 cold → RT-3 hub.PushFrame bridge) stays for a v1.x follow-up
- HB-2 v0(D) Borgee Helper SQLite consumer 已落 #617 (`packages/borgee-helper/internal/grants/sqlite_consumer.go`); 阈值哨 wire 留 v1.x
- ADM-3 v1 host_bridge placeholder 真接 留 ADM-3.bis (HB-1 audit 表 v1 未落)
