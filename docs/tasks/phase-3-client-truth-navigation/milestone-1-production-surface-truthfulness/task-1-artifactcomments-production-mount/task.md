# task-1-artifactcomments-production-mount

Purpose:
- Make the ArtifactComments series reachable from the production ArtifactPanel surface.

Scope:
- Mount `ArtifactComments` in `packages/client/src/components/ArtifactPanel.tsx` for the active artifact.
- Include the already-built ArtifactComments series needed for a usable production surface: `ArtifactCommentBody`, `ArtifactCommentThread`, `ArtifactCommentSearchBox`, and `ArtifactCommentEditHistoryModal` only where their existing API contracts are available.
- Preserve server authorization as the source of truth and avoid leaking protected names or bodies before ACL succeeds.

Out of scope:
- No broad e2e platform rewrite, mobile expansion, modal a11y sweep, or visual redesign.

Depends on:
- Canonical Milestone 3 start

Blueprint anchors:
- `CT-1`: `docs/blueprint/next/migration-analysis.md` §5.3
- `PS-1`: `docs/blueprint/next/migration-analysis.md` §6.1

Acceptance slice:
- A reviewer can reach ArtifactComments from the production ArtifactPanel for an authorized artifact and verify unauthorized access does not leak protected content.

Parallelism:
- First task for this milestone after execution slot clears. Blocks named-surface forbidden-state and e2e reverse-proof work.

Sensitive paths:
- auth, ACL, privacy, protected content visibility
