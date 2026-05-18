# Implementation Design: Helper Enrollment Model And Status

## Scope And Inputs

This task creates the Helper enrollment/status foundation only. It must prove Helper is a distinct enrolled host-management authority with server-side `enrollment_id`, host-side `helper_device_id`, owner/org/host binding, closed allowed-category shape, and status visibility. It does not create jobs, leases, results, Configure OpenClaw execution, local policy, service lifecycle, command channels, or new privacy/compliance UI.

Read inputs:
- Four-piece: `task.md`, `spec.md`, `stance.md`, `acceptance.md`, plus `progress.md`.
- Blueprint anchors: `docs/blueprint/next/remote-actuator-design.md` sections 1.1-1.2 and 5; `docs/blueprint/next/migration-analysis.md` sections 2.1, 2.6, and 6.1.
- Current docs: `docs/current/host-bridge/*`, `docs/current/security/README.md`, `docs/current/server/api-auth-admin-rails.md`, `docs/current/server/data-model-and-migrations.md`, `docs/current/remote-agent/*`.
- Server patterns: remote node owner/token/last-seen, host grants owner/revoke/active-list, agent status explicit status semantics, `mustUser` auth helper, route registration in `server.go`.

Important read-only findings carried into this design:
- Likely implementation write areas are the migration registry plus a new Helper enrollment migration, store model/query helpers, API handler/routes, and current-doc sync. This design does not edit those files.
- Closest patterns are `remote_nodes` for owner/token/last-seen, `host_grants` for owner-scoped list/revoke and active-state filters, `agent_status` for explicit status rows and fail-closed write source, and `auth_helpers.go` for canonical user auth failure behavior.
- A fresh migration grep in this worktree shows max migration `Version: 48`; `v49` appears free now, but implementation must re-grep immediately before claiming the version.
- The old `feat/helper-enrollment-status-foundation` branch is stale. It may be consulted only as a schema sketch if useful; the source of truth is the four-piece, blueprint anchors, current docs, and current package patterns.

## Data Flow

### User-Management Rail

User management endpoints are browser/API-key user rail endpoints behind existing `authMw` and `mustUser`:

```text
user creates local enrollment setup
  -> POST /api/v1/helper/enrollments
  -> auth user from context
  -> validate host_label and allowed_categories
  -> create helper_enrollments row with owner_user_id=user.ID and org_id=user.OrgID
  -> create one-time enrollment secret digest and expiry
  -> return enrollment record plus one-time local enrollment secret once

user lists enrollment status
  -> GET /api/v1/helper/enrollments
  -> owner/org-scoped list active and historical visible rows
  -> derive status from state + last_seen_at freshness
  -> serialize redacted records

user reads one enrollment
  -> GET /api/v1/helper/enrollments/{enrollmentId}
  -> owner_user_id and org_id must match user context
  -> return redacted status record or 403/404

user revokes enrollment foundation state
  -> DELETE /api/v1/helper/enrollments/{enrollmentId}
  -> owner_user_id and org_id must match user context
  -> forward-only update status='revoked', revoked_at=now
  -> credential digests are no longer accepted by helper rail
```

The user-management rail never accepts a helper credential as a user credential and never grants Remote Agent filesystem authority. It may return a generated one-time local enrollment secret only in the `POST` response body. The secret is not stored raw and is not returned by any later GET/list endpoint.

### Helper Credential Rail

Helper credential endpoints are separate from user-management endpoints. They are not mounted behind `authMw` because browser user cookies/API keys are not Helper credentials. They authenticate with the one-time enrollment secret during claim, then with a distinct persistent Helper credential for status updates.

