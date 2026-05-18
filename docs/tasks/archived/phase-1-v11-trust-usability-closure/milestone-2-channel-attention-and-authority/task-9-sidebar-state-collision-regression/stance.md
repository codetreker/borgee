# PM Stance: Sidebar State Collision Regression

## Scope Position

This task closes the private indicator work with regression proof. It does not reopen the visual treatment and does not add new sidebar states.

## Stances

1. Prove collision boundaries, do not redesign them.
   - Constraint: no production UI movement unless the regression reveals a real defect.
   - Reviewer signal: the PR adds focused regression tests and docs only.

2. Attention signals stay independent.
   - Constraint: private identity must not hide unread, selected, pinned, hover, or drag-over state.
   - Reviewer signal: coverage includes Private + unread + selected + pinned + drag-over in one row.

3. Higher-priority override stays intact.
   - Constraint: archived state overrides private/public markers and suppresses unread.
   - Reviewer signal: regression coverage includes archived private + unread.

4. DM status semantics stay DM-only.
   - Constraint: channel private rows must not import `PresenceDot` or emit `data-presence` / `data-failure-badge`.
   - Reviewer signal: source-level regression guards the channel-row component.

5. Authority remains server-side.
   - Constraint: no ACL, membership, visibility, fanout, or management behavior changes.
   - Reviewer signal: changed files stay within tests and docs for this regression slice.
