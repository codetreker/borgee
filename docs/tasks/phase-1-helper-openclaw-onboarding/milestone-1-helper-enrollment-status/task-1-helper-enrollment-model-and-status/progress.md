# Progress

## Resume

| Field | Value |
|---|---|
| Worktree | `.worktrees/task-1-helper-enrollment-model-and-status` |
| Branch | `feat/task-1-helper-enrollment-model-and-status` |
| PR | #934 |
| Owner | Dev |
| State | ACCEPTING |
| Blocker | none |

## Checkpoints

- [x] Worktree created
- [x] Four-piece baseline complete
- [x] Implementation design reviewed
- [x] Implementation complete
- [x] docs/current sync checked or N/A recorded
- [x] Acceptance evidence recorded through `bf-verification`
- [ ] PR merged

## Implementation Evidence

| Item | Evidence | Result |
|---|---|---|
| Scope check | `task.md`, `spec.md`, `stance.md`, and `acceptance.md` reviewed for alignment | PASS |
| Four-piece baseline | `spec.md`, `stance.md`, and `acceptance.md` created; `content-lock.md` N/A because no UI copy or DOM literal is locked | PASS |
| Implementation design review | Architect/PM/Security LGTM; QA blocker patch completed and focused QA re-review returned LGTM | PASS |
| TDD RED migration | `GOTMPDIR=$PWD/.gotmp go test -tags sqlite_fts5 ./internal/migrations -run HelperEnrollments` failed before production code with `undefined: helperEnrollments` | PASS |
| TDD RED store | `GOTMPDIR=$PWD/.gotmp go test -tags sqlite_fts5 ./internal/store -run HelperEnrollment` failed before production code with missing `CreateHelperEnrollment`/claim/status helpers and error symbols | PASS |
| TDD RED API | `GOTMPDIR=$PWD/.gotmp go test -tags sqlite_fts5 ./internal/api -run HelperEnrollment` failed before production code with new Helper endpoints returning `404 Not found` | PASS |
| Implementation | Added v49 `helper_enrollments` migration, store helpers, redacted serializers, user rail routes, Helper claim/status/uninstall rail, and server route wiring | PASS |
| Current-doc sync | Updated `docs/current/server/data-model-and-migrations.md`, `docs/current/server/api-auth-admin-rails.md`, `docs/current/security/README.md`, `docs/current/host-bridge/README.md`, and `docs/current/remote-agent/README.md`; no listed doc was N/A | PASS |
| Migration version re-grep | `rg "Version:\s*[0-9]+" packages/server-go/internal/migrations -o | sed -E 's/.*Version:\s*([0-9]+).*/\1/' | sort -n | tail -12` -> max `49`; helper enrollment migration owns v49 | PASS |
| GREEN migration | `GOTMPDIR=$PWD/.gotmp go test -count=1 -tags sqlite_fts5 ./internal/migrations -run HelperEnrollments` -> `ok borgee-server/internal/migrations 0.007s` | PASS |
| GREEN store | `GOTMPDIR=$PWD/.gotmp go test -count=1 -tags sqlite_fts5 ./internal/store -run HelperEnrollment` -> `ok borgee-server/internal/store 0.049s` | PASS |
| GREEN API | `GOTMPDIR=$PWD/.gotmp go test -count=1 -tags sqlite_fts5 ./internal/api -run HelperEnrollment` -> `ok borgee-server/internal/api 0.069s` | PASS |
| Regression adjacency | `GOTMPDIR=$PWD/.gotmp go test -count=1 -tags sqlite_fts5 ./internal/api -run 'Remote\|HostGrants\|AgentStatus'` -> `ok borgee-server/internal/api 0.111s` | PASS |
| Reverse grep rail separation | `rg "helper.*remote_nodes|remote_nodes.*helper|connection_token.*helper|helper.*connection_token" packages/server-go/internal` and `rg "helper.*host_grants|host_grants.*helper|helper.*user_permissions|user_permissions.*helper" packages/server-go/internal` returned no hits | PASS |
| Reverse grep scope | `rg "job queue|result schema|execute job|arbitrary shell|service manager|\blease\b" packages/server-go/internal/api/helper_enrollments.go packages/server-go/internal/store/helper_enrollment_queries.go packages/server-go/internal/migrations/helper_enrollments.go` returned no hits | PASS |
| Sensitive-field review | `rg "enrollment_secret|helper_credential|persistent_credential_digest|credential_digest" packages/server-go/internal/api packages/server-go/internal/store` showed raw secrets only in create/claim request-response handlers and tests; digests use `json:"-"` model fields and are not serialized | PASS |
| QA verification | QA re-ran diff checks, migration/store/API Helper tests, Remote/HostGrants/AgentStatus adjacency tests, route smoke command, and reverse-grep rail/scope checks | PASS |
| Security review | Independent Security review verified user auth/owner-org scoping, Helper-only credential rail, digest storage/constant-time compare, terminal deny states, serializer redaction, and no scope creep | PASS |
| Diff hygiene | `git diff --check` completed with no output | PASS |
| Broad package suite note | Earlier broad `GOTMPDIR=$PWD/.gotmp go test -count=1 -tags sqlite_fts5 ./internal/migrations ./internal/store ./internal/api ./internal/server` passed migrations/store/server but `./internal/api` failed with existing concurrent suite `sql: database is closed`/missing-table errors unrelated to HelperEnrollment tests; no broad-suite pass is claimed here | INFO |

## Acceptance Evidence

| Check | Evidence | Result |
|---|---|---|
| Segment A - distinct Helper enrollment identity | Migration/store/API tests cover distinct `helper_enrollments` row, one-time secret claim, persistent Helper credential issuance, single-use claim, and no Remote Agent token fallback | PASS |
| Segment B - owner/org/host binding | Store tests prove owner/org stamping; API tests prove owner-scoped CRUD and wrong-owner `403`; serializers omit `org_id` | PASS |
| Segment C - allowed category shape | Migration/schema uses category JSON; store/API tests reject unknown categories such as `shell`; no queue/job/payload implementation added | PASS |
| Segment D - visible Helper status | Store/API tests cover pending, connected heartbeat, offline freshness recovery for same valid credential/device, revoked/uninstalled terminal states, and stale-device failures without `last_seen_at` mutation | PASS |
| Segment E - Remote Agent rail separation | API test rejects user token, Remote Agent connection token, and host grant id as Helper status authority; reverse grep found no Helper auth reuse of remote nodes, host grants, or user permissions | PASS |
| Segment F - current-doc sync | Current docs updated for server data model, API rails, security boundaries, Host Bridge Helper enrollment identity/status, and Remote Agent separation | PASS |

Verifier: QA verification + independent Security review
Date: 2026-05-15
Scope: API/data/security/current-doc
Fixtures: `testutil.NewTestServer` owner/member users, store migrated template, Remote Node/Host Grant separation fixtures; secrets redacted
Out-of-scope findings: Broad `./internal/api` package run still needs separate stabilization; targeted task acceptance and rail-adjacency tests pass.
Decision: LGTM for pre-PR acceptance evidence; broad `./internal/api` full-suite instability is unrelated and not used as acceptance evidence
