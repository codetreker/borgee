# RT-3 ⭐ `agent_task_state_changed` Frame — implementation note

> RT-3 (#488) · Phase 4 · 蓝图 [`realtime.md`](../../../blueprint/current/realtime.md) §0 + §1.1 ⭐ "thinking 必须带 subject" + agent-lifecycle.md §2.3 (busy/idle source 必须 plugin 上行 frame).

## 1. 立场

`AgentTaskStateChangedFrame` server→client push 来自 BPP-2.2 task_started/finished 上行 frame **server 派生** (不另起独立 source). 多端 fanout 走 `Hub.BroadcastToChannel`, 一 user 多 ws session 全收 (跟 P1MultiDeviceWebSocket #197 同源 `hub.onlineUsers map[userID]map[*Client]bool` 数据结构).

## 2. Frame schema (`internal/ws/agent_task_state_changed_frame.go`)

```
{type, cursor, agent_id, state, subject, reason, changed_at}  // 7 字段 byte-identical
```

| 字段 | 备注 |
|---|---|
| `type` | `"agent_task_state_changed"` |
| `cursor` | `hub.cursors.NextCursor()` — RT-3 是第 6 个共序 frame (跟 RT-1.1 ArtifactUpdated / CV-2.2 AnchorCommentAdded / DM-2.2 MentionPushed / CV-4.2 IterationStateChanged / AL-2b AgentConfigUpdate 共一根 sequence; 反约束: 不另起 agent-only 通道) |
| `agent_id` | target agent UUID |
| `state` | 2-enum `'busy'` \| `'idle'` (中间态 reject) |
| `subject` | busy 时**必带非空** (蓝图 §1.1 ⭐); idle 时**必为空** (反字典污染) |
| `reason` | idle + failed-derived 时填 AL-1a 6 字典 byte-identical (复用 `internal/agent/state.go::Reason*` SSOT); 否则空 |
| `changed_at` | Unix ms 语义戳 — cursor IS the order, 此字段是 audit hint |

## 3. ⭐ 反约束 — 沉默胜于假 loading

蓝图 §1.1 字面: "BPP `progress` frame **强制带 `subject` 字段**——plugin 必须告诉 Borgee 'agent 在做什么', 否则不展示" + "沉默胜于假 loading. 假装活物感 = 用户立刻看穿 = 信任崩塌."

`TestRT3_ReverseGrep_NoSubjectFallback` 守门 5 pattern (file 内 prod 路径 count==0):

- empty subject default (字面赋空字符串)
- default-named symbol (`defaultSubject` 等)
- fallback-named symbol (`fallbackSubject` 等)
- 无信息硬编码字符串 (`"thinking"` / `"AI is thinking"`)

`subject` byte-identical 跟 BPP-2.2 `task_lifecycle.go::ValidateTaskStarted` 同源 — server 派生不重写, plugin 上行字面承袭.

## 4. Hub 推送入口

| Method | 用途 |
|--------|------|
| `Hub.PushAgentTaskStateChanged(agentID, channelID, state, subject, reason, changedAt) (cursor int64, sent bool)` | RT-3 派生 push — 分配 cursor + BroadcastToChannel + SignalNewEvents; sent=false 仅当 hub 无 cursor allocator (test seam) |

## 5. RT-3.2 server派生 hook (PR #588 — TaskLifecycleHandler)

`internal/bpp/task_lifecycle_handler.go::TaskLifecycleHandler` — 走 BPP-2.2 `ValidateTaskStarted` / `ValidateTaskFinished` SSOT (BPP-2.2 #485 既有 validator), 验证通过后调 `AgentTaskPusher.PushAgentTaskStateChanged` (生产路径走 `internal/server/server.go::hubAgentTaskPusherAdapter` 跨 bpp↛ws 包边界胶水, 跟 hubLivenessAdapter / channelScopeAdapter 同模式).

**派生映射**:

| Plugin upstream frame | 派生 state | subject | reason |
|---|---|---|---|
| `task_started` (Subject 必带非空, ValidateTaskStarted 守) | `busy` | 透传 plugin Subject | 必空 |
| `task_finished` outcome=`completed` (reason 必空, 字典污染防御) | `idle` | 必空 | 必空 |
| `task_finished` outcome=`failed` + reason ∈ AL-1a 6-dict | `idle` | 必空 | 透传 plugin reason |
| `task_finished` outcome=`cancelled` (reason 必空) | `idle` | 必空 | 必空 |

**fail-closed 路径** (反 fallback push):

- empty subject → `errSubjectEmpty` reject + pusher 0 calls (TestRT3_HandleStarted_EmptySubjectRejected 锁)
- 中间态 outcome (`partial` / `paused` / `pending` / `starting`) → `errOutcomeUnknown` reject + 0 calls (TestRT3_HandleFinished_InvalidOutcome_Rejected)
- completed/cancelled + non-empty reason → 字典污染 reject + 0 calls (TestRT3_HandleFinished_CompletedWithReason_RejectedDictPollution)

**boot wire** (`internal/server/server.go`):

```go
taskLifecycleHandler := bpp.NewTaskLifecycleHandler(
    &hubAgentTaskPusherAdapter{hub: hub}, logger)
pfd.Register(bpp.FrameTypeBPPTaskStarted,  taskLifecycleHandler.StartedAdapter())
pfd.Register(bpp.FrameTypeBPPTaskFinished, taskLifecycleHandler.FinishedAdapter())
```

11 TestRT3_* unit PASS (HandleStarted happy/empty + HandleFinished 4 case + 2 adapter raw decode + nil pusher panic + sentinel chain).

## 6. 锚

- 实施: `internal/ws/agent_task_state_changed_frame.go` + `agent_task_state_changed_frame_test.go` (6 test 全绿) + `rt_3_multi_device_test.go` (live multi-device fanout) + **`internal/bpp/task_lifecycle_handler.go` + `_test.go` (RT-3.2, PR #588, 11 test 全绿)** + `internal/server/server.go` 真挂 pfd.Register 2 frame
- spec brief: [`docs/implementation/modules/rt-3-spec.md`](../../../implementation/modules/rt-3-spec.md)
- stance: [`docs/qa/rt-3-stance-checklist.md`](../../../qa/rt-3-stance-checklist.md)
- acceptance: [`docs/qa/acceptance-templates/rt-3.md`](../../../qa/acceptance-templates/rt-3.md)
- deferred (留 follow-up PR): RT-3.4 DL-4 Web Push fallback (DL-4 #490 已 merge 6/7) / RT-3.3 client UI / RT-3.5 e2e + ⭐ yema G4.x 5 张 signoff 截屏 demo
