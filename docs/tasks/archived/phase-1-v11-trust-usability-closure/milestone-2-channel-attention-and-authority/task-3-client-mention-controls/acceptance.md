# Acceptance: Client Mention Controls

## Source Alignment

- Task: `task-3-client-mention-controls`
- Milestone: `phase-1-v11-trust-usability-closure/milestone-2-channel-attention-and-authority`
- Blueprint anchor: `MR-1` (`docs/blueprint/next/migration-analysis.md` section 3.3)
- Dependencies: Task 1 PR #949 and Task 2 PR #951 merged into `origin/main` before this branch.

## Checks

- [x] Client exposes a channel mention settings surface in the settings channel-management tab.
- [x] Agent members show stored policy and effective delivery state.
- [x] Users with `channel.manage_members` can submit inherit/on/off policy updates through the server endpoint.
- [x] Users without `channel.manage_members` see disabled policy controls.
- [x] `@Everyone` behavior is described as server-computed, ACL/member-scoped, and not client-selectable.
- [x] Client HTTP message sends do not serialize recipient id arrays.
- [x] Client websocket message sends do not serialize recipient id arrays.
- [x] No fanout logic, ACL, notification-center, owner-transfer, leave/delete/archive, history backfill, or broad visual redesign is included.

## Evidence

| Check | Evidence | Result |
|---|---|---|
| RED client tests | `pnpm --filter @borgee/client test -- src/__tests__/channel-management-api.test.ts src/__tests__/ChannelManagementSurface.test.tsx` failed on missing policy helper, serialized `mentions`, and missing UI controls. | PASS |
| RED server test | `GOTMPDIR=/workspace/borgee-gotmp-m2-task3 go test -tags fts5 ./internal/api -run 'TestChannelRequireMentionPolicyAPI'` failed because `effective_require_mention` was absent from member listing. | PASS |
| GREEN focused client tests | `pnpm --filter @borgee/client test -- src/__tests__/channel-management-api.test.ts src/__tests__/ChannelManagementSurface.test.tsx` passed: 133 files, 842 tests, 1 skipped. | PASS |
| GREEN focused server test | `GOTMPDIR=/workspace/borgee-gotmp-m2-task3 go test -tags fts5 ./internal/api -run 'TestChannelRequireMentionPolicyAPI'` passed. | PASS |
| Client typecheck | `pnpm --filter @borgee/client typecheck` passed. | PASS |
| Client build | `pnpm --filter @borgee/client build` passed; Vite emitted the existing large-chunk warning. | PASS |
| Server regression suite | `GOTMPDIR=/workspace/borgee-gotmp-m2-task3 go test -tags fts5 ./...` passed from `packages/server-go`. | PASS |
| Whitespace check | `git diff --check` passed. | PASS |
| Lint command | `pnpm lint` exited 2 because ESLint 9 could not find `eslint.config.*`; this is a repository configuration gap, not a Task3 lint finding. | BLOCKED |
| Post-rebase client focus | After rebasing onto `origin/main` at `a6c6ce3`, `pnpm --filter @borgee/client test -- src/__tests__/channel-management-api.test.ts src/__tests__/ChannelManagementSurface.test.tsx` passed: 134 files, 843 tests, 1 skipped. | PASS |
| Post-rebase server focus | After rebasing, `GOTMPDIR=/workspace/borgee-gotmp-m2-task3 go test -tags fts5 ./internal/api -run 'TestChannelRequireMentionPolicyAPI|TestEveryoneFanout'` passed from cache. | PASS |

Verifier: owner worker
Date: 2026-05-15
Scope: client settings mention controls, client message recipient payload cleanup, member-list effective policy state, current docs
Decision: ready for PR review and CI gate
