# Dev Design: Helper Pull / Lease / Result

## 1. Boundary And Approach

This task adds the first Helper-originated job transport loop after accepted enqueue and outbound service prerequisites. It owns Helper poll or long-poll retrieval, lease claim, ack/result upload, retry/backoff/idempotency semantics, cancellation, stale credential handling, and deterministic revoke/uninstall settlement before local policy or action.

The implementation should extend the existing Helper job path rather than creating a second queue rail:

- Server API: `packages/server-go/internal/api/helper_jobs.go`
- Datalayer interface and SQLite adapter: `packages/server-go/internal/datalayer/helper_jobs.go` and `helper_jobs_sqlite.go`
- Store/state machine: `packages/server-go/internal/store/helper_job_queries.go` plus the existing `helper_jobs` table from migration v51
- Helper client/config: `packages/borgee-helper/internal/outbound` and `packages/borgee-helper/cmd/borgee-helper/main.go`
- Route wiring: `packages/server-go/internal/server/server.go`, near `HelperEnrollmentHandler` and `HelperJobsHandler`

Existing accepted work already reserves `helper_jobs.status` values `queued`, `leased`, `running`, `succeeded`, `failed`, `cancelled`, and `expired`, plus `leased_at`, `lease_expires_at`, `completed_at`, `failure_code`, `failure_message`, and `result_summary_json`. Task 6 should fill those fields through Helper-originated transition methods. Do not add local policy evaluation, OpenClaw execution, sandbox allowlist changes, service lifecycle operations, UI closure, or Remote Agent fallback.

## 2. Dev Scouting Input

Current code shape found during scouting:

- `HelperJobsHandler` currently mounts only user-rail enqueue: `POST /api/v1/helper/enrollments/{enrollmentId}/jobs` behind `authMw`.
- `HelperEnrollmentHandler` already exposes Helper-credential endpoints for claim/status/rotate/uninstall using `Authorization: Bearer <helper_credential>` plus request `helper_device_id`.
- `helperCredentialFromRequest` is local to `helper_enrollments.go`; task 6 should move or duplicate the minimal helper-credential extraction in a shared helper only if needed without changing user-rail auth behavior.
- `HelperJobRepository` currently has only `EnqueueForUser`; add task 6 methods to the same interface so API stays datalayer-backed and `internal/api` does not import `internal/store`.
- Store enqueue already checks claimed enrollment, owner/org, credential presence, freshness, allowed category, closed taxonomy, TTL, idempotency, and stale/revoke/uninstall at enqueue time. Task 6 must repeat Helper credential/device/status checks at poll, lease, ack, and result time because authority can change after enqueue.
- The existing migration already has lease/result columns. Prefer no schema migration unless Dev implementation proves a missing durable field is required. If a new field becomes necessary, append a new forward-only migration and do not mutate v51.
- Current API tests intentionally assert poll/lease/result/ack routes are unmounted. Task 6 implementation must replace those negative route expectations with positive task 6 behavior and keep unrelated later routes unmounted.
- Helper outbound prerequisite code validates exact server origin and state dirs but makes no HTTP requests. Task 6 should add a narrow client in `internal/outbound` that consumes `PreparedConfig`; installed service assets should not grow `--remote-agent`, `--reverse-ws`, `--restart-service`, arbitrary URL, or local-policy flags.

## 3. Security Scouting Input

Security-sensitive findings and required guardrails:

