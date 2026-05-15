# Design: Helper Credential Rotation And Revoke

## Status

Draft for Dev design review. Do not implement production code from this task until Teamlead dispatches implementation after review. TDD is required before implementation.

## Existing Base

Task 1 added `helper_enrollments` and the Helper enrollment API/data-layer rail. The accepted foundation already has:

- User rail: create, list, get, and revoke Helper enrollments through owner/org-scoped user auth.
- Helper rail: claim with one-time enrollment secret, heartbeat/status with persistent Helper credential, and helper-originated uninstall with persistent Helper credential.
- Data columns: `helper_device_id`, `status`, `last_seen_at`, `claimed_at`, `revoked_at`, `uninstalled_at`, `persistent_credential_digest`, and `credential_created_at`.
- Explicit separation from Remote Agent tokens, host grants, and user permissions.

## Proposed Contract

Add Helper credential rotation as a Helper-rail lifecycle operation:

- Endpoint shape: `POST /api/v1/helper/enrollments/{enrollmentId}/rotate-credential`.
- Authentication: `Authorization: Bearer <current-helper-credential>` plus JSON body containing `helper_device_id`.
- Success response: serialized enrollment plus `helper_credential`, returned exactly once.
- Failure response: reuse the existing Helper rail error mapping where possible, with one new stale-credential/stale-device distinction only if tests need it for policy clarity.

This endpoint does not use user auth. A signed-in user can revoke, but cannot rotate on behalf of the local Helper because rotation proves possession of the current Helper credential and device identity.

## Data Model Impact

Prefer extending the existing `helper_enrollments` row rather than adding a credential-history table for this milestone slice:

- Add `credential_rotated_at INTEGER` to record the latest rotation time after claim.
- Add `credential_generation INTEGER NOT NULL DEFAULT 1` or equivalent metadata to make lifecycle review and tests explicit.
- Continue storing only `persistent_credential_digest` for the current credential.
- Keep `credential_created_at` as the initial persistent credential creation timestamp from claim.

The previous credential becomes stale by replacement: once `persistent_credential_digest` changes, the old raw credential no longer matches. A separate credential history table is deferred because this task has no audit UI, no job execution, and no need to accept multiple active credentials. If Security requires explicit replay telemetry later, a future task can add append-only internal audit without changing the current active-credential rule.

## Data-Layer Impact

Extend `datalayer.HelperEnrollmentRepository` with a narrow rotation method:

```go
RotateCredential(ctx context.Context, id, credential, helperDeviceID string, now time.Time) (*HelperEnrollment, string, error)
```

Store behavior:

- Load active enrollment by id, current credential digest, and matching `helper_device_id`.
- Reject missing, revoked, uninstalled, wrong credential, and wrong device before generating a replacement credential.
- Generate a new random credential, store only the digest, bump rotation metadata, update `updated_at`, and return the raw value exactly once.
- Do not update `last_seen_at` as a side effect unless design review explicitly wants rotation to count as freshness. The conservative choice is to keep heartbeat as the freshness signal.

If the implementation keeps existing generic `ErrHelperEnrollmentUnauthorized` and `ErrHelperEnrollmentDeviceMismatch`, tests can still prove stale semantics by checking no mutation. If reviewers require protocol-level stale wording, add datalayer/store errors such as `ErrHelperEnrollmentStaleCredential` but do not expose secrets.

## Revoke And Uninstall Semantics

Revoke remains user-owner/org scoped and terminal. After revoke:

- Current and previous Helper credentials fail heartbeat, rotate, and uninstall.
- Revoke does not call Remote Agent, host grant, user permission, or admin fallback checks.
- Revoke status remains distinct from offline and uninstalled status in serializers.

Uninstall remains Helper-originated and terminal. It requires the current valid Helper credential and matching helper device id. Terminal precedence should stay deterministic:

- If user revoke has already won, later stale/current Helper uninstall attempts return forbidden/inactive and do not change `revoked_at` or status.
- If helper-originated uninstall wins first, later user revoke remains idempotent with the existing task-1 behavior of returning the uninstalled row rather than overwriting it as revoked.

No service disable/removal implementation belongs to this task. The task only makes the authority state enforceable for later service/uninstall work.

## Stale Credential And Device Semantics

Stale credential means a credential that was once valid for the enrollment but is no longer the current digest, or any presented credential that fails the current digest check. The server does not need to distinguish former-valid from never-valid in external responses for this task.

