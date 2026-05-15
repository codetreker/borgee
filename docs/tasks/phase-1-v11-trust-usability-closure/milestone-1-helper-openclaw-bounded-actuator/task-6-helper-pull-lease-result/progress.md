# Progress

## Resume

| Field | Value |
|---|---|
| Worktree | `.worktrees/task-6-helper-pull-lease-result` |
| Branch | `feat/task-6-helper-pull-lease-result` |
| PR | not opened |
| Owner | Blueprintflow tasking worker under Teamlead |
| State | IMPLEMENTED_READY_FOR_REVIEW |
| Blocker | none |

## Checkpoints

- [x] Worktree/branch created from `origin/main` at `10e79bf`.
- [x] `AGENTS.md` reviewed.
- [x] Task, milestone, accepted history, shared task index, and blueprint anchor docs reviewed.
- [x] Executability verified: task 6 is READY/TASKING after accepted PR #934/#936/#937/#938/#939 and is not blocked by another unaccepted task.
- [x] Shared Blueprintflow state refreshed for task 6 TASKING while task 7 remains READY; no task 7 files touched.
- [x] Four-piece task-start docs created: `spec.md`, `stance.md`, `acceptance.md`, `progress.md`.
- [x] `content-lock.md` checked N/A because task-start scope has no UI copy, DOM selectors, or product-facing content literals.
- [x] Product implementation deliberately not started in task-start commit scope.
- [x] Dev and Security scouting inputs produced from server Helper jobs, Helper enrollment credential rail, outbound prereq code, current docs, and accepted task 4/task 5 designs.
- [x] Dev design drafted in `design.md`.
- [x] Progress advanced to READY_FOR_DESIGN_REVIEW.
- [x] Design gate accepted by PM/QA, Architecture, and Security: `PM_QA_DESIGN_LGTM_BOTH`, `ARCHITECT_TASK6_DESIGN_LGTM`, `SECURITY_TASK6_DESIGN_LGTM`.
- [x] Progress advanced through READY_FOR_IMPL to IMPLEMENTING.
- [x] RED tests written and failure evidence captured.
- [x] Implementation completed for Helper poll/lease/ack/result and outbound client.
- [x] Focused and broader verification run.
- [x] Acceptance state updated with RED/GREEN evidence.
- [x] Local commit created; no push/PR opened.

## Implementation Evidence

| Item | Evidence | Result |
|---|---|---|
| Design gate | PM/QA, Architecture, and Security gate signals provided in worker dispatch: `PM_QA_DESIGN_LGTM_BOTH`, `ARCHITECT_TASK6_DESIGN_LGTM`, `SECURITY_TASK6_DESIGN_LGTM` | PASS |
| Progress state | Advanced task from READY_FOR_DESIGN_REVIEW through READY_FOR_IMPL to IMPLEMENTING before product code changes | PASS |
| RED: server/store | `GOTMPDIR=$PWD/.gotmp go test ./internal/api ./internal/store -run 'TestHelperJobsPollAckResultWithHelperCredential|TestHelperJobPollAckResultLeaseIdempotencyAndBoundaries|TestHelperJobHelperAuthorityAndExpiryFailures'` failed before implementation: store compile errors for missing `PollAndLeaseHelperJobForHelper`, `PollHelperJobInput`, `HelperJobStatusLeased`, `AckHelperJobForHelper`, and related task 6 symbols. The same run also exposed that server DB-backed tests need the repo's SQLite FTS5 build tag in this environment (`no such module: fts5`). | RED CAPTURED |
| RED: helper outbound | `GOTMPDIR=$PWD/.gotmp go test ./internal/outbound -run 'TestClientPollAckResultUseFixedPathsAndHelperCredential|TestClientMapsNoWorkTransientAndStopDirectives|TestClientRejectsFullURLOrTraversalIdentifiers'` failed before implementation with missing `NewClient`, `StaticCredentialSource`, `WithHTTPClient`, `PollOptions`, `PollStatusLeased`, `DirectiveProcess`, `ResultRequest`, and `ResultSummary`. | RED CAPTURED |
| GREEN: focused store | `GOTMPDIR=$PWD/.gotmp go test -tags fts5 ./internal/store -run 'TestHelperJobPollAckResultLeaseIdempotencyAndBoundaries|TestHelperJobHelperAuthorityAndExpiryFailures'` | PASS |
| GREEN: focused API | `GOTMPDIR=$PWD/.gotmp go test -tags fts5 ./internal/api -run 'TestHelperJobsPollAckResultWithHelperCredential'` | PASS |
| GREEN: broader server | `GOTMPDIR=$PWD/.gotmp go test -tags fts5 ./internal/api ./internal/datalayer ./internal/store ./internal/migrations` | PASS |
| GREEN: helper | `GOTMPDIR=$PWD/.gotmp go test ./internal/outbound ./install ./cmd/borgee-helper` | PASS |
| Whitespace | `git diff --check` | PASS |
| Local commit | Local commit created in this worktree; no push or PR opened by worker | PASS |
| Scope guard | Implementation stayed on Helper poll/lease/ack/result transport, Helper outbound client shape, and docs/current sync. It did not implement task 7 local policy/manifest/sandbox evaluation, OpenClaw action execution, bounded log upload, service lifecycle restart, sudo cache, or Remote Agent rail reuse. | PASS |