- Helper credentials are not user tokens. Helper poll/lease/ack/result routes must not use `authMw`; they must authenticate with the current persistent Helper credential digest and `helper_device_id` on the Helper enrollment rail.
- User tokens, agent API keys, Remote Agent node tokens, host grants, admin sessions, and user permissions must not authenticate Helper poll/lease/ack/result routes.
- Poll and lease must verify enrollment id, owner/org from the stored enrollment/job rows, helper device id, current credential digest, non-terminal enrollment status, claimed state, and not revoked/uninstalled before returning work.
- Credential rotation makes the old credential stale immediately. Old credentials must receive a deterministic stale/unauthorized response and must not lease, ack, or result jobs.
- Revoke/uninstall wins over queued and leased-before-action jobs. Poll should stop returning work; lease/ack/result should settle cancellable work as `cancelled` or `failed` with `revoked`, `stale_credential`, `cancelled`, `ttl_expired`, or `lease_lost` as appropriate.
- The Helper response payload may include normalized typed job payload needed for later policy handoff, but must not include raw credentials, credential digests, owner/org internals, private file content, private messages, full environment dumps, unbounded logs, or Remote Agent data.
- Result upload accepts terminal metadata only. It must reject arbitrary paths, domains, service unit names, raw command text, scripts, executable paths, token fields, and oversized logs. Bounded log upload itself remains task 8 unless task 6 needs small redacted references in `result_summary_json`.
- Outbound client construction must use `PreparedConfig.ServerOrigin` plus fixed relative paths only. Job payloads, manifest content, Remote Agent state, host grants, and user input cannot supply full URLs.

## 4. API / Route Shape

Add Helper-credential routes beside the existing enqueue handler. Tentative shape:

```text
POST /api/v1/helper/enrollments/{enrollmentId}/jobs/poll
POST /api/v1/helper/enrollments/{enrollmentId}/jobs/{jobId}/ack
POST /api/v1/helper/enrollments/{enrollmentId}/jobs/{jobId}/result
```

`poll` may implement short poll first, with long-poll semantics represented by a bounded `wait_ms` request field if implementation can do it without adding goroutine or cancellation risk. If long-poll waits are added, cap the server wait and honor request context cancellation. A separate `/lease` route is not required if `poll` atomically returns a leased job; atomic poll-and-lease is the simpler v1 shape and avoids a retrieved-but-unleased window. If implementation chooses split poll/lease, design review must verify the same atomicity and duplicate-execution guarantees.

All Helper routes use:

```http
Authorization: Bearer <current helper credential>
Content-Type: application/json
```

All request bodies include `helper_device_id`. They do not accept owner, org, category, arbitrary URLs, paths, service IDs, or local policy decisions.

Poll request:

```json
{
  "helper_device_id": "device-1",
  "wait_ms": 0
}
```

Poll response when no work is available:

```json
{
  "status": "no_work",
  "retry_after_ms": 5000
}
```

Poll response with an atomically leased job:

```json
{
  "status": "leased",
  "job": {
    "job_id": "job-id",
    "enrollment_id": "enrollment-id",
    "job_type": "openclaw.configure_agent",
    "schema_version": 1,
    "payload": {"agent_id":"agent-id","config_schema_version":1,"config_hash":"sha256:..."},
    "manifest_digest": "sha256:...",
    "lease_token": "opaque-server-token",
    "lease_expires_at": 1760000000000,
    "attempt": 1
  }
}
```

Ack request:

```json
{
  "helper_device_id": "device-1",
  "lease_token": "opaque-server-token",
  "ack_status": "received"
}
```

Ack marks `leased` -> `running` after basic envelope receipt only. Ack does not mean local policy allowed the job and does not prove execution success.

Result request:

```json
{
  "helper_device_id": "device-1",
  "lease_token": "opaque-server-token",
  "status": "failed",
  "failure_code": "policy_denied",
  "failure_message": "redacted short reason",
  "result_summary": {
    "audit_refs": ["local-audit-id"],
    "log_refs": []
  }
}
```

Allowed result statuses for task 6 are `succeeded`, `failed`, `cancelled`, and `expired`. Implementation may reject `succeeded` until local policy/action tasks exist if accepting success would overstate execution behavior. If success is accepted for test fixtures, the design must keep it a transport result only and not Configure OpenClaw success.

Required HTTP mapping:

- `200`: no work, lease returned, ack accepted, idempotent ack/result replay accepted, or terminal settlement returned.
- `400`: malformed JSON, unknown fields, invalid result status, invalid failure code, oversized result summary, forbidden fields.
- `401`: missing or wrong Helper credential.
- `403`: wrong helper device id, revoked/uninstalled enrollment, stale credential after rotation, wrong enrollment state, or rail mismatch.
- `404`: job id absent under the authenticated enrollment.
- `409`: lease token mismatch, lease lost, job terminal conflict, or result conflicts with already-recorded terminal state.

