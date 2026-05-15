# Spec Brief: ACL Forbidden-State UX

## 0. Task Contract

- Source task: `task-2-acl-forbidden-state-ux/task.md`.
- Canonical milestone: `docs/tasks/phase-1-v11-trust-usability-closure/milestone-3-client-truth-and-navigation`.
- Dependency: Task1 ArtifactComments production mount, merged in PR #946 at `a6c6ce3`.
- Blueprint anchors: `CT-1` in `docs/blueprint/next/migration-analysis.md` section 5.3 and `PS-1` in section 6.1.

## 1. Constraints

- Add local states to existing production surfaces; do not create a global redirect or full-page forbidden route.
- Treat 401 and 403 from protected surface fetch/write paths as denied access.
- Do not render server error text from denied responses.
- Clear already-rendered protected artifact content when a later authoritative reload is denied.
- Keep Settings `PermissionsView` as a visibility surface only. It does not become a client-side authorization decision.

## 2. Segments

### 2.1 ArtifactPanel Denied Reload

When `ArtifactPanel` has an active artifact and an authoritative reload through `getArtifact` or `listArtifactVersions` is denied, the panel clears artifact, version, anchor, edit, and selection state, then renders a local forbidden message.

### 2.2 ArtifactComments Truthful List States

`ArtifactComments` starts in loading state until `listArtifactComments` succeeds. A successful empty list renders the existing empty state. A denied list or denied post clears comments and renders a forbidden state. Other list failures render a generic unavailable state.

### 2.3 Settings PermissionsView Denied Visibility

`PermissionsView` maps denied `/api/v1/me/permissions` responses to a local forbidden state under the existing Settings mount. Existing loading, empty, error, and capability-row semantics remain intact for non-denied states.

## 3. Reverse Checks

- Focused tests prove red/green behavior for ArtifactComments loading/forbidden, ArtifactPanel denied reload clearing rendered content, and `PermissionsView` denied visibility.
- Current docs describe denied states without adding compliance/privacy product promises.
- Grep/diff review confirms no Task4 e2e scope, sidebar/footer scope, avatar/account scope, or Helper/Remote Nodes placement scope is started.
