# BPP-6 — plugin cold-start handshake + state re-derive

> **单一来源 pointer.** Implementation in
> `packages/server-go/internal/bpp/cold_start_handler.go` +
> `packages/server-go/internal/bpp/envelope.go` (ColdStartHandshakeFrame
> + FrameTypeBPPColdStartHandshake). Wire-up at server boot in
> `packages/server-go/internal/server/server.go`.

## Why

BPP-5 #503 ships `reconnect_handshake` for plugins whose **socket dropped
but process is alive** — they hold `last_known_cursor` and resume the
shared event sequence via RT-1.3 #296 ResolveResume.

But plugins also crash/SIGKILL/restart — when the **plugin process dies**,
in-memory state is lost; on respawn the new process has no
`last_known_cursor` and cannot use the BPP-5 reconnect path. Before
BPP-6, such cold-start cases either silently masqueraded as `connect`
(losing the agent's prior `error/runtime_*` audit trail) or required
ad-hoc out-of-band recovery flows on the plugin side.

BPP-6 ships a dedicated `cold_start_handshake` frame that signals
process restart (no cursor) and triggers a fresh `online` transition
audited as `runtime_crashed`.

## 原则 (蓝图 §1.6 + §2.1 + AL-1 #492 原则一致)

- **cold-start ≠ reconnect.** `cold_start_handshake` = BPP envelope
  第 15 frame, direction lock plugin→server (server 永不发). 字段集
  与 `ReconnectHandshakeFrame` **互斥反向断言** — cold-start 不含
  `LastKnownCursor` / `DisconnectAt` / `ReconnectAt` (spec §0.1).
  AST scan + reverse grep `cold_start.*last_known_cursor\|cold_start.*resume\|cold_start.*cursor` 守门 0 hit.
- **agent state 重新 derive (反向 BPP-5).** Handler steps:
  ① `agent.Tracker.Clear(agentID)` — drop in-memory state;
  ② `Store.AppendAgentStateTransition(agentID, fromState, online,
  runtime_crashed, "")` via AL-1 #492 single-gate, where `fromState`
  is read from `ListAgentStateLog(agentID, 1)` (3 valid edges:
  initial / error / offline → online; if already online, no-op);
  ③ **NO history replay** — handler does not invoke `ResolveResume`
  (spec §0.2). AST reverse-grep `ResolveResume` / `SessionResumeRequest`
  / `ResumeModeIncremental` / `DefaultResumeLimit` 在
  `cold_start_handler.go` ident scan 0 hit.
- **AL-1a reason 6-dict 不扩第 7.** Reason `runtime_crashed` is
  reused byte-identical (反映上次 error → 此次复活语义). reasons 单一来源
  #496 6-dict 不动. AL-1a reason 守护链 BPP-6 = **第 11 处** (BPP-2.2
  第 7 + AL-2b 第 8 + BPP-4 第 9 + BPP-5 第 10 + BPP-6 第 11).
- **Restart count audit-only, count derived from state-log.** No
  separate `plugin_restart_count` column; restart frequency is read
  via `COUNT(WHERE to_state='online' AND reason='runtime_crashed')`
  on demand. AST reverse-grep `plugin_restart_count` /
  `cold_start_count` / `restart_counter` 0 hit (spec §0.3).
- **Best-effort, no retry queue (BPP-4+BPP-5 守护链延伸第 3 处).**
  Handler returns success/failure exactly once; no persistent
  cold-start state, no pending queue, no backoff timer. AST
  reverse-grep forbids `pendingColdStart` / `coldStartQueue` /
  `deadLetterColdStart` — 守护链延伸 from BPP-4 dead_letter_test
  + BPP-5 reconnect_handler_test.
