# Known Gaps

These are current architecture-relevant mismatches. Keep the format fixed so readers can distinguish behavior from assumptions.

## Architecture Debt Index

This top-level page is the global debt map, not the only detail list. Module-local docs own narrower gaps and constraints.

| Area | Where to continue | Scope |
| --- | --- | --- |
| Global realtime, BPP, plugin, and remote mismatches | This page | Cross-module assumptions that can affect more than one owner |
| Admin privacy and server rail | [admin privacy/audit](admin/privacy-audit.md), [admin server rail](admin/server-rail.md) | Admin-only privacy, audit, and authorization constraints |
| Remote-agent protocol and filesystem boundary | [remote-agent protocol](remote-agent/protocol.md), [remote filesystem boundary](remote-agent/filesystem-boundary.md) | Remote request shape, filesystem scope, and user-machine IO assumptions |
| Host bridge helper, installer, and grants | [helper daemon](host-bridge/helper-daemon.md), [installer](host-bridge/installer.md), [host grants](host-bridge/host-grants.md) | Local daemon deployment, IPC, and grants-backed access |
| Validation coverage | [E2E / verification](e2e/) | Harness coverage and release validation limits |

## Plugin WS Event Delivery

Current behavior: OpenClaw has a WS event handler, but server plugin WS currently centers on RPC frames and BPP ingress.

Architecture impact: SSE and poll are the reliable plugin event paths.

Do not assume: `/ws/plugin` is a general event broadcast stream for OpenClaw.

Relevant area: plugin transports, server realtime.

## Plugin WS Transport Selection

Current behavior: OpenClaw TypeScript transport types include `ws`, while validated config exposes `auto`, `sse`, and `poll`.

Architecture impact: WS is code-present but not a normal configured event transport.

Do not assume: users can enable WS event transport through current schema.

Relevant area: plugin transports and config.

## BPP SDK Connect Shape

Current behavior: The Go SDK sends a BPP `connect` frame after websocket dial; server plugin WS authenticates at upgrade and does not register `connect` as plugin-upstream.

Architecture impact: SDK connect semantics and server plugin socket handshake are not the same boundary.

Do not assume: SDK `connect` is processed as the server's plugin authentication handshake.

Relevant area: BPP SDK and server plugin socket.

## BPP Heartbeat Semantics

Current behavior: Heartbeat is modeled, but server liveness derives from plugin socket activity timestamps.

Architecture impact: Any inbound plugin frame can refresh liveness.

Do not assume: heartbeat frames are the only liveness source.

Relevant area: BPP internals and realtime Hub.

## Remote-Agent Request Shape

Current behavior: Server remote proxy wraps path data under `params`; the Node remote-agent reads `path` at the top level.

Architecture impact: Remote file proxy behavior has a request-shape mismatch.

Do not assume: remote `ls/read/stat` works end to end without checking the payload shape.

Relevant area: remote-agent protocol.

## Event Stream Split

Current behavior: Poll/SSE/backfill use the hot realtime cursor stream; the data-layer event bus is a separate audit/retention-oriented path.

Architecture impact: There are two event concepts with different consumers.

Do not assume: data-layer events are the same cursor stream used for realtime recovery.

Relevant area: server realtime and data layer.

## Offline Plugin Frames

Current behavior: Server-to-plugin frames target a live plugin connection and are dropped or logged when offline.

Architecture impact: Recovery depends on pull/resume paths, not a durable server delivery queue.

Do not assume: server queues plugin frames for later delivery.

Relevant area: BPP internals and plugin lifecycle.

## Implementation Anchors

- Plugin transport/config: `packages/plugins/openclaw/src/ws-client.ts`, `packages/plugins/openclaw/src/gateway.ts`, `packages/plugins/openclaw/src/config-schema.ts`, `packages/plugins/openclaw/src/types.ts`
- Server plugin/BPP: `packages/server-go/internal/ws/plugin.go`, `packages/server-go/internal/bpp`
- BPP SDK: `packages/server-go/sdk/bpp`
- Remote-agent boundary: `packages/server-go/internal/ws/remote.go`, `packages/remote-agent/src/agent.ts`
- Event streams: `packages/server-go/internal/api/poll.go`, `packages/server-go/internal/datalayer`
