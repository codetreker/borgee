# Acceptance: ACL Forbidden-State UX

## Source Alignment

- Task: `task-2-acl-forbidden-state-ux`
- Milestone: `phase-1-v11-trust-usability-closure/milestone-3-client-truth-and-navigation`
- Dependency: Task1 ArtifactComments production mount, merged in PR #946.
- Blueprint anchors: `CT-1` and `PS-1`.

## Checks

Acceptance checks:

- `ArtifactPanel` clears already-rendered artifact content and shows a local forbidden state when an authoritative reload is denied.
- `ArtifactComments` shows loading before successful list authorization, shows empty only after successful empty list, and shows a non-leaky forbidden state on denied list or post.
- Settings `PermissionsView` shows a non-leaky forbidden state for denied capability visibility while preserving existing empty/error/loading/capability states.
- Current docs record the local forbidden-state behavior.

Negative checks:

- No denied response body is rendered to users.
- No client-side state grants access or weakens backend ACL authority.
- No Task4 e2e reverse-proof work, sidebar/footer work, avatar/account work, Helper/Remote Nodes placement, or new privacy/compliance product surface is started.

## Required Evidence Before PR

- RED: focused tests fail before implementation because forbidden/loading anchors are absent.
- GREEN: focused tests pass for ArtifactPanel, ArtifactComments, PermissionsView, and SettingsPage.
- Client typecheck and build pass.
- `git diff --check` passes.
