# Known Gaps

This page records code-confirmed gaps, mismatches, and explicit non-goals. It prevents maintainers from documenting missing behavior as current behavior.

## OpenClaw WS Transport Is Code-Present But Schema-Inaccessible

Responsible for: documenting the current plugin transport mismatch. Not responsible for: changing the config schema.

`BorgeeTransport` includes `"ws"`, and `startBorgeeGateway` has a `transport === "ws"` branch that constructs `PluginWsClient`. The config schema only accepts `auto`, `sse`, and `poll`, so normal config validation cannot select `ws`. Evidence: `packages/plugins/openclaw/src/types.ts`, `packages/plugins/openclaw/src/config-schema.ts`, `packages/plugins/openclaw/src/gateway.ts`, `packages/plugins/openclaw/src/ws-client.ts`.

## `/ws/plugin` Does Not Currently Broadcast Event Frames To OpenClaw

Responsible for: documenting the server/plugin contract as implemented. Not responsible for: replacing SSE/poll fallback.

The OpenClaw WS client handles server frames shaped as `{type:"event", event, data}`. The server `/ws/plugin` handler currently handles RPC frames (`api_request`, `api_response`, `response`, `ping`, `pong`) and routes non-RPC BPP envelopes to `PluginFrameDispatcher`; the inspected server path does not show a plugin event fanout that emits `{type:"event"}` frames. Current reliable plugin event consumption is therefore SSE or poll. Evidence: `packages/plugins/openclaw/src/ws-client.ts`, `packages/server-go/internal/ws/plugin.go`, `packages/server-go/internal/bpp/plugin_frame_dispatcher.go`.

## SDK `ConnectFrame` Is Not A Server `/ws/plugin` Handshake

Responsible for: documenting the in-tree SDK/server boundary mismatch. Not responsible for: changing SDK API.

The Go SDK dials a websocket and sends a BPP `ConnectFrame`. server-go authenticates `/ws/plugin` at WebSocket upgrade using bearer/API key, then routes unknown non-RPC frame types through `PluginFrameDispatcher`. `connect` is modeled as server-to-plugin in the BPP envelope and is not registered as plugin-upstream. Evidence: `packages/server-go/sdk/bpp/client.go`, `packages/server-go/internal/ws/plugin.go`, `packages/server-go/internal/bpp/envelope.go`, `packages/server-go/internal/bpp/plugin_frame_dispatcher.go`.

## BPP Heartbeat Frame Exists, Watchdog Uses Inbound Activity

Responsible for: distinguishing modeled heartbeat envelope from active server liveness behavior. Not responsible for: browser `/ws` heartbeat.

`heartbeat` is a plugin-to-server BPP envelope, and the SDK can send it. server-go does not register `heartbeat` in `PluginFrameDispatcher`; instead every inbound `/ws/plugin` frame updates `PluginConn.lastSeenAt`, and the watchdog scans that timestamp map every 10 seconds with a 30 second stale threshold. Evidence: `packages/server-go/internal/bpp/envelope.go`, `packages/server-go/sdk/bpp/client.go`, `packages/server-go/internal/ws/plugin.go`, `packages/server-go/internal/server/server.go`, `packages/server-go/internal/bpp/heartbeat_watchdog.go`.

## Remote Proxy Payload Shape Mismatch

Responsible for: documenting the current remote-agent request shape risk. Not responsible for: helper daemon file IO.

server-go's `hubRemoteAdapter.ProxyRequest` sends `{action, params:{path}}` inside the remote request data. `remote-agent` reads `data.path` directly when handling `ls`, `read`, and `stat`. With these two code paths alone, the agent does not receive the requested path at `data.path`. Evidence: `packages/server-go/internal/server/server.go`, `packages/remote-agent/src/agent.ts`, `packages/server-go/internal/ws/remote.go`.

## Hot Event Cursor And Data-Layer EventBus Are Separate Streams

Responsible for: preventing cursor model confusion. Not responsible for: admin audit retention details.

`/api/v1/poll`, `/api/v1/stream`, and `/api/v1/events` use `store.Event.Cursor` and `GetEventsSinceWithChanges`. The data-layer EventBus persists to `channel_events` or `global_events` and supports audit/retention paths; it is not the same hot cursor stream used by poll/SSE/backfill. Evidence: `packages/server-go/internal/api/poll.go`, `packages/server-go/internal/store/models.go`, `packages/server-go/internal/store/queries_phase3.go`, `packages/server-go/internal/datalayer/eventbus.go`, `packages/server-go/internal/datalayer/events_store.go`.

## Offline Plugin Frames Are Dropped, Not Queued

Responsible for: documenting current BPP delivery semantics. Not responsible for: inventing retry queues.

Server-to-plugin frames such as `agent_config_update` and `permission_denied` are point-to-point writes to the current `PluginConn`. If the plugin is offline, `agent_config_update` logs a dead-letter-style audit event and returns `sent=false`; it does not persist a retry queue. Reconnect/cold-start paths rely on plugin pull/resume behavior rather than server-side frame queues. Evidence: `packages/server-go/internal/ws/agent_config_push.go`, `packages/server-go/internal/ws/permission_denied_frame.go`, `packages/server-go/internal/bpp/dead_letter.go`, `packages/server-go/internal/bpp/reconnect_handler.go`, `packages/server-go/internal/bpp/cold_start_handler.go`.
