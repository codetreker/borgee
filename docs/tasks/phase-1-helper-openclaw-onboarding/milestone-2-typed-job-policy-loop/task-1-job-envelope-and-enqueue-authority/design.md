# Dev Design: Job Envelope And Enqueue Authority

## 1. Boundary And Approach

This task adds the server-side authority boundary for creating Helper jobs. It follows the accepted Helper enrollment pattern: REST handler under `internal/api`, repository interface in `internal/datalayer`, SQLite adapter, store query file, forward-only migration, server route wiring, and API/store/datalayer/migration tests.

The route is user rail only. It uses `authMw`, derives `owner_user_id` and `org_id` from the authenticated user, and never accepts Helper credentials, Remote Agent tokens, host grants, admin identity, user permission fallback, client-supplied owner/org/device/category, or local execution fields as enqueue authority.

Task 1 creates only the enqueue surface and durable envelope. It must not mount Helper poll, lease, ack/result, execution, service lifecycle, local policy execution, bounded log upload, or Configure OpenClaw closure UI.

## 2. API Shape

Add a handler, tentatively `HelperJobsHandler`, wired near `HelperEnrollmentHandler` in `packages/server-go/internal/server/server.go`:

```text
POST /api/v1/helper/enrollments/{enrollmentId}/jobs
```

The route is wrapped only with `authMw`. There is no helper-credential version of this endpoint.

Request schema, strict JSON object:

```json
{
  "job_type": "openclaw.configure_agent",
  "schema_version": 1,
  "payload": {
    "agent_id": "agent-id",
    "channel_id": "channel-id"
  },
  "idempotency_key": "optional-client-retry-key"
}
```

Rules:

- `job_type` is required and must be in the closed v1 taxonomy below.
- `schema_version` is required and must be `1` for task 1.
- `payload` is required and decoded by job type with `json.Decoder.DisallowUnknownFields` or equivalent typed validation. The shared `readJSON` helper is not enough because it currently allows unknown fields.
- `idempotency_key` is optional. If present, trim and bound it, for example 1..128 bytes of an opaque client retry token. It is not authority and cannot change the effective idempotency scope.
- Extra top-level fields fail with `400`.
- Extra payload fields fail with `400`.
- Any fields named or equivalent to `shell`, `argv`, `command`, `raw_command`, `executable_path`, `script`, `service_unit`, arbitrary `path`, arbitrary `domain`, arbitrary `url`, credentials, environment, owner/org/device/category, or raw manifest content fail closed.

Response schema for accepted or converged idempotent enqueue:

```json
{
  "job": {
    "job_id": "job-id",
    "enrollment_id": "enrollment-id",
    "job_type": "openclaw.configure_agent",
    "schema_version": 1,
    "status": "queued",
    "category": "openclaw_config",
    "created_at": 1760000000000,
    "expires_at": 1760000300000,
    "idempotency_key": "optional-client-retry-key",
    "payload_hash": "sha256:...",
    "manifest_digest": "sha256:..."
  }
}
```

The public serializer exposes safe metadata and status only. It never exposes credentials, credential digests, enrollment secret digests, token values, environment variables, raw private payload content, private file content, unbounded logs, or internal owner/org fields. `payload_hash` and `manifest_digest` may be exposed only if they are digests of public/server-owned manifest material and not credential-derived; if there is doubt, omit them from the public serializer and keep them internal.

Error response shape should stay close to existing `writeJSONError` conventions but include deterministic codes if that helper supports it in the implementation branch. Required enqueue-denial reasons are: `unknown_job_type`, `job_type_not_enabled`, `schema_invalid`, `extra_field`, `forbidden_field`, `not_found`, `wrong_owner`, `wrong_org`, `pending_or_unclaimed`, `revoked`, `uninstalled`, `stale_enrollment`, `delegation_denied`, `manifest_required`, `idempotency_conflict`, and `ttl_invalid`.

## 3. Closed Job Taxonomy And Category Mapping

Enrollment `allowed_categories` are broad delegation buckets, not job types. Task 1 must define a separate closed `job_type` taxonomy and map each type to a category gate. A category match is necessary but not sufficient; job-type schema, owner/org/enrollment state, manifest binding, idempotency, and forbidden-field checks still apply.

