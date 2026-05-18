# Spec: Production Surface E2E Reverse Proof

## 0. Task Contract

- Source task: `task-4-production-surface-e2e-reverse-proof/task.md`.
- Canonical milestone: `docs/tasks/phase-1-v11-trust-usability-closure/milestone-3-client-truth-and-navigation`.
- Dependencies: `task-2-acl-forbidden-state-ux` and `task-3-security-permission-surface-reachability`.
- Blueprint anchor: `CT-1` in `docs/blueprint/next/migration-analysis.md` section 5.3.

## 1. Constraints

- Add a focused Playwright e2e reverse proof for the named production surfaces only.
- Use browser-driven product paths for ArtifactPanel, ArtifactComments, and Settings.
- Use server-backed setup for users, channel creation, artifact creation, archived-channel denial, and comment ACL denial.
- Keep assertions non-leaky: forbidden UI must not render server error codes, protected comments, channel archive implementation details, or mocked permission payload secrets.
- Do not expand into broad e2e harness work, mobile coverage, modal accessibility, Task5 sidebar/footer, Task6 account/logout, or Task7 Helper/Remote Nodes placement.

## 2. Segments

### 2.1 ArtifactComments Production Mount

The e2e path signs in through the normal user cookie, opens the production canvas, creates an artifact through the ArtifactPanel UI, and observes the real ArtifactComments mount and comment list/post REST requests.

### 2.2 ArtifactComments Forbidden State

The e2e path uses a real server ACL denial for `GET /api/v1/artifacts/{artifactId}/comments`, then verifies ArtifactComments renders the non-leaky forbidden state and removes the composer.

### 2.3 ArtifactPanel Archived-Channel Denial

The e2e setup archives the channel through the same `PUT /api/v1/channels/{channelId}` contract used by the client, then verifies artifact creation is denied and ArtifactPanel shows only the generic forbidden state.

### 2.4 Settings PermissionsView States

The e2e path reaches Settings through the product shell and verifies the mounted PermissionsView empty, forbidden, and error states without rendering injected response-body secrets.

## 3. Reverse Checks

- The spec must fail if ArtifactComments is not mounted from production ArtifactPanel.
- The spec must fail if comment ACL denial is only a string/mock assertion and not backed by a server denial response.
- The spec must fail if archived channel setup uses a stale route or if ArtifactPanel leaks the rejected artifact title/archive reason.
- The spec must fail if Settings no longer reaches PermissionsView from the production shell.

