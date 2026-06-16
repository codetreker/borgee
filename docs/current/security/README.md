# Security And Privacy Boundaries

Security in the current architecture is rail-oriented. Each rail has its own credential, authorization source, runtime boundary, and privacy surface. The important design rule is that authority does not automatically cross rails: a user cookie is not an admin session, and a remote node token is not a user session.

## Overview

**Role**
This document defines the cross-module security boundaries that keep user API, admin API, plugin connection, and Remote Agent responsibilities separate.

**Boundary**
The boundary is the rail. Requests are first classified by rail, then authenticated and authorized by that rail's own mechanism. Shared storage does not imply shared authority.

**Collaborators**
The security model spans user auth, admin sessions, capability checks, plugin WebSocket auth, remote node tokens, and audit surfaces.

**Internal Architecture**

- Identity rails: user, admin, plugin agent, remote node.
- Authorization sources: user permissions, admin sessions, API keys, and remote node tokens.
- Enforcement points: HTTP middleware and WebSocket handshake.
- Privacy surfaces: serializers, metadata-only admin views, and user-scoped audit.

**Key Flows**

```text
user API:      user credential -> user context -> owner/capability/resource checks
admin API:     admin session cookie -> admin context -> admin rail only
plugin WS:     API key -> agent/plugin connection -> scoped API bridge
remote WS:     remote node token -> remote connection -> ls/read/stat tunnel
channel attention: channel manager credential -> channel/org/member checks -> agent owner require-mention ceiling
everyone fanout: message sender credential -> channel membership -> server-computed @Everyone recipients
```

**Invariants**

- Admin authority lives on the admin rail, not in `users.role`.
- User API authority lives in user identity plus owner/capability checks.
- Remote node tokens authenticate the remote WebSocket connection only; a remote token is not a user session.
- Privacy-sensitive raw data is either omitted from serializers or exposed only to the owner/admin rail designed for that data.
- Audit is layered: durable server audit for admin actions, and best-effort notifications where appropriate.

## Cross-Rail Matrix