```text
local helper claims setup
  -> POST /api/v1/helper/enrollments/{enrollmentId}/claim
  -> request includes one-time enrollment_secret and helper_device_id
  -> server loads row by enrollment_id, requires pending and not expired
  -> verify secret digest constant-time
  -> persist helper_device_id and persistent_credential_digest
  -> set status='connected', claimed_at=now, last_seen_at=now
  -> clear/expire one-time secret digest
  -> return persistent helper credential once plus redacted enrollment summary

helper reports status/heartbeat
  -> POST /api/v1/helper/enrollments/{enrollmentId}/status
  -> Authorization: Bearer <helper credential> or equivalent Helper-only header
  -> server loads row by enrollment_id
  -> reject unless credential digest matches, helper_device_id matches body/header,
     status is active, owner_user_id/org_id are present, and row is not revoked/uninstalled
  -> reject stale-device attempts when helper_device_id/credential mismatch the claimed row
  -> update last_seen_at and optional helper status details
  -> return redacted status summary

helper reports uninstall acknowledgement
  -> POST /api/v1/helper/enrollments/{enrollmentId}/uninstall
  -> same Helper credential gate as status endpoint
  -> set status='uninstalled', uninstalled_at=now
  -> do not accept future last_seen updates from that credential
```

This task implements the minimal Helper credential rail needed for claim, heartbeat, and helper-originated uninstall: one-time enrollment secret exchange, persistent Helper credential issuance, persistent credential digest storage, status heartbeat, and Helper-credential uninstall. This is intentionally narrow and does not include credential rotation cadence, multi-credential history, helper pull, job queue auth, stale-device replacement, or revoke race settlement.

Task 1 uses testable stale-device/stale-update predicates only:
- a second claim attempt on an already claimed enrollment fails closed with `409` and cannot update `helper_device_id`, credential digest, or `last_seen_at`;
- a status/uninstall request with the wrong `helper_device_id` or wrong Helper credential fails closed and cannot update `last_seen_at`;
- offline/stale freshness for the same valid `helper_device_id` plus valid Helper credential is recoverable and may update `last_seen_at` and return to connected.

There is no `stale` terminal status in task 1. Freshness stale/offline is a derived visibility state, not an authority-preserving terminal state.

### Store And DB Flow

API handlers call store helpers instead of open-coded SQL in handlers except where the package already has no extracted helper pattern. Store helpers own:
- creation with owner/org stamping from the authenticated user;
- claim with one-time secret digest verification and single-use update;
- owner/org-scoped reads;
- active/status list filters;
- forward-only revoke/uninstall state changes;
- fail-closed last-seen updates that only affect active claimed rows.

DB is the only source of truth for enrollment identity/status. Host labels are display metadata and are never enough for lookup authority. Every read/write/status path binds `enrollment_id` plus `owner_user_id`/`org_id` on user rail, and `enrollment_id` plus Helper credential digest plus `helper_device_id` on Helper rail.

### Current Docs Sync Flow

After implementation changes accepted behavior, update current docs in the same task PR:
- `docs/current/host-bridge/README.md` and/or `helper-daemon.md`: Helper enrollment/status foundation and out-of-scope job execution.
- `docs/current/security/README.md`: add Helper enrollment credential rail as distinct from Remote Agent and host grants.
- `docs/current/server/data-model-and-migrations.md`: add `helper_enrollments` as a durable aggregate and migration entry.
- `docs/current/remote-agent/README.md` or `protocol.md`: reinforce that Remote Agent tokens are not Helper enrollment credentials.

If implementation discovers current docs already cover the accepted behavior, record a no-op rationale in progress/review notes; do not leave Segment F implicit.

## Data Model

### Migration Plan

Create a new forward-only migration in `packages/server-go/internal/migrations`, add it to `registry.go`, and test it directly. Current grep shows max version `48`, so plan for:

```text
Version: 49
Name: helper_enrollment_status_foundation
```

Implementation must re-run `rg "Version:\s*[0-9]+" packages/server-go/internal/migrations` immediately before coding and choose the next free version if `v49` has been taken.

### Table: `helper_enrollments`

Proposed fields:

