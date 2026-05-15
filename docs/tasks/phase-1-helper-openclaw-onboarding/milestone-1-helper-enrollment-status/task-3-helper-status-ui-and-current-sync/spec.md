# Spec Brief: Helper Status UI And Current Sync

## 0. Constraints

Task contract: make Helper enrollment status truthful in the user-facing product surface and sync the accepted status/UI contract into `docs/current` after implementation. This task consumes the Helper enrollment/status foundation from `task-1-helper-enrollment-model-and-status`; it does not own the foundation model, credential lifecycle, revoke race settlement, job execution, or shared task-state remediation.

Blueprint anchors:

- `HB-RA-1A` (`remote-actuator-design.md` section 1.2): Helper status must stay bounded to explicit Helper enrollment, outbound Helper status, allowed category visibility, revoke/uninstall visibility, and redacted bounded status. It must not imply arbitrary command authority, service lifecycle success, or Configure OpenClaw success.
- `HB-RA-1B` (`remote-actuator-design.md` section 11): UI/API must expose Helper online/offline, last seen, allowed job categories, and revoke/uninstall state. This task accepts enrollment-status visibility only; job queued/running/succeeded/failed, failure reason, logs, and audit views are outside this task unless a later task adds job execution.
- `PS-1` (`migration-analysis.md` section 6.1): preserve backend/security rail separation and data minimization. Do not add a new user-facing privacy/compliance product surface, audit viewer, legal promise copy, or compliance center.

Current implementation context:

- Task 1 has established server-side Helper enrollment routes in `packages/server-go/internal/api/helper_enrollments.go`, a datalayer repository projection, and `helper_enrollments` current-doc anchors.
- Task 3 should read those user-rail list/detail responses and render their existing redacted fields: `enrollment_id`, `host_label`, optional `helper_device_id`, `allowed_categories`, `status`, `fresh`, `last_seen_at`, `created_at`, `claimed_at`, `revoked_at`, and `uninstalled_at`.
- Exact UI wording and DOM literals are not locked at this four-piece stage. They should be locked only after design review chooses implementation copy and test selectors.

## 1. Segmentation

Segment A: Helper status API client contract.
The accepted client has typed API helpers for the user-management Helper enrollment surface, starting with listing and reading Helper enrollments. It must not call Helper credential endpoints from browser/user UI, and it must not expose raw enrollment secrets or persistent Helper credentials in list/detail status.

Segment B: User-facing Helper status surface.
The accepted UI lets a reviewer distinguish connected, offline, revoked, and uninstalled Helper enrollment states, including last-seen/freshness and allowed category visibility. Status must be framed as Helper enrollment/device visibility, not as OpenClaw installed/configured/connected proof.

Segment C: Allowed category presentation.
The accepted UI presents the closed allowed-category values as bounded delegation categories. It must not present them as runnable commands, service names, shell affordances, or job results.

Segment D: Revoked/uninstalled truthfulness.
Revoked and uninstalled states are visible and distinct from offline. Terminal states must not collapse into a generic success, generic disconnected, or indefinite pending state. Revoked/uninstalled status does not claim that local cleanup or future revoke race settlement has completed beyond the server-known enrollment state.

Segment E: Current-doc sync.
After implementation, `docs/current` is updated for the user SPA Helper status surface, client API contract, Host Bridge status boundary, and rail separation. If a candidate file already accurately describes the accepted behavior, the task records a reviewer-checkable no-op rationale in `progress.md`.

## 2. Carry-Over

Carry into task execution/design, but do not solve here:

- Credential rotation, stale-device replacement, deterministic queue/lease settlement, local uninstall execution, and helper local policy remain task 2 or later implementation scope.
- Job progress UI, bounded log UI, failure reason UI, job terminal states, and Configure OpenClaw workflow closure remain outside this milestone task.
- OpenClaw plugin/runtime status remains separate from Helper enrollment status. Helper connected means the enrolled Helper/device has recently reported status; it does not mean OpenClaw is configured or reachable.
- Exact product placement may be finalized during design review. The default direction is a user-owned global sidepane or equivalent shell surface, not the Remote Agent node manager and not Settings privacy/compliance.

## 3. Reverse Checks

- If the UI labels Helper connected as Configure OpenClaw connected, OpenClaw configured, job succeeded, install succeeded, or service running, it violates this task.
- If the UI merges Helper status into Remote Agent nodes, remote filesystem browsing, host grants, or user permissions as one combined authority rail, it violates `HB-RA-1A` rail separation.
- If the browser calls Helper credential claim/status/uninstall endpoints or handles persistent Helper credentials for display, it violates the user/Helper credential separation.
- If allowed categories are rendered as arbitrary commands, shell execution, service names, scripts, or job payloads, it exceeds the accepted category boundary.
- If revoked/uninstalled/offline are visually or semantically indistinguishable, a reviewer cannot accept the status slice.
- If current docs describe accepted Helper status as Remote Agent status, privacy/compliance promise UI, job execution status, or OpenClaw success, docs sync is incomplete.

## 4. Out-Of-Scope

- Production code implementation before design review and TDD.
- Server data model changes beyond narrow API/read projection needs discovered during design review.
- Helper daemon implementation, credential rotation, Helper credential storage, pull queues, leases/results, job execution, job logs, local audit ingestion, service lifecycle operations, or OpenClaw configuration validation.
- New privacy dashboard, compliance center, legal promise copy, user-facing audit stream, admin impact expansion, or impersonation UI.
- Remote Agent filesystem proxy behavior, Remote Agent token lifecycle, remote node status semantics, or merged Remote Agent/Helper management UI.
