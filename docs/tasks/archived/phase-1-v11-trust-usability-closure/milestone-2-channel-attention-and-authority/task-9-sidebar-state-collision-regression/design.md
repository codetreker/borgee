# Design: Sidebar State Collision Regression

## Decision

Task 9 adds regression proof only. The existing Task 8 private marker remains in the leading channel marker slot, and this task verifies that it does not consume row-level, trailing, overlay, archive, preview, or DM-only state surfaces.

## Files

- `packages/client/src/__tests__/Sidebar-state-collision-regression.test.tsx`
  - Mocks `useSortable` to force drag-over state without broad e2e setup.
  - Renders `SortableChannelItem` with private + unread + selected + pinned + drag-over state and asserts each signal remains in its own DOM anchor.
  - Renders archived and public preview variants to prove archived overrides private/unread and public previews do not become private rows.
  - Reads source to ensure `SortableChannelItem` stays out of DM-only `PresenceDot`, `data-presence`, and `data-failure-badge` semantics.
  - Reads task/current docs so the regression evidence remains documented.
- `docs/current/client/ui/channel-sort-groups.md`
  - Records Task9 sidebar state collision regression as the current proof for this row state boundary.
- Task9 docs under this directory
  - Capture spec, stance, regression matrix, progress, and acceptance evidence.

## Boundary Handling

- Private state remains a channel-row leading-slot marker.
- Unread remains `.unread-badge`; pinned remains `.channel-pinned-indicator`; selected remains `.channel-item-active`; drag-over remains `.drop-indicator`.
- Archived rows remain authoritative over public/private and suppress unread.
- Public preview rows remain public preview rows and do not imply private non-member visibility.
- DM presence/fault remains in `Sidebar.tsx` DM rows, not in channel row components.

## Test Design

The regression/evidence test was written before Task9 docs and first failed because `acceptance.md` was missing. Task9 then adds the required task/current evidence while leaving production UI code unchanged.