| Field | Type | Null | Notes |
|---|---|---:|---|
| `id` | `TEXT PRIMARY KEY` | no | Server-side `enrollment_id`; generated with existing `idgen.NewID()` pattern. |
| `owner_user_id` | `TEXT` | no | User owner; never optional. Mirrors Remote Node user ownership but uses Helper-specific naming. |
| `org_id` | `TEXT` | no | Stamped from owner user at create time; authorization input, not serializer output. |
| `host_label` | `TEXT` | no | Human-readable host label, max length such as 255. Not an authority key. |
| `helper_device_id` | `TEXT` | yes until claim | Host-side helper instance identity. Required after claim/status. |
| `allowed_categories` | `TEXT` | no | JSON array of closed v1 category enum values. Server validates exact strings and max count. |
| `status` | `TEXT` | no | CHECK enum below. |
| `last_seen_at` | `INTEGER` | yes | Unix ms. Only active claimed credentials may update. |
| `created_at` | `INTEGER` | no | Unix ms. |
| `updated_at` | `INTEGER` | no | Unix ms. |
| `claimed_at` | `INTEGER` | yes | Set when one-time secret is exchanged. |
| `revoked_at` | `INTEGER` | yes | Forward-only user revoke timestamp. |
| `uninstalled_at` | `INTEGER` | yes | Helper-originated uninstall timestamp for task 1. |
| `enrollment_secret_digest` | `TEXT` | yes | One-time local enrollment secret digest; raw secret never stored. Cleared or made unusable after claim/expiry. |
| `enrollment_secret_expires_at` | `INTEGER` | yes | Short TTL for local enrollment claim. |
| `persistent_credential_digest` | `TEXT` | yes until claim | Digest for the minimal persistent Helper credential issued at claim. Raw credential never stored or serialized. Required after claim. |
| `credential_created_at` | `INTEGER` | yes until claim | Issuance timestamp for the minimal persistent Helper credential; rotation is later scope. |

Status enum:
- `pending`: server enrollment created, waiting for local Helper claim.
- `connected`: claimed and recently seen; returned as fresh/connected while `last_seen_at` is within the freshness window.
- `offline`: claimed but not fresh; derived for responses from `last_seen_at` age rather than used as a stale-device terminal authority state.
- `revoked`: user revoked; terminal for future authority.
- `uninstalled`: uninstall known; terminal for future authority.

No `stale` status is created in task 1. Stale-device rejection is tested by already-claimed claim attempts and helper credential/device mismatches; stale freshness for the same valid helper credential is recoverable through the heartbeat endpoint.

Allowed category enum for this foundation:
- `openclaw_lifecycle`
- `openclaw_config`
- `helper_lifecycle`
- `status_collect`

These are category-level delegation inputs, not job types or payload schemas. They intentionally do not include arbitrary shell, argv, executable path, client-supplied scripts, service unit names, queue, lease, or result concepts.

Indexes:
- `idx_helper_enrollments_owner_org` on `(owner_user_id, org_id, status)` for user list and active filters.
- `idx_helper_enrollments_device` on `(helper_device_id)` where `helper_device_id IS NOT NULL` for stale-device/status lookup.
- `idx_helper_enrollments_last_seen` on `(last_seen_at)` for freshness queries if needed.
- `idx_helper_enrollments_secret_expiry` on `(enrollment_secret_expires_at)` where `enrollment_secret_digest IS NOT NULL` for cleanup/fail-closed tests.

Do not create foreign keys with cascade semantics that would erase enrollment/revocation history. Follow current logical-FK style used by `host_grants` and other forward-only audit-sensitive rows.

### Store Model And Serializers

Add a `HelperEnrollment` model/query file near the existing store query files. JSON tags on DB model must not accidentally expose sensitive/internal fields. Prefer explicit API response structs over direct DB model serialization.

Redacted response fields:

```json
{
  "enrollment_id": "...",
  "helper_device_id": "...",
  "host_label": "Mac Studio",
  "allowed_categories": ["openclaw_lifecycle", "openclaw_config"],
  "status": "connected",
  "last_seen_at": 1778840000000,
  "fresh": true,
  "created_at": 1778839900000,
  "claimed_at": 1778839950000,
  "revoked_at": null,
  "uninstalled_at": null
}
```