Closed v1 taxonomy:

| Job type | Category gate | Task 1 enqueue payload stance |
|---|---|---|
| `openclaw.configure_agent` | `openclaw_config` | **Enabled in task 1.** References an existing server-owned `agent_id` and optional `channel_id`; server resolves `agent_configs` by `agent_id` and captures the effective config version/hash before enqueue. |
| `openclaw.install_from_manifest` | `openclaw_lifecycle` | Recognized but rejected in task 1 with `job_type_not_enabled` or `manifest_required` until manifest binding and install authority land in the owning task. No client paths, URLs, scripts, commands, binary URLs, or package manager args. |
| `borgee_plugin.configure_connection` | `openclaw_config` | Recognized but rejected in task 1 with `job_type_not_enabled` until the connection/channel binding source exists in the owning task. |
| `service.lifecycle` | `openclaw_lifecycle` or `helper_lifecycle` by fixed target enum | Recognized but rejected in task 1 with `job_type_not_enabled` until service lifecycle authority lands. No client service unit. |
| `state.write` | `openclaw_config` | Recognized but rejected in task 1 with `job_type_not_enabled` until local-policy/state-write authority lands. No arbitrary file paths or raw private content. |
| `status.collect` | `status_collect` | Recognized but rejected in task 1 with `job_type_not_enabled` until status/log collection lands. No client log paths, domains, or unbounded selectors. |
| `delegation.revoke` | `helper_lifecycle` | Recognized but rejected in task 1 with `job_type_not_enabled` until revoke settlement authority lands. No local command or path fields. |
| `helper.uninstall` | `helper_lifecycle` | Recognized but rejected in task 1 with `job_type_not_enabled` until uninstall settlement authority lands. No arbitrary deletion paths, service units, or scripts. |

Implementation should use a single validation table or function that returns: `job_type`, category gate, schema version, payload decoder, whether manifest binding is required, and whether the type is currently enqueueable. Unknown strings fail closed. Task 1's enabled set is exactly `openclaw.configure_agent`. Every other v1 taxonomy entry is recognized-but-rejected until its owning task supplies the server-side binding and authority checks. The design must not use optional language such as "may reject" for service, local-policy, lifecycle, install/uninstall, write, status/log, or revoke jobs, and it must not accept a generic command/action type.

## 4. Payload Schemas

Each payload is a typed Go struct with `DisallowUnknownFields` at the job-type envelope and payload layers. Prefer decoding `payload` as `json.RawMessage`, then dispatching to the job-type decoder. After decoding, re-marshal the normalized struct for persistence and payload hashing so field order and whitespace do not affect idempotency.

Task 1 payload structs should be intentionally small. Examples:

```go
type OpenClawConfigureAgentPayload struct {
    AgentID   string `json:"agent_id"`
    ChannelID string `json:"channel_id,omitempty"`
}

type StatusCollectPayload struct {
    Scope string `json:"scope"` // closed enum: helper, openclaw
}
```

`openclaw.configure_agent` does not accept `agent_config_id`. The existing server config store is `agent_configs(agent_id, schema_version, blob, created_at, updated_at)`, so enqueue validation must bind by `agent_id` and read the current config row for that agent. If no config row exists, enqueue uses the server-defined empty config state only if that is already valid for the agent config API; otherwise reject with `not_found` or `schema_invalid` and create no job row.

For `openclaw.configure_agent`, the effective payload persisted for idempotency is server-derived after all authority checks. It includes the normalized client-visible intent (`agent_id`, optional `channel_id`) plus a server-captured config binding such as `{agent_id, schema_version, config_hash}` derived from the `agent_configs.blob` bytes after canonical normalization. The client cannot supply config version, config hash, raw blob, or BPP/plugin reload fields.

The implementation must not add path/domain/service/string escape hatches to make future work easier. Later local-policy and service-lifecycle tasks can extend schemas by adding a new `schema_version` with closed fields and tests.

## 5. Job Envelope Model And Migration

Add a store model, tentatively `HelperJob`, and forward-only migration. Current highest migration is v50 (`helper_credential_rotation_metadata`), so the likely next migration is v51. Re-check `packages/server-go/internal/migrations/registry.go` before implementation because parallel task branches can claim a version.

