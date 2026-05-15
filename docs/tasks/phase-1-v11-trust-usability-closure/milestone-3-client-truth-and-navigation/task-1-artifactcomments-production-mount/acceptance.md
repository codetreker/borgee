# Acceptance: ArtifactComments Production Mount

## Source Alignment

- Task: `task-1-artifactcomments-production-mount`
- Milestone: `phase-1-v11-trust-usability-closure/milestone-3-client-truth-and-navigation`
- Blueprint anchors: `migration-analysis.md` section 5.3 and section 6.1
- Dependencies: canonical Milestone 3 start.

## Checks

Acceptance checks:

- `ArtifactPanel` mounts `ArtifactComments` after an active artifact exists.
- The mounted comment surface receives the authoritative artifact id from `ArtifactPanel` artifact state.
- The comment list request uses the existing `listArtifactComments` REST contract.
- Comment bodies render through the existing sanitized `ArtifactCommentBody` path.
- Current docs record that ArtifactComments is production-mounted while search/thread/history remain gated by their required upstream contracts.

Negative checks:

- No Settings `PermissionsView`, sidebar/footer, avatar/account, Helper/Remote Nodes, or channel-authority files are changed.
- No comment body is rendered from realtime `body_preview` as authoritative content.
- No new user-facing privacy/compliance product promise is added.
- No server ACL or API behavior is weakened or bypassed.

## Evidence Required Before PR

- RED: focused ArtifactPanel test fails because the comments surface is absent.
- GREEN: focused ArtifactPanel/ArtifactComments/ArtifactCommentBody tests pass after implementation.
- Broader client verification covers the changed component and existing comment tests.
- `git diff --check` passes.
