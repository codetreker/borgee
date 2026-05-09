# BPP-4 — heartbeat watchdog + dead-letter audit log

> **Source-of-truth pointer.** Implementation in
> `packages/server-go/internal/bpp/heartbeat_watchdog.go` +
> `packages/server-go/internal/bpp/dead_letter.go`. Wire-up at server
> boot in `packages/server-go/internal/server/server.go` +
> `packages/server-go/internal/ws/al_2b_2_agent_config_push.go`.

## Why

- **Watchdog**: Plugin liveness was previously only tracked by an
  `alive bool` flag flipped by ping/pong; there was no "missing
  heartbeat → flip agent to error" path. BPP-4.1 adds the missing
  watchdog so a killed plugin reflects as `error/network_unreachable`
  in the agent UI within 30s, matching blueprint `plugin-protocol.md`
  §1.6 "缺心跳按未知" + module BPP-4 acceptance "kill plugin → 30s
  内 agent 显示 error".
- **Dead-letter**: `server → plugin` push (RT-1/CV-2/DM-2/CV-4/AL-2b
  shared sequence) used to silently drop on plugin offline. BPP-4.2
  adds an audit log entry per drop so operators can see what was
  missed; reconnect path is unchanged (RT-1.3 cursor replay).

## 设计 (蓝图 §1.6 + §1.5 字面)

- **Borgee 不取消 in-flight 任务.** Watchdog flips agent state only;
  it does NOT send `cancel` / `abort` / `kill` frames (grep 检查
  `cancel.*task|abort.*inflight|server.*kill.*runtime` 0 hit守门).
- **30s threshold is single-source.** `BPP_HEARTBEAT_TIMEOUT_SECONDS = 30`
  in `heartbeat_watchdog.go` is the only declaration; CI grep prevents
  drift. Changing it requires touching 3 places (constant +
  `bpp-4-spec.md` §0.2 + `bpp-4-content-lock.md` §1.①).
- **AL-1a 6-dict reason 不扩.** Watchdog uses
  `agent.ReasonNetworkUnreachable` directly; BPP-4 is the **9th lock
  link** in the AL-1a reason chain (AL-1a #249 + AL-3 #305 + CV-4
  #380 + AL-2a #454 + AL-1b #458 + AL-4 #387/#461 + BPP-2.2 #485 +
  AL-2b #481 + **BPP-4**). The blueprint UI label `runtime_disconnected`
  ("重连中…") is client-side; server-side stays on
  `network_unreachable`.
- **Best-effort, no retry queue.** Dead-letter is `log warn + audit`,
  never a persistent queue. AST reverse-grep守门 forbids
  `pendingAcks` / `retryQueue` / `deadLetterQueue` / `ackTimeout`
  identifiers in `internal/bpp/` source. RT-1.3 #296 cursor replay is
  the recovery path.

## Wire path

### Watchdog (every 10s)

```
goroutine watchdog.Run(ctx)
    ↓ ticker fires every BPP_HEARTBEAT_TICKER_INTERVAL (10s)
HeartbeatWatchdog.scanOnce()
    ↓ source.SnapshotLastSeen() → map[agent_id]lastSeenAt
hub.SnapshotPluginLastSeen   (via hubLivenessAdapter bridge)
    ↓ for each agent: now - lastSeenAt > 30s ?
        yes → sink.SetError(agentID, ReasonNetworkUnreachable)
              + log.Warn("bpp.heartbeat_timeout", …)
              + markedErr[agentID] = true   (防重复)
        no  → markedErr[agentID] = false   (reconnect cleared)
agent.Tracker.SetError → snapshot map → /api/v1/agents reads via withState
```

### Dead-letter (per failed push)

```
hub.PushAgentConfigUpdate(agentID, …)
    ↓ pc := hub.GetPlugin(agentID)
    ↓ pc == nil (plugin offline)
bpp.LogFrameDroppedPluginOffline(logger, DeadLetterAuditEntry{
    Actor:  "server",
    Action: "frame_drop",
    Target: agentID,
    When:   createdAt,
    Scope:  "agent_config_update:cursor=<cur>",
})
    ↓ logger.Warn("bpp.frame_dropped_plugin_offline", …)
return cur, false   (caller decides; not blocking)
```

## Constants (single source)

| Name                              | Value | Source                                              |
|-----------------------------------|-------|-----------------------------------------------------|
| `BPP_HEARTBEAT_TIMEOUT_SECONDS`   | `30`  | `heartbeat_watchdog.go` (蓝图 BPP-4 acceptance)     |
| `BPP_HEARTBEAT_TICKER_INTERVAL`   | `10s` | `heartbeat_watchdog.go` (≤ 阈值/3 防错过窗口)       |
| Reason on timeout                 | `network_unreachable` | `agent.ReasonNetworkUnreachable` (AL-1a 6-dict 第 9 处) |
| Watchdog log key                  | `bpp.heartbeat_timeout`     | `heartbeat_watchdog.go::scanOnce`     |
| Dead-letter log key               | `bpp.frame_dropped_plugin_offline` | `dead_letter.go::LogFrameDroppedPluginOffline` |
| Audit schema fields               | `actor / action / target / when / scope` | byte-identical with HB-1 + HB-2 audit (三处同源) |

## Tests

- `internal/bpp/heartbeat_watchdog_test.go` — 8 unit tests (threshold
  constant lock; trigger on 31s+ timeout; not spammy on repeated scan;
  reconnect clears marked; multi-plugin isolated; log key on timeout;
  panic on nil source/sink).
- `internal/bpp/dead_letter_test.go` — 4 unit tests (log key
  byte-identical; nil-logger no-op; 5-field schema reflect lock; AST
  reverse-grep `pendingAcks/retryQueue/deadLetterQueue/ackTimeout` 0
  hit in `internal/bpp/` non-test sources).

Regression rows: `REG-BPP4-001..009` in
`docs/qa/regression-registry.md`.

## Adding a new dead-letter call site

1. Identify the `server → plugin` push function (e.g. a new BPP frame
   pusher on `*ws.Hub`).
2. On the `pc == nil` (plugin offline) branch, call
   `bpp.LogFrameDroppedPluginOffline(h.logger, bpp.DeadLetterAuditEntry{
   Actor: "server", Action: "frame_drop", Target: agentID, When:
   <unix_ms>, Scope: "<frame_type>:cursor=<cursor>"})`.
3. Do NOT add a retry queue or timer — the AST reverse-grep test will
   fail.
