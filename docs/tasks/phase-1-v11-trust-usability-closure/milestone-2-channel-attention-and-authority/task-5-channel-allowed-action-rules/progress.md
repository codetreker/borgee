# Progress

## Resume

| Field | Value |
|---|---|
| Worktree | `.worktrees/m2-task5-channel-allowed-action-rules` |
| Branch | `feat/m2-task5-channel-allowed-action-rules` |
| PR | #953 |
| Owner | M2 Task5 owner worker |
| State | ACCEPTING |
| Blocker | none |

## Checkpoints

- [x] Latest `origin/main` fetched at Task 4 merge `077cb8c6c4f2c3984221d14f1957f2a41e1a81ed`.
- [x] Dependency preflight completed: Task 5 depends on Task 4 only; Task 2 PR #951 is not a dependency.
- [x] Isolated worktree and branch created from `origin/main`.
- [x] Dependency install completed in the isolated worktree because `node_modules` was absent.
- [x] Baseline focused channel-management tests passed after install.
- [x] Task spec, stance, design, and acceptance docs created.
- [x] RED tests written and verified failing.
- [x] Implementation added for read-only allowed-action rules.
- [x] Focused GREEN tests passed.
- [x] Branch rebased onto current `origin/main` at `0dd35a9` after Task 2 PR #951, Task 3 PR #955, Task 8 PR #952, Helper PR #954, and M3 Task1 PR #946 merged; Task 3 and Task 9 remain non-blocking for Task 5.
- [x] Full verification passed.
- [x] PR opened as #953.
- [ ] CI passed.
- [ ] PR merged and worktree cleaned up.

## Baseline Evidence

| Command | Result |
|---|---|
| `timeout 600s pnpm --filter @borgee/client test -- ChannelManagementSurface channel-management-api --testTimeout=10000` | Initial attempt failed because `vitest` was missing in the new worktree; after `pnpm install`, PASS with `132` test files, `834` tests passed, `1` skipped |

## RED/GREEN Evidence

| Command | Evidence | Result |
|---|---|---|
| `timeout 600s pnpm --filter @borgee/client test -- ChannelManagementSurface channel-management-api --testTimeout=10000` | RED failed before implementation because `buildChannelAllowedActionRules` was missing and Settings rows had no action availability DOM | PASS |
| `timeout 600s pnpm --filter @borgee/client test -- ChannelManagementSurface channel-management-api --testTimeout=10000` | GREEN after current-main rebase; `134` test files, `847` tests passed, `1` skipped | PASS |
| `timeout 600s pnpm --filter @borgee/client typecheck` | `tsc --noEmit` exited 0 | PASS |
| `timeout 600s pnpm --filter @borgee/client build` | `tsc -b && vite build` exited 0 with the repo's existing large-chunk warning | PASS |
| `git diff --check` | exited 0 | PASS |
| Reverse guards | `ChannelManagementSurface.tsx` has no `leaveChannel(`/`deleteChannel(`/`archiveChannel(`; diff has no Task6/8/9 or Sidebar production edits | PASS |

## Implementation Evidence

| Acceptance segment | Evidence | Result |
|---|---|---|
| Allowed-action rule SSOT | `buildChannelAllowedActionRules` and `canLeaveChannel` added to `packages/client/src/lib/channelManagement.ts` | PASS |
| Settings action availability | `ChannelManagementSurface` renders read-only `data-action` / `data-allowed` rows for leave/delete/archive/owner-transfer | PASS |
| Owner leave prevention | `canLeaveChannel` returns false for current-user-created channels; `ChannelView` uses it for the existing leave button | PASS |
| No mutation controls | Settings action availability renders list items, not buttons, and imports no mutation API | PASS |

## Scope Locks

- In scope: read-only allowed-action rules, Settings row visibility, owner leave hiding, tests, task docs, current docs.
- Out of scope: Task6 server/client enforcement, mutation buttons, owner transfer, notification/collapse/sort/pin/group/private-indicator/sidebar/footer work.
