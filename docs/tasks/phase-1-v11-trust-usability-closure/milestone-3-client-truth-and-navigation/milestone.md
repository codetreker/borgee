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
| ArtifactComments production mount | ACCEPTED | `task-1-artifactcomments-production-mount` | PR #946 (`a6c6ce3`) | complete |
| ACL forbidden-state UX | ACCEPTED | `task-2-acl-forbidden-state-ux` | PR #957 (`16e2db6`) | complete |
| Settings PermissionsView reachability | ACCEPTED | `task-3-security-permission-surface-reachability` | PR #944 (`0877a9b`) | complete |
| Production-surface e2e reverse proof | ACCEPTED | `task-4-production-surface-e2e-reverse-proof` | PR #960 (`84a0315`) | complete |
| Sidebar footer primary entries | ACCEPTED | `task-5-sidebar-footer-primary-entries` | PR #947 (`47dc680`) | complete |
| Avatar account panel/logout | ACCEPTED | `task-6-avatar-account-panel-logout` | PR #950 (`05fff88`) | complete |
| Helper/Remote Nodes placement | ACCEPTED | `task-7-helper-remote-nodes-entry-placement` | PR #962 (`2e58127`) | complete |

Milestone 3 is accepted. This Task12 closure PR only records the state sync; it does not reopen client truth or navigation scope.

## Exit Gates

- ArtifactComments/ArtifactPanel and Settings `PermissionsView` are reachable through production UI when claimed.
- Forbidden states do not leak private channel, artifact, message, file, or body content before authorization succeeds.
- Sidebar/account IA movement does not merge Helper and Remote Agent credentials, grants, or enforcement rails.
- No new user-facing privacy/compliance product surface is introduced through gh#654.