Use deterministic JSON error codes for Helper routes, matching the existing `writeJSONErrorCode` pattern. Required codes include `unauthorized`, `device_mismatch`, `revoked`, `uninstalled`, `stale_credential`, `no_work`, `lease_lost`, `ttl_expired`, `cancelled`, `terminal_conflict`, `schema_invalid`, `forbidden_field`, and `not_found`.

## 5. Datalayer / Store Model

Extend `datalayer.HelperJobRepository` with task 6 methods:

```go
PollAndLeaseForHelper(ctx context.Context, input HelperJobPollInput, now time.Time) (*HelperJobLease, HelperJobPollDirective, error)
AckForHelper(ctx context.Context, input HelperJobAckInput, now time.Time) (*HelperJob, error)
CompleteForHelper(ctx context.Context, input HelperJobResultInput, now time.Time) (*HelperJob, error)
```

The exact names can change, but responsibilities should stay separated:

- `PollAndLeaseForHelper` authenticates Helper enrollment authority, expires stale active jobs, settles revoked/cancelled queued work, atomically selects one queued job for that enrollment/device, writes `leased_at`, `lease_expires_at`, `status='leased'`, and returns a bounded lease projection.
- `AckForHelper` authenticates the same Helper rail, checks `lease_token` if implemented, verifies non-expired lease ownership, and moves `leased` -> `running` idempotently.
- `CompleteForHelper` authenticates the same Helper rail, verifies lease ownership and non-terminal state, validates terminal status/failure/result summary, writes `status`, `failure_code`, `failure_message`, `result_summary_json`, `completed_at`, clears `active_idempotency_scope`, and records `updated_at`.

The existing table does not have a `lease_token` column. There are two acceptable implementation paths:

- Preferred if no migration is needed: derive an opaque lease token from server-side row state and a server secret that is never stored in the row, such as an HMAC over `job_id`, `enrollment_id`, `helper_device_id`, `leased_at`, and `lease_expires_at`. The verifier recomputes it from row state.
- If the codebase has no suitable server secret or deterministic token helper, append a new migration with `lease_token_digest TEXT` and store only a digest. Do not store raw lease tokens.

No existing v51 migration body should be edited. If schema changes are needed, add v52 or the next available version after re-checking `packages/server-go/internal/migrations/registry.go`.

Store transactions should use row updates with status predicates, not read-modify-write assumptions. For SQLite/Gorm, a safe pattern is one transaction that:

- Loads and validates the enrollment by id, credential digest, and helper device id.
- Settles expired active jobs using the existing `expireActiveHelperJobs` helper, extended if needed to include lease expiry.
- Finds one `queued` job for that enrollment where `expires_at > now` ordered by `created_at ASC`.
- Updates exactly that row with `WHERE id = ? AND status = 'queued' AND active_idempotency_scope IS NOT NULL AND expires_at > ?`.
- If `RowsAffected != 1`, loops once or returns `no_work` rather than risking duplicate lease.

Do not return owner/org internals through datalayer projections. It is acceptable for the Helper lease projection to include `PayloadJSON`, `PayloadHash`, `ManifestDigest`, `JobType`, `Category`, `SchemaVersion`, `ExpiresAt`, `LeaseExpiresAt`, and safe result state because the Helper needs those fields for task 7 policy handoff.

## 6. Helper Poll Client Shape

Add a narrow client under `packages/borgee-helper/internal/outbound`, for example `client.go`, that consumes `PreparedConfig` and a local Helper credential source.

Client responsibilities:

