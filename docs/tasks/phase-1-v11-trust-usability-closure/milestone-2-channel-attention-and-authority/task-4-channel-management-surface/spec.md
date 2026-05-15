# Spec Brief: Channel Management Surface

## 0. Constraints

Task contract: add the first user-facing channel management surface so users can inspect channels they created and channels they joined. This task owns the route/entry point, basic listing, and client/API tests needed to prove the listing uses the existing server-authoritative channel list. It must not implement leave/delete/archive/owner-transfer action rules, channel authority mutation checks, notification/collapse/sort rewrites, private-channel visual treatment, or sidebar/footer IA production edits.

Blueprint anchors:

- `CH-1` (`migration-analysis.md` section 4.3): channel management v1 focuses on membership, ownership, and allowed actions; notification, collapse, and sort rewrites stay out unless explicitly reopened.
- `CH-1` (`docs/blueprint/next/README.md` section 2.4): channel management may live in Settings or an in-channel settings/index surface if membership/ownership authority stays clear.
- `PS-1` (`migration-analysis.md` section 6.1): preserve existing privacy/security boundaries and do not add new user-facing privacy/compliance product scope.

Dependency base:

- Canonical Milestone 2 start is sufficient for this task. It is independent from mention policy work if files stay disjoint.
- Task 5 owns allowed action rules. Task 6 owns server/client enforcement for actions. Task 7 owns private indicator state inventory. Milestone 3 owns sidebar/footer IA production changes.

## 1. Segmentation

Segment A: settings entry.
Add a reachable channel management entry inside the existing user Settings surface. Do not add a new sidebar/footer primary entry, and do not change sidebar private/unread/fault/presence indicators.

Segment B: channel management listing.
Show two distinct channel groups: channels created by the current user and channels joined by the current user but created by someone else. The surface may show read-only metadata such as name, visibility, topic, and member count. It must not show destructive or membership-changing actions.

Segment C: server-authoritative data source.
Use the existing channel list API/client state as the source of truth for `created_by`, `is_member`, visibility, topic, and member count. Client code may classify the list for display, but must not infer authority beyond the returned channel fields.

Segment D: empty, loading, and privacy-safe states.
Render a clear empty state when no created or joined channels exist. The listing must not expose hidden channel bodies, private message content, or names not already present in the authorized channel list response.

Segment E: tests and docs/current sync.
Add focused API/client tests for the classification and settings entry behavior. Update current docs for the implemented Settings channel-management surface and leave later action-rule/authority work as known follow-up scope.

## 2. Carry-Over

Carry into later tasks, but do not solve here:

- Task 5 decides visible leave/delete/archive/owner-transfer availability, including the owner cannot leave self-created/owned channels rule.
- Task 6 enforces server/client authority checks for any management actions exposed later.
- Task 7 inventories private/unread/fault/presence sidebar state collisions and DOM/CSS anchors.
- Milestone 3 decides sidebar/footer account/entry IA production changes.

## 3. Reverse Checks

- If this task adds leave, delete, archive, owner-transfer, notification, collapse, sort, or private-indicator behavior, scope has drifted.
- If this task adds a sidebar/footer primary entry, it conflicts with Milestone 3 sidebar/footer ownership.
- If the list uses client-only guesses to decide ownership or membership instead of channel fields from the existing API/state, authority is unclear.
- If the surface exposes protected content beyond names/metadata already in the authorized channel list, it violates `PS-1`.

## 4. Out Of Scope

- Channel leave/delete/archive/owner-transfer buttons or policy.
- Server mutation endpoints or new action authorization checks.
- Notification preference, collapse, sorting, pinning, grouping, or broad channel settings rewrites.
- Private-channel icon/sidebar treatment or state-collision inventory.
- Sidebar/footer IA production edits, account-panel work, or Remote Nodes/Helper entry movement.