Proposed table:

```sql
CREATE TABLE IF NOT EXISTS helper_jobs (
  id                    TEXT PRIMARY KEY,
  owner_user_id         TEXT NOT NULL,
  org_id                TEXT NOT NULL,
  enrollment_id         TEXT NOT NULL,
  helper_device_id      TEXT,
  job_type              TEXT NOT NULL,
  category              TEXT NOT NULL,
  schema_version        INTEGER NOT NULL,
  payload_json          TEXT NOT NULL,
  payload_hash          TEXT NOT NULL,
  manifest_digest       TEXT,
  manifest_binding_json TEXT,
  idempotency_key       TEXT,
  idempotency_scope     TEXT NOT NULL,
  active_idempotency_scope TEXT,
  status                TEXT NOT NULL CHECK (status IN ('queued','leased','running','succeeded','failed','cancelled','expired')),
  failure_code          TEXT,
  failure_message       TEXT,
  created_at            INTEGER NOT NULL,
  updated_at            INTEGER NOT NULL,
  expires_at            INTEGER NOT NULL,
  leased_at             INTEGER,
  lease_expires_at      INTEGER,
  completed_at          INTEGER,
  result_summary_json   TEXT,
  FOREIGN KEY(enrollment_id) REFERENCES helper_enrollments(id)
);
```

Indexes and constraints:

```sql
CREATE INDEX IF NOT EXISTS idx_helper_jobs_owner_org
  ON helper_jobs(owner_user_id, org_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_helper_jobs_enrollment_status
  ON helper_jobs(enrollment_id, status, expires_at);

CREATE INDEX IF NOT EXISTS idx_helper_jobs_status_expiry
  ON helper_jobs(status, expires_at);

CREATE UNIQUE INDEX IF NOT EXISTS idx_helper_jobs_active_idempotency_scope
  ON helper_jobs(active_idempotency_scope)
  WHERE active_idempotency_scope IS NOT NULL;
```

The migration must not create a permanent global unique index over `idempotency_scope`. One acceptable design is the nullable `active_idempotency_scope` shown above: set it to `idempotency_scope` only while the row is non-terminal and inside its active TTL window, clear it before marking `expired`/terminal, and enforce uniqueness only on non-null values. Equivalent designs are acceptable if they preserve the same invariant: active queued/leased/running rows protect replay/conflict inside the TTL window, while expired or terminal rows stop participating in enqueue convergence and cannot permanently block the same effective job after expiry.

`idempotency_scope` is a server-computed digest over at least owner, org, enrollment, job type, schema version, normalized effective payload hash, manifest digest or explicit no-manifest marker, and optional bounded `idempotency_key`. The active-window uniqueness mechanism is the convergence mechanism. The store still needs transaction logic to distinguish same-scope convergence from same client key with different effective payload.

`manifest_binding_json` is server-owned metadata only. It may contain safe public manifest identifiers or artifact digests. It must not contain raw credentials, private file contents, shell snippets, or arbitrary URLs supplied by the client.

## 6. Repository And Store Responsibilities

Add `HelperJobRepository` to `internal/datalayer`, wire it through `DataLayer`, and implement `NewSQLiteHelperJobRepository` beside the enrollment repo. API handlers should depend on the repository interface, not import store directly.

Repository method shape:

```go
EnqueueForUser(ctx context.Context, input EnqueueHelperJobInput, now time.Time) (*HelperJob, error)
GetForUser(ctx context.Context, jobID, ownerUserID, orgID string, now time.Time) (*HelperJob, error) // optional for task 1 if a GET route is added; no GET route is required.
```

`EnqueueHelperJobInput` must carry server-derived `owner_user_id` and `org_id`, route-derived `enrollment_id`, validated `job_type`, typed normalized payload, payload hash, manifest binding, optional idempotency key, and TTL policy inputs. It must not accept client-supplied owner/org/device/category authority.

Store responsibilities:

