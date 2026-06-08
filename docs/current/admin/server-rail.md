# Admin Server Rail

The admin server rail is the separate control plane for operator-level actions. It uses its own identity table, session table, cookie, middleware, and route prefix. It is not a user API shortcut and not a capability wildcard layered on top of normal user auth.

## Overview

**Role**
The admin rail gives operators a bounded management surface for users, permissions, channels, runtime metadata, and audit visibility. It centralizes admin authentication and keeps admin authority out of user sessions.

**Boundary**
The boundary is the admin session. A request must carry the admin session cookie, resolve to an unexpired server-side session, and pass through admin middleware before it reaches admin handlers.

**Collaborators**
The admin rail collaborates with bootstrap configuration, admin session storage, admin middleware, management handlers, runtime metadata views, and audit endpoints. It does not collaborate with user cookie auth, dev bypass, or remote node tokens.

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

The production login path verifies the configured admin bcrypt hash and requires a stored hash with cost at least 10 during bootstrap. The only exception is the explicit `BORGEE_TEST_FAST_ADMIN_PASSWORD` test hook used by the Playwright server process; it short-circuits repeated e2e admin login checks against the configured plaintext while leaving bootstrap hash validation and the default production path unchanged.

**Invariants**

- Admin identity is stored outside the user table.
- The admin cookie contains an opaque token, not an admin id or JWT claims.
- User auth middleware does not grant admin rail access.
- Admin runtime visibility is metadata-only and excludes selected raw owner-facing fields.
- Legacy user-rail admin god-mode routes are not part of the current wiring.

## Management Surface Design

The admin rail is a collection of operator tools, not a blanket bypass. Some surfaces are read-only by design, such as runtime metadata. Some surfaces are mutating, such as user management and force channel deletion. Mutating surfaces must be evaluated for audit coverage and user-notification expectations rather than assumed safe because they are behind admin auth.

Permission management on the admin rail uses the same capability vocabulary as the user authorization model. This keeps arbitrary capability strings from being introduced through the admin UI while still keeping admin rail authentication separate from ordinary capability checks.

Canonical server audit storage is `audit_events`. The `admin_actions` name remains as a compatibility view and store facade for existing admin audit helpers and projections.

## Server Rail Surfaces

| Surface | Admin SPA page | Rail exposure | Current posture |
| --- | --- | --- | --- |
| Admin auth/session | Login/settings session surfaces | Admin API | Login/logout/me endpoints backed by admin session cookie. |
| Dashboard stats | Yes | Admin API | Read-only counts and org aggregation. |
| Users and user detail | Yes | Admin API | User create/update/delete, password/role/disabled changes, permissions, and owned-agent metadata. |
| Invites | Yes | Admin API | Invite create/list/revoke. |
| Channels | Yes | Admin API | Channel metadata plus explicit force-delete operation. |
| Archived channels | Yes | Admin API | Read-only archived channel list. |
| Channel description history | Yes | Admin API | Read-only description edit history. |
| Runtime metadata | Yes | Admin API | Read-only agent runtime metadata; raw owner diagnostics are narrowed. |
| Heartbeat lag | Yes | Admin API | Read-only rolling heartbeat lag snapshot. |
| Admin audit log | Yes | Admin API | Filterable admin action projection. |
| Multi-source audit | Yes | Admin API | Read-only merged audit projection across configured sources. |
| Audit retention override | No | Admin API only | Server-only operator endpoint records `audit_retention_override`; no SPA page currently calls it. |
| Heartbeat retention override | No | Admin API only | Server-only operator endpoint records the same audit action with heartbeat target metadata; no SPA page currently calls it. |
| Message edit history | No | Admin API only | Read-only admin endpoint for message edit history; no admin SPA page/client wrapper currently exposes it. |
| Artifact comment edit history | No | Admin API only | Read-only admin endpoint for artifact-comment edit history; no admin SPA page/client wrapper currently exposes it. |
| User impersonation grant lifecycle | User SPA, not Admin SPA | User API plus audit projection | User-owned grant create/read/revoke; admin rail consumes audit visibility but has no SPA impersonation page. |
| Retention sweepers | No | Server-only background jobs | Audit, heartbeat, and cold event retention jobs run as server background processes; current cold event retention only reaps rows with explicit eligible `retention_days`. |

## Metadata-Only Runtime View

The runtime admin view summarizes process descriptors and heartbeat metadata. It intentionally omits raw last-error details that remain part of the owner-facing runtime view. This is the main example of the admin rail being powerful but not unlimited: operational visibility is allowed, raw user-facing diagnostic text is narrowed.

## Out Of Scope

The admin rail does not authenticate with user cookies, connect Remote Agent nodes, or browse user content.

## Known Gaps

- Audit coverage is not uniform across all admin write surfaces.

## Implementation Anchors

- `packages/server-go/internal/admin/`
- `packages/server-go/internal/migrations/`
- `packages/server-go/internal/server/server.go`
- `packages/server-go/internal/api/admin*.go`
- `packages/server-go/internal/api/runtimes.go`
