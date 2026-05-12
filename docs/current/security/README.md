# Security And Privacy Boundaries

Security in the current architecture is rail-oriented. Each rail has its own credential, authorization source, runtime boundary, and privacy surface. The important design rule is that authority does not automatically cross rails: a user cookie is not an admin session, a remote node token is not a host grant, and a helper grant is not a general app capability.

## Overview

**Role**
This document defines the cross-module security boundaries that keep user API, admin API, plugin connection, Remote Agent, Host Bridge helper, and installer responsibilities separate.

**Boundary**
The boundary is the rail. Requests are first classified by rail, then authenticated and authorized by that rail's own mechanism. Shared storage does not imply shared authority.

**Collaborators**
The security model spans user auth, admin sessions, capability checks, plugin WebSocket auth, remote node tokens, host grants, helper ACL, installer manifest verification, and audit surfaces.

**Internal Architecture**

- Identity rails: user, admin, plugin agent, remote node, helper agent, installer operator.
- Authorization sources: user permissions, admin sessions, API keys, remote node tokens, host grants, manifest signatures.
- Enforcement points: HTTP middleware, WebSocket handshake, helper IPC handshake, helper ACL, installer verification.
- Privacy surfaces: serializers, metadata-only admin views, user-scoped audit, local helper audit.

**Key Flows**

```text
user API:      user credential -> user context -> owner/capability/resource checks
admin API:     admin session cookie -> admin context -> admin rail only
plugin WS:     API key -> agent/plugin connection -> scoped API bridge
remote WS:     remote node token -> remote connection -> file request tunnel
helper IPC:    local agent id -> ACL -> host grant lookup -> local action
installer:     manifest fetch -> signature verify -> local operator deploy
```

**Invariants**

- Admin authority lives on the admin rail, not in `users.role`.
- User API authority lives in user identity plus owner/capability checks.
- Host grants are separate from user API capabilities.
- Remote Agent and Host Bridge use different credentials, transports, and local enforcement models.
- Privacy-sensitive raw data is either omitted from serializers or exposed only to the owner/admin rail designed for that data.
- Audit is layered: durable server audit for admin actions, local JSONL for helper IPC, and best-effort notifications where appropriate.

## Cross-Rail Matrix

| Rail | Credential | Authorization Source | Runtime Boundary | Privacy Surface | Key Invariant |
| --- | --- | --- | --- | --- | --- |
| User API | user cookie, Bearer API key, development bypass in development | user row plus owner/capability checks | HTTP middleware and handlers | user serializers hide internal columns | user credential does not enter admin rail |
| Admin API | opaque admin session cookie | admin session row joined to admin identity | admin middleware | admin views use explicit whitelists for sensitive metadata | admin is not represented as user god-mode |
| Capability checks | authenticated user context | user permission rows and scoped resources | authorization helper or legacy permission middleware | no direct serializer surface | app capabilities do not authorize host helper grants |
| Plugin WebSocket | API key | user/agent row behind the key | plugin connection in hub | plugin lifecycle audit uses server audit source where wired | plugin API bridge is not Remote Agent |
| Remote Agent | remote node token | remote node ownership plus online connection | reverse WebSocket and local allowlist | remote token hidden from node JSON | remote token is not a host grant |
| Host Bridge helper | local handshake agent id | host grant row by agent and scope | UDS IPC, ACL, sandbox, read-only IO | local JSONL audit | helper cannot create grants |
| Installer | optional fetch Bearer plus ed25519 public key | manifest signature and operator confirmation | local package/service manager | no app data surface | installation does not authorize later helper requests |

## Key Security Invariants

**Rail separation**
Each rail has a distinct credential and entry point. Cross-rail reuse is intentionally limited: API keys can authenticate user API and plugin handshake paths, but not admin sessions; remote node tokens can authenticate remote WebSocket connections, but not user API; host grants can be consumed by helper ACL, but not by Remote Agent.

**Owner before capability**
Resource ownership gates appear alongside capability checks. Remote nodes, host grants, runtime owner actions, and user privacy audit views all use owner scoping to keep cross-user access from being implied by broad credentials.

**Metadata-only admin reads**
Admin rail may read operational metadata, but selected raw fields remain owner-only or omitted. The runtime admin view is the clearest example: it exposes process metadata while omitting raw error reason text.

**Audit is not one uniform sink**
Server admin actions, plugin lifecycle events, helper IPC, and host grant changes do not all land in one durable table today. Architecture readers should treat audit as layered by source, with different persistence and ingestion properties.

## Out Of Scope

This page does not define new privileges or future unification. It records the current rail model and the invariants maintainers should preserve when changing auth, remote access, host grants, helper IPC, or admin audit.

## Known Gaps

- Legacy permission middleware and the newer capability helper do not have identical wildcard semantics for agents.
- Helper audit is local and best-effort, not yet ingested as a durable server audit source.
- Remote node token delivery is not clearly modeled in the user API.
- Some admin write paths have durable audit hooks and others do not.

## Implementation Anchors

- `packages/server-go/internal/auth` (`AuthMiddleware`, `HasCapability`, `RequirePermission`, capability constants)
- `packages/server-go/internal/admin` (`Handler`, `RequireAdmin`, `AdminSession`)
- `packages/server-go/internal/ws/plugin.go` (`PluginConn`, `HandlePlugin`)
- `packages/server-go/internal/ws/remote.go` (`RemoteConn`, `HandleRemote`)
- `packages/server-go/internal/api/remote.go` (`RemoteHandler`)
- `packages/server-go/internal/api/host_grants.go` (`HostGrantsHandler`)
- `packages/borgee-helper/internal/acl` (`Gate`)
- `packages/borgee-helper/internal/ipc` (`Handler`)
- `packages/borgee-installer/internal/manifest` (`Verify`)
- `packages/server-go/internal/api/admin.go` (`AdminHandler`)
- `packages/server-go/internal/api/admin_endpoints.go` (`AdminEndpointsHandler`)
- `packages/server-go/internal/api/runtimes.go` (`AdminRuntimeHandler`)
