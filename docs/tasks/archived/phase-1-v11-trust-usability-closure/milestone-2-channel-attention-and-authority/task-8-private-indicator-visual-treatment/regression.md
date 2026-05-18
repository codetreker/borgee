# Regression: Private Indicator Visual Treatment

## Automated Coverage

`packages/client/src/__tests__/SortableChannelItem-private-indicator.test.tsx` locks these cases:

- Private + unread + selected + pinned: private marker stays in the leading slot; unread and pinned remain visible.
- Public baseline: public rows still render `#` and no private metadata.
- Private + archived: archived marker and badge win; unread is suppressed.
- Static channel rows: if the server returns a private static row, it uses the same quiet private marker without preview semantics.
- DM separation: channel rows do not emit `data-presence` or `data-failure-badge` selectors.

## Later Carry-Over

Task 9 should extend this into broader sidebar collision regression if it adds screenshot/e2e coverage across drag-over and adjacent DM fault rows.
