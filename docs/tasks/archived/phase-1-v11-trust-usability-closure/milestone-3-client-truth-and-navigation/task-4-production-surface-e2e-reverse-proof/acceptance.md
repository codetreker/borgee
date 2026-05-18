# Acceptance: Production Surface E2E Reverse Proof

## Checks

| Segment | Check | Evidence |
|---|---|---|
| 2.1 ArtifactComments Production Mount | Browser path reaches ArtifactPanel, creates an artifact, renders ArtifactComments, lists comments, posts a comment, and shows the posted row. | `production-surface-reverse-proof.spec.ts` first test. |
| 2.2 ArtifactComments Forbidden State | Browser path receives a real server ACL denial and renders only the non-leaky forbidden state with no composer. | Spec asserts server `comment.cross_channel_reject` was observed but not rendered. |
| 2.3 ArtifactPanel Archived-Channel Denial | Archived channel rejects artifact create and ArtifactPanel hides the rejected title/archive reason. | Spec archives via `PUT /api/v1/channels/{id}` and expects create `403`. |
| 2.4 Settings PermissionsView States | Settings reaches PermissionsView and renders empty, forbidden, and error states without response-body secret leaks. | Spec intercepts `/api/v1/me/permissions` while navigating through production Settings. |

## Required Commands

- `GOTMPDIR=$PWD/.go-tmp npm test -- tests/production-surface-reverse-proof.spec.ts --reporter=list` from `packages/e2e`
- `./node_modules/.bin/vitest run --environment jsdom packages/client/src/__tests__/ArtifactPanel-artifact-comments.test.tsx packages/client/src/__tests__/ArtifactComments.test.tsx packages/client/src/__tests__/PermissionsView.test.tsx packages/client/src/__tests__/SettingsPage.test.tsx`
- `./node_modules/.bin/tsc -b packages/client`
- `git diff --check`

## Verification Notes

- RED: focused e2e failed before fixes with missing `[data-cv5-forbidden]` and archive `404` from `PATCH /api/v1/channels/{id}`.
- GREEN: focused e2e passed after aligning comment denial and archive setup with real server/client contracts.
- Component baseline passed with 4 files and 19 jsdom tests.
- Client TypeScript verification passed. Global e2e TypeScript verification is blocked by an inherited unused `@ts-expect-error` in `chat-two-user-collab.spec.ts`, outside this task's file ownership.