- **No transient `cold_starting` state.** AL-1 5-state graph (#492)
  is unchanged — single-gate jumps any → online directly. (Mirrors
  BPP-5 conceptual `connecting` deferred — name only, no persisted
  state.)
- **ADM-0 §1.3 red-line.** admin-api does NOT mount this path;
  reverse grep `admin.*cold_start.*handshake\|admin.*BPP6` 在
  `internal/api/admin*.go` 0 hit.

## Frame schema (envelope.go)

```go
type ColdStartHandshakeFrame struct {
    Type          string `json:"type"`           // "cold_start_handshake"
    PluginID      string `json:"plugin_id"`
    AgentID       string `json:"agent_id"`
    RestartAt     int64  `json:"restart_at"`     // Unix ms
    RestartReason string `json:"restart_reason"` // e.g. "sigkill", "panic", "oom"; opaque, audit-only
}
```

## Field set vs ReconnectHandshakeFrame (BPP-5) — 字段集互斥反向断言

| Field            | BPP-5 (reconnect) | BPP-6 (cold_start) |
|------------------|:---:|:---:|
| Type             | ✓ | ✓ |
| PluginID         | ✓ | ✓ |
| AgentID          | ✓ | ✓ |
| LastKnownCursor  | ✓ | **✗** (互斥反向断言, spec §0.1) |
| DisconnectAt     | ✓ | **✗** (互斥反向断言) |
| ReconnectAt      | ✓ | **✗** (互斥反向断言) |
| RestartAt        | ✗ | ✓ |
| RestartReason    | ✗ | ✓ |

`TestBPP6_FrameSet_NoReconnectFields` asserts the 互斥 invariant via
`reflect`. CI lint grep 检查 `cold_start.*last_known_cursor\|cold_start.*resume\|cold_start.*cursor` 0 hit.

## Wire path (server.go boot)

```go
coldStartHandler := bpp.NewColdStartHandler(s, ownerResolver, srv.agentTracker, logger)
pfd.Register(bpp.FrameTypeBPPColdStartHandshake, coldStartHandler)
```

The handler is wired into the existing BPP-3 #489 `PluginFrameDispatcher`
(no new ws hub method, no new sub-protocol). Reuses the same
`OwnerResolver` + `AgentErrorClearer` interface seams as BPP-5
reconnect handler.

`AgentStateAppender` is a new interface seam (`AppendAgentStateTransition`
+ `ListAgentStateLog`) routing through `*store.Store` — bpp 包不直
import store 业务边界, 跟 BPP-3/4/5 同 interface 注入模式.

## Validation order (cold_start_handler.go::Dispatch)

1. Decode raw → `ColdStartHandshakeFrame` (malformed → wrapped error).
2. `frame.AgentID` non-empty (else 400 `bpp.cold_start_handshake_invalid`).
3. cross-owner check: `owner.OwnerOf(frame.AgentID) == sess.OwnerUserID`;
   mismatch → `errColdStartCrossOwnerReject` + log warn
   `bpp.cold_start_cross_owner_reject` (跟 BPP-3/4/5 ACL 同模式).
4. Resolve current state via `ListAgentStateLog(agentID, 1)`:
   no history → `from = AgentStateInitial`; last row → `from = last.ToState`.
5. If `from != AgentStateOnline`, `Store.AppendAgentStateTransition(
   agentID, from, online, runtime_crashed, "")` (AL-1 #492 single-gate;
   ValidateTransition守 graph + reason). If `from == AgentStateOnline`
   already, skip transition (same-state would reject), but step 6 still
   runs.
6. `clearer.Clear(frame.AgentID)` — drop in-memory tracker state.
7. Log info `bpp.cold_start_handshake_received` w/ plugin_id, agent_id,
   restart_at, restart_reason, from_state.

## Tests (cold_start_handler_test.go) — 9 unit cases

- `TestBPP6_FieldOrder` — 5 字段顺序锁定 byte-identical.
- `TestBPP6_DirectionLock` — `FrameDirection() == DirectionPluginToServer`.
- `TestBPP6_FrameSet_NoReconnectFields` — 字段集互斥反向断言 (spec §0.1).
- `TestBPP6_Handler_TransitionsToOnline_FromInitial` / `_FromError` /
  `_FromOffline` — 3 valid edges via AL-1 #492 single-gate.
- `TestBPP6_Handler_CrossOwnerReject` — sentinel `errColdStartCrossOwnerReject`.
- `TestBPP6_Handler_NilSafeCtor` — 3 deps panic on nil.
- `TestBPP6_RestartCount_DerivedFromStateLog` — 3 cold-start dispatches
  → COUNT(to_state='online' AND reason='runtime_crashed') == 3 (设计 ③
  反向 derive).
- `TestBPP6_Handler_DoesNotInvokeResolveResume` — AST identifier scan
  on `cold_start_handler.go` for `ResolveResume`/`SessionResumeRequest`/
  `ResumeModeIncremental`/`DefaultResumeLimit` 0 hit (反向 BPP-5).
- `TestBPP6_NoColdStartQueueInBPPPackage` — AST ident scan on all
  internal/bpp/*.go (production) for `pendingColdStart` / `coldStartQueue` /
  `deadLetterColdStart` / `plugin_restart_count` / `coldStartCount` /
  `restartCounter` 0 hit. **AL-1 best-effort 守护链延伸第 3 处** (BPP-4
  `TestBPP4_NoRetryQueueInBPPPackage` + BPP-5
  `TestBPP5_NoReconnectQueueInBPPPackage` + BPP-6 此).

Plus envelope-level: `TestBPPEnvelopeFrameWhitelist` count==15
(BPP-5 14 + BPP-6 cold_start_handshake +1) +
`TestBPPEnvelopeDirectionLock` data-plane 7→8.

## Cross-milestone byte-identical 守护链

- BPP-1 #304 envelope reflect lint (whitelist 14→15 自动覆盖)
- BPP-3 #489 PluginFrameDispatcher (handler 复用, 不开新 hub method)
- BPP-5 #503 reconnect 反向帧 (字段集互斥)
- AL-1 #492 single-gate `AppendAgentStateTransition` (any→online valid edge)
- REFACTOR-REASONS #496 6-dict (复用 `runtime_crashed`, AL-1a 守护链第 11 处)
- BPP-4 #499 + BPP-5 #503 best-effort AST 守护链延伸第 3 处
- ADM-0 §1.3 red-line (admin god-mode 不挂 cold_start)
