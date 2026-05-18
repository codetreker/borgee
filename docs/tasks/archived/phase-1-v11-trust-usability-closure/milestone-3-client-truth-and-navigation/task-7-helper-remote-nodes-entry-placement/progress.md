# Progress

## Resume

| Field | Value |
|---|---|
| Worktree | `.worktrees/m3-task7-helper-remote-nodes-entry-placement` |
| Branch | `m3-task7-helper-remote-nodes-entry-placement` |
| PR | https://github.com/codetreker/borgee/pull/962 |
| Owner | M3 Task7 rescue owner worker |
| State | PR_VERIFYING |
| Blocker | none |

## Dependency Decision

Task7 is unblocked. It depends only on M3 Task5 `task-5-sidebar-footer-primary-entries`; PR #947 is merged and its merge commit `47dc6805abaf98fffcd727ec5917b641367f2eeb` is an ancestor of the Task7 branch. Known M3 Task6 PR #950 is also merged into `origin/main`, but no unmerged work blocks Task7.

## Checkpoints

- [x] Fetched latest `origin/main`
- [x] Created isolated Task7 worktree/branch from `origin/main`
- [x] Verified Task5 dependency is merged and present
- [x] Read Task7 contract, Task5 contract, milestone, and current client shell docs
- [x] Installed workspace dependencies
- [x] Baseline client jsdom/node tests run before Task7 edits
- [x] TDD RED tests written and verified failing
- [x] Production implementation complete
- [x] Current-doc sync completed
- [x] Focused green tests and typecheck passed
- [x] Full verification complete
- [x] PR opened
- [ ] CI reviewed
- [ ] Merge/cleanup complete

## Evidence Log

| Item | Evidence | Result |
|---|---|---|
| Fetch | `git fetch origin main --prune` -> `origin/main` at `84a0315ec76dc18da8350953e588022cf3f9c7f6` | PASS |
| Worktree | `git worktree add .worktrees/m3-task7-helper-remote-nodes-entry-placement -b m3-task7-helper-remote-nodes-entry-placement origin/main` | PASS |
| Rescue rebase | `git fetch origin main --prune`; `git rebase origin/main` after `origin/main` advanced to `1e6d54c` | PASS |
| Task5 dependency | `gh pr view 947 --json ...` -> MERGED; `git merge-base --is-ancestor 47dc6805abaf98fffcd727ec5917b641367f2eeb HEAD` | PASS |
| Task6 current-main state | `gh pr view 950 --json ...` -> MERGED; `git merge-base --is-ancestor 05fff8813ab87afd83c82c77dd6253971028fa80 HEAD` | INFO |
| Dependency install | `pnpm install` | PASS |
| Baseline jsdom | `pnpm --filter @borgee/client test:jsdom -- packages/client/src/__tests__/Sidebar-footer-primary.test.tsx packages/client/src/__tests__/SettingsPage.test.tsx` -> 104 files passed, 661 tests passed, 1 skipped | PASS |
| Baseline node | `pnpm --filter @borgee/client test:node -- packages/client/src/__tests__/main-view.test.ts` -> 30 files passed, 192 tests passed | PASS |
| TDD RED | `pnpm --filter @borgee/client test:jsdom -- packages/client/src/__tests__/SettingsPage.test.tsx` and Sidebar run failed as expected on missing Runtime tab and existing footer runtime entries | PASS |
| GREEN focused jsdom | From `packages/client`: `pnpm --filter @borgee/client test:jsdom -- src/__tests__/SettingsPage.test.tsx src/__tests__/Sidebar-footer-primary.test.tsx` -> 104 files passed, 662 tests passed, 1 skipped | PASS |
| Client typecheck | `pnpm --filter @borgee/client typecheck` -> exit 0 | PASS |
| Rescue focused jsdom | Post-rebase `pnpm --filter @borgee/client test:jsdom -- src/__tests__/SettingsPage.test.tsx src/__tests__/Sidebar-footer-primary.test.tsx` -> 105 files passed, 666 tests passed, 1 skipped | PASS |
| Rescue client typecheck | `pnpm --filter @borgee/client typecheck` -> exit 0 | PASS |
| Rescue full jsdom | `pnpm --filter @borgee/client test:jsdom` -> 104 files passed, 662 tests passed, 1 skipped | PASS |
| Rescue full node | `pnpm --filter @borgee/client test:node` -> 30 files passed, 192 tests passed | PASS |
| Rescue CI-style client test | Post-rebase `pnpm --filter @borgee/client test` -> 135 files passed, 858 tests passed, 1 skipped | PASS |
| Rescue client build | Post-rebase `pnpm --filter @borgee/client build` -> `tsc -b && vite build` completed; Vite emitted the existing large-chunk warning | PASS |
| Repo lint command | `pnpm lint` exits before linting: ESLint 9 cannot find `eslint.config.(js\|mjs\|cjs)` and this worktree has no ESLint config file | BLOCKED - repo tooling |
| Diff hygiene | `git diff --check` -> exit 0 | PASS |
| PR opened | https://github.com/codetreker/borgee/pull/962 | PASS |

## Current-Doc Sync Targets

| File | Handling |
|---|---|
| `docs/current/client/app-shell-state.md` | Update shell view notes so Settings Runtime owns Remote Nodes and Helper Status launch points |
| `docs/current/client/ui/sidepane.md` | Update sidepane sketch for footer overflow and Settings Runtime placement |
| `docs/current/client/ui/main-desktop.md` | Update footer sketch and desktop notes |
| `docs/current/client/ui/settings.md` | Add Runtime tab sketch and rail-separation notes |
| `docs/current/client/ui-map.md` | Update surface hierarchy and Settings sidepane map |

## Acceptance State

Task7 is implemented and PR #962 is open for CI. Fresh rescue verification passed focused/full client tests, typecheck, CI-style client test, client build, and diff hygiene. The repo-level `pnpm lint` script is not runnable in this worktree because ESLint 9 has no flat config file in the repository. CI, merge, and cleanup are still pending.
