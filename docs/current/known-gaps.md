# Known Gaps

These are current architecture-relevant mismatches. Keep the format fixed so readers can distinguish behavior from assumptions.

## Architecture Debt Index

This top-level page is the global debt map, not the only detail list. Module-local docs own narrower gaps and constraints.

| Area | Where to continue | Scope |
| --- | --- | --- |
| Global realtime, BPP, plugin, and remote mismatches | This page | Cross-module assumptions that can affect more than one owner |
| Admin privacy and server rail | [admin privacy/audit](admin/privacy-audit.md), [admin server rail](admin/server-rail.md) | Admin-only privacy, audit, and authorization constraints |
| Remote-agent protocol and filesystem boundary | [remote-agent protocol](remote-agent/protocol.md), [remote filesystem boundary](remote-agent/filesystem-boundary.md) | Remote request shape, filesystem scope, and user-machine IO assumptions |
| Remote node v1 limits | [remote-agent protocol](remote-agent/protocol.md), [remote filesystem boundary](remote-agent/filesystem-boundary.md) | Binding ACL, symlink containment, self-update, read-only scope |
| Validation coverage | [E2E / verification](e2e/) | Harness coverage and release validation limits |

## Plugin WS Event Delivery

Current behavior: OpenClaw has a WS event handler, but server plugin WS currently centers on RPC frames and BPP ingress.

Architecture impact: SSE and poll are the reliable plugin event paths.

Do not assume: `/ws/plugin` is a general event broadcast stream for OpenClaw.

Relevant area: plugin transports, server realtime.

## Channel Management Mutations Not Implemented

Current behavior: the user Settings sidepane has a channel-management tab that groups non-DM channels into channels created by the current user and channels joined by the current user. The tab renders read-only leave/delete/archive/owner-transfer availability from the authorized channel list already present in client app state. Self-created or owned channels do not show leave as available, joined-only non-general channels can show leave as available, delete/archive require the matching permission state as well as channel ownership, and owner transfer is unavailable for v1.

Architecture impact: users can inspect ownership, membership grouping, and action availability. Server-side user-rail channel mutations enforce the current authority boundary: creators cannot leave their own channel, non-members cannot leave or manage a channel, delete/archive require channel creator authority, member management cannot remove the channel creator, and cross-org management attempts fail closed.

Do not assume: Settings channel management can leave, delete, archive, transfer ownership, change membership, change notification preferences, collapse/sort/pin/group channels, or execute any mutation just because the server routes now enforce authority for existing mutation surfaces.

Relevant area: [client feature surfaces](client/feature-surfaces.md), [settings UI sketch](client/ui/settings.md).

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

## Channel RequireMention Client Controls

Current behavior: The server stores and enforces per-channel agent `requireMention` policy, including manager-only updates and the agent-owner ceiling for non-mention delivery. The server also handles `@Everyone` as a reserved broadcast token: recipients are computed from channel membership, client-supplied recipient ids are rejected, agent-originated broadcasts are rejected, and repeated broadcasts are rate-limited per sender/channel. Channel member payloads expose the stored policy for later clients.

Architecture impact: The current browser client does not yet provide a dedicated control or explanatory surface for channel managers to inspect and change this policy, nor a dedicated client mention-control surface explaining `@Everyone` behavior.

Do not assume: users can manage per-channel agent attention from the client UI just because the server API and message-routing behavior exist. Do not assume `@Everyone` has a dedicated client-side affordance before the client mention-control task lands.

Relevant area: channel management, client mention controls, and agent attention UX.

## Remote Node Binding Channel ACL Not Enforced

Current behavior: creating a channel-to-path binding is scoped to the remote node's owner, but the server does not verify that the caller is a member of the supplied `channel_id`. A node owner can bind their node's path to a channel without a channel-membership check.

Architecture impact: remote bindings are not a hardened multi-user authorization boundary; node ownership is the only enforced scope at binding creation.

Do not assume: a remote binding implies the binder has access to the bound channel, or that v1 remote node sharing is safe for multi-user channels.

Relevant area: [remote-agent protocol](remote-agent/protocol.md), server remote bindings.

## Remote Agent Symlink Containment Absent

Current behavior: the daemon's path gate compares the resolved request path against the startup allowlist but does not realpath-resolve symlinks. A symlink inside an allowed directory that points outside it is not separately contained.

Architecture impact: directory containment is path-prefix based, not filesystem-canonical; symlink escape is not defended at the daemon boundary.

Do not assume: an allowed directory fully sandboxes reads to files physically under it.

Relevant area: [remote filesystem boundary](remote-agent/filesystem-boundary.md), daemon path gate.

## Implementation Anchors

- Plugin transport/config: `packages/plugins/openclaw/src/ws-client.ts`, `packages/plugins/openclaw/src/gateway.ts`, `packages/plugins/openclaw/src/config-schema.ts`, `packages/plugins/openclaw/src/types.ts`
- Server plugin/BPP: `packages/server-go/internal/ws/plugin.go`, `packages/server-go/internal/bpp`
- BPP SDK: `packages/server-go/sdk/bpp`
- Remote-agent boundary: `packages/server-go/internal/ws/remote.go`, `packages/borgee/internal/remotews`
- Event streams: `packages/server-go/internal/api/poll.go`, `packages/server-go/internal/datalayer`
