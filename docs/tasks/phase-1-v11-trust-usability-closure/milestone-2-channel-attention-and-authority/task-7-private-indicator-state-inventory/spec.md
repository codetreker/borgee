# Spec Brief: Private Indicator State Inventory

## 0. Constraints

Task contract: produce a concrete inventory for the current channel/sidebar row states that private indicators must coexist with before Task 8 changes the visual treatment. This is a scouting task. It must not change production sidebar/footer code, CSS, authority, ACL, channel management behavior, or the M3 Task5 footer IA surface.

Blueprint anchors:

- `CH-1` (`docs/blueprint/next/migration-analysis.md` section 4.3): private channel lock UI must not collide with unread, fault, or presence badges; channel management scope remains membership/ownership and allowed actions.
- `PS-1` (`docs/blueprint/next/migration-analysis.md` section 6.1): UI movement cannot weaken privacy/security boundaries or invent user-facing privacy/compliance product scope.

Dependency base:

- The task is valid under the explicit parallel UI slot named by Teamlead. It does not depend on channel management production edits because it records current sidebar/channel row anchors only.
- Task 8 consumes this inventory before changing private indicator visuals. M3 Task5 owns footer primary-entry edits; this task must not touch those production surfaces.

## 1. Segmentation

Segment A: Source anchor inventory.
Record the current component, DOM, and CSS anchors for channel rows, grouped channel rows, preview rows, DM rows, and footer-adjacent rows relevant to private/unread/fault/presence/selection/hover collisions.

Segment B: State matrix.
Produce a matrix covering private, unread, fault, presence, selection, hover, drag-over, pinned, preview, and archived combinations that Task 8 must preserve or deliberately keep out of scope.

Segment C: Collision boundaries.
Name which states occupy the leading icon slot, row background, trailing accessory area, absolute overlay area, or DM-only presence area so the next visual task can avoid hiding higher-priority state.

Segment D: Current-doc note.
Update `docs/current` with a current-behavior note that describes existing channel row state anchors and the known visual-collision risk without promising future UI changes.

Segment E: Scope lock.
Record verification evidence that this task made docs/current and task-doc changes only, with no production sidebar/footer edits.

## 2. Carry-Over

Carry into Task 8:

- Any replacement private-channel marker must stay compatible with `.unread-badge`, `.channel-item-active`, `.channel-item:hover`, `.drop-indicator`, archived/preview/pinned accessories, and DM-only presence/fault dots.
- Task 8 should add or update regression proof for the specific collision cases named in `state-matrix.md`.
- If Task 8 adds a new channel-level fault or presence marker, that is scope expansion unless the milestone owner explicitly opens it; the current inventory shows those markers are DM-only today.

## 3. Reverse Checks

- If this PR edits `packages/client/src/components/Sidebar.tsx`, `ChannelList.tsx`, `SortableChannelItem.tsx`, `ChannelGroupComponent.tsx`, `GroupHeader.tsx`, or `packages/client/src/index.css`, it has become production UI work and violates this task.
- If this PR changes channel authority, ACL, membership, mention delivery, channel management, footer IA, or account/sidebar primary entries, it has drifted out of scope.
- If the inventory treats a future private indicator redesign as current behavior, it violates `docs/current` truthfulness.
- If the matrix omits unread, selection, hover, or DM-only presence/fault states, Task 8 lacks the concrete collision inputs this task is supposed to provide.

## 4. Out Of Scope

- No production React, CSS, API, server, migration, or test-code edits.
- No private-indicator visual redesign.
- No footer/sidebar primary-entry cleanup; M3 Task5 owns that.
- No channel management action work, ACL change, owner leave behavior, or channel membership authority change.
- No new user-facing privacy/compliance promise or privacy dashboard.
