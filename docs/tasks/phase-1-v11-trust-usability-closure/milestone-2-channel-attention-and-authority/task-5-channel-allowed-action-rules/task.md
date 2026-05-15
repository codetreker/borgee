# task-5-channel-allowed-action-rules

Purpose:
- Make allowed channel actions explicit and prevent misleading owner leave behavior.

Scope:
- Define visible leave/delete/archive/owner-transfer action availability, with self-created or owned channels not showing misleading leave.
- Keep delete/archive/owner-transfer as explicit task decisions rather than accidental side effects.

Out of scope:
- No broad channel settings rewrite.
- No notification/collapse/sort controls unless explicitly scoped later.

Depends on:
- `task-4-channel-management-surface`

Blueprint anchors:
- `CH-1`: `docs/blueprint/next/migration-analysis.md` §4.3

Acceptance slice:
- A reviewer can verify self-created or owned channels do not expose a misleading leave action and other action availability is explicit.

Parallelism:
- Can run after task 1. Blocks authority enforcement task.

Sensitive paths:
- channel ownership, destructive action authority
