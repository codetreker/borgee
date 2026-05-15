# Acceptance: Private Indicator State Inventory

## Source Alignment

- Task: `task-7-private-indicator-state-inventory`
- Milestone: `phase-1-v11-trust-usability-closure/milestone-2-channel-attention-and-authority`
- Blueprint anchors: `CH-1` (`migration-analysis.md` section 4.3) and `PS-1` (`migration-analysis.md` section 6.1)
- Dependency: explicit parallel UI slot named by Teamlead; Task 8 remains blocked on this inventory.

## Segment A: Source Anchor Inventory

Acceptance checks:

- [x] The inventory names channel row, grouped row, preview row, DM row, and CSS anchors relevant to private/sidebar state collisions.
- [x] The inventory distinguishes channel rows from DM rows using current `data-kind="channel"` and `data-kind="dm"` anchors.

Evidence:

- `state-matrix.md` Source Anchors table records `ChannelList`, `ChannelGroupComponent`, `SortableChannelItem`, `Sidebar` DM rows, and CSS class anchors.

## Segment B: State Matrix

Acceptance checks:

- [x] The matrix covers private, unread, fault, presence, selection, hover, drag-over, pinned, preview, and archived states.
- [x] The matrix records which visual area each state occupies.

Evidence:

- `state-matrix.md` State Matrix table covers the required states and names leading-slot, trailing-slot, whole-row, absolute-overlay, and DM-only areas.

## Segment C: Collision Boundaries

Acceptance checks:

- [x] Collision boundaries are concrete enough for Task 8 to implement visual treatment without rediscovering current anchors.
- [x] Regression seeds identify the minimum later proof cases for private/unread/fault/presence coexistence.

Evidence:

- `state-matrix.md` Collision Boundaries and Regression Seeds sections list Task 8/9 carry-over cases.

## Segment D: Current-Doc Note

Acceptance checks:

- [x] `docs/current` records current channel-row state anchors without promising a future redesign.
- [x] The current-doc note names the known current collision risk as a gap for later treatment.

Evidence:

- `docs/current/client/ui/channel-sort-groups.md` includes a Current Channel Row State Anchors section.

## Segment E: Scope Lock

Acceptance checks:

- [x] No production sidebar/footer component or CSS files are edited.
- [x] No channel authority, ACL, membership, or privacy/compliance product behavior changes are included.

Evidence:

- `progress.md` Final Verification records the production-edit negative check and `git diff --check` result.
