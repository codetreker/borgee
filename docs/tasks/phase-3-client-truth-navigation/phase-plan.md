# Phase 3: Client Truth And Navigation

> Superseded for v1.1 execution grouping. This file is retained as accepted planning history and task-detail context. This is no longer a current Phase; the work is remapped into `docs/tasks/phase-1-v11-trust-usability-closure/milestone-3-client-truth-and-navigation/`. The old Phase 3 boundary was an execution slot, not a prerequisite, integration, or downstream coordination boundary.

## Source Anchors

- `CT-1`: Client truthfulness and forbidden-state visibility.
- `IA-1`: Sidebar footer and account entry IA.
- `PS-1`: Privacy scope guard; do not expand user-facing privacy/compliance product scope.
- Source issues: gh#724, gh#669, gh#670, gh#654.

## Value Loop

A user can reach the product surfaces Borgee claims to have, understands forbidden/empty/error states, and uses a calmer account/sidebar entry model without adding privacy/compliance product noise.

## Milestones

| Milestone | Goal | Status | Task-split trigger |
|---|---|---|---|
| `milestone-1-production-surface-truthfulness` | Mount already-built client surfaces in production and show forbidden states without leaking private names/bodies | PLANNED | Break down after phase-plan acceptance |
| `milestone-2-sidebar-account-entry` | Reduce footer clutter and make avatar/account panel the logout/account entry without merging Helper/Remote Agent rails | PLANNED | Break down after or alongside truthfulness if shared shell components are clear |

Historical note: this superseded Phase had 2 user-facing milestones, which are now one canonical client truth/navigation milestone.

## Exit Gates

Strict checks:

- ArtifactComments/ArtifactPanel and Settings `PermissionsView` are reachable through production UI when claimed.
- Forbidden states do not leak private channel, artifact, message, file, or body content before authorization succeeds.
- Sidebar/account IA movement does not merge credentials, grants, or enforcement rails.
- No new user-facing privacy/compliance product surface is introduced through gh#654.

User-perceivable checks:

- Users see truthful forbidden, unavailable, loading, and empty states instead of blank screens or fake success.
- Users can find account/logout and primary workspace/settings/agent entries without footer clutter.

Carry-over checks:

- gh#707, gh#697, gh#702, gh#607, and gh#675 remain backlog unless explicitly pulled into a later task.
