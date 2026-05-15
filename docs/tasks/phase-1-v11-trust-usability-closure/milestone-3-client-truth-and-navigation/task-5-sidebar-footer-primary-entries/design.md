# Implementation Design: Sidebar Footer Primary Entries

## Scope And Inputs

Owned task path:

- `docs/tasks/phase-1-v11-trust-usability-closure/milestone-3-client-truth-and-navigation/task-5-sidebar-footer-primary-entries/`

Owned implementation areas:

- `packages/client/src/components/Sidebar.tsx`
- `packages/client/src/index.css`
- Focused Sidebar/app shell tests under `packages/client/src/__tests__/`
- Current docs under `docs/current/client/`

Do not edit production files for M3 Task1 ArtifactComments, M3 Task3 Settings Permissions, M3 Task7 final Helper/Remote Nodes placement internals, or M2 Task7 inventory-only visual-state work.

## Chosen Approach

Use a compact primary footer row plus a secondary overflow menu.

Primary row:

- Avatar/current-user identity signal.
- Agents, Workspaces, and Settings as first-class footer actions when their callbacks and role gates permit them.
- A More (`⋯`) overflow control only when secondary actions are available.

Secondary menu:

- Invitations, Remote Nodes, Helper Status, and Logout remain reachable under More.
- Pending invitation count remains attached to the Invitations action inside the secondary menu.
- Logout keeps the existing `logout()` API call and `onLogout` callback behavior.

Alternative 1: remove secondary entries from Sidebar entirely. Rejected because Task 6 and Task 7 have not yet provided replacement access paths, and the task contract says removed/secondary entries must remain reachable when in scope.

Alternative 2: keep all existing buttons but visually de-emphasize some. Rejected because the acceptance target is a smaller primary entry set, not merely lower contrast for the same eight exposed controls.

Alternative 3: build the avatar account panel now and move logout into it. Rejected because Task 6 explicitly owns avatar account panel/logout behavior.

## Data And State Flow

No new durable data model, API contract, or app-shell state value is required.

The existing sidebar props remain the navigation contract:

```text
Sidebar footer click
  -> existing callback, e.g. onWorkspacesOpen/onSettingsOpen
  -> App.requestMainView(target)
  -> existing unsaved-change guard
  -> existing mainView sidepane selection
```

The existing pending invitation refresh stays in `Sidebar.tsx`. The only behavioral change is presentation: the badge moves from a primary bell button to the secondary Invitations menu item.

The secondary menu uses local component state and closes on action selection or outside click. It should reuse the existing document `mousedown` style already used for the add-channel dropdown.

## Role And Authority Rules

- Member sessions may see primary Agents, Workspaces, Settings, and secondary Invitations, Remote Nodes, Helper Status, Logout depending on supplied callbacks.
- Agent sessions must not see owner-only controls. Keep the existing `state.currentUser.role !== 'agent'` checks for Agents, Invitations, Remote Nodes, Helper Status, and Settings. Logout remains available as an account/session action.
- Workspaces remains callback-gated and may be visible to agent sessions if the existing app shell supplies it.

## UI And CSS

- Keep icon-button style and existing emoji/icon convention.
- Give the footer stable, compact groups: `.sidebar-footer-primary`, `.sidebar-footer-primary-actions`, and `.sidebar-footer-secondary-menu`.
- Keep button dimensions stable; do not introduce visible text labels into the footer row.
- The secondary menu may use text rows because it opens only on demand and needs clear labels for less-frequent actions.

## Edge Cases

- No secondary callbacks: do not render the More toggle.
- Pending invitation count > 99: preserve `99+` behavior.
- Logout failure: preserve current reload fallback.
- Current user is missing: preserve current behavior and render no footer.
- Agent role: no owner-only primary or secondary controls; logout remains reachable.
- Repeated click on More: toggles menu; selecting an action closes the menu.

## Test Plan

Use TDD before production changes:

1. Add a focused Sidebar footer test that renders a member with all callbacks and asserts the primary action group contains avatar, Agents, Workspaces, Settings, and More only; Invitations, Remote Nodes, Helper Status, and Logout are absent from the primary group.
2. Add a secondary menu test that opens More and asserts Invitations, Remote Nodes, Helper Status, and Logout are reachable; clicking each action calls its existing callback/API path.
3. Add a pending invitation badge test for the secondary Invitations action.
4. Add an agent role-gating test asserting owner-only controls do not appear in primary or secondary footer controls.

After implementation, run focused Sidebar tests, client typecheck, `git diff --check`, and PR-lint-relevant checks.

## Current Docs

Update current docs to say:

- The sidebar footer primary row is limited to avatar/account identity, Agents, Workspaces, and Settings.
- Invitations, Remote Nodes, Helper Status, and Logout are secondary overflow actions until later account/runtime placement tasks replace them.
- This is not the Task 6 account panel and not the Task 7 final Helper/Remote Nodes placement.

Likely docs:

- `docs/current/client/ui-map.md`
- `docs/current/client/ui/main-desktop.md`
- `docs/current/client/ui/sidepane.md`

## Review Checklist

- [x] Architect: no new app-shell state or route system; existing callbacks and guards remain authoritative.
- [x] PM: primary footer is calmer without deleting reachable secondary surfaces.
- [x] QA: tests cover primary set, secondary reachability, badge carry-over, logout, and role gates.
- [x] Security: role gates and Helper/Remote Agent rail separation remain intact; no new privacy/compliance product surface.
