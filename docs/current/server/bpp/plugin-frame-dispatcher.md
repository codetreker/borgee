# Plugin Frame Dispatcher (BPP-3)

> **Source-of-truth pointer.** Implementation in
> `packages/server-go/internal/bpp/plugin_frame_dispatcher.go` +
> `packages/server-go/internal/api/agent_config_ack_handler.go`. Wire-up
> at server boot in `packages/server-go/internal/server/server.go`.

## Why

Before BPP-3, the `internal/ws/plugin.go` read loop only knew the
`{type, id, data}` RPC envelope (api_request / api_response /
ping / pong / response). Any other `type` was silently dropped. The
BPP-1 #304 envelope contract (10 frames, shape `{type, …payload-direct-
fields}`) had no ingress path on the server side, so AL-2b #481's
`agent_config_ack` ingress was deferred with an explicit comment
pointing here.

BPP-3 ships the unified plugin-upstream BPP frame dispatcher boundary so
every Plugin → Server frame has a single registration point.

## Boundary 原则

- **Two envelopes, two paths.** RPC envelope (`{type, id, data}`) stays
  in `plugin.go` (request-reply). BPP envelope (`{type, …payload-
  direct}`) routes through `bpp.PluginFrameDispatcher` (fire-and-forget
  event stream). Mixing them in one dispatcher would break the direction-
  lock invariant.
- **`internal/ws` does not import `internal/bpp`.** A `PluginFrameRouter`
  interface lives on the hub; the concrete `*bpp.PluginFrameDispatcher`
  is bridged via `pluginFrameRouterAdapter` at server boot. Same seam
  pattern as BPP-2.1 ActionHandler / cv-4.2 IterationStatePusher.
- **Direction lock at registration.** `Register` walks the
  `AllBPPEnvelopes()` reflection list; a `DirectionServerToPlugin` frame
  registered here panics. Defense-in-depth — registering a server →
  plugin frame on the plugin-upstream router is a definitional bug.
- **Forward-compat at routing.** Unknown `type` → log warn + soft skip
  (returns `(false, nil)`). Plugin upgrade may send frames the server
  doesn't yet understand on rolling rollouts; rejecting would break the
  rollout. Same for malformed JSON.
- **Each frame type has one dispatcher.** Duplicate `Register` panics.

## Registered frames (current)

| Frame type             | Direction       | Adapter             | Terminal handler                     |
|------------------------|-----------------|---------------------|--------------------------------------|
| `agent_config_ack`     | plugin → server | `AckFrameAdapter`   | `api.AgentConfigAckHandlerImpl`      |

`AgentConfigAckHandlerImpl` is the AL-2b deferred ack ingress sink. It
emits structured logs per status:

- `applied`  → `Info("bpp.agent_config_ack_applied", …)`  — audit trail
- `stale`    → `Warn("bpp.agent_config_ack_stale", …)`    — plugin will
  re-poll `GET /agents/:id/config` (蓝图 §1.5 "runtime 不缓存")
- `rejected` → `Warn("bpp.agent_config_ack_rejected", …)` — reason from
  AL-1a 6-dict (already validated upstream by `bpp.AckDispatcher`)

Cross-owner ACL is gated by `api.AgentOwnerResolver` (走 `store.GetAgent`
SSOT, byte-identical 跟 anchor #360 / DM-2 #372 / agent_config.go PATCH
owner gate). Mismatch → `errAckCrossOwnerReject`, frame is dropped, log
warn `bpp.plugin_frame_dispatch_failed`.

## Wire path (per frame)

```
plugin.go read loop
    ↓ (msg.Type ∉ {api_request, api_response, ping, pong, response})
hub.pluginFrameRouterSnapshot()
    ↓ (PluginSessionContext{OwnerUserID})
ws.PluginFrameRouter.Route(rawBytes)
    ↓ (pluginFrameRouterAdapter bridge)
bpp.PluginFrameDispatcher.Route
    ↓ (peek `type`, lookup registry)
FrameDispatcher.Dispatch(rawJSON, sess)
    ↓ (e.g. AckFrameAdapter decodes → AgentConfigAckFrame)
bpp.AckDispatcher.Dispatch(frame, AckSessionContext)
    ↓ (Status enum + Reason dict + cross-owner check)
api.AgentConfigAckHandlerImpl.HandleAck → log
```

## Tests

- `internal/bpp/plugin_frame_dispatcher_test.go` — 15 unit tests
  (Register direction-lock + envelope-whitelist + duplicate panics; Route
  happy / unknown-type soft-skip / malformed JSON soft-skip / dispatcher
  error propagation; AckFrameAdapter integration).
- `internal/api/agent_config_ack_handler_test.go` — 5 unit tests
  (HandleAck applied/rejected/stale logs; nil-logger no-op;
  AgentOwnerResolver real store GetAgent + missing).

Regression rows: `REG-BPP3-001..006` in
`docs/qa/regression-registry.md`.

## Adding a new Plugin → Server frame

1. Define the frame struct in `internal/bpp/envelope.go` with
   `FrameDirection() == DirectionPluginToServer`. Add it to
   `bppEnvelopeWhitelist`.
2. Implement a typed dispatcher (validation + handler interface seam),
   pattern after `agent_config_ack_dispatcher.go`.
3. Implement an adapter that satisfies `bpp.FrameDispatcher` (raw JSON
   → typed struct → typed dispatcher), pattern after `AckFrameAdapter`.
4. Wire at server boot in `internal/server/server.go`:
   ```go
   pfd.Register(bpp.FrameTypeBPPNewFrame, bpp.NewNewFrameAdapter(...))
   ```
5. Register tests + regression rows.
