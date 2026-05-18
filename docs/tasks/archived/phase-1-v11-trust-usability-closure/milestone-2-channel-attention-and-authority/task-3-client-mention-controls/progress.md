# Progress

## Resume

| Field | Value |
|---|---|
| Worktree | `.worktrees/m2-task3-preflight-execution` |
| Branch | `feat/m2-task3-preflight-execution` |
| PR | pending |
| Owner | Blueprintflow owner worker |
| State | ACCEPTING |
| Blocker | none |

## Checkpoints

- [x] Fetched `origin/main` before preflight.
- [x] Created isolated worktree/branch from `origin/main` at `419c5bf`.
- [x] Dependency checked: Task 3 depends on Task 1 and Task 2 only; Task 1 PR #949 (`c25ef60`) and Task 2 PR #951 (`3659ce1`) are in `origin/main`.
- [x] Baseline client tests run before edits: `pnpm --filter @borgee/client test -- src/__tests__/ChannelManagementSurface.test.tsx src/__tests__/ChannelManagementSurface.test.tsx` passed with 133 files, 839 tests, 1 skipped.
- [x] Four-piece task docs created: `spec.md`, `stance.md`, `design.md`, `acceptance.md`, `progress.md`.
- [x] RED tests written and observed failing for missing behavior.
- [x] Implementation completed.
- [x] Current docs synced.
- [x] Full verification completed.
- [x] Rebased onto `origin/main` at `a6c6ce3` after PR #946 merged.
- [ ] PR opened.
- [ ] CI passed and merge completed.

## Implementation Evidence

Implemented in this task branch:

- Added `effective_require_mention` to channel member listings so client controls can render server-derived current state.
- Added client require-mention policy API/types for inherit/on/off updates.
- Added expandable settings channel-row mention controls with `@Everyone` server-authority copy and agent policy selects.
- Removed client-supplied mention recipient id arrays from HTTP and websocket message send bodies while preserving local pending mention metadata.
- Updated current docs for settings surface, message send boundary, and channel attention policy visibility.

## Verification Evidence

Focused evidence already run:

- `pnpm --filter @borgee/client test -- src/__tests__/channel-management-api.test.ts src/__tests__/ChannelManagementSurface.test.tsx` -> PASS, 133 files, 842 tests, 1 skipped.
- `GOTMPDIR=/workspace/borgee-gotmp-m2-task3 go test -tags fts5 ./internal/api -run 'TestChannelRequireMentionPolicyAPI'` -> PASS.

Full verification and CI gate evidence will be appended before merge.

Full verification evidence:

- `pnpm --filter @borgee/client typecheck` -> PASS.
- `pnpm --filter @borgee/client build` -> PASS; Vite emitted the existing large-chunk warning.
- `GOTMPDIR=/workspace/borgee-gotmp-m2-task3 go test -tags fts5 ./internal/api -run 'TestChannelRequireMentionPolicyAPI|TestEveryoneFanout'` -> PASS.
- `GOTMPDIR=/workspace/borgee-gotmp-m2-task3 go test -tags fts5 ./...` from `packages/server-go` -> PASS.
- `git diff --check` -> PASS.
- `pnpm lint` -> BLOCKED by repo ESLint 9 config gap: ESLint could not find `eslint.config.(js|mjs|cjs)`.

Post-rebase focused evidence:

- `pnpm --filter @borgee/client test -- src/__tests__/channel-management-api.test.ts src/__tests__/ChannelManagementSurface.test.tsx` -> PASS, 134 files, 843 tests, 1 skipped.
- `GOTMPDIR=/workspace/borgee-gotmp-m2-task3 go test -tags fts5 ./internal/api -run 'TestChannelRequireMentionPolicyAPI|TestEveryoneFanout'` -> PASS (cached).
- `git diff --check HEAD~1..HEAD` -> PASS.

## Acceptance State

Local task implementation is ready for PR creation, CI, and merge handling.
