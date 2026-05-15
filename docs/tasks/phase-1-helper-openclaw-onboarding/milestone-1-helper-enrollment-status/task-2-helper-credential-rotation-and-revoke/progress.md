# Progress

## Resume

| Field | Value |
|---|---|
| Worktree | `.worktrees/task-2-helper-credential-rotation-and-revoke` |
| Branch | `feat/task-2-helper-credential-rotation-and-revoke` |
| PR | #936 |
| Owner | Dev/Writer helper under Teamlead |
| State | READY_FOR_PR |
| Blocker | Coverage gate still exits 1 because `internal/migrations` remains 69.9% under the 70% package threshold and unrelated critical uncovered ranges remain outside task-2 helper enrollment test ownership; task-owned helper rotation critical range is fixed. |

Review note: Design gate returned ARCHITECT_LGTM, PM_LGTM, SECURITY_LGTM, and QA_LGTM_REFRESH after the QA_BLOCKED patch that added positive post-rotation authority coverage for the new rotated credential plus same device.

## Process Repair Carried By This PR

PR #935 was closed before landing the shared task-1 acceptance-state cleanup. This task-2 PR therefore carries the remediation that belongs in the next real task PR:

- Task 1 is marked accepted through PR #934 and merge commit `547f869`.
- Stale task-1 Active Task Resume state is cleared from shared task docs.
- Task 2 is marked TASKING and is not marked accepted.
- Task 3 remains READY and parallel after task 1, subject to file ownership/conflict checks.
- AGENTS.md records the process rule that one task PR carries all task-related four-piece, Dev design, implementation, tests, docs/current sync, progress, and acceptance state; no closure/status follow-up PR should be opened for state that belongs to the task.

## Checkpoints

- [x] Worktree/branch confirmed
- [x] Required task, milestone, shared task, AGENTS, and blueprint anchor docs reviewed
- [x] Process repair added to AGENTS.md
- [x] Shared state remediated for task 1 accepted / task 2 TASKING / task 3 READY
- [x] Four-piece baseline complete (`task.md`, `spec.md`, `stance.md`, `acceptance.md`)
- [x] `content-lock.md` checked N/A because task 2 has no UI copy or DOM literals
- [x] Dev design drafted for review
- [x] Dev design reviewed
- [x] TDD RED tests written before implementation
- [x] Implementation complete
- [x] docs/current sync checked or N/A recorded after implementation
- [x] Acceptance evidence recorded
- [ ] PR opened
- [ ] PR merged

## Current Evidence

| Item | Evidence | Result |
|---|---|---|
| Dependency base | Task 1 merge commit present at branch base: `547f869` (`feat(helper): add enrollment status foundation (#934)`) | PASS |
| Required anchor read | `task.md`, milestone, `docs/tasks/README.md`, `AGENTS.md`, `remote-actuator-design.md` §1.2/§5/§10, and `migration-analysis.md` §6.1 reviewed | PASS |
| Task 2 boundary | Four-piece and design preserve no typed job execution and no Remote Agent/host grant/user permission fallback | PASS |
| Content lock | No UI copy or DOM literals identified in task 2 tasking/design scope | N/A |

## Implementation Evidence

