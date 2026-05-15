# Milestone 3: Client Truth And Navigation

## Capability Goal

Make selected production surfaces reachable and truthful, show forbidden states without leaking protected data, and simplify account/sidebar navigation without expanding privacy/compliance product scope.

## Remapped Prior Structure

This milestone replaces the old Phase 3 milestone split:

- `phase-3-client-truth-navigation/milestone-1-production-surface-truthfulness`
- `phase-3-client-truth-navigation/milestone-2-sidebar-account-entry`

Old Phase 3 was an execution slot, not a prerequisite or integration boundary. Those folders remain detailed task homes; this file is the authoritative coarse milestone grouping.

## Acceptance Boundary

Accepted by this milestone:

- ArtifactComments production mount inside the claimed production surface.
- ACL forbidden-state UX that is visible and non-leaky.
- Settings `PermissionsView` reachability without turning gh#654 into a privacy/compliance product expansion.
- Production-surface e2e reverse proof for selected surfaces.
- Sidebar footer primary entries, avatar account panel/logout, and Helper/Remote Nodes placement without merging rails.

Rejected by this milestone:

- Broad e2e platform rewrite, mobile coverage expansion, modal a11y sweep, broad visual redesign, or account settings expansion unless separately scoped.
- User-facing privacy/compliance product promises.
- Helper/Remote Agent credential, grant, or enforcement rail merge.

## Task Index

| Task | Status | Prior path | Depends on | Parallel? |
|---|---|---|---|---|
| ArtifactComments production mount | PLANNED | `phase-3-client-truth-navigation/milestone-1-production-surface-truthfulness/task-1-artifactcomments-production-mount` | Milestone start | no |
| ACL forbidden-state UX | PLANNED | `phase-3-client-truth-navigation/milestone-1-production-surface-truthfulness/task-2-acl-forbidden-state-ux` | ArtifactComments mount | yes, after mount |
| Settings PermissionsView reachability | PLANNED | `phase-3-client-truth-navigation/milestone-1-production-surface-truthfulness/task-3-security-permission-surface-reachability` | Milestone start | yes, if shell/settings files are separable |
| Production-surface e2e reverse proof | PLANNED | `phase-3-client-truth-navigation/milestone-1-production-surface-truthfulness/task-4-production-surface-e2e-reverse-proof` | forbidden-state UX and PermissionsView reachability | no |
| Sidebar footer primary entries | PLANNED | `phase-3-client-truth-navigation/milestone-2-sidebar-account-entry/task-1-sidebar-footer-primary-entries` | Milestone start | yes, if shell files do not conflict with truthfulness work |
| Avatar account panel/logout | PLANNED | `phase-3-client-truth-navigation/milestone-2-sidebar-account-entry/task-2-avatar-account-panel-logout` | sidebar footer primary entries | yes, after footer entry budget |
| Helper/Remote Nodes placement | PLANNED | `phase-3-client-truth-navigation/milestone-2-sidebar-account-entry/task-3-helper-remote-nodes-entry-placement` | sidebar footer primary entries | yes, after footer entry budget |

## Exit Gates

- ArtifactComments/ArtifactPanel and Settings `PermissionsView` are reachable through production UI when claimed.
- Forbidden states do not leak private channel, artifact, message, file, or body content before authorization succeeds.
- Sidebar/account IA movement does not merge Helper and Remote Agent credentials, grants, or enforcement rails.
- No new user-facing privacy/compliance product surface is introduced through gh#654.