- Build URLs by joining `PreparedConfig.ServerOrigin` with fixed relative paths only.
- Attach `Authorization: Bearer <helper credential>`.
- Send `helper_device_id` in every request body.
- Decode typed poll, ack, and result responses with unknown-field rejection where practical.
- Convert `401`, `403 stale_credential`, `403 revoked`, and `403 uninstalled` into stop directives for the daemon loop.
- Convert `200 no_work` and transient `5xx` into bounded retry/backoff directives.
- Persist only queue cursor or retry state under `PreparedConfig.QueueStateDir` if implementation needs crash recovery for backoff. Do not write raw credentials, job payload dumps, private content, or unbounded logs to state dirs.

Daemon integration should be minimal in this task. It can add a disabled-by-default loop that starts only when outbound prerequisites and credential/device configuration are present. If the current daemon has no credential storage path yet, design implementation should add only the interface/constructor and focused client tests, then record the daemon loop as blocked until credential persistence is available. Do not invent a broad credential file format unless Dev design review explicitly accepts it.

If a loop is started, it must be cancellable by context, use bounded sleeps, and never block UDS serving or audit setup. Default installed services already pass outbound origin/state config from task 5; task 6 should not add service lifecycle or Remote Agent flags.

## 7. Auth / Credential Checks

Implement a shared store helper, not an API-only shortcut, to validate Helper route authority. It should be stricter than enqueue freshness:

- Enrollment id exists.
- Enrollment is claimed: `claimed_at`, `helper_device_id`, and `persistent_credential_digest` are set.
- `status` is not `pending`, `revoked`, or `uninstalled` and terminal timestamps are nil.
- Request credential matches the current persistent credential digest in constant time.
- Request `helper_device_id` matches the enrollment row.
- Job rows returned or modified belong to that enrollment and device.

Old credentials after rotation must fail as `stale_credential` or `unauthorized` without returning work. Wrong device id fails as `device_mismatch`. Revoke/uninstall fails as `revoked` or `uninstalled` and should trigger settlement of queued or leased-before-action jobs where this task owns the state transition.

This Helper rail is separate from user auth. Do not route these endpoints through `authMw`, and do not accept API keys, Remote Agent tokens, host grants, admin sessions, or user permission fallbacks.

## 8. Lease, Ack, Result Statuses

State transitions for task 6:

```text
queued -> leased      poll-and-lease succeeds
queued -> cancelled   cancellation/revoke/uninstall wins before lease
queued -> expired     server TTL expires before lease
leased -> running     ack succeeds before lease expiry
leased -> cancelled   cancellation/revoke/uninstall wins before action
leased -> expired     lease or TTL expires before ack/result
leased -> failed      Helper reports terminal failure before local action
running -> succeeded  Helper reports terminal success from later policy/action handoff
running -> failed     Helper reports terminal failure
running -> cancelled  cancellation observed at safe boundary
running -> expired    TTL/lease expiry wins before terminal result
```

Because task 7 local policy and later OpenClaw action are out of scope, task 6 should not create any actual host action path. It should only make terminal statuses representable and durable. If the implementation accepts `succeeded`, tests must frame it as a synthetic transport result and docs/current must still say Configure OpenClaw execution is not implemented.

Failure codes accepted at result time should be a closed enum. Include at least:

- `schema_invalid`
- `unknown_job_type`
- `policy_denied`
- `manifest_invalid`
- `artifact_invalid`
- `path_denied`
- `domain_denied`
- `service_denied`
- `revoked`
- `stale_credential`
- `wrong_owner`
- `wrong_org`
- `ttl_expired`
- `lease_lost`
- `cancelled`
- `execution_failed`

Store only bounded `failure_message`; cap length and strip control characters or reject them. `result_summary_json` should be a small typed object with bounded arrays of opaque audit/log references, not raw logs.

## 9. Idempotency, Retry, And Cancellation

Idempotency rules:

- Poll-and-lease is atomic. A duplicate poll after successful lease should not lease a second job ahead of the active leased/running job for the same Helper unless the design intentionally allows concurrency. For v1, keep one active lease per enrollment/device for lower risk.
- Ack replay with the same valid lease token is idempotent and returns current job state if already `running` or terminal.
- Result replay with the same terminal payload is idempotent and returns the stored terminal state.
- Result replay with a different terminal payload after completion fails with `terminal_conflict`.
- Enqueue idempotency remains as task 4 implemented it; task 6 clears `active_idempotency_scope` only on terminal states so a future same effective job can be enqueued after completion/expiry.

