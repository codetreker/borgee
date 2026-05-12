# Admin Server Rail

The admin server rail is the separate control plane for operator-level actions. It uses its own identity table, session table, cookie, middleware, and route prefix. It is not a user API shortcut and not a capability wildcard layered on top of normal user auth.

## Overview

**Role**
The admin rail gives operators a bounded management surface for users, permissions, channels, runtime metadata, and audit visibility. It centralizes admin authentication and keeps admin authority out of user sessions.

**Boundary**
The boundary is the admin session. A request must carry the admin session cookie, resolve to an unexpired server-side session, and pass through admin middleware before it reaches admin handlers.

**Collaborators**
The admin rail collaborates with bootstrap configuration, admin session storage, admin middleware, management handlers, runtime metadata views, and audit endpoints. It does not collaborate with user cookie auth, dev bypass, remote node tokens, or host grant ownership.

**Internal Architecture**

- Bootstrap: environment-configured admin identity is inserted or reused at startup.
- Session lifecycle: login creates an opaque server-side session token; logout deletes it.
- Middleware: admin session resolution creates the admin context for protected routes.
- Management surface: handlers operate under the admin prefix and use explicit serializers and whitelists.
- Audit integration: selected write paths emit durable audit rows and user-visible notifications.

**Key Flows**

```text
bootstrap -> admin row exists
login -> bcrypt verify -> opaque session row -> admin cookie
admin request -> session lookup -> admin context -> handler
logout -> session delete -> cookie cleared
```

**Invariants**

- Admin identity is stored outside the user table.
- The admin cookie contains an opaque token, not an admin id or JWT claims.
- User auth middleware does not grant admin rail access.
- Admin runtime visibility is metadata-only and excludes selected raw owner-facing fields.
- Legacy user-rail admin god-mode routes are not part of the current wiring.

## Management Surface Design

The admin rail is a collection of operator tools, not a blanket bypass. Some surfaces are read-only by design, such as runtime metadata. Some surfaces are mutating, such as user management and force channel deletion. Mutating surfaces must be evaluated for audit coverage and user-notification expectations rather than assumed safe because they are behind admin auth.

Permission management on the admin rail uses the same capability vocabulary as the user authorization model. This keeps arbitrary capability strings from being introduced through the admin UI while still keeping admin rail authentication separate from ordinary capability checks.

## Metadata-Only Runtime View

The runtime admin view summarizes process descriptors and heartbeat metadata. It intentionally omits raw last-error details that remain part of the owner-facing runtime view. This is the main example of the admin rail being powerful but not unlimited: operational visibility is allowed, raw user-facing diagnostic text is narrowed.

## Out Of Scope

The admin rail does not authenticate with user cookies, create host grants for users, connect Remote Agent nodes, or act as helper daemon authority.

## Known Gaps

- Audit coverage is not uniform across all admin write surfaces.
- The impersonation-grant validation helper exists but is not clearly wired into current production write handlers.

## Implementation Anchors

- `packages/server-go/internal/admin/auth.go` (`Handler`, `Admin`, `AdminSession`, `Bootstrap`)
- `packages/server-go/internal/admin/middleware.go` (`RequireAdmin`, `AdminFromContext`)
- `packages/server-go/internal/migrations/admin_admins.go`
- `packages/server-go/internal/migrations/admin_sessions.go`
- `packages/server-go/internal/server/server.go` (admin route wiring)
- `packages/server-go/internal/api/admin.go` (`AdminHandler`)
- `packages/server-go/internal/api/runtimes.go` (`AdminRuntimeHandler`)
- `packages/server-go/internal/api/admin_endpoints.go` (`AdminEndpointsHandler`)
- `packages/server-go/internal/api/admin_grant_check.go` (`RequireImpersonationGrant`)