## Review Blocker Repair Evidence

| Item | Evidence | Result |
|---|---|---|
| Repair RED: stale/uninstall settlement | `GOTMPDIR=$PWD/.gotmp go test -tags sqlite_fts5 ./internal/store -run 'TestHelperJob(StaleCredentialSettlesActiveJobsAndCurrentCredentialCanPoll\|UninstallSettlementCoversRunningJob)$' -count=1 -v` failed before repair: stale credential poll/ack/result left leased/running jobs active with no `stale_credential` terminal settlement; uninstall result left a running job active instead of `cancelled`/`uninstalled`. | RED CAPTURED |
| Repair RED: helper stop directives | `go test ./internal/outbound -run TestClientAckAndResultMapCredentialStopDirectives -count=1 -v` failed before repair with `got.Directive undefined (type JobState has no field or method Directive)`, proving ack/result could not return daemon-loop stop directives. | RED CAPTURED |
| Repair GREEN: stale/uninstall settlement | `GOTMPDIR=$PWD/.gotmp go test -tags sqlite_fts5 ./internal/store -run 'TestHelperJob(StaleCredentialSettlesActiveJobsAndCurrentCredentialCanPoll\|UninstallSettlementCoversRunningJob)$' -count=1 -v` passed after repair. | PASS |
| Repair GREEN: helper stop directives | `GOTMPDIR=$PWD/.gotmp go test ./internal/outbound -run TestClientAckAndResultMapCredentialStopDirectives -count=1 -v` passed after repair. | PASS |
| Focused server store/API | `GOTMPDIR=$PWD/.gotmp go test -tags sqlite_fts5 ./internal/store ./internal/api -count=1` passed. | PASS |
| Helper outbound package | `GOTMPDIR=$PWD/.gotmp go test ./internal/outbound -count=1` passed. | PASS |
| Broader server packages | `GOTMPDIR=/workspace/.gotmp-borgee-task6 go test -p 1 -tags sqlite_fts5 ./... -count=1` passed. A first broad attempt with `GOTMPDIR` inside `packages/server-go/.gotmp` failed because repo-walking tests inspected Go build scratch files, so the final passing run uses a temp dir outside the repo. | PASS |
| Broader Helper packages | `GOTMPDIR=/workspace/.gotmp-borgee-task6 go test ./... -count=1` passed. | PASS |
| Whitespace | `git diff --check` passed. | PASS |
| Build tag note | The server verification used `sqlite_fts5`, matching `packages/server-go/Makefile`; prior notes that mention `fts5` are historical evidence from the original implementation pass, not the tag used for this repair gate. | PASS |

## Task-Prep Evidence

