# Progress: ACL Forbidden-State UX

## State

- Status: ACCEPTING
- Worktree: `.worktrees/m3-task2-acl-forbidden-state-ux`
- Branch: `feat/m3-task2-acl-forbidden-state-ux`
- PR: pending
- Blocker: none

## Dependency Decision

- Task2 depends on `task-1-artifactcomments-production-mount` only.
- Current `origin/main` contains Task1 PR #946 at `a6c6ce316052797666729f7acb90b86f041b04b6`.
- Current `origin/main` also contains the known adjacent M3 merged SHAs for Task3, Task5, and Task6.
- Decision: unblocked; implement Task2 from current `origin/main`.

## Log

- Reviewed canonical Phase 1 v11, Milestone 3, Task1, Task2, and Task3 docs.
- Created isolated worktree and branch from `origin/main`.
- Installed dependencies with `pnpm install` in the task worktree.
- Ran focused baseline before task changes: ArtifactPanel, ArtifactComments, PermissionsView, and SettingsPage tests passed.
- Added failing tests first for ArtifactComments loading/forbidden states, ArtifactPanel denied reload content clearing, and `PermissionsView` denied capability visibility.
- Implemented local non-leaky denied states without changing server authority or starting Task4.
- Added task stance, spec, design, content lock, acceptance, progress, and current-doc sync.
- Ran focused and package-level client verification.

## Verification Evidence

| Command | Result |
|---|---|
| `./node_modules/.bin/vitest run --environment jsdom packages/client/src/__tests__/ArtifactPanel-artifact-comments.test.tsx packages/client/src/__tests__/ArtifactComments.test.tsx packages/client/src/__tests__/PermissionsView.test.tsx` before implementation | RED: 4 expected failures because `[data-cv5-loading]`, `[data-cv5-forbidden]`, `[data-artifact-forbidden]`, and `[data-ap2-forbidden]` were absent. |
| `./node_modules/.bin/vitest run --environment jsdom packages/client/src/__tests__/ArtifactPanel-artifact-comments.test.tsx packages/client/src/__tests__/ArtifactComments.test.tsx packages/client/src/__tests__/PermissionsView.test.tsx` | PASS: 3 files, 14 tests. |
| `./node_modules/.bin/vitest run --environment jsdom packages/client/src/__tests__/ArtifactPanel-artifact-comments.test.tsx packages/client/src/__tests__/ArtifactComments.test.tsx packages/client/src/__tests__/PermissionsView.test.tsx packages/client/src/__tests__/SettingsPage.test.tsx` | PASS: 4 files, 19 tests. |
| `./node_modules/.bin/tsc -b packages/client` | PASS. |
| `pnpm --filter @borgee/client build` | PASS with existing large-chunk warning. |
| `pnpm --filter @borgee/client test` | PASS: 134 files, 844 passed, 1 skipped. |

## Acceptance State

- ArtifactPanel denied reload: implemented and covered.
- ArtifactComments loading/forbidden/unavailable distinction: implemented and covered.
- Settings PermissionsView denied visibility: implemented and covered.
- Current-doc sync: complete.
