# PM Stance: ArtifactComments Production Mount

## Scope Position

This task closes a truthfulness gap: the product already has artifact comment components and REST contracts, so the production artifact panel should expose them for the active artifact instead of leaving them as unreachable implementation inventory.

## Stances

1. Production reachability beats hidden capability.
   - Constraint: an authorized user with an active artifact can reach ArtifactComments in the production ArtifactPanel.
   - Reviewer signal: the mounted component is visible from the normal artifact flow, not only isolated tests or sketches.

2. REST remains the authority for comment content.
   - Constraint: the comment surface calls `listArtifactComments`; realtime frames remain wake-up signals and are not rendered as authoritative bodies.
   - Reviewer signal: the test proves ArtifactPanel mounting triggers the comment list request for the active artifact id.

3. Use available comment-series contracts, but do not fake missing ones.
   - Constraint: `ArtifactCommentBody` is safe to use because the list API returns `body`; search/thread/history surfaces stay out until the production parent has the required virtual channel id, reply state, or history trigger state.
   - Reviewer signal: this task does not invent client-only IDs or placeholder data to make deeper comment features appear wired.

4. Privacy scope stays narrow.
   - Constraint: no user-facing privacy/compliance product surface is introduced, and no protected content is exposed before server ACL succeeds.

## Out-Of-Scope Locks

- No Settings Permissions task work.
- No sidebar/footer or account panel work.
- No Helper/Remote Nodes IA movement.
- No channel authority changes.
- No broad e2e reverse-proof task.
