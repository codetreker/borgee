# Plugin Transports

## Role

Transports are how the OpenClaw plugin receives Borgee events and keeps a cursor high-water mark. SSE and poll are the current reliable event paths; WS exists in code as a plugin socket path with caveats listed in `../known-gaps.md`.

## Boundary

| Transport | Role | Collaborators | Out Of Scope |
| --- | --- | --- | --- |
| SSE | Preferred streaming event path | server event stream | Server cursor generation |
| Poll | Long-poll fallback or forced mode | server poll endpoint | Browser reconnect policy |
| Plugin WS | Optional RPC/request path and code-present event path | server plugin socket | Guaranteed event broadcast |
| Cursor store | Local resume hint | transport loops | Durable source of truth |

## Internal Architecture

```mermaid
stateDiagram-v2
  [*] --> startup
  startup --> poll: forced poll
  startup --> ws: code path ws
  startup --> probe: auto or forced sse
  probe --> sse: stream available
  probe --> poll: auto fallback
  probe --> stop: auth failure
  sse --> probe: non-auth disconnect
  poll --> probe: recovery probe
```

## Key Flows

### SSE

The plugin opens the server event stream with bearer auth and an optional last event id. Incoming frames are parsed into event name, id, and data. Any bytes count as liveness; heartbeat frames do not become inbound messages. Non-auth disconnects reconnect with backoff.

### Poll

Poll sends the current cursor and timeout to the server. If events arrive, the plugin advances and persists the cursor, then dispatches supported event kinds through the same inbound path as SSE. Consecutive errors back off.

### Auto Recovery

Auto mode probes SSE first. If unavailable, it falls back to poll and periodically probes SSE again. A successful recovery probe aborts the poll session and returns to SSE.

### WS Code Path

The WS branch connects to the plugin socket, can send RPC requests, and can answer server `request` frames for local file reads. It also has an event-frame handler, but current server behavior makes SSE/poll the event path to rely on.

## Invariants

- Cursor persistence is best-effort and local to the plugin process.
- Event filtering happens before OpenClaw inbound dispatch.
- Auth failures stop the transport loop instead of silently switching accounts.
- `ws` is not exposed by the current config schema even though the code branch exists.

## Implementation Anchors

- Gateway selection: `packages/plugins/openclaw/src/gateway.ts`, `startBorgeeGateway`
- SSE client: `packages/plugins/openclaw/src/sse-client.ts`, `connectSSE`, `runSSELoop`
- Poll client: `packages/plugins/openclaw/src/api-client.ts`, `pollBorgeeEvents`
- WS client: `packages/plugins/openclaw/src/ws-client.ts`, `PluginWsClient`
- Cursor store: `packages/plugins/openclaw/src/cursor-store.ts`
- Transport config: `packages/plugins/openclaw/src/config-schema.ts`, `packages/plugins/openclaw/src/types.ts`
