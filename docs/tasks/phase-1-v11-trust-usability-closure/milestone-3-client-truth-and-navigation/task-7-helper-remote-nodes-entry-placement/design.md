# Design

## Chosen IA Location

Task7 places runtime entry points inside user Settings under a new local `runtime` tab. This keeps repeated shell entries small after Task5/Task6 while still giving Remote Nodes and Helper Status one coherent runtime-management home.

## Production Changes

- `packages/client/src/components/Settings/SettingsPage.tsx`
  - Extend local settings tab state with `runtime`.
  - Render a Runtime tab with two separate buttons: `data-runtime-entry="remote-nodes"` and `data-runtime-entry="helper-status"`.
  - Mark the entries with separate rail metadata: `data-authority-rail="remote-agent"` and `data-authority-rail="helper-actuator"`.
  - Accept optional callbacks so the app shell can open the existing sidepanes without introducing routes.
- `packages/client/src/App.tsx`
  - Stop passing Remote Nodes and Helper Status callbacks to `Sidebar`.
  - Pass those callbacks to `SettingsPage`, using the existing `requestMainView` guard path.
- `packages/client/src/components/Sidebar.tsx`
  - Keep the footer overflow for Invitations only.
  - Leave avatar/account, Agents, Workspaces, Settings, and More behavior otherwise unchanged.
- `packages/client/src/index.css`
  - Add compact Settings runtime entry styling without changing the underlying feature panels.

## Rail Separation

The UI uses separate entries and separate test selectors for Remote Agent and Helper actuator rails. No production code moves data between Remote Nodes and Helper Status, and no API/client functions for credentials, grants, helper enrollments, remote node tokens, or enforcement checks are changed.

## Testing Strategy

- Add a SettingsPage regression test that fails until Runtime contains separate Remote Nodes and Helper Status entries and invokes their separate callbacks.
- Update the Sidebar footer regression test so it fails until Remote Nodes and Helper Status leave the footer overflow.
- Re-run focused jsdom coverage, client typecheck, lint, build, full client tests, and diff hygiene before PR.
