# PM Stance: Private Indicator Visual Treatment

## Scope Position

This task ships the visual treatment that Task 7 prepared. It is a channel-row UI slice, not an authority or channel-management slice.

## Stances

1. Private stays identifiable, but quieter.
   - Constraint: replace the colorful lock emoji with a muted leading-slot marker.
   - Reviewer signal: private rows render `data-private="true"` and a `.channel-private-indicator` marker.

2. Higher-priority attention states win.
   - Constraint: unread, pinned, archived, selected, hover, drag-over, and DM-only fault/presence states stay visible and separate.
   - Reviewer signal: regression coverage includes private + unread + selected + pinned and archived override cases.

3. Authority stays server-side.
   - Constraint: no ACL, membership, visibility, management, or fanout behavior changes.
   - Reviewer signal: only client row rendering/CSS/tests and docs are touched.

4. DM status semantics stay DM-only.
   - Constraint: channel private state must not reuse `data-presence` or failure badge semantics.
   - Reviewer signal: tests assert channel rows do not emit DM presence/fault selectors.

5. Footer/sidebar IA stays out of scope.
   - Constraint: no sidebar footer, avatar/account, Helper, or Remote Nodes placement edits.
   - Reviewer signal: this PR only edits channel-row files under the channel rail.
