# Implementation Design: Avatar Account Panel Logout

## Scope And Inputs

Owned task path:

- `docs/tasks/phase-1-v11-trust-usability-closure/milestone-3-client-truth-and-navigation/task-6-avatar-account-panel-logout/`

Owned implementation areas:

- `packages/client/src/components/Sidebar.tsx`
- `packages/client/src/index.css`
- Focused Sidebar/app-shell tests under `packages/client/src/__tests__/`
- Current docs under `docs/current/client/`

Do not edit production files for ArtifactComments, Settings PermissionsView reachability, channel authority, NodeManager, HelperStatusPanel, or Helper/Remote Nodes final placement.

## Chosen Approach

Use a local account-panel dropdown owned by `Sidebar.tsx`.

Avatar account trigger:

- Convert the current avatar from a static `div` to an icon-style button with the same compact footprint.
- Keep the avatar as the first primary footer entry.
- Toggle a local account panel on click.

Account panel:

- Render above the footer, aligned with the avatar/account trigger area.
- Show `Account`, the current user's display name, and role label.
- Provide a single `Logout` action that calls the existing `handleLogout` path.

More overflow:

- Remove Logout from the secondary More menu.
- Keep Invitations, Remote Nodes, and Helper Status in More when callbacks and role gates permit them.
- Render More only when those secondary owner actions exist; logout alone no longer creates More.

Alternative 1: keep Logout in More and duplicate it in the account panel. Rejected because Task6 acceptance says logout moves into the avatar panel, and duplicate logout would leave account/session behavior split across two IA locations.

Alternative 2: add account settings and profile controls now. Rejected by the task and blueprint: account panel v1 is account summary plus logout only.

Alternative 3: create a new app-shell view mode for Account. Rejected because the panel is local account/session chrome, not a global sidepane surface.

## Data And State Flow

No new durable data, API, route, or app-shell main view is required.

```text
Avatar click
  -> Sidebar local showAccountPanel state
  -> account panel renders currentUser summary

Account panel Logout click
  -> existing handleLogout()
  -> logout() API
  -> onLogout callback or reload fallback
```

The account panel closes on outside click and before invoking logout. More-menu outside-click behavior remains separate.

## Role And Authority Rules

- Member sessions can open the account panel and log out.
- Agent sessions can open the account panel and log out.
- The account panel must not expose owner-only navigation or settings controls.
- Existing owner-only gates for Agents, Invitations, Remote Nodes, Helper Status, and Settings remain unchanged.

## UI And CSS

- Reuse compact icon-button/avatar sizing and existing sidebar palette variables.
- Keep the panel small, menu-like, and anchored above the footer; do not create a full-page account surface.
- The panel can use text because it is an on-demand menu.
- Keep stable dimensions so the footer row does not shift when the panel opens.

## Edge Cases

- Missing current user: preserve current behavior and render no footer.
- Long display name: truncate in the account panel rather than widening the sidebar.
- Logout failure: preserve the existing reload fallback.
- Agent role: account panel still renders; owner-only secondary actions remain hidden.
- No owner secondary callbacks: More does not render just for logout.

## Test Plan

Use TDD before production changes:

1. Add a Sidebar account panel test that clicks the avatar trigger and expects account summary plus Logout.
2. Add a logout test that clicks account-panel Logout and verifies `logout()` plus `onLogout`.
3. Update the secondary More test so More no longer contains Logout and still preserves Invitations, Remote Nodes, and Helper Status.
4. Add an agent-session test proving agent users can log out through the account panel without seeing owner-only More actions.

After implementation, run focused Sidebar tests, adjacent app-shell tests, client typecheck, client build, full client tests, and `git diff --check`.

## Current Docs

Update current docs to say:

- Avatar is now the account panel entry.
- Logout lives in the account panel, not More.
- Invitations, Remote Nodes, and Helper Status remain secondary overflow entries until Task7.
- This is not account settings expansion and not a privacy/compliance product surface.

Likely docs:

- `docs/current/client/ui-map.md`
- `docs/current/client/app-shell-state.md`
- `docs/current/client/ui/main-desktop.md`
- `docs/current/client/ui/sidepane.md`

## Review Checklist

- [x] Architect: no new app-shell state or route system; local Sidebar state is enough.
- [x] PM: account behavior is discoverable from avatar and logout is not duplicated in More.
- [x] QA: tests cover account panel open, logout path, More-menu removal, and agent access.
- [x] Security: no owner-only action is added to account panel; no privacy/compliance product surface is introduced.
