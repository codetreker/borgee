# Progress

## Resume

| Field | Value |
|---|---|
| Worktree | `.worktrees/m2-task2-everyone-fanout-acl-rate-loop` |
| Branch | `feat/m2-task2-everyone-fanout-acl-rate-loop` |
| PR | not opened |
| Owner | M2 Task2 owner worker |
| State | ACCEPTING |
| Blocker | none; Task1 dependency satisfied by PR #949 / `c25ef60b1b1b3ccf71ba1997e70523e34b73ca34` |

## Checkpoints

- [x] Fetched latest `origin/main` and created isolated task worktree/branch from Task1 merge commit.
- [x] Confirmed Task2 depends only on Task1; no dependency on open Task4 channel-management UI or Task7 private-indicator inventory.
- [x] Scope kept to server mention fanout, store helpers, tests, Task2 docs, and current server/security docs.
- [x] RED tests written and verified failing for missing store helper, accepted client recipient ids, missing `@Everyone` history, missing rate limit, and reserved-token display-name leakage.
- [x] Implementation completed for server-computed `@Everyone` recipients, client recipient rejection, sender/channel rate limit, agent loop guard, and reserved-token parser behavior.
- [x] Affected package tests passed after implementation.
- [x] Rebased onto current `origin/main` after PR #947 and PR #945 landed; preserved the Task7 active row and added this Task2 row in shared task-status docs.
- [x] Full relevant verification passed.
- [ ] PR opened.
- [ ] CI passed.
- [ ] Merged and cleaned up.

## Verification Evidence

| Command | Result |
|---|---|
| `GOTMPDIR=/workspace/borgee-gotmp-m2-task2 go test -tags fts5 ./internal/store ./internal/api -run 'TestListEveryoneMentionTargetsUsesChannelMembership|TestEveryoneFanout'` | RED: failed on missing `ListEveryoneMentionTargets`, accepted client recipient ids, missing rate limit, and missing `message_mentions` rows |
| `GOTMPDIR=/workspace/borgee-gotmp-m2-task2 go test -tags fts5 ./internal/store -run 'TestCreateMessageFullDoesNotTreatEveryoneAsDisplayNameMention'` | RED: failed because legacy display-name parsing treated reserved `@Everyone` as user `Everyone` |
| `GOTMPDIR=/workspace/borgee-gotmp-m2-task2 go test -tags fts5 ./internal/store ./internal/api -run 'TestListEveryoneMentionTargetsUsesChannelMembership|TestEveryoneFanout'` | PASS after implementation |
| `GOTMPDIR=/workspace/borgee-gotmp-m2-task2 go test -tags fts5 ./internal/store ./internal/api` | PASS: `internal/store` and `internal/api` |
| `GOTMPDIR=/workspace/borgee-gotmp-m2-task2 go test -tags fts5 ./...` | PASS from `packages/server-go` after rebase onto current `origin/main` |
| `git diff --check` | PASS |
| `find . -name .gotmp -print` | PASS: no repo-local `.gotmp` artifact |

## Scope Locks

- Did not touch channel management UI, Settings channel management surface, or Task4 client files.
- Did not touch private indicator inventory or sidebar visual state files.
- Did not add schema migrations or notification subsystem rewrites.
