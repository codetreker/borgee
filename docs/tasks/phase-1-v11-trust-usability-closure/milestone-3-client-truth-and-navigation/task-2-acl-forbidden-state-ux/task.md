# task-2-acl-forbidden-state-ux

Purpose:
- Replace blank or misleading unauthorized states with explicit non-leaky forbidden states on named production surfaces.

Scope:
- Add local/in-surface forbidden, unavailable, loading, or empty states for `ArtifactPanel`/`ArtifactComments` access failures and the Settings `PermissionsView` surface.
- Ensure forbidden UI never becomes authorization and never exposes private channel, artifact, message, file, or body content before server ACL succeeds.

Out of scope:
- No global privacy/compliance product promise.
- No redirect/full-page state unless task design proves it is better for ArtifactPanel/ArtifactComments or Settings `PermissionsView`.

Depends on:
- `task-1-artifactcomments-production-mount`

Blueprint anchors:
- `CT-1`: `docs/blueprint/next/migration-analysis.md` §5.3
- `PS-1`: `docs/blueprint/next/migration-analysis.md` §6.1

Acceptance slice:
- A reviewer can trigger forbidden states for ArtifactComments/ArtifactPanel and Settings PermissionsView and verify the user sees truthful non-leaky UI instead of blank UI, fake loading, or leaked protected content.

Parallelism:
- Can run after task 1. Can run alongside task 3 if protected surfaces are separable.

Sensitive paths:
- auth, ACL, privacy, protected content visibility
