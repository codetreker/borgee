# Dev Design: ArtifactComments Production Mount

## 1. Boundary And Approach

Mount the existing `ArtifactComments` component from `ArtifactPanel` once `ArtifactPanel` has an active artifact. This keeps the change inside the canvas/artifact surface and avoids Settings, sidebar/footer, and channel authority files.

The production data boundary stays REST-first. `ArtifactPanel` owns the active artifact head and passes `artifact.id` to `ArtifactComments`; `ArtifactComments` continues to call `listArtifactComments` and to refresh from `artifact_comment_added` signal frames without rendering preview bodies from the frame.

## 2. Implementation Plan

1. Add an ArtifactPanel-focused test that drives the create-artifact path and expects the comments surface to mount for the created artifact id.
2. Import and render `ArtifactComments` in `ArtifactPanel` below the artifact body area and before ancillary artifact panels such as anchors, versions, and iteration controls.
3. Update `ArtifactComments` rows to render bodies with `ArtifactCommentBody`, because `listArtifactComments` already returns each comment body and the markdown sanitizer path is available.
4. Sync `docs/current/client/feature-surfaces.md` with the implemented production mount and remaining limits.

## 3. Non-Goals

- Do not add Settings `PermissionsView` reachability.
- Do not change Sidebar, account/logout, Helper status, Remote Nodes, or channel management files.
- Do not add search/thread/history production wiring until the parent can supply the required virtual channel id, reply state, or history trigger state.
- Do not add server endpoints, ACL changes, or e2e reverse proof in this task.

## 4. Verification

- `../../node_modules/.bin/vitest run src/__tests__/ArtifactPanel-artifact-comments.test.tsx`
- `../../node_modules/.bin/vitest run src/__tests__/ArtifactPanel-artifact-comments.test.tsx src/__tests__/ArtifactComments.test.tsx src/__tests__/ArtifactCommentBody.test.tsx`
- `../../node_modules/.bin/vitest run src/__tests__/ArtifactPanel-artifact-comments.test.tsx src/__tests__/ArtifactPanel-kind-switch.test.tsx src/__tests__/ArtifactComments.test.tsx src/__tests__/ArtifactCommentBody.test.tsx`
- `git diff --check`
