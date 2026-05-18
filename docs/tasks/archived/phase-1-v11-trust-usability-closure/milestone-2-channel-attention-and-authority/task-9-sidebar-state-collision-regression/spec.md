# Spec Brief: Sidebar State Collision Regression

## 0. Constraints

Task contract: prove the private indicator visual treatment does not collide with other sidebar state signals. This is a regression/evidence slice, not a new visual treatment.

Dependency base:

- Task 8 private indicator visual treatment is satisfied by PR #952 (`9659ce14f4f36778972aee58cccd87f08a2c0b4d`).
- Task 9 runs after Task 8 and does not depend on channel authority, mention fanout, channel management, or Helper/OpenClaw work.

Blueprint anchors:

- `CH-1` (`docs/blueprint/next/migration-analysis.md` section 4.3): private channel UI must remain truthful and not collide with attention or status signals.
- `PS-1` (`docs/blueprint/next/migration-analysis.md` section 6.1): no new privacy/compliance product surface is added by this proof.

## 1. Product Slice

Reviewers need a durable regression proof that private-channel rows keep their private marker separate from unread, selected, hover, drag-over, pinned, archived, preview, and DM-only presence/fault signals.

## 2. Implementation Scope

- Add focused client regression coverage for the combined private/unread/selected/pinned/drag-over row state.
- Lock the archived override and public preview boundary so private state does not falsely imply unread activity or public discovery behavior.
- Add a source-level guard that `SortableChannelItem` does not import or emit DM-only presence/fault semantics.
- Record the regression suite and current channel row anchors in task and current docs.

## 3. Out Of Scope

- No visual redesign, markup restyling, CSS movement, channel sorting behavior, DnD behavior, notification behavior, server/API behavior, ACL, membership, fanout, or channel management changes.
- No channel-level presence/fault model.
- No sidebar footer, account/avatar, Helper, Remote Nodes, or Milestone 3 IA changes.
