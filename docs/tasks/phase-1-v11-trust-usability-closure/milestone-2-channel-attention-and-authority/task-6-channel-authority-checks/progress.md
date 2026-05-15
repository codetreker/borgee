# Progress

## Resume

| Field | Value |
|---|---|
| Worktree | `.worktrees/m2-task6-channel-authority-checks` |
| Branch | `m2-task6-channel-authority-checks` |
| PR | pending |
| Owner | M2 Task6 owner worker |
| State | READY_FOR_PR |
| Blocker | none |

## Checkpoints

- [x] Latest `origin/main` fetched and task docs read.
- [x] Parent checkout had unrelated local changes; isolated worktree created from `origin/main` without touching them.
- [x] Dependencies installed in the worktree because `node_modules` was absent.
- [x] Baseline focused client and Go channel tests passed.
- [x] Spec, stance, design, acceptance, and progress docs created.
- [x] RED tests written and verified failing.
- [x] Implementation added for server authority checks and client truthfulness rules.
- [x] Focused GREEN tests passed.
- [x] Full verification passed.
- [ ] PR opened.
- [ ] CI passed.
- [ ] PR merged and worktree cleaned up.

## Evidence

| Command | Result |
|---|---|
| `pnpm install --frozen-lockfile` | PASS; lockfile unchanged, ignored-build-scripts warning only |
| `go mod download` | PASS |
| `pnpm --filter @borgee/client exec vitest run --reporter=dot src/__tests__/channel-management-api.test.ts src/__tests__/ChannelManagementSurface.test.tsx` | Baseline PASS; 2 files, 11 tests |
| `GOTMPDIR=$PWD/.tmp/go-build go test -tags sqlite_fts5 ./internal/api -run 'Test(ChannelCRUD\|ChannelMembers\|P0ChannelLifecycle\|P0ChannelDeleteCascades\|ChannelRequireMentionPolicyAPI)$'` | Baseline PASS after using repo-local `GOTMPDIR` and `sqlite_fts5` tag |
| RED focused client run | FAIL as expected: delete/archive remained available without server permission state |
| RED focused Go run | FAIL as expected: missing server authority checks accepted denied actions |
| `pnpm --filter @borgee/client exec vitest run --reporter=dot src/__tests__/ChannelManagementSurface.test.tsx src/__tests__/channel-management-api.test.ts src/__tests__/SettingsPage.test.tsx` | GREEN PASS; 3 files, 18 tests |
| `GOTMPDIR=$PWD/.tmp/go-build go test -tags sqlite_fts5 ./internal/api -run 'Test(ChannelCRUD\|ChannelMembers\|P0ChannelLifecycle\|P0ChannelDeleteCascades\|ChannelRequireMentionPolicyAPI\|ChannelAuthorityChecks)$'` | GREEN PASS |
| `GOTMPDIR=$PWD/.tmp/go-build go test -tags sqlite_fts5 ./internal/api -count=1` | PASS; full API package after fixture correction |
| `pnpm --filter @borgee/client exec vitest run --reporter=dot --testTimeout=10000` | PASS; 134 files, 853 tests passed, 1 skipped |
| `pnpm --filter @borgee/client typecheck` | PASS |
| `pnpm --filter @borgee/client build` | PASS with existing large chunk warning |
| `GOTMPDIR=/workspace/.go-tmp-m2-task6 go test -tags sqlite_fts5 ./...` | PASS; rerun used a temp dir outside the worktree so repo scan tests did not race Go build scratch deletion |
| `git diff --check` | PASS |

## Scope Locks

- In scope: channel membership/ownership authority checks, Settings action truthfulness, existing modal/header action gates, tests, task docs, current docs.
- Out of scope: Settings mutation buttons, owner transfer, admin force-delete, notification/collapse/sort/pin/group work, private indicator work, Task9 sidebar regression.
