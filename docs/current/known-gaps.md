# Known Gaps

Known gaps are current implementation boundaries or mismatches that matter architecturally. They are not feature requests.

## Plugin WS Event Delivery

The OpenClaw WS client has a branch for server event frames, but the server plugin socket currently focuses on RPC frames and BPP upstream dispatch. SSE and poll are the reliable plugin event-consumption paths.

## Plugin Transport Schema

The OpenClaw TypeScript transport type and gateway include `ws`, while the config schema exposes `auto`, `sse`, and `poll`. Treat WS as code-present but not normally selectable through validated config.

## BPP SDK Connect Shape

The Go SDK sends a BPP `connect` frame after websocket dial. The server plugin socket authenticates at websocket upgrade and does not register `connect` as a plugin-upstream frame.

## BPP Heartbeat Semantics

The heartbeat frame is modeled, and the SDK can send it. Server liveness currently derives from any inbound plugin socket frame updating connection activity, then a watchdog interprets staleness.

## Remote-Agent Request Shape

The server remote proxy wraps path data under `params`, while the Node remote-agent request handler reads `path` at the top level. That boundary needs care when documenting remote file proxy behavior.

## Event Streams

The hot realtime cursor stream used by poll/SSE/backfill is separate from the data-layer event bus used by audit/retention-oriented paths.

## Offline Plugin Frames

Server-to-plugin frames are point-to-point to a live plugin connection. Offline plugin frames are dropped or logged, not persisted into a server-side delivery queue.

## Implementation Anchors

- Plugin WS and transports: `packages/plugins/openclaw/src/ws-client.ts`, `packages/plugins/openclaw/src/gateway.ts`, `packages/plugins/openclaw/src/config-schema.ts`, `packages/plugins/openclaw/src/types.ts`
- Server plugin socket and BPP dispatch: `packages/server-go/internal/ws/plugin.go`, `packages/server-go/internal/bpp/plugin_frame_dispatcher.go`, `packages/server-go/internal/bpp/envelope.go`
- BPP SDK: `packages/server-go/sdk/bpp`
- Remote-agent boundary: `packages/server-go/internal/server/server.go`, `packages/server-go/internal/ws/remote.go`, `packages/remote-agent/src/agent.ts`
- Event streams: `packages/server-go/internal/api/poll.go`, `packages/server-go/internal/datalayer`
