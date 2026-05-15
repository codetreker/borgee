# Milestone 3: Client Truth And Navigation

## Capability Goal

Make selected production surfaces reachable and truthful, show forbidden states without leaking protected data, and simplify account/sidebar navigation without expanding privacy/compliance product scope.

## Canonical Task Homes

This milestone is the only execution home for the client truth and navigation work. The former client-truth/navigation phase folder was never executed and has been removed to avoid presenting it as an available Phase.

The collapsed planning content is represented here as canonical tasks: production surface truthfulness and sidebar/account navigation both belong inside this one milestone.

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

| Task | Status | Canonical path | Depends on | Parallel? |
|---|---|---|---|---|
| ArtifactComments production mount | ACCEPTING | `task-1-artifactcomments-production-mount` | Milestone start | no |
| ACL forbidden-state UX | PLANNED | `task-2-acl-forbidden-state-ux` | ArtifactComments mount | yes, after mount |
| Settings PermissionsView reachability | ACCEPTING | `task-3-security-permission-surface-reachability` | Milestone start | yes, if shell/settings files are separable |
| Production-surface e2e reverse proof | PLANNED | `task-4-production-surface-e2e-reverse-proof` | forbidden-state UX and PermissionsView reachability | no |
| Sidebar footer primary entries | PLANNED | `task-5-sidebar-footer-primary-entries` | Milestone start | yes, if shell files do not conflict with truthfulness work |
| Avatar account panel/logout | PLANNED | `task-6-avatar-account-panel-logout` | sidebar footer primary entries | yes, after footer entry budget |
| Helper/Remote Nodes placement | PLANNED | `task-7-helper-remote-nodes-entry-placement` | sidebar footer primary entries | yes, after footer entry budget |

## Exit Gates

- ArtifactComments/ArtifactPanel and Settings `PermissionsView` are reachable through production UI when claimed.
- Forbidden states do not leak private channel, artifact, message, file, or body content before authorization succeeds.
- Sidebar/account IA movement does not merge Helper and Remote Agent credentials, grants, or enforcement rails.
- No new user-facing privacy/compliance product surface is introduced through gh#654.