Never serialize `org_id`, raw secrets, credential material, credential digests, one-time secret digests, local private paths, Remote Agent tokens, user API keys, or token equivalents. `owner_user_id` is optional for owner-facing responses; omit it unless an existing API pattern requires echoing user id. Do not include local filesystem paths in this foundation.

## API Contracts

### `POST /api/v1/helper/enrollments`

Auth: user rail via `authMw` and `mustUser`.

Request:

```json
{
  "host_label": "Mac Studio",
  "allowed_categories": ["openclaw_lifecycle", "openclaw_config", "status_collect"]
}
```

Validation:
- `host_label` required, trimmed, bounded length, not used as authority.
- `allowed_categories` required, non-empty, unique, all values in closed enum, max count <= enum size.
- Reject unknown fields if the local helper pattern supports it; at minimum reject unknown enum values.

Response `201`:

```json
{
  "enrollment": { "enrollment_id": "...", "host_label": "...", "allowed_categories": ["..."], "status": "pending", "created_at": 1778839900000 },
  "enrollment_secret": "one-time-secret-returned-once",
  "enrollment_secret_expires_at": 1778840200000
}
```

Status codes:
- `400` invalid JSON, missing label, invalid/empty categories, oversized fields.
- `401` unauthenticated via existing auth middleware/helper behavior.
- `500` DB/entropy failures, with no secret material in logs.

### `GET /api/v1/helper/enrollments`

Auth: user rail.

Response `200`:

```json
{ "enrollments": [ { "enrollment_id": "...", "host_label": "...", "status": "offline", "fresh": false, "last_seen_at": 1778830000000, "allowed_categories": ["status_collect"] } ] }
```

Behavior:
- Lists only rows where `owner_user_id=user.ID` and `org_id=user.OrgID`.
- Empty list returns `[]`, not null.
- Non-terminal claimed rows derive `connected` vs `offline` from `last_seen_at` freshness; terminal `revoked` and `uninstalled` always win.

### `GET /api/v1/helper/enrollments/{enrollmentId}`

Auth: user rail.

Status codes:
- `200` redacted enrollment.
- `401` unauthenticated.
- `403` authenticated but wrong owner/org if the row exists and ownership mismatch is distinguishable by the implementation pattern.
- `404` not found.

Do not allow lookup by host label. Do not return internal credential fields.

### `DELETE /api/v1/helper/enrollments/{enrollmentId}`

Auth: user rail.

Behavior:
- Owner/org scoped.
- Forward-only revoke: set `status='revoked'`, `revoked_at=now`, `updated_at=now`.
- Idempotent on already revoked rows: return current revoked timestamp.
- If already uninstalled, keep `uninstalled` terminal state unless Security/PM review chooses revoke precedence; either way, future authority stays denied.

Response `200`:

```json
{ "enrollment_id": "...", "status": "revoked", "revoked_at": 1778841000000 }
```

### `POST /api/v1/helper/enrollments/{enrollmentId}/claim`

Auth: Helper one-time enrollment secret. Not user `authMw`, not Remote Agent token.

Request:

```json
{
  "enrollment_secret": "one-time-secret",
  "helper_device_id": "host-generated-device-id"
}
```

Behavior:
- Load by `enrollment_id`.
- Require `status='pending'`, no `revoked_at`, no `uninstalled_at`, and secret not expired.
- Verify digest in constant time.
- Validate `helper_device_id` non-empty, bounded, and stable-format enough for storage. Exact host derivation is later Helper implementation scope.
- Generate the minimal persistent Helper credential for task 1 and store only its digest.
- Set `claimed_at`, `last_seen_at`, `updated_at`, `status='connected'`; clear or invalidate one-time digest.

Response `201`:

```json
{
  "enrollment": { "enrollment_id": "...", "helper_device_id": "...", "host_label": "...", "allowed_categories": ["..."], "status": "connected", "last_seen_at": 1778840000000 },
  "helper_credential": "persistent-helper-credential-returned-once"
}
```

