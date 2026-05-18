# Acceptance

## Checklist

- [x] Dependency preflight confirms Task 4 PR #948 and Task 5 PR #953 are merged before implementation.
- [x] Server rejects creator leave, non-member leave, non-creator delete, non-creator archive, and creator removal.
- [x] Server membership/mention-management paths require the manager to be a channel member and preserve existing manager permission and owner-ceiling checks.
- [x] Client Settings action availability reflects server permission state for delete/archive instead of ownership alone.
- [x] Existing member modal destructive controls match channel creator authority and do not expose creator removal.
- [x] Settings remains read-only for leave/delete/archive/owner-transfer mutation actions.
- [x] Current docs and task docs are updated.

## Verification Evidence

| Acceptance segment | Evidence | Result |
|---|---|---|
| Baseline client | `pnpm --filter @borgee/client exec vitest run --reporter=dot src/__tests__/channel-management-api.test.ts src/__tests__/ChannelManagementSurface.test.tsx` passed before Task6 changes with 2 files and 11 tests | PASS |
| Baseline server | `GOTMPDIR=$PWD/.tmp/go-build go test -tags sqlite_fts5 ./internal/api -run 'Test(ChannelCRUD\|ChannelMembers\|P0ChannelLifecycle\|P0ChannelDeleteCascades\|ChannelRequireMentionPolicyAPI)$'` passed before Task6 changes | PASS |
| RED client | Focused client tests failed because ownership alone still made delete/archive available without server permission hints | PASS |
| RED server | `TestChannelAuthorityChecks` failed because creator leave, non-member leave, non-creator delete/archive, and creator removal were accepted | PASS |
| GREEN client | `pnpm --filter @borgee/client exec vitest run --reporter=dot src/__tests__/ChannelManagementSurface.test.tsx src/__tests__/channel-management-api.test.ts src/__tests__/SettingsPage.test.tsx` passed with 3 files and 18 tests | PASS |
| GREEN server | `GOTMPDIR=$PWD/.tmp/go-build go test -tags sqlite_fts5 ./internal/api -run 'Test(ChannelCRUD\|ChannelMembers\|P0ChannelLifecycle\|P0ChannelDeleteCascades\|ChannelRequireMentionPolicyAPI\|ChannelAuthorityChecks)$'` passed | PASS |
| Full client tests | `pnpm --filter @borgee/client exec vitest run --reporter=dot --testTimeout=10000` passed with 134 files, 853 tests passed, 1 skipped | PASS |
| Client type/build | `pnpm --filter @borgee/client typecheck` and `pnpm --filter @borgee/client build` passed; build emitted the existing large-chunk warning | PASS |
| Full server tests | `GOTMPDIR=/workspace/.go-tmp-m2-task6 go test -tags sqlite_fts5 ./...` passed | PASS |
| Whitespace | `git diff --check` passed | PASS |

## Remaining Work

- Owner transfer remains unavailable for v1.
- Settings still does not execute channel management mutations.
- Task9 sidebar state collision regression remains separate.