| Rail | Credential | Authorization Source | Runtime Boundary | Privacy Surface | Key Invariant |
| --- | --- | --- | --- | --- | --- |
| User API | user cookie, Bearer API key, development bypass in development | user row plus owner/capability checks | HTTP middleware and handlers | user serializers hide internal columns | user credential does not enter admin rail |
| Channel attention policy | user cookie or Bearer API key on the user rail | channel manager permission plus channel org/member validation and the target agent's global `require_mention` setting | `GET /api/v1/channels/{channelId}/members`, `PUT /api/v1/channels/{channelId}/members/{userId}/require-mention`, and message-create dispatch | policy state is membership metadata; implicit delivery does not write mention history | channel managers can force or inherit mention-required behavior, but cannot set `off` unless the agent owner already allowed broader delivery |
| `@Everyone` mention fanout | user cookie or Bearer API key on the user rail | existing message-send permission plus channel membership/visibility gates; recipients are current channel members only | `POST /api/v1/channels/{channelId}/messages` message-create path | broadcast recipients are server-computed `message_mentions`; client recipient arrays are rejected/omitted; offline agent fallback keeps the fixed no-body owner nudge | agents cannot trigger `@Everyone`, and repeated sender/channel broadcasts are rate-limited |
| Image content URL allowlist (#1108 F5) | user cookie or Bearer API key on the user rail | write-time scheme allowlist (`store.IsAllowedImageContentURL`: http(s):// or single-leading-slash same-origin path) when `content_type == image` | `POST /api/v1/channels/{channelId}/messages`, the WS `send` rail, and `PUT /api/v1/messages/{messageId}` (edit preserves `content_type`, so an edited image stays an image) | rejected bodies return 400 `INVALID_CONTENT`; the client mirrors the guard at render and at edit-save | a stored image body cannot become a `javascript:`/`data:`/protocol-relative render vector — at create or at edit |
| Admin API | opaque admin session cookie | admin session row joined to admin identity | admin middleware | admin views use explicit whitelists for sensitive metadata | admin is not represented as user god-mode |
| Capability checks | authenticated user context | user permission rows and scoped resources | authorization helper or legacy permission middleware | no direct serializer surface | app capabilities are scoped to user resources and do not cross into other rails |
| Plugin WebSocket | API key | user/agent row behind the key | plugin connection in hub | plugin lifecycle audit uses server audit source where wired | plugin API bridge is not Remote Agent |
| Remote Agent | remote node token | remote node ownership plus online connection | reverse WebSocket and local allowlist | remote token hidden from node JSON | remote token is not a user session |

## Key Security Invariants

**Rail separation**
Each rail has a distinct credential and entry point. Cross-rail reuse is intentionally limited: API keys can authenticate user API and plugin handshake paths, but API keys do not authenticate admin sessions; remote node tokens can authenticate remote WebSocket connections, but not the user API. The remote rail boundary is node-ownership plus connection scoping; per the remote-agent v1 limits, binding creation does not yet verify channel membership (see [known gaps](../known-gaps.md)), so the remote rail should be described as ownership and connection separation rather than as a multi-user channel-authorization guarantee.

**Owner before capability**
Resource ownership gates appear alongside capability checks. Remote nodes, runtime owner actions, and user privacy audit views all use owner scoping to keep cross-user access from being implied by broad credentials.

**Agent attention owner ceiling**
Per-channel `requireMention` is a membership policy, not a capability grant. Channel managers can set `on` to narrow delivery or `inherit` to follow the agent's global policy. They can set `off` only when the agent's global `require_mention` flag is already false, preserving agent-owner authorization as the ceiling for broader non-mention delivery. Client member listings display server-derived effective state; the browser does not compute or override the owner ceiling.

**Broadcast mention authority**
`@Everyone` fanout is computed by the server from channel membership after the sender passes the normal message-create gates. Clients cannot submit recipient ids, explicit mentions are parsed from persisted content, agents cannot trigger the broadcast token, and repeated broadcasts from the same sender in the same channel are rate-limited.

**Metadata-only admin reads**
Admin rail may read operational metadata, but selected raw fields remain owner-only or omitted. The runtime admin view is the clearest example: it exposes process metadata while omitting raw error reason text.

**Audit is not one uniform sink**
Server admin actions and plugin lifecycle events do not all land in one durable table today. Architecture readers should treat audit as layered by source, with different persistence and ingestion properties.

## Out Of Scope

This page does not define new privileges or future unification. It records the current rail model and the invariants maintainers should preserve when changing auth, remote access, or admin audit.

## Boundary Impact Summary

- Some rails have intentionally separate but not yet unified audit sinks, so cross-source audit completeness varies by module.
- Capability and legacy permission checks are close but not identical, which matters for agent wildcard reasoning.

## Implementation Anchors

- `packages/server-go/internal/auth`
- `packages/server-go/internal/admin`
- `packages/server-go/internal/ws/plugin.go` (`PluginConn`)
- `packages/server-go/internal/ws/remote.go` (`RemoteConn`)
- `packages/server-go/internal/api/remote.go` (`RemoteHandler`)
- `packages/borgee` (remote agent daemon)
- `packages/server-go/internal/api/channels.go` (`ChannelHandler` require-mention policy endpoint)
- `packages/server-go/internal/api/messages.go` (`@Everyone` message-create fanout guard)
- `packages/server-go/internal/api/mention_dispatch.go`
- `packages/server-go/internal/store/require_mention_policy.go`
- `packages/server-go/internal/api/admin.go` (`AdminHandler`)
- `packages/server-go/internal/api/admin_endpoints.go` (`AdminEndpointsHandler`)
- `packages/server-go/internal/api/runtimes.go` (`AdminRuntimeHandler`)
