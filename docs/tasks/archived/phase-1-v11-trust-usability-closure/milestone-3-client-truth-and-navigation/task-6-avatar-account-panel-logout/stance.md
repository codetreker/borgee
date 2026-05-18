# Stance: Avatar Account Panel Logout

## Decision

Implement the avatar as the account entry and move Logout into a compact account panel.

The account panel v1 contains only:

- Current account summary from existing `currentUser` state.
- Logout using the existing logout API/callback path.

## Rationale

Task5 already created the calmer footer primary set. Leaving logout in More after Task6 would keep account/session behavior split away from the account identity signal. Moving logout into the avatar panel matches `IA-1` while keeping the surface small and recoverable.

## Non-Goals

- No account settings product expansion.
- No privacy/compliance promise UI.
- No Helper/Remote Nodes placement change.
- No ArtifactComments, Settings Permissions, or channel authority edits.

## Dependency Decision

Unblocked. Task6 depends on `task-5-sidebar-footer-primary-entries`, and `origin/main` contains Task5 merge `47dc6805abaf98fffcd727ec5917b641367f2eeb`. Task1 PR #946 remains open, but Task6 does not depend on Task1, Task2, or Task4 by the canonical task document.
