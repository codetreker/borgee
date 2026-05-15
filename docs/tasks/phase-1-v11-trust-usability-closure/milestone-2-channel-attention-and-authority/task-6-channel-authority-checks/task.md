# task-6-channel-authority-checks

Purpose:
- Ensure channel management actions remain server-authoritative and match client-visible options.

Scope:
- Add server/client checks for membership and ownership actions exposed by the management surface.
- Prevent cross-channel or cross-org action leakage and preserve backend authority as the source of truth.

Out of scope:
- No unrelated channel sorting, notification, or private-indicator work.

Depends on:
- `task-4-channel-management-surface`
- `task-5-channel-allowed-action-rules`

Blueprint anchors:
- `CH-1`: `docs/blueprint/next/migration-analysis.md` §4.3
- `PS-1`: `docs/blueprint/next/migration-analysis.md` §6.1

Acceptance slice:
- A reviewer can prove allowed and denied channel management actions are enforced server-side and reflected truthfully in the client.

Parallelism:
- Runs after surface and action-rule tasks.

Sensitive paths:
- auth, channel ACL, cross-org data isolation
