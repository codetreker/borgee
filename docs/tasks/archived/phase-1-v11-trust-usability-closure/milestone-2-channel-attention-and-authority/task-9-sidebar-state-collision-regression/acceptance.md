# Acceptance: Sidebar State Collision Regression

## Source Alignment

- Task: `task-9-sidebar-state-collision-regression`
- Milestone: `phase-1-v11-trust-usability-closure/milestone-2-channel-attention-and-authority`
- Blueprint anchors: `CH-1` and `PS-1`
- Dependency: Task 8 private indicator visual treatment merged in PR #952.

## Checks

- [x] Dependency preflight confirms Task 8 PR #952 is present on `origin/main`.
- [x] Private + unread + selected + pinned + drag-over remains visibly and semantically separated in one channel row.
- [x] Archived private rows still override private/public state and suppress unread.
- [x] Public preview rows do not gain private metadata or unread attention state.
- [x] DM-only presence/fault semantics stay out of channel private rows.
- [x] Current docs and task docs record the regression proof.
- [x] No ACL, membership, fanout, management, API/server, footer/sidebar IA, or broad visual redesign is included.

## Verification Evidence

| Acceptance segment | Evidence | Result |
|---|---|---|
| Dependency preflight | `git log --oneline -n 30` from fetched `origin/main` showed Task8 merge `9659ce1 feat(client): quiet private channel indicator (#952)` before Task9 implementation | PASS |
| Baseline jsdom | `pnpm --filter @borgee/client test:jsdom -- packages/client/src/__tests__/SortableChannelItem-private-indicator.test.tsx` passed before Task9 changes with 104 files, 661 tests passed, 1 skipped | PASS |
| RED regression/evidence test | `pnpm --filter @borgee/client test:jsdom -- src/__tests__/Sidebar-state-collision-regression.test.tsx` failed because Task9 `acceptance.md` did not exist | PASS |
| GREEN focused regression | `pnpm --filter @borgee/client exec vitest run --project=jsdom --reporter=dot src/__tests__/Sidebar-state-collision-regression.test.tsx` passed with 1 file and 4 tests | PASS |
| Client typecheck | `pnpm --filter @borgee/client typecheck` passed | PASS |
| Client build | `pnpm --filter @borgee/client build` passed; Vite emitted the existing large-chunk warning only | PASS |
| Full client test suite | `pnpm --filter @borgee/client test` passed with 135 files, 857 tests passed, 1 skipped | PASS |
| Whitespace | `git diff --check` passed | PASS |
| Lint script | `pnpm lint` is blocked by the repo's missing ESLint v9 flat config (`eslint.config.*`); no lint signal was available from this script | BLOCKED |

## Remaining Work

- CI and acceptance review after PR creation.