Retry/backoff rules:

- Server `no_work` returns `retry_after_ms`; Helper honors it with local caps and jitter.
- Transient server errors use bounded exponential backoff in the Helper client. Do not spin tightly.
- `401`, `stale_credential`, `revoked`, and `uninstalled` stop the loop and write a bounded local status/audit marker for later task 8 handoff.
- Lease expiry or `lease_lost` causes the Helper to stop acting on the local job and report or record a non-success terminal path if possible.

Cancellation rules:

- User revoke/uninstall and server-side cancellation must prevent future poll from returning queued jobs.
- If cancellation wins before local policy/action, settlement is `cancelled` or `failed` with a closed failure code, not success.
- This task may add a store method to settle active jobs for an enrollment when revoke/uninstall is observed by poll/ack/result. It should not change the user revoke handler into a service lifecycle executor.

## 10. Stale / Revoke Settlement

Task 6 must distinguish offline ambiguity from explicit authority loss:

- Missing or old credential after rotation: reject Helper call as stale/unauthorized and return no job payload.
- Revoked enrollment: reject Helper call as `revoked`, settle queued and leased-before-action jobs as cancelled/revoked where task 6 owns the transition, clear active idempotency scopes, and return a stop directive to the Helper client.
- Uninstalled enrollment: reject Helper call as `uninstalled`, settle queued/leased-before-action jobs similarly, and return a stop directive.
- Wrong device id: reject as `device_mismatch`; do not settle jobs because a different device should not be able to perturb the enrollment queue.
- TTL expiry: existing `expireActiveHelperJobs` should cover queued/leased/running rows; extend it to clear active scopes and record `ttl_expired` consistently.
- Lease expiry: treat as `expired` or `lease_lost` before accepting ack/result. Do not let late results overwrite terminal expiry unless idempotent replay is provably the same terminal state.

## 11. RED Test Plan

Write failing tests before implementation. Suggested focused RED tests:

API tests in `packages/server-go/internal/api/helper_jobs_test.go`:

- `TestHelperJobsPollLeasesOneQueuedJobWithHelperCredential`: enqueue a job, poll with current Helper credential/device, expect one leased response with safe payload and lease fields; second immediate poll returns no work or same active lease by defined v1 rule, not a duplicate lease.
- `TestHelperJobsHelperRoutesRejectWrongRails`: user token, agent API key, Remote Agent token, host grant id, missing token, old rotated Helper credential, and wrong device id cannot poll/ack/result.
- `TestHelperJobsAckAndResultAreIdempotentAndBounded`: ack moves leased to running; repeated ack is OK; result writes terminal state and clears active idempotency; repeated same result is OK; conflicting result gets `409 terminal_conflict`; response does not leak owner/org/digests/credentials/log bodies.
- `TestHelperJobsPollSettlesRevokedCancelledExpired`: revoked/uninstalled/stale/expired enrollments return deterministic errors or no-work stop directives and never return job payload.
- Replace the existing task 4 negative route assertions for poll/ack/result with task 6 positives while keeping local-policy/log/service/OpenClaw later routes unmounted.

Store tests in `packages/server-go/internal/store/helper_job_queries_test.go`:

- Atomic lease update changes one queued row to leased, sets lease timestamps, and prevents duplicate leases across repeated calls.
- Lease cannot be claimed with wrong credential digest, wrong device, revoked/uninstalled enrollment, terminal job, expired job, or wrong enrollment.
- Ack and result require matching lease token/state and enforce idempotent replay/conflict behavior.
- Terminal result clears `active_idempotency_scope`, enabling a later same effective enqueue after terminal settlement.
- Revoke/uninstall/TTL settlement clears active scopes and uses closed failure codes.

Datalayer tests in `packages/server-go/internal/datalayer/helper_jobs_test.go`:

