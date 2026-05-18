# Spec

Task contract: make the channel management actions from Tasks 4 and 5 truthful against server authority. The server remains the source of truth for membership and ownership actions, and the client must not expose an action as available unless the caller has the matching channel authority signal.

## Blueprint Anchors

- `CH-1` (`docs/blueprint/next/migration-analysis.md` section 4.3): channel management v1 focuses on membership, ownership, allowed actions, and clear owner leave behavior.
- `PS-1` (`docs/blueprint/next/migration-analysis.md` section 6.1): preserve privacy/security boundaries and avoid cross-channel or cross-org leakage.

## Dependencies

- Task 4 channel management surface: merged as PR #948.
- Task 5 allowed action rules: merged as PR #953 at `6ae46043c897438367469b5c70dae0f84f036581`.

## In Scope

- Server-side guards for user-rail channel membership and ownership actions:
  - creator cannot leave their own channel;
  - non-members cannot leave or manage a channel;
  - delete and archive require same-org channel creator authority in addition to existing permission rows;
  - member add/remove and require-mention management require channel membership and same-org authority where applicable;
  - channel creator cannot be removed through member management.
- Client truthfulness updates for existing channel-management affordances:
  - Settings allowed-action rules consider server permission state for delete/archive;
  - member modal destructive actions require current user ownership plus permission;
  - member modal does not show remove controls for the channel creator.
- Focused tests proving allowed and denied paths on server and client.
- Current docs and task docs sync.

## Out Of Scope

- No new Settings mutation buttons for leave, delete, archive, owner transfer, notification, collapse, sort, pin, or group actions.
- No owner-transfer implementation.
- No private-indicator/sidebar state work and no M2 Task9 regression work.
- No admin-rail channel force-delete changes.

## Acceptance Slice

A reviewer can prove that visible channel management availability is not merely client-derived, and that user-rail mutation endpoints reject misleading or cross-boundary actions even when a caller has broad permission rows.