- Load the enrollment by `enrollment_id` inside the enqueue transaction.
- Verify owner and org match authenticated user.
- Verify enrollment has owner/org set, is claimed, has helper device id, has active credential digest metadata, is not pending, revoked, uninstalled, terminal, or stale. Stale enrollment rejection is mandatory for task 1.
- Verify the job category gate is included in enrollment `allowed_categories`.
- Verify job type/schema/payload are already normalized by API/datalayer validation, and defensively reject unknown taxonomy values again at the store boundary.
- For `openclaw.configure_agent`, verify the referenced `agent_id` belongs to the authenticated owner/org and is an agent row, then load `agent_configs` by `agent_id` to derive the effective config version/hash. If a `channel_id` is present, verify the channel belongs to or is accessible inside the same org and that the owner/agent has the required channel authority. Cross-owner, cross-org, non-agent, missing channel, or inaccessible channel bindings fail closed and create no job row.
- Compute server TTL and `expires_at` from `now`.
- Before idempotency lookup/insert, settle active rows whose `expires_at <= now` out of active convergence by marking them `expired` or clearing their active idempotency key. Then insert `queued` row or return the existing active idempotent row.
- Never insert a queued job for invalid input or authorization denial.

Sentinel errors in datalayer should mirror store errors:

- `ErrHelperJobInvalidInput`
- `ErrHelperJobUnknownType`
- `ErrHelperJobTypeNotEnabled`
- `ErrHelperJobSchemaInvalid`
- `ErrHelperJobForbiddenField`
- `ErrHelperJobEnrollmentNotFound`
- `ErrHelperJobForbidden`
- `ErrHelperJobEnrollmentInactive`
- `ErrHelperJobEnrollmentUnclaimed`
- `ErrHelperJobDelegationDenied`
- `ErrHelperJobManifestRequired`
- `ErrHelperJobIdempotencyConflict`
- `ErrHelperJobExpired`

HTTP mapping:

- `400`: unknown type, schema invalid, extra field, forbidden field, TTL/idempotency format invalid.
- `401`: handled by `authMw` before the handler.
- `403`: wrong owner/org, inactive enrollment, pending/unclaimed, revoked, uninstalled, stale, delegation denied.
- `404`: enrollment not found for an absent row. Existing enrollment code returns `403` for wrong owner and `404` for missing; keep that pattern.
- `409`: idempotency conflict for same client retry key with different effective payload/manifest binding.
- `201`: new queued job.
- `200`: duplicate retry converged to an existing same-scope job.

## 7. Auth, Authority, And Enrollment State

The API handler calls `mustUser`, gets `user.ID` and `user.OrgID`, and passes those as server-derived values. It must not read owner/org/device/category from JSON or query parameters.

Enrollment checks fail closed for:

- nonexistent enrollment;
- wrong owner or wrong org;
- missing owner/org on the row;
- pending or unclaimed enrollment;
- missing `helper_device_id` or active credential digest metadata;
- revoked status or `revoked_at` set;
- uninstalled status or `uninstalled_at` set;
- stale enrollment. Freshness is required for enqueue: `last_seen_at` must be present and `now - last_seen_at <= 5 minutes`, matching the Helper enrollment serializer freshness window. Missing `last_seen_at`, future-incoherent timestamps that cannot be trusted, or any stale/unknown freshness state fail closed with `stale_enrollment` and create no job row;
- missing required category gate in `allowed_categories`;
- job type requiring manifest binding when no server-side binding exists.

Helper credential rail endpoints from `HelperEnrollmentHandler` remain claim/status/rotate/uninstall only. A Helper credential must receive `401` on the user-rail enqueue endpoint because `authMw` does not treat it as a user session.

Remote Agent connection tokens, `remote_nodes`, `host_grants`, and `user_permissions` do not authorize enqueue. Tests must seed those rails and prove they do not change rejection outcomes.

## 8. Idempotency And TTL

The server is authoritative for both idempotency and TTL.

TTL:

- Task 1 sets `created_at`, `updated_at`, and `expires_at` from server time.
- Default TTL should be short and bounded, for example five minutes for enqueue-only records until lease/result tasks choose final values. The handler/store computes `expires_at = now + boundedDefaultTTL`; the client has no TTL authority.
- The request must not accept arbitrary `ttl`, `expires_at`, `deadline`, `lease_expires_at`, or equivalent fields. Any such top-level or payload field returns `400 ttl_invalid` or `extra_field` and creates no job row.
- Later pull/lease code must not lease expired jobs. Task 1 should make store query helpers treat `now >= expires_at` as terminal/non-executable even if no background expirer exists yet.
- If task 1 adds an expiry settlement helper, it may mark queued expired rows as `expired` and clear their active idempotency participation, but it must not mount polling or execution routes.