Status codes:
- `400` invalid body or missing device id.
- `401` missing/invalid/expired one-time secret.
- `404` unknown enrollment id.
- `409` already claimed, revoked, or uninstalled.
- `500` DB/entropy failures.

### `POST /api/v1/helper/enrollments/{enrollmentId}/status`

Auth: persistent Helper credential digest issued by the claim endpoint. This endpoint must not accept user cookies, user API keys, Remote Agent connection tokens, host grants, or user permissions.

Request:

```json
{
  "helper_device_id": "host-generated-device-id",
  "state": "connected"
}
```

Allowed request `state` values for this foundation: `connected` only for heartbeat/freshness. Do not accept `uninstalled` here, and do not accept job states.

Behavior:
- Require credential digest match for `enrollment_id`.
- Require body/header `helper_device_id` to match row.
- Require row owner/org fields non-empty even though helper does not present user identity.
- Reject terminal states: revoked/uninstalled rows cannot update `last_seen_at` and cannot become connected again.
- Reject task-1 stale-device attempts: wrong Helper credential, wrong `helper_device_id`, or any already-claimed enrollment trying to claim again. These failures cannot update `last_seen_at`.
- If the row is offline/stale by freshness only, the same valid Helper credential and same `helper_device_id` may update `last_seen_at` and return the response to connected/fresh.

Response `200` redacted enrollment status. Errors: `401` invalid credential, `403` device mismatch or inactive terminal row, `404` unknown enrollment, `409` revoked/uninstalled conflict if chosen by handler convention.

### `POST /api/v1/helper/enrollments/{enrollmentId}/uninstall`

Auth: persistent Helper credential rail only for task 1. User rail does not call this endpoint; user `DELETE /api/v1/helper/enrollments/{enrollmentId}` remains revoke.

Behavior:
- Require credential digest match for `enrollment_id`.
- Require body/header `helper_device_id` to match row.
- Require row owner/org fields non-empty and row not already revoked.
- Set `status='uninstalled'`, `uninstalled_at=now`, `updated_at=now`.
- Do not delete the row.
- Do not accept future status heartbeats for that credential.

Response `200` redacted terminal status. Errors: `401` invalid credential, `403` device mismatch or revoked row, `404` unknown enrollment, `409` already uninstalled if the handler convention does not return an idempotent terminal response.

## Edge Cases And Fail-Closed Behavior

Segment A: distinct Helper enrollment identity
- Missing or invalid local enrollment secret rejects with `401`; the server never falls back to user cookies, API keys, Remote Agent tokens, host grants, or user permissions.
- Claiming an already claimed enrollment is the task-1 stale-device/stale-claim predicate: it returns `409` and cannot rotate credentials, replace `helper_device_id`, expose credential material, or update `last_seen_at`.
- A helper claim cannot omit `helper_device_id`; no device id means no Helper identity.

Segment B: owner/org/host binding
- User reads/writes require `owner_user_id=user.ID` and `org_id=user.OrgID`; host label alone is never accepted.
- Wrong owner/org returns `403` or not-found according to existing handler convention, but must not return row contents.
- Rows with empty owner/org from legacy or corrupt data are treated inactive: no status update, no connected response, no future authority.

Segment C: allowed categories
- Empty, duplicate, unknown, oversized, arbitrary command-like, or job-type-like category values are rejected with `400`.
- Allowed categories are exposed as delegation visibility only. They do not create queue rows, job payload schemas, lease/result behavior, or local policy approval.

