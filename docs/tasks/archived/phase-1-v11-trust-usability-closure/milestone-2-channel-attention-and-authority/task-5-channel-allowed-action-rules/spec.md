# Spec

Task contract: define visible channel allowed-action rules on the existing Settings channel-management surface so users can distinguish channels they may leave from channels they own or created. This task owns read-only action availability for leave, delete, archive, and owner transfer. It does not execute those mutations or make server authorization decisions.

## Blueprint Anchors

- `CH-1` (`docs/blueprint/next/migration-analysis.md` section 4.3): channel management v1 focuses on membership, ownership, and allowed actions. Notification, collapse, and sort rewrites stay out unless a task explicitly reopens them.
- `CH-1` (`docs/blueprint/next/migration-analysis.md` section 4.2): self-created or owned channels must not present a misleading leave option; owner transfer is not part of the current default commitment.

## Dependencies

- Depends on Task 4 channel management surface. Satisfied by PR #948 (`077cb8c6c4f2c3984221d14f1957f2a41e1a81ed`).
- Does not depend on Task 2 `@Everyone` fanout work. Task 2 PR #951 remains separate and unmerged at task start.
- Task 6 depends on this task for the visible rule contract before server/client action enforcement.

## In Scope

- Add a single client-side allowed-action rule helper for non-DM channel management rows.
- Show read-only action availability for `leave`, `delete`, `archive`, and `owner-transfer` in the Settings channel-management tab.
- Hide the active channel header leave button for self-created or owned channels by using the same leave rule.
- Preserve Task 4 grouping: created channels and joined-only channels stay distinct, and DM channels remain outside this surface.
- Add tests that prove owner/self-created channels do not expose leave as an allowed action and joined-only channels do.
- Sync task docs and current docs.

## Out Of Scope

- No mutation buttons, endpoint calls, or optimistic state changes for leave/delete/archive/owner-transfer in Settings.
- No server authorization or cross-org enforcement changes; Task 6 owns server/client authority checks.
- No owner transfer implementation.
- No notification, collapse, sort, pin, group, private-indicator, sidebar/footer, or M2 Task6/8/9 work.

## Rule Contract

- `leave`: available only for joined, non-DM, non-general channels not created by the current user.
- `delete`: visibly available for non-general channels created by the current user; execution remains unimplemented in Settings until Task 6.
- `archive`: visibly available for non-general channels created by the current user; execution remains unimplemented in Settings until Task 6.
- `owner-transfer`: unavailable in this v1 task for every channel row.

## Drift Checks

- If this task imports or calls `leaveChannel`, `deleteChannel`, or `archiveChannel` from the Settings channel-management component, scope has drifted into Task 6.
- If this task edits Task6/8/9 task folders, private indicator treatment, or sidebar/footer production ownership, scope has drifted.
