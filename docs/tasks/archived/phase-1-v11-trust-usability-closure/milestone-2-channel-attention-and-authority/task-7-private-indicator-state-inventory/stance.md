# PM Stance: Private Indicator State Inventory

## Scope Position

This task turns the current sidebar/channel-row state surface into a concrete inventory for the later visual-treatment task. Its product value is reducing ambiguity before UI edits, not shipping a visual change.

## Stances

1. Inventory before treatment.
   - Constraint: this task records the current row states and collision boundaries; Task 8 owns the actual private-indicator visual treatment.
   - Reviewer signal: the PR changes docs and task evidence only, with no production sidebar or footer edits.

2. Private state must not hide higher-priority signals.
   - Constraint: private-channel indication must coexist with unread count, selected row state, hover state, drag-over insertion state, archived/preview/pinned accessories, and any DM-only presence/fault dots.
   - Reviewer signal: `state-matrix.md` names where each state currently renders and which states compete for the same visual area.

3. Current behavior stays current-doc truth.
   - Constraint: `docs/current` may describe today's anchors and gaps, but it must not promise the future quieter private indicator.
   - Reviewer signal: current-doc notes are phrased as current behavior plus known collision risk.

4. No authority or privacy boundary change hides behind UI language.
   - Constraint: this task cannot alter ACL, channel membership, visibility authority, or privacy/compliance product scope.
   - Reviewer signal: matrix rows refer to display anchors only and keep server/channel authority out of scope.

5. DM presence/fault is separate from channel private state.
   - Constraint: agent DM presence and fault dots live in the DM rail; private channel indicators live in channel rows. A future channel-private treatment must not borrow DM presence semantics.
   - Reviewer signal: inventory separates `data-kind="channel"` rows from `data-kind="dm"` rows.

6. M3 sidebar/footer IA stays untouched.
   - Constraint: footer primary-entry cleanup, avatar/account entry, and Helper/Remote Nodes placement remain owned by M3 tasks.
   - Reviewer signal: this PR does not edit sidebar footer production code or claim footer IA acceptance.

## Out-Of-Scope Locks

- No production sidebar/footer component or CSS edits.
- No private indicator redesign.
- No channel authority, ACL, ownership, leave/delete/archive, or membership action change.
- No broad visual redesign or pixel-art restyling.
- No new user-facing privacy/compliance product surface.