Idempotency:

- A duplicate retry with the same server-computed effective scope returns the existing row and does not create another job.
- If a client supplies the same `idempotency_key` for the same owner/org/enrollment/job type but changes normalized payload or manifest binding, return `409 idempotency_conflict`.
- If no client key is supplied, the server still deduplicates by natural effective scope for the active TTL window.
- Once the prior matching row is expired or terminal, it no longer participates in convergence or conflict detection. A later enqueue of the same effective job creates a new queued row, subject to the same current authorization, freshness, config/channel binding, and category checks.
- Scope includes at minimum owner, org, enrollment, job type, schema version, normalized payload hash, and manifest binding. Include category only as a derived check, not as client authority.

## 9. Status And Failure Taxonomy

Persisted status values established now:

- `queued`: accepted, not leased, before expiry.
- `leased`: reserved for later Helper pull task.
- `running`: reserved for later Helper ack/result task.
- `succeeded`: reserved for later result task.
- `failed`: terminal failure after a job exists.
- `cancelled`: terminal cancellation, including future revoke/uninstall settlement.
- `expired`: terminal non-executable TTL expiry.

Failure codes established now, even if later tasks produce some of them:

- `policy_denied`
- `schema_invalid`
- `unknown_job_type`
- `job_type_not_enabled`
- `manifest_invalid`
- `manifest_required`
- `artifact_invalid`
- `path_denied`
- `domain_denied`
- `service_denied`
- `revoked`
- `uninstalled`
- `stale_credential`
- `stale_enrollment`
- `wrong_owner`
- `wrong_org`
- `ttl_expired`
- `lease_lost`
- `cancelled`
- `execution_failed`
- `idempotency_conflict`

Invalid enqueue attempts should normally return HTTP errors and not create rows. If implementation chooses to persist rejected attempts for audit later, those records must not share the executable `helper_jobs` queue table unless status is terminal and excluded from all future lease queries. Task 1 does not need an audit table.

## 10. Test Plan

Write RED tests before production code. The first implementation commit after this design should add failing tests that prove the intended route and store behavior do not exist yet.

API tests in `packages/server-go/internal/api/helper_jobs_test.go` should copy the helper enrollment style:

- Happy path: owner creates and claims an enrollment with `openclaw_config`, then POSTs `openclaw.configure_agent`; response is `201`, status `queued`, safe fields only.
- User rail only: no token returns `401`; Helper credential returns `401`; Remote Agent token, host grant id/token-like value, and admin/user-permission fallback do not authorize enqueue.
- Route boundary: only `POST /api/v1/helper/enrollments/{enrollmentId}/jobs` is mounted. Later-scope endpoints such as Helper job poll, lease, result upload, ack, logs, service lifecycle, local policy, install, uninstall, or execution routes return the normal unmounted-route response.
- Wrong owner/org: another user cannot enqueue against the owner's enrollment and gets `403`.
- Missing enrollment: unknown id returns `404`.
- Pending/unclaimed enrollment: owner cannot enqueue before claim and gets `403`.
- Stale enrollment: enrollment with missing `last_seen_at` or `last_seen_at` older than five minutes gets `403 stale_enrollment`; no job row is created.
- Revoked enrollment: owner revokes enrollment, then enqueue gets `403` and no job row is created.
- Uninstalled enrollment: helper marks uninstall, then owner enqueue gets `403` and no job row is created.
- Invalid category: enrollment with only `status_collect` cannot enqueue `openclaw.configure_agent`; enrollment with `openclaw_config` can.
- Unknown job type: `command.run` or `shell` gets `400` and no job row. Recognized-but-disabled v1 types such as `service.lifecycle`, `state.write`, `openclaw.install_from_manifest`, and `helper.uninstall` get deterministic rejection (`400` or `403`, as mapped by implementation) with `job_type_not_enabled` or `manifest_required` and no job row.
- Config/channel binding: `openclaw.configure_agent` with cross-owner/cross-org/non-agent `agent_id`, nonexistent `agent_id`, inaccessible `channel_id`, or stale config binding gets rejected and creates no job row. Valid enqueue captures the current config schema version/hash in the effective payload used for idempotency.
- Extra top-level field: adding `owner_user_id` or `category` gets `400`.
- Extra payload field: adding `shell`, `argv`, `executable_path`, `script`, `service_unit`, `path`, `domain`, `url`, `credential`, or `env` gets `400`.
- TTL authority: adding client `ttl`, `expires_at`, `deadline`, or `lease_expires_at` at the top level or inside payload gets `400` and no job row; successful enqueue returns a server-generated bounded `expires_at`.
- Idempotent retry: same key and effective payload returns existing job with `200` or documented convergence status; row count remains one.
- Idempotency conflict: same key with different payload returns `409`; row count remains one.
- Idempotency after expiry: once an active row expires or is terminal, re-enqueueing the same effective job is allowed to create a new row and is not blocked by a permanent unique index.
- Serializer redaction: no owner/org internals, credentials, digests that are not explicitly safe, raw payload private content, tokens, or logs.

