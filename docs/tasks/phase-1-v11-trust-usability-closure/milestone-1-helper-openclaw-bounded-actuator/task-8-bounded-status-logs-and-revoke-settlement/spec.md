# Spec Brief: Bounded Status Logs And Revoke Settlement

## 0. Constraints

Task contract: make Helper job status, logs, and revoke/uninstall races settle truthfully after the Task6 pull/lease/result rail and Task7 local policy denial shape. Accepted behavior is limited to deterministic terminal state for revoked, stale, denied, expired, failed, cancelled, and lease-lost work, plus bounded redacted log/audit references.

Blueprint anchors:

- `HB-RA-1A` (`remote-actuator-design.md` section 1.2): status and logs stay bounded and redacted; failed jobs cannot look successful or spin indefinitely; revoke/uninstall prevents future work and deterministically settles queued or leased work.
- `HB-RA-1B` (`remote-actuator-design.md` sections 10 and 11): revoke/uninstall are policy state changes, not UI hints; queued, leased, and running work must settle with terminal status and reason; UI/API expose job status, failure reason, bounded redacted logs, and revoke/uninstall state.
- `PS-1` (`migration-analysis.md` section 6.1): preserve privacy/security boundaries without adding a new user-facing privacy/compliance product surface.

Dependencies: Task6 PR #943 (`c2c61e6e8500218ae0e841a9edde3f1187c78c7d`) supplies Helper poll/lease/ack/result transport semantics. Task7 PR #942 (`642fb5761b141a633169f39e31f77931bf85f0c1`) supplies local policy denial reasons. This task must preserve both.

## 1. Segmentation

Segment A: Deterministic terminal state.
Queued, leased, running, and helper-reported terminal jobs settle into only `succeeded`, `failed`, `cancelled`, or `expired` terminal state with a deterministic failure reason where the outcome is not success. Revoked, uninstalled, stale credential, TTL expired, lease lost, policy denied, schema invalid, and execution failed outcomes cannot remain active or be overwritten as success.

Segment B: Revoke/uninstall/stale precedence.
Revocation, uninstall, and stale credential authority changes win over queued, leased, and running work. Future work is blocked by previous tasks; this task ensures active work is cancelled or expired with the correct reason when the helper rail observes those authority changes.

Segment C: Bounded result metadata.
Helper result upload accepts terminal metadata only: closed status, closed failure code, short redacted failure message, and bounded opaque `audit_refs`/`log_refs`. It must not accept raw logs, tokens, credentials, private file/message content, full environment dumps, arbitrary paths, URLs, service unit names, commands, scripts, or unbounded blobs.

Segment D: API visibility without sensitive leakage.
Server/API responses expose terminal status, failure reason, bounded redacted failure message when present, and normalized result-summary references when present. Responses must continue to hide owner/org internals, helper credentials, credential digests, payload hashes, raw stored JSON fields, private content, and Remote Agent data.

Segment E: Current-doc sync and task evidence.
Accepted current behavior in `docs/current` must describe the bounded terminal settlement and redacted result-reference boundary, while retaining remaining gaps for OpenClaw execution, service lifecycle, sudo, and Remote Agent rail separation.

## 2. Carry-Over

- OpenClaw install/config execution, Borgee plugin channel binding, service lifecycle reliability, and Configure OpenClaw terminal UI stay in Tasks9-12.
- Local policy manifest/artifact/path/domain/service decisions remain Task7 behavior; Task8 only preserves and reports the denial reason without widening policy authority.
- Helper poll cadence, lease token generation, ack semantics, and transport stop directives remain Task6 behavior unless a narrow bug is required for deterministic settlement.
- User-facing privacy/compliance dashboards, legal promise copy, user audit views, and impersonation/privacy product surfaces remain out of scope under `PS-1`.
- Raw log storage, streaming logs, local file upload, and full local audit ingestion remain out of scope; this task accepts bounded opaque references only.

## 3. Reverse Checks

- If revoked, uninstalled, stale, expired, denied, cancelled, failed, or lease-lost work can stay queued/leased/running indefinitely, the task fails `HB-RA-1A` and `HB-RA-1B`.
- If failed/cancelled/expired/revoked work can be replayed or overwritten as `succeeded`, the task fails terminal truthfulness.
- If result upload or API response accepts or emits raw tokens, credentials, private file/message content, full environment dumps, command text, script bodies, arbitrary paths/URLs/service units, or unbounded logs, the task fails the bounded redaction boundary and `PS-1`.
- If implementation uses Remote Agent credentials, grants, reverse WebSocket transport, file-proxy status, or Remote Agent UI authority for Helper settlement, it violates rail separation.
- If the task implements OpenClaw action success, service lifecycle, sudo, install-butler behavior, or Configure OpenClaw closure UI, it exceeds scope.

## 4. Out-Of-Scope

- No Task9 OpenClaw install/config action, plugin binding, service lifecycle operation, boot/crash restart, sudo cache, or privileged long-lived service.
- No Remote Agent rail reuse, Remote Agent file-proxy token/grant/status changes, reverse-WS fallback, or admin/user permission fallback for Helper authority.
- No new user-facing privacy/compliance product surface, privacy dashboard, compliance center, legal text, or user-facing audit product.
- No raw log ingestion, file content upload, message content upload, environment dump storage, arbitrary path/URL/service-unit acceptance, or unbounded log API.
