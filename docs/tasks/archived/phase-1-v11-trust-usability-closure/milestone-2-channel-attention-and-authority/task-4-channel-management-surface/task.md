# task-4-channel-management-surface

Purpose:
- Give users a clear place to understand joined and created channels.

Scope:
- Add the channel management surface route/entry and basic joined/created channel listing needed for ownership and membership actions.
- Keep notification/collapse/sort rewrites out unless explicitly reopened later.

Out of scope:
- No hard delete/archive/owner-transfer commitment.
- No private-channel visual treatment; that is milestone 3.

Depends on:
- Canonical Milestone 2 start

Blueprint anchors:
- `CH-1`: `docs/blueprint/next/migration-analysis.md` §4.3
- `PS-1`: `docs/blueprint/next/migration-analysis.md` §6.1

Acceptance slice:
- A reviewer can reach a management surface that distinguishes joined and created channels without changing channel authority.

Parallelism:
- First task for this milestone when the canonical milestone opens. Blocks action-rule tasks.

Sensitive paths:
- channel ACL, ownership visibility, privacy
