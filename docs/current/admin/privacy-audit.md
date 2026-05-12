# Admin Privacy Audit

Admin privacy audit is the design that makes admin impact visible without making the full audit stream public to every user. It combines a durable server-side audit record, a user-scoped privacy view, an admin-scoped audit view, best-effort user notification, and a user-controlled impersonation grant surface.

## Overview

**Role**
Privacy audit records selected admin impacts and provides two visibility modes: affected users can see actions targeting them, while admins can inspect broader operational audit data through the admin rail.

**Boundary**
The boundary is audience. User-facing privacy views are target-user scoped. Admin-facing audit views are admin-session scoped. Helper local audit is outside this durable server audit boundary unless explicitly ingested.

**Collaborators**
Privacy audit collaborates with admin write handlers, audit storage, user settings/privacy surfaces, system DM notification, impersonation grants, retention/archival behavior, and multi-source admin audit queries.

**Internal Architecture**

- Audit row: durable server record of actor, target, action, metadata, and time.
- User view: filtered projection for the affected user's own records.
- Admin view: broader filterable projection for admin operators.
- Notification path: best-effort system DM after audit insertion.
- Impersonation grant path: user-created state that expresses temporary admin support consent.

**Key Flows**

```text
audited admin write:
  admin handler commits change -> insert audit row -> best-effort system DM

user privacy read:
  user request -> current user id -> target-user filtered audit rows

admin audit read:
  admin session -> filterable audit query -> admin projection

impersonation consent:
  user creates grant -> active 24h state -> user can revoke
```

**Invariants**

- The durable audit row is the source of truth; notification is best-effort.
- User privacy reads cannot select another target user by query parameter.
- Admin audit reads require the admin rail, not the user rail.
- Impersonation consent is user-created, time-bounded, and revocable.
- Multi-source audit is a projection across sources, not proof that all sources share one persistence model.

## Privacy Model

The user privacy view is deliberately narrow. It answers: "what admin actions affected me?" It does not expose all admin activity, all actor details, or other users' records.

The admin audit view is broader but still structured. It supports filtering and archival state, and it is accessed through the admin rail. This keeps operational audit review separate from user-facing transparency.

## Notification Model

System DM notification is paired with durable audit but does not replace it. The architecture favors preserving the audit row even if a user-visible message cannot be delivered. This prevents notification fragility from weakening audit durability.

## Impersonation Grant Model

The user-owned impersonation grant represents a temporary support consent state. It is stored separately from the audit row because it is active state: it can expire or be revoked. Audit records remain historical; grants represent current permission to perform support activity.

## Out Of Scope

Privacy audit does not guarantee helper JSONL ingestion, does not make all admin writes uniformly audited today, and does not by itself enforce impersonation checks.

## Known Gaps

- Some admin write paths have audit hooks and some do not.
- `RequireImpersonationGrant` currently has no production admin write-handler call sites; it is present as a helper with tests, so the current limitation is wiring, not grant storage or user grant CRUD.
- Helper-local audit is not yet part of the durable server audit projection.

## Implementation Anchors

- `packages/server-go/internal/store/admin_actions.go` (`AdminAction`, `InsertAdminAction`, `EmitAdminActionAudit`, `ImpersonationGrant`)
- `packages/server-go/internal/api/admin_endpoints.go` (`AdminEndpointsHandler`)
- `packages/server-go/internal/api/admin.go` (`AdminHandler` audit hook call sites)
- `packages/server-go/internal/api/admin_audit_query.go` (`AdminAuditMultiSourceHandler`)
- `packages/server-go/internal/migrations/admin_actions.go`
- `packages/server-go/internal/migrations/admin_impersonation_grants.go`
- `packages/server-go/internal/migrations/admin_audit_events_rename.go`
