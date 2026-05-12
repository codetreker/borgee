# Host Grants

Host grants are the server-side consent records for Host Bridge. They are not general app permissions and not Remote Agent directory bindings. Their job is to give the helper a small, durable, user-owned fact to consult when a local agent asks for a host capability.

## Overview

**Role**
Host grants model user consent for host capabilities. They connect a user, optional agent, grant type, scope, TTL, and revocation state into a row that can be checked by the helper.

**Boundary**
The boundary is ownership plus scope. A user can manage their own grants through the user API. A helper can consume only grants matching the requesting agent and normalized scope. Admin rail does not create host grants for users.

**Collaborators**
Host grants collaborate with user authentication, helper SQLite lookup, helper ACL scope normalization, and local helper audit. They are deliberately separate from `user_permissions` capabilities and remote node tokens.

**Internal Architecture**

- API layer: creates, lists, and revokes grants for the authenticated user.
- Schema layer: stores grant type, scope, TTL, grant time, expiry, and revocation.
- Helper lookup layer: reads active grants by agent and scope on every request.
- Revocation model: marks rows revoked rather than deleting them.

**Key Flows**

```text
create:
  user request -> validate grant type/scope/ttl -> insert grant

consume:
  helper normalizes request target -> lookup active grant by agent and scope
  -> allow, expired, or not found

revoke:
  user request -> owner check -> stamp revoked_at
  -> next helper lookup no longer treats row as active
```

**Invariants**

- Host grants are user-owned; cross-user management is rejected.
- Grant type vocabulary is independent from capability vocabulary.
- Revocation is forward-only state, not a hard delete in the normal revoke path.
- Helper consumption is read-only and does not cache grant state.
- Filesystem scopes and network scopes are explicit strings, not implicit path permissions.

## Scope Model

The helper interprets scopes after normalizing the request target. Filesystem actions become filesystem scopes; network egress actions become egress scopes. Server-side grant creation validates the grant vocabulary and non-empty scope, while helper-side ACL is the stricter consumer of normalized scope semantics.

## TTL And Revocation Model

Short-lived grants carry an expiry timestamp; persistent grants omit it. Revoked grants stay in the database with a revocation timestamp so the history of consent changes is not erased. The helper filters revoked grants out of active decisions and treats expired grants as denied.

## Out Of Scope

Host grants do not authorize Remote Agent, do not replace user API capabilities, and do not create an admin-wide host access path.

## Known Gaps

- Server grant creation does not fully validate helper-specific scope shape.
- User-level install/exec grants and helper agent-scoped lookup are not fully reconciled in the current architecture.
- Server-side host grant audit is not yet a durable audit-events source.

## Implementation Anchors

- `packages/server-go/internal/api/host_grants.go` (`HostGrantsHandler`)
- `packages/server-go/internal/migrations/host_grants.go` (`hostGrants` migration)
- `packages/borgee-helper/internal/acl` (`Gate.Decide`)
- `packages/borgee-helper/internal/grants` (`SQLiteConsumer.LookupRaw`)
- `packages/server-go/internal/auth/middleware.go` (`AuthMiddleware`)