Stale device means the presented `helper_device_id` does not match the claimed enrollment device. Rotation must not rebind the device because that would let a copied credential migrate Helper authority to another host.

Required non-mutation rules:

- Stale credential does not update `last_seen_at`, `updated_at`, credential digest, rotation metadata, `revoked_at`, or `uninstalled_at`.
- Wrong device does not update `last_seen_at`, credential digest, rotation metadata, or device binding.
- Terminal revoke/uninstall state does not become connected through heartbeat or rotation.

## API And Serializer Impact

The new rotation response may include raw `helper_credential` only in the rotation response body. Existing enrollment serializers must continue to omit:

- raw enrollment secret
- raw Helper credential
- credential digest
- owner/org internals

Do not add UI copy, DOM literals, or client surfaces in this task. Task 3 owns status UI/current-doc sync for product-visible status after its own review.

## TDD Plan For Implementation Dispatch

Write RED tests before production changes:

- Store/datalayer rotation success: current credential + matching device returns one new credential, stores a new digest, bumps rotation metadata, and old credential fails heartbeat/rotate/uninstall.
- Store/datalayer stale paths: wrong credential, old credential after rotation, wrong device, pending enrollment, revoked enrollment, and uninstalled enrollment do not mutate rows.
- API rotation route: current Helper credential succeeds; old credential fails after rotation; user token, Remote Agent token, host grant id, and wrong device fail.
- Revoke/uninstall precedence: revoke blocks future rotate/uninstall/heartbeat; uninstall blocks future rotate/heartbeat; terminal timestamps are not overwritten unexpectedly.
- Reverse-grep or focused tests confirm no Remote Agent/host grant/user permission fallback enters Helper credential paths.

Then implement the smallest data/API changes needed to make those tests pass.

## Docs/Current Sync

After implementation, update current docs if behavior changes are accepted:

- `docs/current/server/data-model-and-migrations.md`: credential rotation metadata and active-digest rule.
- `docs/current/server/api-auth-admin-rails.md`: Helper rail rotation endpoint and no user/Remote Agent fallback.
- `docs/current/security/README.md`: stale credential/device, revoke, uninstall, and rail separation.
- `docs/current/host-bridge/README.md` if it already describes Helper credential lifecycle enough to need rotation/revoke clarification.
- `docs/current/remote-agent/README.md` only if a separation statement needs updating.

If any listed current-doc file is unchanged, record the no-op rationale in `progress.md` before acceptance.

## Edge Cases

- Rotation request races with revoke: whichever transaction observes terminal revoke first must prevent new credential issuance. Prefer transactional conditional update on non-terminal status.
- Two rotation requests race with the same old credential: only one should win. The loser should fail because the digest no longer matches.
- Rotation request races with uninstall: terminal state wins; no new credential should be issued after uninstall.
- Clock behavior: server time owns rotation metadata; helper-provided time is not accepted.
- Expired one-time enrollment secret: still claim-only; it has no role in rotation.
- Offline but claimed Helper: if the current credential and device are valid and the enrollment is non-terminal, rotation may succeed even if freshness is offline, because offline is derived from heartbeat age rather than revocation.

## Alternatives Considered

1. Store only current digest plus rotation metadata. Recommended for this task because it enforces stale credential behavior, keeps the schema small, and avoids audit/product scope expansion.
2. Add a credential history table. More explicit for forensic replay analysis, but unnecessary before job execution and risks expanding audit scope beyond the milestone.
3. User-triggered rotation from the Web user rail. Rejected for this task because it would rotate credentials without proving local Helper possession and would blur revoke/rotation authority.
4. Device rebind during rotation. Rejected because rotation must not become a silent host migration path.

## Security And Privacy Review Points

- Confirm raw credentials are generated with existing cryptographic randomness, returned only once, stored only as digests, and omitted from serializers/logs.
- Confirm constant-time digest comparison remains in credential validation.
- Confirm stale credential/device failures do not mutate authority state.
- Confirm revoke/uninstall terminal state cannot be bypassed by rotation.
- Confirm no Remote Agent token, host grant id, user token, admin session, plugin API key, or user permission can authorize Helper rotation.
- Confirm no typed job execution, arbitrary shell, service manager operation, or Configure OpenClaw success state is introduced.
- Confirm no new user-facing privacy/compliance product surface is added while backend rail separation is preserved.
