# Dev Design: Bounded Status Logs And Revoke Settlement

## 1. Data Flow

Task6 already owns enqueue, helper poll, atomic lease, ack, result upload, TTL expiry, lease-lost expiry, and helper stop directives. Task7 already owns local policy allow/deny reasons. Task8 hardens the terminal result boundary and exposes safe terminal metadata.

Flow:

1. User-owned enqueue creates a typed queued job through the existing user rail.
2. Helper polls with current Helper credential and `helper_device_id`; Task6 atomically leases or returns `no_work`.
3. Helper acks receipt; Task6 moves leased work to running.
4. Helper/local policy or execution boundary reports terminal status through result upload.
5. Store validates terminal status/reason/message/refs, redacts bounded message content, persists terminal fields, clears active idempotency scope, and returns safe serialized state.
6. If revoke/uninstall/stale/TTL/lease expiry wins before terminal upload, the store settlement helpers write deterministic terminal state and reason and prevent later success overwrite.

## 2. Data Model

No migration is expected. Existing `helper_jobs` columns are sufficient: `status`, `failure_code`, `failure_message`, `result_summary_json`, `completed_at`, `active_idempotency_scope`, `lease_expires_at`, and `expires_at`.

Task8 changes validation/serialization semantics only:

- `failure_message` stores a bounded redacted short message.
- `result_summary_json` stores normalized JSON with only `audit_refs` and `log_refs` arrays.
- Non-success terminal statuses require deterministic reason codes before persistence.

## 3. API Contract

Existing helper-credential route stays unchanged:

```text
POST /api/v1/helper/enrollments/{enrollmentId}/jobs/{jobId}/result
```

Allowed request fields remain `helper_device_id`, `lease_token`, `status`, `failure_code`, `failure_message`, and `result_summary`. Task8 tightens validation so `result_summary` only contains bounded `audit_refs` and `log_refs`, and unknown raw-log/private/sensitive fields return `400 forbidden_field` through existing strict decoding.

Safe job response adds terminal metadata when present:

```json
{
  "job": {
    "job_id": "job-id",
    "status": "failed",
    "failure_code": "policy_denied",
    "failure_message": "token=[redacted]",
    "result_summary": {"audit_refs":["audit-1"],"log_refs":["log-1"]}
  }
}
```

The response must not expose raw `result_summary_json`, payload hashes, owner/org internals, credentials, raw logs, private content, or Remote Agent state.

## 4. Edge Cases

- Terminal replay with identical metadata returns the existing terminal row.
- Terminal replay with different status, reason, message, or refs returns terminal conflict.
- `failed`, `cancelled`, and `expired` require a valid reason code; `succeeded` must not include reason/message/refs that make it look like a hidden failure.
- TTL expiry writes `expired` + `ttl_expired`; lease expiry writes `expired` + `lease_lost`.
- Revoke/uninstall settle queued/leased/running as `cancelled` + `revoked`/`uninstalled`.
- Stale credentials settle leased/running as `cancelled` + `stale_credential` and do not consume queued work for the current credential.
- Failure messages are bounded, control characters are removed, and sensitive token/credential/authorization/env/private-content patterns are redacted before storage.
- Result summaries reject raw log fields, unknown fields, refs containing path separators/control chars, too many refs, and oversized JSON.

## 5. Options

Option A: add a new durable log/audit table. Rejected for this task because the task scope says bounded references only, and raw log ingestion would expand privacy/security blast radius.

Option B: keep existing columns, tighten validation/redaction, and expose normalized safe metadata. Chosen because it preserves Task6 storage, fits Task8 acceptance, and avoids adding raw log storage before Task9-12 execution surfaces exist.

Option C: defer all log/status exposure to Configure OpenClaw UI. Rejected because Task8 explicitly owns bounded status/log settlement before OpenClaw closure work.

## 6. Integration

Primary files:

- `packages/server-go/internal/store/helper_job_queries.go`: terminal input validation, redaction, settlement helpers.
- `packages/server-go/internal/api/helper_jobs.go`: safe serializer for failure message and normalized result refs.
- `packages/server-go/internal/datalayer/helper_jobs*.go`: projection should keep normalized result summary available to API.
- `packages/borgee-helper/internal/outbound/client.go`: preserve fixed paths and narrow result shape; add client-side guard only if needed for bounded messages/refs.
- `docs/current/host-bridge/*` and `docs/current/security/README.md`: sync current settlement/log boundary.

Reverse impact: enqueue authority, poll/lease/ack route shapes, local policy evaluator, Remote Agent files, service lifecycle assets, and OpenClaw execution code should not need changes.

## 7. Sensitive-Task Threat Model

Assets: Helper credential authority, host-management job status, local audit/log reference identifiers, user/org/enrollment binding, and private host/user content that must not enter logs.

Threats and controls:

- Secret leakage through failure messages: redact credential/token/authorization/env/private-content patterns and bound message length.
- Raw log upload through result summary: strict allowlist `audit_refs`/`log_refs`; reject unknown fields and oversized JSON.
- Terminal-state forgery: lease token and Helper credential checks remain Task6 authority; conflicting terminal replay is rejected.
- Revoke bypass: route authority validation settles active work before accepting ack/result when revoked/uninstalled/stale authority is observed.
- Rail confusion: no Remote Agent credential, host grant, admin session, user token, or reverse-WS path is accepted on Helper result routes.

## 8. Test Plan

RED first:

- Store tests for redacted failure messages, terminal reason requirements, and terminal conflict on changed refs/message.
- API tests proving result responses expose safe `failure_message`/`result_summary` and reject raw log/sensitive fields.
- Existing settlement tests continue proving revoke/uninstall/stale/ttl/lease-lost terminal state.

GREEN verification:

- `GOTMPDIR=$PWD/.gotmp go test -tags sqlite_fts5 -count=1 ./internal/store ./internal/api ./internal/datalayer` from `packages/server-go`.
- `GOTMPDIR=$PWD/.gotmp go test -count=1 ./internal/outbound ./internal/jobpolicy` from `packages/borgee-helper`.
- Broader package verification as time permits before PR.
