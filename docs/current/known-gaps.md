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

## Validation Coverage Boundaries

Current behavior: default PR CI covers server-side helper IPC primitive and host-grants/manifest anchors, plus static Helper service/plist/sandbox asset checks and daemon outbound prerequisite validation tests. It still does not run the real helper daemon runtime or sandbox integration by default. The docs sync guard is mapped-module only and currently misses the current helper, installer, and host-bridge docs paths. The installer gate is path-scoped or manual.

Architecture impact: validation signals are reliable inside their stated boundaries, but they are not proof of full host-bridge runtime coverage.

Do not assume: E2E or default CI has run the privileged helper daemon/sandbox path.

Relevant area: [E2E / verification](e2e/), host bridge.

## Helper Pull Loop Not Implemented

Current behavior: Helper job enqueue exists server-side, and the installed Helper service now has outbound HTTPS address-family/sandbox prerequisites, exact public-origin startup validation that classifies literal host/IP input with `netip` and rejects localhost/private/link-local/metadata literal origins by default, and explicit Helper-owned queue/status/audit-handoff state roots. A pure Helper local job-policy evaluator exists for delivered server-owned job views: it validates strict typed payload schemas, local owner/org/enrollment/device/category/revocation/stale/expiry state, signed runtime manifest digests, artifact cache digests, declared path/domain/service bindings, and supplied sandbox/profile affordances. Production assets use the exact `https://app.borgee.io` allowlist. The validator does not resolve allowed hostnames or guard against DNS answers/CNAMEs resolving to private, link-local, or metadata addresses. The Helper daemon still does not poll, long-poll, lease jobs, upload results or acks, upload bounded logs, wire local policy to transport/action execution, run OpenClaw actions, restart services, or use sudo cache.

Architecture impact: the service/sandbox/config boundary and pure local policy boundary are ready for later Helper-originated pull/action work, but job progress and Configure OpenClaw success must not be inferred from these prerequisites.

Do not assume: a queued Helper job will be pulled or executed by a local Helper.

Do not assume: local policy decisions are uploaded, settled, or visible in job progress yet.

Do not assume: Helper startup validation prevents DNS rebinding or private/link-local/metadata DNS resolution for an otherwise allowed hostname; that remains future hardening or runtime network-policy scope.

Relevant area: [host bridge helper daemon](host-bridge/helper-daemon.md), server Helper jobs.

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

## Cold Event Retention Defaults Not Applied

Current behavior: The server starts the cold event retention job, but the sweeper only deletes `channel_events` and `global_events` rows that have an explicit non-negative `retention_days`. The ordinary cold event writer does not set `retention_days` on insert.

Architecture impact: Per-kind default retention policy exists as policy code, but it is not currently applied to regular cold event rows written without row-level retention metadata.

Do not assume: channel or global cold events age out by default just because their kind has a configured default retention window.

Relevant area: server data layer and data lifecycle.

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
