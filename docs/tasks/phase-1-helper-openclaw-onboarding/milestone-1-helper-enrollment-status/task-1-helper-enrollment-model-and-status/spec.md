# Spec Brief: Helper Enrollment Model And Status

## 0. Constraints

Task contract: establish Helper as a distinct enrolled host-management authority before any typed job can run. Accepted behavior is limited to the enrollment identity/status foundation: a server-side Helper enrollment record, host/helper device identity, owner/org/host binding, allowed job category shape, server-side visibility authority, and current-doc sync for the accepted foundation.

Blueprint anchors:

- `HB-RA-1A` (`remote-actuator-design.md` §1.1-§1.2; `migration-analysis.md` §2.1): Helper work starts from explicit local enrollment, then Web may later enqueue bounded typed jobs; this task only creates the identity/status basis needed for that later flow. Helper transport stays outbound/pull-first as a product boundary, and server/helper authority checks must remain owner/org/enrollment/delegation/revocation aware.
- `HB-RA-1B` (`remote-actuator-design.md` §5): enrollment must create a Helper/device identity independent from Remote Agent file-proxy tokens. The enrollment record binds owner, org, host label, and allowed job taxonomy; `helper_device_id` supports host instance identity and last-seen/stale-device semantics.
- `PS-1` (`migration-analysis.md` §6.1): this task must preserve existing privacy/security boundaries and must not introduce new user-facing privacy/compliance product surfaces.

The status foundation must be truthful but narrow. It may represent connected/online freshness, offline/last-seen freshness, revoked, and uninstalled enrollment states when supported by the accepted model, but it must not imply Configure OpenClaw, job execution, credential rotation, service lifecycle, or OpenClaw connectivity has succeeded.

## 1. Segmentation

Segment A: Helper enrollment authority.
The accepted system has a distinct Helper enrollment authority with its own server-side record and identifiers. This segment satisfies the task contract for Helper identity, host/helper device identity, owner/org/host binding, and server-side visibility authority, anchored to `remote-actuator-design.md` §5 and the `HB-RA-1A` separation guardrail in §1.2.

Segment B: Allowed job category shape.
The accepted system can record or expose the bounded category shape that the enrollment delegates for later OpenClaw/Helper lifecycle and configuration work. This segment records categories as a closed delegation shape for visibility and authorization inputs only; it does not create a job queue, execute jobs, define job payload schemas, or validate manifests/artifacts. This is anchored to `remote-actuator-design.md` §1.2 and §5, plus `migration-analysis.md` §2.1.

Segment C: Enrollment status authority.
The accepted system can distinguish Helper enrollment/device status in the foundation layer: connected or fresh, offline or stale by last seen, revoked, and uninstalled when that state is known. This segment is limited to enrollment/device status and server visibility; it must not report job queued/running/succeeded/failed status or OpenClaw connected status. This is anchored to `remote-actuator-design.md` §1.2 and §5 and the milestone acceptance boundary.

Segment D: Rail separation and current-doc sync.
The accepted system keeps Helper enrollment credentials/grants/enforcement separate from Remote Agent file-proxy credentials/grants/enforcement, and updates `docs/current` if accepted behavior changes the current enrollment identity/status foundation. This is anchored to `remote-actuator-design.md` §1.2 and §5, `migration-analysis.md` §2.1, and `migration-analysis.md` §6.1.

## 2. Carry-Over

Carry into later task execution/design, but do not solve here:

- Exact helper credential token shape, rotation cadence, stale-device handling, local storage rules, and invalidation behavior from `remote-actuator-design.md` §1.1 and `migration-analysis.md` §2.2 are Dev design and later credential lifecycle scope, not this spec brief.
- Queue, lease, ack, result, retry, TTL, cancellation, terminal failure, and clock-authority details from `HB-RA-1B` remain future task/design work. This task may create enrollment fields that later gates need, but it must not implement the queue contract.
- Manifest/artifact signing, path/domain allowlists, service permission matrix, sandbox profile, Linux outbound-poll mechanics, revoke race mechanics, and service lifecycle actions remain outside this foundation task except where their future category names are represented as allowed job categories.
- Credential rotation/revoke/uninstall mechanics beyond enrollment state representation carry to later milestone tasks. This task may represent revoked/uninstalled states as accepted enrollment status values; it must not implement full rotation, revocation race settlement, helper auth invalidation, or local uninstall actions.
- UI placement and status UI polish carry to later status UI work unless current-doc sync is required for accepted behavior changed by this task.

## 3. Reverse Checks

- If the implementation reuses a Remote Agent file-proxy token, Remote Agent node credential, Remote Agent grant, or Remote Agent enforcement path for Helper host-management authority, it violates the task contract and `remote-actuator-design.md` §1.2 and §5.
- If a user/admin/privacy surface is added to explain compliance, audit, DPA/GDPR, impersonation, or privacy promises, it violates `PS-1` in `migration-analysis.md` §6.1. Internal enforcement/audit facts may remain backend controls; they must not become a new user-facing privacy/compliance product surface in this task.
- If the accepted behavior lets Web run arbitrary shell, argv, executable paths, scripts, service unit names, unknown job types, or unbounded host commands, it violates `HB-RA-1A` in `remote-actuator-design.md` §1.2. This task should not execute any job at all.
- If the status model makes Configure OpenClaw, OpenClaw connectivity, queued/running/succeeded/failed jobs, or service lifecycle success appear complete, it exceeds this task and the milestone boundary.
- If enrollment authority is not owner/org scoped, not host/device scoped, or cannot distinguish revoked/uninstalled/offline/fresh enrollment states at the foundation level, the task has not met its acceptance slice.
- If current docs still describe accepted behavior as only the older Remote Agent/file-proxy or grant-backed helper boundary after this task changes the accepted enrollment/status foundation, current-doc sync is incomplete.

## 4. Out-Of-Scope

- Job queue, lease/result contract, helper pull client, job execution, Configure OpenClaw execution, OpenClaw plugin install/configuration, service lifecycle actions, and bounded log/result reporting.
- Exact credential token format, credential rotation, stale-device rejection mechanics, helper credential local storage, helper auth invalidation, and revoke race settlement.
- Manifest signing, artifact binding, sandbox/permission profile, platform service manager operations, install-butler behavior, privileged setup flow, and OS-specific packaging.
- New user-facing privacy/compliance surfaces, privacy dashboards, compliance centers, legal promise copy, user-facing audit views, or impersonation authorization UI.
- Remote Agent file browsing/proxy behavior, Remote Agent token lifecycle, Remote Agent grants, Remote Agent UI behavior, and any merge of Remote Agent and Helper authority rails.
