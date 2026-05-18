# Progress: Production Surface E2E Reverse Proof

## State

- Status: ACCEPTING
- Worktree: `.worktrees/m3-task4-production-surface-e2e-reverse-proof`
- Branch: `task/m3-task4-production-surface-e2e-reverse-proof`
- PR: pending
- Blocker: none

## Log

- Resumed already-started Task4 worktree from `origin/main` base `16e2db6`.
- Confirmed dependencies `task-2-acl-forbidden-state-ux` and `task-3-security-permission-surface-reachability` are satisfied.
- Added focused Playwright spec for ArtifactComments production mount, ArtifactComments forbidden state, ArtifactPanel archived-channel forbidden state, and Settings PermissionsView states.
- RED: focused e2e initially passed 2 tests and failed 2 tests: comment denial stayed at owner-authorized `200`, and archive setup used stale `PATCH /api/v1/channels/{id}` returning 404.
- Fixed comment denial by fulfilling the browser request with an actual server response fetched under the unauthorized user's API context, then asserting the server denial code is not rendered.
- Fixed archived-channel setup to call `PUT /api/v1/channels/{id}`, matching client and server contracts.
- Added Task4 spec/design/progress/acceptance docs and current e2e doc sync.

## Verification Evidence

- `GOTMPDIR=$PWD/.go-tmp npm test -- tests/production-surface-reverse-proof.spec.ts --reporter=list` from `packages/e2e` -> PASS: 4 tests.
- `./node_modules/.bin/vitest run --environment jsdom packages/client/src/__tests__/ArtifactPanel-artifact-comments.test.tsx packages/client/src/__tests__/ArtifactComments.test.tsx packages/client/src/__tests__/PermissionsView.test.tsx packages/client/src/__tests__/SettingsPage.test.tsx` -> PASS: 4 files, 19 tests.
- `./node_modules/.bin/tsc -b packages/client` -> PASS.
- `git diff --check` -> PASS.
- Non-gating inherited e2e typecheck gap: `./node_modules/.bin/tsc -p packages/e2e/tsconfig.json --noEmit` fails on existing `packages/e2e/tests/chat-two-user-collab.spec.ts(26,1)` unused `@ts-expect-error`, outside Task4 scope.

## Acceptance State

- ArtifactComments production mount reverse proof: complete
- ArtifactComments forbidden-state reverse proof: complete
- ArtifactPanel archived-channel denial reverse proof: complete
- Settings PermissionsView empty/forbidden/error reverse proof: complete
- Current-doc sync: complete