| Item | Evidence | Result |
|---|---|---|
| Base | `origin/main` resolved to `10e79bf` (`docs(tasks): coarsen v1.1 phase plan`) before worktree creation | PASS |
| Worktree/branch | Created `/workspace/borgee/.worktrees/task-6-helper-pull-lease-result` on `feat/task-6-helper-pull-lease-result` tracking `origin/main` | PASS |
| Required instructions | Read `AGENTS.md`; kept parent Teamlead git/gh restriction as worker-owned git operations | PASS |
| Required task docs | Read task 6 `task.md`, Milestone 1 `milestone.md`, `accepted-history.md`, shared `docs/tasks/README.md`, Phase 1 `phase-plan.md`, and `docs/blueprint/next/README.md` | PASS |
| Blueprint anchors | Read `remote-actuator-design.md` sections 1.2, 6, 8, and 10 | PASS |
| Dependency state | Verified accepted history records PR #934 (`547f869`), PR #936 (`1ca5f95`), PR #937 (`2872905`), PR #938 (`64d56f1`), and PR #939 (`96dc0dc`) | PASS |
| Task 6 unlock | Verified task 6 depends on accepted task 5 only; milestone and shared task index list task 6 READY after PR #939 | PASS |
| Shared state | Updated shared state so task 6 is TASKING from this worktree/branch and task 7 remains READY | PASS |
| Task 7 ownership | No files under `task-7-local-policy-manifest-and-sandbox-profile/` changed | PASS |
| Four-piece | Created task-start `spec.md`, `stance.md`, and `acceptance.md`; this file records progress and content-lock N/A | PASS |
| Product code | No product code changes made in task-start commit scope | PASS |

## Design Scout Evidence

| Item | Evidence | Result |
|---|---|---|
| Server API | Inspected `packages/server-go/internal/api/helper_jobs.go`; existing `HelperJobsHandler` mounts only user-rail enqueue, so task 6 must add Helper-credential poll/ack/result routes without changing enqueue auth | PASS |
| Datalayer | Inspected `packages/server-go/internal/datalayer/helper_jobs.go` and `helper_jobs_sqlite.go`; task 6 should extend `HelperJobRepository` instead of importing store from API | PASS |
| Store model | Inspected `packages/server-go/internal/store/helper_job_queries.go`, `models.go`, and migration v51; existing table already includes `leased_at`, `lease_expires_at`, `completed_at`, `failure_code`, `failure_message`, and `result_summary_json` | PASS |
| Credential rail | Inspected `packages/server-go/internal/api/helper_enrollments.go`, store enrollment queries, and datalayer enrollment tests; Helper routes use bearer Helper credential plus `helper_device_id`, separate from user/authMw rails | PASS |
| Outbound prereq | Inspected `packages/borgee-helper/internal/outbound/prereq.go`, prereq tests, daemon startup, and install asset tests; task 6 client should consume `PreparedConfig` and fixed relative paths without adding Remote Agent/service lifecycle/sudo flags | PASS |
| Current docs | Inspected `docs/current/host-bridge/helper-daemon.md`, `docs/current/host-bridge/README.md`, `docs/current/security/README.md`, and `docs/current/known-gaps.md`; design names required docs/current sync targets | PASS |
| Accepted dependency docs | Reused accepted task 4 and task 5 designs for enqueue authority and outbound prerequisite boundaries | PASS |
| Design coverage | `design.md` includes API/route shape, datalayer/store model, Helper poll client shape, auth/credential checks, lease/result statuses, idempotency/retry/cancellation, stale/revoke settlement, RED test plan, docs/current sync, and non-goals | PASS |
| Task 7 ownership | Design keeps local policy/manifest/sandbox execution as handoff/non-goal and does not edit task 7 files | PASS |

## Scope Locks

- In scope: outbound Helper poll/long-poll retrieval, lease, ack, result upload, retry/backoff/idempotency/cancellation semantics, stale credential/revoke settlement, and interface handoff to local policy/status tasks.
- Out of scope: local policy execution, manifest/artifact enforcement, sandbox allowlist expansion, OpenClaw action, Configure OpenClaw UI closure, service lifecycle restart/boot/crash, sudo cache, privileged long-lived service behavior, and Remote Agent rail reuse.

## Acceptance State

Task 6 is IMPLEMENTED_READY_FOR_REVIEW after accepted design review. `content-lock.md` remains N/A for this scope. Helper outbound poll/lease/ack/result transport, atomic lease transitions, idempotent ack/result, bounded terminal metadata, stale/revoked/uninstalled/expired settlement, Helper outbound client shape, and docs/current sync are implemented. Task 7 local policy/manifest/sandbox work, OpenClaw execution, bounded log upload, service lifecycle, and Remote Agent rail reuse remain out of scope.