Segment D: visible Helper status
- `revoked` and `uninstalled` are terminal visible states and win over freshness.
- `offline` is derived from stale `last_seen_at` for claimed non-terminal rows; it is not OpenClaw disconnected and not Configure OpenClaw failed.
- Offline/stale freshness is recoverable for the same valid Helper credential and same `helper_device_id`; a valid heartbeat may update `last_seen_at` and return the response to connected/fresh.
- Wrong Helper credential or wrong `helper_device_id` is treated as stale-device/stale-update rejection in task 1; it fails closed and cannot update `last_seen_at`.
- `pending` with expired one-time secret stays not connected and cannot silently become active.
- Failed claim/status writes do not partially expose credentials; if DB update fails after credential generation, response is `500` and the credential is not accepted unless the transaction committed.

Segment E: Remote Agent rail separation
- No code path joins `helper_enrollments` to `remote_nodes.connection_token` for auth.
- No Helper endpoint accepts `/ws/remote` token format or Remote Agent bearer token.
- No Helper enrollment state grants Remote Agent filesystem list/read authority, and no Remote Agent node/binding grants Helper host-management authority.

Segment F: current-doc sync
- If implementation changes behavior, docs/current must describe Helper enrollment/status as its own rail.
- Docs must not claim Configure OpenClaw success, OpenClaw connected status, arbitrary host command support, post-install sudo caching, or user-facing privacy/compliance product expansion.

General edge cases:
- Invalid JSON: `400` with no mutation.
- Oversized `host_label`, `helper_device_id`, or category list: `400`.
- Concurrent claim: single transaction/update predicate wins; loser returns `409` and receives no credential.
- Concurrent revoke vs status: terminal revoke wins; status update predicate includes active status and no `revoked_at`/`uninstalled_at`.
- Concurrent uninstall vs status: terminal uninstall wins; status update predicate includes active status and no `uninstalled_at`.
- Clock/freshness: server clock is authoritative. Helper-provided timestamps are ignored for `last_seen_at`.
- Logging: log ids and high-level action only; never log raw secret, persistent credential, digests, local private paths, or token equivalents.

## Implementation Options

### Option 1: Single `helper_enrollments` Aggregate With Minimal Credential Rail (Chosen)

Create one table for enrollment identity, category delegation, status, one-time secret digest, and the minimal persistent Helper credential digest. Add user-management endpoints plus fixed Helper claim, heartbeat, and helper-originated uninstall endpoints that use separate auth rails.

Rationale:
- Best matches the task's foundation scope: one authority record can prove owner/org/host/device/category/status without introducing a queue or lifecycle subsystem.
- Keeps rail separation explicit because Helper credentials live in Helper-specific digest fields, not Remote Agent tokens, host grants, or user permissions.
- Makes QA predicates concrete: already-claimed claim attempts, wrong credential, and wrong `helper_device_id` are stale-device/stale-update failures that cannot update `last_seen_at`, while offline freshness for the same valid credential remains recoverable.
- Easy to test with current store/API patterns: migration test, store helper tests, owner/org API tests, credential redaction tests, reverse-grep tests.
- Leaves task 2 free to add rotation, multi-credential stale-device replacement, helper pull, queue auth, and revoke race settlement without needing to split an early queue model.

### Option 2: Reuse `remote_nodes` With Additional Helper Columns (Rejected)

This would add Helper-specific fields to `remote_nodes` and treat a remote node as the enrolled helper host.

Rejection reason:
- Violates the explicit rail separation guardrail. Remote Agent tokens are for file-proxy/reverse-WS behavior; Helper enrollment is host-management authority.
- Makes ownership and last-seen look convenient but conflates filesystem browsing status with Helper actuator status.
- Fails Acceptance E negative checks because a shared token/grant/enforcement model would be too easy to introduce.

### Option 3: Reuse `host_grants` Or `user_permissions` As Authority (Rejected)

This would represent allowed categories as host grant scopes or app permissions.

Rejection reason:
- Host grants are user consent rows consumed by helper IPC for current read-only capability checks; they are not Helper enrollment identity or device status.
- User permissions are app capabilities and must not become host-management authority.
- Neither model gives a first-class `enrollment_id` + `helper_device_id` + status + one-time enrollment rail.

### Option 4: Split Credentials Into A Separate `helper_credentials` Table (Deferred)