| Item | Evidence | Result |
|---|---|---|
| Design review | ARCHITECT_LGTM, PM_LGTM, SECURITY_LGTM, and QA_LGTM_REFRESH after positive rotated-credential coverage patch | PASS |
| Review blocker | ARCHITECT_BLOCKED found heartbeat/uninstall final writes could race with credential rotation and revoke could overwrite uninstall; QA found stale current-doc wording around rotation | FIXED |
| RED migrations | `GOTMPDIR=$PWD/.gotmp go test -count=1 -tags sqlite_fts5 ./internal/migrations -run HelperEnrollments` failed before production code with `undefined: helperCredentialRotation` | PASS |
| RED store | `GOTMPDIR=$PWD/.gotmp go test -count=1 -tags sqlite_fts5 ./internal/store -run HelperEnrollmentCredentialRotation` failed before production code with missing `RotateHelperEnrollmentCredential`, `CredentialRotatedAt`, and `CredentialGeneration` | PASS |
| RED datalayer | `GOTMPDIR=$PWD/.gotmp go test -count=1 -tags sqlite_fts5 ./internal/datalayer -run HelperEnrollmentRepository` failed before production code with `repo.RotateCredential undefined` | PASS |
| RED API | `GOTMPDIR=$PWD/.gotmp go test -count=1 -tags sqlite_fts5 ./internal/api -run HelperEnrollment` failed before production code with rotate route returning `404 Not found` | PASS |
| RED review blocker | `GOTMPDIR=$PWD/.gotmp go test -count=1 -tags sqlite_fts5 ./internal/store -run 'HelperEnrollment(CredentialRotation|TerminalRace|RevokeDoesNotOverwrite)'` failed before the blocker fix with missing `helperEnrollmentCredentialRaceHook` and `helperEnrollmentRevokeRaceHook` seams | PASS |
| Implementation | Added v50 helper credential rotation migration, store rotation transaction, datalayer repository method, Helper-rail API route, rotation metadata serialization, conditional heartbeat/uninstall writes bound to the validated credential digest/device, and terminal-safe revoke update | PASS |
| GREEN migrations | `GOTMPDIR=$PWD/.gotmp go test -count=1 -tags sqlite_fts5 ./internal/migrations -run HelperEnrollments` -> `ok borgee-server/internal/migrations 0.063s` | PASS |
| GREEN store race slice | `GOTMPDIR=/var/tmp go test -count=1 -tags sqlite_fts5 ./internal/store -run 'HelperEnrollment(CredentialRotation|TerminalRace|RevokeDoesNotOverwrite)'` -> `ok borgee-server/internal/store 0.058s` in QA refresh; `/tmp` without `GOTMPDIR` is not executable in this runtime | PASS |
| GREEN store full Helper slice | `GOTMPDIR=$PWD/.gotmp go test -count=1 -tags sqlite_fts5 ./internal/store -run HelperEnrollment` -> `ok borgee-server/internal/store 0.067s` | PASS |
| GREEN store helper coverage refresh | `GOTMPDIR=/var/tmp go test -count=1 -tags sqlite_fts5 ./internal/store -run HelperEnrollment` -> `ok borgee-server/internal/store 0.067s` after adding missing-id rotation coverage | PASS |
| GREEN datalayer | `GOTMPDIR=$PWD/.gotmp go test -count=1 -tags sqlite_fts5 ./internal/datalayer -run HelperEnrollmentRepository` -> `ok borgee-server/internal/datalayer 0.047s` | PASS |
| GREEN API | `GOTMPDIR=$PWD/.gotmp go test -count=1 -tags sqlite_fts5 ./internal/api -run HelperEnrollment` -> `ok borgee-server/internal/api 0.062s` | PASS |
| Touched package breadth | `GOTMPDIR=$PWD/.gotmp go test -count=1 -tags sqlite_fts5 ./internal/migrations ./internal/store ./internal/datalayer ./internal/api` passed migrations/store/datalayer, then `./internal/api` failed in existing broad-suite instability with `sql: database is closed` and missing-table errors outside Helper enrollment tests; no broad API pass is claimed | INFO |
| DL boundary | `GOTMPDIR=$PWD/.gotmp go test -count=1 -tags sqlite_fts5 ./internal/api -run TestDL12_DirectStoreImportBaseline` -> `ok borgee-server/internal/api 0.007s` | PASS |
| Migration version re-grep | `rg "Version:\s*[0-9]+" internal/migrations -o | sed -E 's/.*Version:\s*([0-9]+).*/\1/' | sort -n | tail -12` -> max `50`; helper credential rotation migration owns v50 | PASS |
| Reverse grep rail separation | `rg "helper.*remote_nodes|remote_nodes.*helper|connection_token.*helper|helper.*connection_token" internal/api internal/store internal/datalayer internal/migrations` and `rg "helper.*host_grants|host_grants.*helper|helper.*user_permissions|user_permissions.*helper" internal/api internal/store internal/datalayer internal/migrations` returned no hits | PASS |
| Reverse grep scope | `rg "job queue|result schema|execute job|arbitrary shell|service manager|\blease\b" internal/api/helper_enrollments.go internal/store/helper_enrollment_queries.go internal/datalayer/helper_enrollments.go internal/datalayer/helper_enrollments_sqlite.go internal/migrations/helper_credential_rotation.go` returned no hits | PASS |
| Diff hygiene | `git diff --check` completed with no output | PASS |
| Coverage gate refresh | `CI=true THRESHOLD_TOTAL=85 THRESHOLD_FUNC=50 THRESHOLD_PACKAGE=70 THRESHOLD_PRINT=85 BUILD_TAGS='sqlite_fts5 race_heavy' COVERPROFILE=coverage.out FAIL_ON_CRITICAL_BLOCKS=false RACE_DETECTION=false GOTMPDIR=/var/tmp go run ./scripts/lib/coverage/` -> exit 1; total 85.6%, `internal/store` 89.2%, `RotateHelperEnrollmentCredential` 81.1%, previous task-owned critical range `internal/store/helper_enrollment_queries.go:(261:66)-(265:14)` no longer listed; remaining blocker is `internal/migrations` 69.9% under the 70% package threshold plus unrelated critical ranges outside task-2 helper enrollment store-test ownership | BLOCKED |
| docs/current sync | Updated current server data model/migrations, API/auth rails, security rail matrix/diagram, Host Bridge, and Remote Agent separation docs for implemented rotation behavior and current-credential semantics | PASS |

## Acceptance State

Task 2 implementation is READY_FOR_PR. It is not marked accepted until PR review and merge complete.

## Acceptance Evidence

| Check | Evidence | Result |
|---|---|---|
| Segment A - credential rotation lifecycle | Store/datalayer/API tests cover current credential + matching device rotation, raw new credential returned once, digest replacement, generation/rotated metadata, preserved `credential_created_at`, old credential stale for heartbeat/rotate/uninstall, and new credential positive heartbeat/uninstall | PASS |
| Segment B - stale credential and device semantics | Store/API tests cover wrong credential, old credential, wrong device, pending, revoked, and uninstalled rotation failures without authority mutation; race tests prove stale-after-validation credential changes cannot mutate heartbeat/uninstall state; new credential plus same device updates heartbeat/freshness | PASS |
| Segment C - revoke authority | Store tests prove revoked rows reject rotation and keep terminal revoke timestamp/status; revoke race test proves revoke does not overwrite helper-originated uninstall if uninstall wins after revoke's read; API/user rail revoke remains owner/org-scoped from task 1 | PASS |
| Segment D - helper-originated uninstall authority | Store/datalayer/API tests prove uninstall requires the current rotated credential plus same device, remains terminal for later heartbeat, and fails inactive when terminal state wins before the final conditional update | PASS |
| Segment E - API/data-model and rail separation | API uses `datalayer.HelperEnrollmentRepository`; DL baseline passes; reverse-grep checks show no Remote Agent, host grant, or user permission fallback; no queue/lease/job/service execution terms added to implementation paths | PASS |
| Segment F - current-doc sync and progress state | Current docs updated for rotation behavior and progress records explicit RED, GREEN, docs, and acceptance evidence in this task PR | PASS |
