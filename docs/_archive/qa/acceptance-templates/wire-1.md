# Acceptance Template — WIRE-1 (server.go production wire grep 硬锚 + acceptance §2 整改)

> Spec brief `wire-1-spec.md` (飞马 v0). Owner: 战马C 实施 / 飞马 review / 烈马 验收.
>
> **WIRE-1 范围**: 整改"acceptance §2 走过场没真测 production wire"系统问题 (烈马交叉核验自责) — 加 CI step `acceptance-wire-grep` 守门, 反向断 acceptance template 标 `NewXxxXxxx(...)` SSOT 在 server.go body 必 ≥1 hit. 立场承袭 INFRA-3 #594 progress-line-budget + post-#621 G4.audit closure pattern. **0 server prod 行为改 + CI step 硬锚**.

## 验收清单

### §1 数据契约 (CI step 字面锁)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 1.1 `release-gate.yml::acceptance-wire-grep` step 加, 跑 `git grep "NewXxx" server.go` 验各 acceptance §1/§2 标的 production seam wire | CI yml | `release-gate.yml` 字面 + CI run PASS |
| 1.2 5 PR retroactive verify (RT-3/DL-2/DL-3/AP-2/HB-2 v0(D)) — server.go grep ≥1 hit each (NewTaskLifecycleHandler / NewEventsRetentionSweeper / NewThresholdMonitor / capability bundle endpoint / borgee-helper grant) | grep | reverse grep test PASS, 5 hit ≥1 each |

### §2 行为不变量 (反 走过场 acceptance §2)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 2.1 acceptance template 反向 grep guard — 标 `NewXxxYyy\|server.NewXxx\|StartCxx\|RegisterXxxRoutes` SSOT pattern 必在 server.go ≥1 hit (反 server.go wire 死代码) | unit + grep | `TestWire1_AcceptanceWireGrep_Guard` PASS |
| 2.2 future PR review 模板加 `[ ] server.go wire grep ≥1 hit` 硬锚 (跟 PR #612 反 spam 标准 / NAMING-1 codebase-wide 命名规范同精神) | inspect | template 文件存在 |

### §3 closure (REG + cov gate + 跨 milestone 锁)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 3.1 既有全包 unit + e2e + vitest 全绿不破 + post-#621 haystack gate 三轨过 (Func=50/Pkg=70/Total=85) | full test + CI | go-test-cov SUCCESS |
| 3.2 0 endpoint 行为改 + 0 schema (CI step + 文档 only) | git diff | `git diff main -- internal/migrations/` 0 行 ✅ |
| 3.3 立场承袭 跨 14 milestone const SSOT 锁链 + ADM-0 §1.3 admin god-mode 红线 (反 admin path bypass wire grep) | grep | reverse grep test PASS |

## REG-WIRE1-* (initial ⚪ → 🟢 flipped 2026-05-01 战马C 实施)

- REG-WIRE1-001 🟢 wire-1 DL-2 cold consumer 真接 (`factory.go:38` `EventBus: NewInProcessEventBusWithStore(eventStore)`, hot-only `NewInProcessEventBus()` 已删除 dead code) — `TestFactory_EventBus_ColdConsumer_Wired` + `_GlobalRoute_Wired` PASS (Publish → channel_events / global_events 真 INSERT, 1s poll deterministic)
- REG-WIRE1-002 🟢 wire-2 DL-3 offloader 真启 (`server.go:460` `NewEventsArchiveOffloader(...).Start(s.ctx)` 跟 ThresholdMonitor 同精神 1h ticker, ctx-aware shutdown) — `TestEventsArchiveOffloader_Start_TickerLoop` + `_ZeroInterval` + `_RunOnceLog_DBError` + `_RunOnceLog_Triggered` PASS (反 goroutine leak)
- REG-WIRE1-003 🟢 wire-3 RT-3 AgentTaskNotifier 真接 (`task_lifecycle_handler.go::SetPushFanout` + `fanoutPush` 调 `notifier.NotifyAgentTask` per channel member, 反 self-push agent 自己 + 空 user_id) — 4 wire test PASS (`TestWire3_TaskStarted_PushFanoutPerMember` + `_TaskFinished_IdleFanout` + `_NilFanout_NoOp` + `_MembersErr_Skipped`)
- REG-WIRE1-004 🟢 0 endpoint URL / 0 routes.go / 0 schema 改 — `git diff origin/main -- internal/server/server.go` 0 HandleFunc / `internal/migrations/` 0 行 / 0 ALTER COLUMN
- REG-WIRE1-005 🟢 ctx-aware 真守 (反 leak) — `Start(s.ctx)` 跨 RetentionSweeper + ThresholdMonitor + EventsArchiveOffloader 3 处 + AgentTaskNotifier 走 hub.PushAgentTaskStateChanged ctx 既有
- REG-WIRE1-006 🟢 post-#621 haystack gate 三轨过 (TOTAL 85.7% / 0 func<50% / datalayer 91.4% / bpp 93.7%) + 既有 25+ packages 全 PASS

## 退出条件

- §1-§3 全绿 — 一票否决
- CI step `acceptance-wire-grep` 守门 + 5 PR retroactive verify
- 反 走过场 acceptance §2 整改 (PR review 模板 + reverse grep guard)
- 全包 PASS + haystack gate + 0 schema / 0 endpoint
- 登记 REG-WIRE1-001..006

## 更新日志

| 日期 | 作者 | 变化 |
|---|---|---|
| 2026-05-01 | 烈马 | v0 — acceptance template 草稿. 立场承袭 G4.audit closure 烈马交叉核验自责 (5 PR acceptance §2 没真 grep server.go wire 字面) + INFRA-3 #594 progress-line-budget CI 守门链同精神. 跨十五 milestone const SSOT 锁链延伸 (BPP-2 + REFACTOR-REASONS + DM-9 + CHN-15 + AP-4-enum + DL-1 + REFACTOR-1 + REFACTOR-2 + NAMING-1 + DL-2 + DL-3 + ADM-3 v1 + AP-2 v1 + CS v1 + WIRE-1). |