A separate credential table could model rotation, device generations, stale credentials, and audit history.

Deferral reason:
- Rotation/stale-device lifecycle is the next planned task. Splitting now increases schema and handler surface before this task needs it.
- The chosen single-table digest fields are enough for task-1 claim, heartbeat, helper-originated uninstall, raw-secret avoidance, and rail separation. Later migration can append `helper_credentials` if rotation needs multi-credential history.

## Integration And Reverse-Grep Impacts

Implementation write areas expected after design review:
- `packages/server-go/internal/migrations/helper_enrollments.go` and `registry.go`.
- `packages/server-go/internal/store/models.go` or a dedicated helper enrollment query/model file, plus store tests.
- `packages/server-go/internal/api/helper_enrollments.go`, route registration in `server.go`, and API tests.
- `docs/current/host-bridge/*`, `docs/current/security/README.md`, `docs/current/server/data-model-and-migrations.md`, and `docs/current/remote-agent/*` as needed for Segment F.

Reverse-grep checks to run during implementation/review:
- `rg "Version:\s*[0-9]+" packages/server-go/internal/migrations` before selecting migration version.
- `rg "helper.*remote_nodes|remote_nodes.*helper|connection_token.*helper|helper.*connection_token" packages/server-go/internal` should not show Helper auth reuse.
- `rg "helper.*host_grants|host_grants.*helper|helper.*user_permissions|user_permissions.*helper" packages/server-go/internal` should not show Helper authority reuse. Incidental docs/test negative assertions are acceptable if clearly testing separation.
- `rg "helper_credential|enrollment_secret|credential_digest|persistent_credential_digest" packages/server-go/internal` should show no serializer exposure and no logs containing raw material.
- `rg "job queue|lease|result schema|execute job|arbitrary shell|service manager" packages/server-go/internal/api packages/server-go/internal/store` should not find product implementation introduced by this task.

Likely test commands:
- `cd packages/server-go && go test ./internal/migrations -run Helper`
- `cd packages/server-go && go test ./internal/store -run Helper`
- `cd packages/server-go && go test ./internal/api -run Helper`
- Helper store/API tests must include: claim returns a persistent Helper credential once; already-claimed claim returns `409` and does not mutate `helper_device_id`, credential digest, or `last_seen_at`; wrong credential status fails and does not update `last_seen_at`; wrong `helper_device_id` status fails and does not update `last_seen_at`; same valid credential/device can recover offline freshness by updating `last_seen_at`; helper-originated uninstall requires Helper credential and blocks future heartbeat.
- `cd packages/server-go && go test ./internal/api -run 'Remote|HostGrants|AgentStatus'` for nearby regression checks if touched behavior is near those rails.
- `cd packages/server-go && go test ./internal/server -run Routes` or the nearest route smoke tests if route registration coverage exists.
- `cd packages/server-go && go test ./...` before final task verification if runtime allows.

## Security / Privacy Threat Model

Assets:
- Helper enrollment authority, one-time enrollment secrets, persistent Helper credentials/digests, owner/org binding, helper device identity, host labels, allowed categories, status freshness, revocation/uninstall state, and future host-management authority inputs.

Trust boundaries:
- Browser/user API rail to server.
- Local Helper claim/status rail to server.
- Server store boundary for durable authority.
- Remote Agent rail boundary, which must remain separate.
- Host Bridge host-grant/helper IPC boundary, which must remain separate.
- Local host filesystem/process boundary, which is out of scope for this task except that no private paths are serialized.

Actors and capabilities:
- Owner user can create/list/read/revoke their own Helper enrollments inside their org.
- Local Helper can claim with a one-time secret, receive a minimal persistent Helper credential, update heartbeat status with that credential, and report helper-originated uninstall with that credential.
- Other users/orgs must not read, claim, revoke, or update status for the enrollment.
- Remote Agent can connect with remote node token but has no Helper authority.
- Admin rail is not a user-management or Helper credential bypass for this task.
- Attacker may steal host label, guess ids, replay expired one-time secrets, reuse Remote Agent tokens, or attempt stale/revoked credential heartbeats.