Store tests in `packages/server-go/internal/store/helper_job_queries_test.go`:

- Enqueue transaction derives owner/org from loaded enrollment and rejects mismatches.
- Category normalization is not reused as job type acceptance.
- Active idempotency convergence deduplicates duplicate inserts only while the prior row is active.
- Same client idempotency key with changed effective payload conflicts only against active rows; expired/terminal rows do not block new enqueue.
- Expired queued jobs are terminal/non-executable and are not returned by any future lease candidate query helper if such a helper is introduced in task 1.

Datalayer tests in `packages/server-go/internal/datalayer/helper_jobs_test.go`:

- SQLite adapter maps store sentinel errors to datalayer sentinel errors.
- Repository projection excludes store-only sensitive fields.

Migration tests in `packages/server-go/internal/migrations/helper_jobs_test.go`:

- v51 creates `helper_jobs` with expected columns, status `CHECK`, indexes, and an active-window idempotency uniqueness mechanism, without a global unique index that permanently blocks an `idempotency_scope` after expiry.
- Registry order remains strictly increasing.

No client tests are required in task 1 because there is no Configure OpenClaw UI or job status UI in scope.

## 11. Docs Current Sync Targets

Because task 1 will add server product behavior, implementation should sync these docs in the same task PR:

- `docs/current/server/data-model-and-migrations.md`: replace the current non-goal that Helper enrollment does not model a job queue with the new enqueue-only `helper_jobs` aggregate and keep lease/result/execution as non-goals.
- `docs/current/server/api-auth-admin-rails.md`: add user-rail Helper job enqueue and explicitly state Helper credentials, Remote Agent tokens, host grants, and admin rail do not authorize enqueue.
- `docs/current/server/startup-routing.md`: mention the new user-authenticated Helper jobs route wiring.
- `docs/current/host-bridge/README.md` and/or `docs/current/security/README.md`: record that server enqueue exists for typed jobs but Helper poll, local policy, execution, bounded logs, and Configure OpenClaw success are not implemented by task 1.

Do not update client UI docs unless later implementation adds a real UI surface, which this task must not do.

## 12. Non-Goals And Future Boundaries

Task 1 must not implement or mount:

- Helper outbound poll or long-poll.
- Lease acquisition, lease renewal, ack, result upload, retry execution, cancellation settlement, or bounded log upload.
- Helper local policy engine, sandbox profile, artifact cache validation, path/domain/service allowlists, or host execution.
- Linux service permission repair, service manager operations, or local service lifecycle execution.
- Configure OpenClaw closure UI, job progress UI, logs UI, or OpenClaw connected/success claims.
- Remote Agent rail changes, shared Helper/Remote Agent credentials, shared grants, or user permission fallback for host jobs.
- Admin route for enqueue or user-facing privacy/compliance/audit product surfaces.

Future tasks own:

- Task 2: outbound service prerequisites and sandbox/network permission repair.
- Task 3: Helper pull, lease, ack/result, retry, and cancellation loop.
- Task 4: local policy manifest/artifact binding, allowlists, service IDs, and sandbox profile.
- Task 5: bounded status/logs, revoke/uninstall settlement, and terminal result truthfulness.
