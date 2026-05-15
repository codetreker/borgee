# Spec Brief: ArtifactComments Production Mount

## 0. Constraints

Task contract: make the existing ArtifactComments surface reachable from the production ArtifactPanel for the active artifact. Preserve server ACL as the authority for artifact/comment access, and do not add broad e2e, mobile, modal a11y, visual redesign, Settings Permissions, sidebar/footer, or privacy/compliance product scope.

Blueprint anchors:

- `CT-1` (`migration-analysis.md` section 5.3): selected already-built surfaces must be production-reachable when claimed.
- `PS-1` (`migration-analysis.md` section 6.1): do not add user-facing privacy/compliance product expansion, and do not weaken backend/security/privacy controls.

## 1. Segmentation

Segment A: production mount.
Mount `ArtifactComments` from `ArtifactPanel` only after an authoritative active artifact exists, passing the artifact id from the loaded/created artifact state.

Segment B: comment-body rendering.
Use existing comment rendering contracts where the required inputs already exist. `ArtifactCommentBody` can render each returned comment body through the existing sanitized markdown path because `listArtifactComments` returns the full `body`.

Segment C: non-leaky authority boundary.
Do not infer protected names or bodies in client-side fallback states. Listing comments still goes through `listArtifactComments`; failures remain non-authoritative client state and do not grant access.

## 2. Out Of Scope

- No Settings `PermissionsView` reachability work.
- No sidebar/footer, avatar/account, Helper/Remote Nodes placement, or rail movement.
- No broad e2e platform rewrite or production-surface reverse-proof task.
- No new privacy dashboard, compliance center, audit UI, or legal promise copy.
- No server ACL changes or new comment APIs.

## 3. Reverse Checks

- If ArtifactPanel can claim an artifact surface while comments are unreachable for an authorized artifact, `CT-1` is not satisfied.
- If the client renders comment body content from realtime preview payloads instead of REST, privacy and ACL centralization are weakened.
- If this task touches Settings, sidebar/footer, Helper/Remote Nodes placement, or channel authority, it overlaps other tasks and must stop.