Abuse cases:
- Reusing `remote_nodes.connection_token` as Helper credential.
- Treating host label as authority and crossing owner/org boundaries.
- Serializing credential material or org/internal fields to user responses.
- Allowing revoked/uninstalled enrollments to update `last_seen_at` and appear connected.
- Allowing wrong-device or wrong-credential stale-device attempts to update `last_seen_at`.
- Treating offline freshness for a valid credential as an unrecoverable terminal state.
- Accepting arbitrary categories that later become arbitrary command preauthorization.
- Letting user cookies/API keys authenticate Helper status writes.
- Letting user rail call the helper-originated uninstall endpoint instead of using user DELETE for revoke.
- Turning status into Configure OpenClaw success or job execution state.

Mitigations:
- Distinct `helper_enrollments` aggregate with `enrollment_id`, `helper_device_id`, `owner_user_id`, `org_id`, `host_label`, `allowed_categories`, and terminal status fields.
- Distinct Helper credential rail: one-time secret digest for claim plus minimal persistent credential digest for heartbeat/uninstall; no raw secret persistence and no secret returned after initial response.
- User rail always gates by owner and org. Helper rail gates by enrollment id, credential digest, helper device id, active status, and non-empty owner/org fields.
- Terminal `revoked`/`uninstalled` states are deny states for future authority and block last-seen writes.
- Task-1 stale-device/stale-update rejection is concrete: already-claimed claim attempts, wrong Helper credential, or wrong `helper_device_id` cannot update `last_seen_at`; offline freshness for the same valid credential/device can recover.
- Closed category enum with explicit rejection of unknown categories, shell/argv/script/path/service/job payload concepts.
- Explicit response serializers omit `org_id`, credential material, digests, one-time secrets, local private paths, and token equivalents.
- No new privacy/compliance UI; backend enforcement and docs-current security boundaries only.

Verification:
- Migration test proves schema fields, CHECK enums, indexes, idempotency, and `v49`/chosen version registration.
- Store tests prove create stamps owner/org, list/read scopes owner/org, claim issues persistent Helper credential once, claim is single-use, wrong credential/device cannot update `last_seen_at`, offline freshness can recover with the same valid credential/device, revoke/uninstall are terminal, and last-seen update predicates reject revoked/uninstalled rows.
- API tests prove 401/403/404/409/400 cases, redaction, empty lists, unknown category rejection, wrong owner/org rejection, no user-auth fallback for Helper status rail, and Helper-credential-only uninstall.
- Reverse-grep tests or review commands prove no `remote_nodes.connection_token`, Remote Agent routes, `host_grants`, or `user_permissions` are used as Helper authority.
- Serializer tests prove no org id, raw secret, digest, or token-equivalent fields appear in JSON.

Privacy decisions:
- Collection: owner id, org id, host label, helper device id, allowed categories, status timestamps, and credential digests. No private file content, local private paths, logs, or environment dumps in this task.
- Purpose: bind explicit local Helper enrollment to the owner/org/host/device and provide truthful status visibility for later bounded OpenClaw onboarding.
- Retention/deletion: foundation uses forward-only revoke/uninstall timestamps, not hard delete, matching host-grant audit-sensitive patterns. Any future deletion/retention policy is out of this task unless existing project retention rules require it.
- Disclosure/logging/export: user responses are redacted; internal logs include ids/action/timestamps only; no new user-facing privacy/compliance product surface or export flow.

## Review Checklist

- Architect: data flow keeps Helper enrollment separate from Remote Agent, Host Grants, and User Permissions; option rationale is explicit.
- PM: scope proves enrollment/status/category foundation without claiming Configure OpenClaw or job execution.
- Security: owner/org binding, Helper credential rail, terminal deny states, and serializer redaction are reviewable.
- QA: edge cases map to Acceptance Segments A-F and include test commands plus reverse-grep evidence.
