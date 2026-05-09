# BPP-5 — plugin reconnect handshake + cursor resume 协议化

> **单一来源 pointer.** Implementation in
> `packages/server-go/internal/bpp/reconnect_handler.go` +
> `packages/server-go/internal/bpp/envelope.go` (ReconnectHandshakeFrame
> + FrameTypeBPPReconnectHandshake). Wire-up at server boot in
> `packages/server-go/internal/server/server.go`.

## Why

BPP-4 #499 watchdog flips a stale plugin's agent state to
`error/network_unreachable` after 30s of missed heartbeats. The plugin
is expected to reconnect. Before BPP-5, reconnect went through the
generic `connect` handshake — but `connect` is for first-time
identity + capability negotiation, not for cursor recovery. Plugins
that reconnected lost their place in the shared event sequence and had
to do a full replay (or implement ad-hoc cursor tracking).

BPP-5 ships a dedicated `reconnect_handshake` frame that carries
`last_known_cursor` and reuses RT-1.3 #296 cursor replay as the
recovery path.

## 原则 (蓝图 §1.6 + §2.1 + RT-1.3 原则一致)

- **reconnect_handshake = BPP envelope 第 14 frame.** direction lock
  plugin→server (server 永不发). 不另开 channel. `ConnectFrame` and
  `ReconnectHandshakeFrame` 字段集不交 — `Connect` carries Token +
  Capabilities (首次身份), `Reconnect` carries `last_known_cursor` +
  `disconnect_at` + `reconnect_at` (cursor 恢复).
- **cursor resume 复用 RT-1.3 既有 mechanism.** Handler calls
  `bpp.ResolveResume(EventLister, SessionResumeRequest{Mode:
  ResumeModeIncremental, Since: frame.LastKnownCursor}, channelIDs,
  DefaultResumeLimit)`. Server-side has only ONE cursor sequence —
  shared across RT-1 / CV-2 / DM-2 / CV-4 / AL-2b / RT-3 / BPP-3.1 /
  BPP-5. AST scan reverse-grep守门 forbids `bpp5.*sequence` /
  `reconnect.*cursor.*= 0` / `new.*resume.*dict`.
- **AL-1 5-state error → online via reverse valid edge.** Handler
  calls `agent.Tracker.Clear(agentID)`. Combined with
  `hub.GetPlugin(agentID) != nil` (set by BPP-1 connect prior to this
  reconnect frame), the next `ResolveAgentState` returns `online`.
  No persisted "connecting" intermediate state — that name in the spec
  is conceptual only. The 5-state graph (#492) has the direct
  `error → online` valid edge byte-identical.
- **AL-1a reason 6-dict 不扩第 7.** `connecting` is reason-less
  (transient). BPP-5 = the **9th lock chain link**: AL-1a #249 → AL-3
  #305 → CV-4 #380 → AL-2a #454 → AL-1b #458 → AL-4 #387/#461 →
  BPP-2.2 #485 → AL-2b #481 → BPP-4 #499 → **BPP-5**.
- **Best-effort, no retry queue.** Handler returns success/failure
  exactly once; no persistent reconnect state, no pending-acks queue,
  no backoff timer. AST reverse-grep forbids
  `pendingReconnects` / `reconnectQueue` / `deadLetterReconnect` —
  守护链延伸 from BPP-4 dead_letter_test.

## Frame schema (envelope.go)

```go
type ReconnectHandshakeFrame struct {
    Type            string `json:"type"`             // "reconnect_handshake"
    PluginID        string `json:"plugin_id"`
    AgentID         string `json:"agent_id"`
    LastKnownCursor int64  `json:"last_known_cursor"`
    DisconnectAt    int64  `json:"disconnect_at"`    // Unix ms
    ReconnectAt     int64  `json:"reconnect_at"`     // Unix ms
}
```

`bppEnvelopeWhitelist` count: 13 → 14. BPP-1 #304 reflection lint
auto-covers — adding the frame struct without registering it (or vice
versa) is a CI red.

## Wire path

```
plugin sends ReconnectHandshakeFrame (BPP-3 PluginFrameDispatcher boundary)
    ↓ FrameTypeBPPReconnectHandshake match
ReconnectHandler.Dispatch(rawJSON, sess)
    ↓ json.Unmarshal → ReconnectHandshakeFrame
    ↓ owner.OwnerOf(agent_id) → cross-owner check
    ↓ events.GetLatestCursor() → cursor regression check (trust-but-log)
    ↓ scope.ChannelIDsForOwner(sess.OwnerUserID)
bpp.ResolveResume(events, SessionResumeRequest{Mode: incremental,
                                                Since: LastKnownCursor},
                   channelIDs, DefaultResumeLimit)
    ↓ (replay events not pushed back here; live frames continue via
       hub broadcast; client is responsible for pulling missed events
       via GET /api/v1/events?since=… per RT-1.2 backfill if needed)
agent.Tracker.Clear(agentID)
    ↓ hub.GetPlugin(agentID) != nil + tracker.errors[agentID] cleared
    → ResolveAgentState returns online
log.Info("bpp.reconnect_handshake_resolved", …)
```

## Constants & error codes

| Name                                  | Value                            | Source                                    |
|---------------------------------------|----------------------------------|-------------------------------------------|
| `FrameTypeBPPReconnectHandshake`      | `"reconnect_handshake"`          | `envelope.go`                              |
| Cursor regression log key             | `"bpp.reconnect_cursor_regression"` | `reconnect_handler.go`                  |
| Cross-owner log + error code          | `"bpp.reconnect_cross_owner_reject"` | `reconnect_handler.go::ReconnectErrCodeCrossOwnerReject` |
| Success log key                       | `"bpp.reconnect_handshake_resolved"` | `reconnect_handler.go`                |

## Tests

- `internal/bpp/reconnect_handler_test.go` — 9 unit tests:
  - §1 frame schema (3): field order + direction lock + ConnectFrame
    field set 不交.
  - §2 server handler (5): ResolveResume call args / Clear on success /
    cross-owner reject + log + no-clear / cursor regression
    trust-but-log / panic on nil deps.
  - §4 反向约束 (1): AST scan reconnect-queue identifiers 0 hit
    (BPP-4 dead_letter_test 守护链延伸).

Regression rows: `REG-BPP5-001..009` in
`docs/qa/regression-registry.md`.

## Adding a new BPP-5 后续 frame field

1. Add field to `ReconnectHandshakeFrame` struct in `envelope.go`.
2. Update `TestBPP5_ReconnectHandshakeFrame_FieldOrder` to include the
   new (name, json-tag) entry.
3. Update `TestBPP5_ConnectFrame_NoReconnectFields` if the new field
   could plausibly appear on `ConnectFrame` too — the 字段集不交反向断言
   guards 脱节.
4. Plugin SDK schema must add the field in the SAME PR (跟 BPP-1
   envelope CI lint 同模式).
