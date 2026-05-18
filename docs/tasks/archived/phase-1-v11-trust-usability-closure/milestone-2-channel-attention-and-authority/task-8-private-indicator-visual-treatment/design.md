# Design: Private Indicator Visual Treatment

## Decision

Keep the private-channel indicator in the existing leading channel marker slot, but render it as a muted text lock marker instead of the colorful `🔒` emoji. The marker is intentionally small and monochrome through normal sidebar text color inheritance.

## Files

- `packages/client/src/components/SortableChannelItem.tsx`
  - Add a shared `ChannelVisibilityMarker` helper used by sortable member rows and static channel rows.
  - Add `data-kind="channel"` to channel rows and `data-private="true"` only when the active row is private and not archived.
  - Add `data-private-indicator="true"`, `.channel-private-indicator`, `aria-label="私有频道"`, and `title="私有频道"` to the private marker.
- `packages/client/src/index.css`
  - Keep `.channel-hash` fixed-width in the leading slot.
  - Style `.channel-private-indicator` as a compact muted badge that becomes slightly clearer on active/hover row states.
- `packages/client/src/__tests__/SortableChannelItem-private-indicator.test.tsx`
  - Cover private + unread + selected + pinned, public baseline, archived override, and static-row fallback.
- `docs/current/client/ui/channel-sort-groups.md`
  - Sync current channel-row state anchors.

## Boundary Handling

- Archived rows still override the private/public marker with `📦`, still render `.archived-badge`, and still suppress unread badges.
- Unread remains in `.unread-badge`; pinned remains in `.channel-pinned-indicator`; drag-over remains `.drop-indicator`.
- Channel rows do not emit DM-only `data-presence` or `data-failure-badge` selectors.

## Test Design

The test was written before implementation and first failed because current rows lacked `data-private`, `data-kind`, and the quiet private marker. The implementation then made the test pass without changing server/API behavior.
