# PM Stance: Sidebar Footer Primary Entries

## Scope Position

This task is the first sidebar/account IA slice. It should reduce footer clutter now without pretending later account-panel and runtime-placement decisions are already finished.

## Stances

1. The primary footer set is small.
   - Anchors: `IA-1` section 7.3; task contract scope.
   - Constraint: first-class footer controls should be avatar/account identity, Agents, Workspaces, and Settings.
   - Blacklist grep: primary footer rendering `Remote Nodes`, `Helper Status`, `Logout`, and `Agent 邀请` as equal siblings of the primary entries.

2. Secondary surfaces remain reachable until their owner tasks replace them.
   - Anchors: task contract scope; Task 6 and Task 7 dependencies.
   - Constraint: Invitations, Remote Nodes, Helper Status, and Logout can be demoted, but not deleted from navigation in this task.
   - Blacklist grep: removed callbacks with no replacement path for `onInvitationsOpen`, `onRemoteNodesOpen`, `onHelperStatusOpen`, or `onLogout`.

3. Avatar is identity/account affordance, not the account panel yet.
   - Anchors: `IA-1` section 7.3; Task 6 dependency.
   - Constraint: this task may keep avatar visible as the account identity signal, but must not build account summary/settings/logout panel behavior.
   - Blacklist grep: `account panel`, `account settings`, `profile editor` in production code introduced by this task.

4. Helper and Remote Agent rails stay separate.
   - Anchors: `PS-1` section 6.1; `IA-1` section 7.3.
   - Constraint: any secondary runtime entry labels must not imply shared credentials, grants, status, or enforcement.
   - Blacklist grep: `shared rail`, `shared credential`, `Remote Agent Helper`, `Helper Remote Agent`.

5. Agent sessions do not gain owner controls.
   - Anchors: existing Sidebar role checks; `PS-1` capability boundaries.
   - Constraint: demoting controls into overflow must preserve the same `state.currentUser.role !== 'agent'` gates for owner sidepanes. Logout remains an account/session action and stays reachable.
   - Blacklist grep: owner-only callbacks rendered for `role === 'agent'`.

6. The change is app-shell IA, not feature-surface implementation.
   - Anchors: milestone acceptance boundary.
   - Constraint: do not change ArtifactComments, Settings PermissionsView, Remote Nodes internals, Helper status internals, channel sort, or notification semantics.
   - Blacklist grep: edits to `ArtifactComments`, `PermissionsView`, `NodeManager`, or `HelperStatusPanel` production files unless a hard dependency is escalated.

## Out-Of-Scope Locks

- No account panel finalization or logout relocation behind avatar.
- No Helper/Remote Nodes final placement.
- No privacy/compliance product surface.
- No broad sidebar redesign or new route system.
