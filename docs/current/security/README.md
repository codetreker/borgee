# Security And Privacy Boundaries

Security in the current architecture is rail-oriented. Each rail has its own credential, authorization source, runtime boundary, and privacy surface. The important design rule is that authority does not automatically cross rails: a user cookie is not an admin session, a remote node token is not a host grant, a host grant is not a Helper enrollment, and a helper grant is not a general app capability.

## Overview

**Role**
This document defines the cross-module security boundaries that keep user API, admin API, plugin connection, Remote Agent, Helper enrollment, Host Bridge helper, and installer responsibilities separate.

**Boundary**
The boundary is the rail. Requests are first classified by rail, then authenticated and authorized by that rail's own mechanism. Shared storage does not imply shared authority.

**Collaborators**
The security model spans user auth, admin sessions, capability checks, plugin WebSocket auth, remote node tokens, Helper enrollment credentials, host grants, helper ACL, the installer verifier path, and audit surfaces.

**Internal Architecture**

- Identity rails: user, admin, plugin agent, remote node, Helper enrollment, helper agent, installer operator.
- Authorization sources: user permissions, admin sessions, API keys, remote node tokens, Helper enrollment secrets/credential digests, host grants, and local operator confirmation.
- Enforcement points: HTTP middleware, WebSocket handshake, Helper claim/status handlers, helper IPC handshake, helper ACL, and the installer verifier path.
- Privacy surfaces: serializers, metadata-only admin views, user-scoped audit, Helper enrollment redacted status, local helper audit.

**Key Flows**

```text
user API:      user credential -> user context -> owner/capability/resource checks
admin API:     admin session cookie -> admin context -> admin rail only
plugin WS:     API key -> agent/plugin connection -> scoped API bridge
remote WS:     remote node token -> remote connection -> intended list/read tunnel
helper enroll: one-time enrollment secret -> persistent Helper credential -> status/rotation/uninstall only
helper IPC:    local agent id -> ACL -> host grant lookup -> local action
installer:     manifest fetch -> partial verifier path -> local artifact deploy
```

**Invariants**

- Admin authority lives on the admin rail, not in `users.role`.
- User API authority lives in user identity plus owner/capability checks.
- Host grants are separate from user API capabilities.
- Helper enrollment credentials are separate from user API credentials, Remote Agent tokens, host grants, and user permissions.
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
| Remote Agent | remote node token | remote node ownership plus online connection | reverse WebSocket and intended local allowlist; current envelope caveat applies | remote token hidden from node JSON | remote token is not a host grant |
| Helper enrollment | one-time enrollment secret, then current persistent Helper credential digest | helper enrollment row scoped by owner, org, enrollment id, helper device id, status, allowed categories, and credential generation | HTTP claim/status/rotate/uninstall handlers; no job execution in this foundation | enrollment serializers omit org id, raw secrets, digests, and token equivalents; claim/rotation return raw credentials once | Helper credential is not a user credential, Remote Agent token, host grant, or user permission |
| Host Bridge helper | local handshake agent id | host grant row by agent and scope | UDS IPC, ACL, sandbox, read-only IO | local JSONL audit | helper cannot create grants |
| Installer | optional fetch Bearer plus configured verification key where wired | partial verifier path and operator confirmation | local package/service manager for caller-supplied artifact path | no app data surface | installation does not authorize later helper requests; deployment trust is partial wiring until envelope shape, signing-key injection, and local artifact binding align |

## Key Security Invariants

**Rail separation**
Each rail has a distinct credential and entry point. Cross-rail reuse is intentionally limited: API keys can authenticate user API and plugin handshake paths, but not admin sessions; remote node tokens can authenticate remote WebSocket connections, but not user API or Helper status; Helper credentials can claim, rotate, and update Helper enrollment lifecycle state, but not Remote Agent or host grants; host grants can be consumed by helper ACL, but not by Remote Agent or Helper enrollment. Current Remote Agent filesystem proxying still carries an implementation caveat, so the rail boundary should be described as ownership and connection separation rather than as a settled filesystem-security guarantee.

**Owner before capability**
Resource ownership gates appear alongside capability checks. Remote nodes, Helper enrollments, host grants, runtime owner actions, and user privacy audit views all use owner scoping to keep cross-user access from being implied by broad credentials. Helper enrollment status also binds org id and helper device id; host label alone is display metadata, not authority.

**Metadata-only admin reads**
Admin rail may read operational metadata, but selected raw fields remain owner-only or omitted. The runtime admin view is the clearest example: it exposes process metadata while omitting raw error reason text.

**Audit is not one uniform sink**
Server admin actions, plugin lifecycle events, helper IPC, and host grant changes do not all land in one durable table today. Architecture readers should treat audit as layered by source, with different persistence and ingestion properties.

## Out Of Scope

This page does not define new privileges or future unification. It records the current rail model and the invariants maintainers should preserve when changing auth, remote access, host grants, helper IPC, or admin audit.

## Boundary Impact Summary

- Some rails have intentionally separate but not yet unified audit sinks, so cross-source audit completeness varies by module.
- Remote Agent's rail separation is clearer than its current end-to-end filesystem proxy contract.
- Helper enrollment currently provides identity/status authority only: claim, heartbeat, credential rotation, revoke, and helper-originated uninstall state. It does not provide a job queue, command channel, service lifecycle execution, or Configure OpenClaw success state.
- Installer deployment trust is partial wiring; [../host-bridge/installer.md](../host-bridge/installer.md) owns the envelope, signing-key, and artifact-binding details.
- Capability and legacy permission checks are close but not identical, which matters for agent wildcard reasoning.

## Implementation Anchors

- `packages/server-go/internal/auth`
- `packages/server-go/internal/admin`
- `packages/server-go/internal/ws/plugin.go` (`PluginConn`)
- `packages/server-go/internal/ws/remote.go` (`RemoteConn`)
- `packages/server-go/internal/api/remote.go` (`RemoteHandler`)
- `packages/server-go/internal/api/helper_enrollments.go` (`HelperEnrollmentHandler`)
- `packages/server-go/internal/api/host_grants.go` (`HostGrantsHandler`)
- `packages/server-go/internal/store/helper_enrollment_queries.go`
- `packages/server-go/internal/migrations/helper_enrollments.go` (`helper_enrollments` schema)
- `packages/borgee-helper/internal/acl` (`Gate`)
- `packages/borgee-helper/internal/ipc`
- `packages/borgee-installer/internal/manifest`
- `packages/server-go/internal/api/admin.go` (`AdminHandler`)
- `packages/server-go/internal/api/admin_endpoints.go` (`AdminEndpointsHandler`)
- `packages/server-go/internal/api/runtimes.go` (`AdminRuntimeHandler`)
