# Acceptance: Private Indicator Visual Treatment

## Source Alignment

- Task: `task-8-private-indicator-visual-treatment`
- Milestone: `phase-1-v11-trust-usability-closure/milestone-2-channel-attention-and-authority`
- Blueprint anchor: `CH-1` (`docs/blueprint/next/migration-analysis.md` section 4.3)
- Dependency: Task 7 state inventory merged in PR #945.

## Checks

- [x] Private channel rows remain identifiable through a leading-slot marker.
- [x] The private marker is quieter than the previous colorful lock emoji.
- [x] Unread badges remain visible and in the trailing unread slot.
- [x] Pinned indicators remain visible and separate from private state.
- [x] Selected and hover row states keep the private marker legible without replacing row state styling.
- [x] Archived state overrides private/public markers and suppresses unread.
- [x] Channel rows do not reuse DM-only presence or failure selectors.
- [x] No ACL, membership, channel management, fanout, API, server, footer/sidebar IA, or broad visual redesign is included.

## Evidence

- `packages/client/src/__tests__/SortableChannelItem-private-indicator.test.tsx` covers the state combinations and selector boundaries.
- `docs/current/client/ui/channel-sort-groups.md` records the current row anchors after the visual treatment.