- New repository methods project safe Helper lease/result state without owner/org/raw credential leakage.
- Store sentinel errors map to datalayer sentinel errors for unauthorized, device mismatch, stale credential, revoked, uninstalled, lease lost, terminal conflict, and no work.

Helper outbound tests in `packages/borgee-helper/internal/outbound`:

- Client builds only fixed relative poll/ack/result URLs from `PreparedConfig.ServerOrigin`.
- Client sends bearer Helper credential and `helper_device_id`, rejects malformed response bodies, maps stop directives, and applies bounded retry/backoff for no-work/transient failures.
- Tests prove job payload or manifest cannot provide full URL overrides.

Asset/static tests in `packages/borgee-helper/install` only need updates if service flags change. They should continue proving no `--remote-agent`, `--reverse-ws`, `--restart-service`, broad host-control, or sudo flags are introduced.

Verification commands after implementation should include at least:

```bash
GOTMPDIR=$PWD/.gotmp go test ./internal/api ./internal/datalayer ./internal/store ./internal/migrations
```

from `packages/server-go`, plus:

```bash
GOTMPDIR=$PWD/.gotmp go test ./internal/outbound ./install ./cmd/borgee-helper
```

from `packages/borgee-helper`. Broaden to package/module tests if implementation touches shared auth, server wiring, migrations, or daemon loop behavior.

## 12. Docs / Current Sync

After implementation, update current docs to reflect the accepted behavior exactly:

- `docs/current/host-bridge/helper-daemon.md`: Helper now has outbound poll/lease/ack/result client behavior if implemented; still no local policy, OpenClaw action, service lifecycle, sudo cache, or inbound server dial.
- `docs/current/host-bridge/README.md`: Add Helper pull/lease/result flow to key flows and update known gaps.
- `docs/current/security/README.md`: Add Helper poll/lease/result rail to the cross-rail matrix, including Helper credential/device checks and Remote Agent non-reuse.
- `docs/current/known-gaps.md`: Remove or narrow "Helper Pull Loop Not Implemented" only to the exact accepted behavior; keep local policy, bounded logs, OpenClaw execution, service lifecycle, and DNS/runtime network-policy gaps if still true.

Docs must not claim Configure OpenClaw success, local policy execution, bounded log upload, service restart, or UI terminal closure from task 6 alone.

## 13. Non-Goals

This task must not implement or widen into:

- Task 7 local policy execution, manifest/artifact verification, path/domain/service allowlist enforcement, or sandbox profile expansion.
- OpenClaw install/config execution, Borgee plugin channel binding, service lifecycle restart/boot/crash, or Configure OpenClaw terminal UI.
- Bounded log upload beyond small opaque references in result summary; task 8 owns logs/status settlement.
- Sudo cache, privileged long-lived service behavior, installer trust changes, or service-manager execution.
- Inbound server dial, generic host-control listener, shell, argv, script, executable path, arbitrary local path, or arbitrary network domain authority.
- Remote Agent rail reuse, including credentials, reverse WebSocket transport, remote-node rows, host grants, file-proxy status, or permission fallbacks.

## 14. Acceptance Mapping

- API/route shape: Helper-credential poll/ack/result routes are mounted; user enqueue route remains user-rail only.
- Datalayer/store model: existing `helper_jobs` table supports atomic lease/result transitions; new methods are repository-backed and keep `internal/api` out of `internal/store`.
- Helper poll client: outbound client uses validated origin plus fixed paths and maps no-work/retry/stop directives.
- Auth checks: current Helper credential and matching device id are required for every Helper route; stale/revoked/uninstalled authority fails closed.
- Lease/result statuses: duplicate lease, stale lease, conflicting terminal replay, and TTL expiry settle to deterministic non-success states.
- Idempotency/retry/cancellation: ack/result replay is idempotent where payload matches; conflicts are explicit; revoke/uninstall/cancellation wins before local action.
- Docs/current sync: current docs state exactly what task 6 implements and what remains for task 7/task 8/later OpenClaw closure.
- Product code remains blocked until design review accepts this plan.
