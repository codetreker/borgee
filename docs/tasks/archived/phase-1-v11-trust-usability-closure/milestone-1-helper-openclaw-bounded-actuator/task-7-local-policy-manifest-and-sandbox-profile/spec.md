# Spec Brief: Local Policy / Manifest / Sandbox Profile

## 0. Constraints

Task contract: start the Helper local policy task by defining the local revalidation boundary that must pass before any host-management action can run. This task owns schema validation, signed manifest/artifact binding, allowlisted paths/domains, declared service IDs, sandbox-profile alignment, and local denial of revoked, stale, wrong-owner, or wrong-org work. It must not implement Helper poll/lease/result transport, OpenClaw actions, service lifecycle behavior, Configure OpenClaw UI, sudo cache, or Remote Agent rail reuse.

Blueprint anchors:

- `HB-RA-1A` (`remote-actuator-design.md` section 1.2): server enqueue authorization and Helper local policy both validate owner, org, enrollment, delegation, job type, manifest/artifact, paths/domains, service IDs, and revocation state; Web sends schema-bound typed jobs, not arbitrary shell commands or client-provided execution authority.
- `HB-RA-1B` (`remote-actuator-design.md` sections 7 and 8): closed v1 typed jobs must reject unknown types, extra fields, arbitrary argv/executable/script/unit/path/domain authority, and Helper policy/sandbox permits only declared OpenClaw/Borgee paths, signed manifest/artifact cache validation, outbound Borgee queue/status domains, and declared enrolled service IDs.
- `PS-1` (`migration-analysis.md` section 6.1): preserve existing backend/security boundaries and Helper/Remote Agent rail separation without adding user-facing privacy/compliance product scope.

Dependency base:

- Canonical tasks 1-5 are accepted through PR #934, PR #936, PR #937, PR #938, and PR #939.
- Task 4 supplies the server-authorized typed job envelope and enqueue authority. Task 5 supplies outbound service/sandbox prerequisites. Task 6 may run in parallel and owns pull, lease, ack, result upload, retry/backoff, idempotency, cancellation, and stale credential transport behavior.

## 1. Segmentation

Segment A: fixed schema and typed job validation.
The accepted task records and later implements Helper-side schema revalidation for the closed v1 typed job set. Unknown job types, schema-version drift, extra fields, client-supplied execution authority, and malformed payloads fail before action.

Segment B: signed manifest and artifact binding.
The accepted task records and later implements local verification that a job's manifest digest and artifact reference bind to the signed manifest/artifact material approved for the enrolled Helper scope. Manifest or artifact mismatch, replay outside the accepted binding, missing required digest, or untrusted signing authority fails closed.

Segment C: path and domain allowlists.
The accepted task records and later implements Helper-side path/domain checks derived from signed manifest, enrollment state, and service prerequisite configuration. Jobs cannot add arbitrary paths/domains through payload fields, and policy must deny local/private/unknown domains unless already permitted by the bounded prerequisite and manifest authority.

Segment D: declared service ID allowlist.
The accepted task records and later implements service ID checks for later controlled lifecycle work. Only service identifiers declared by signed manifest or enrollment state are policy-eligible; arbitrary unit names, client-supplied service IDs, and out-of-scope services are denied.

Segment E: identity, revocation, and stale authority denial.
The accepted task records and later implements local rejection for wrong owner, wrong org, revoked enrollment/delegation, stale Helper credential/device state, and policy state that has changed between enqueue, lease, and pre-action validation.

Segment F: sandbox-profile alignment.
The accepted task records how local policy decisions align with the task 5 sandbox/service prerequisite: policy must not promise access that the sandbox denies, and sandbox permissions must not become broader than policy-eligible paths, domains, cache, state, and declared service IDs.

## 2. Carry-Over

Carry into later Dev design, but do not solve in this task-start package:

- Exact schema files, enum names, manifest signing format, digest algorithm constants, cache layout, path normalization APIs, service-manager adapters, and test fixture names.
- Exact task 6 pull/lease/result endpoint shape, retry/backoff, idempotency, cancellation settlement, result upload, and bounded log upload mechanics.
- Exact OpenClaw install/config/plugin/channel-binding action code, service lifecycle restart/boot/crash behavior, privileged installer handoff, and Configure OpenClaw terminal UI.
- Exact docs/current promotion text; sync only after implementation proves the policy boundary.

## 3. Reverse Checks

- If a job can run without local Helper revalidation of schema, owner, org, enrollment, job type, manifest/artifact, path/domain, service ID, and revocation state, the task violates `HB-RA-1A`.
- If any payload can introduce shell, argv, executable path, script, arbitrary local path, arbitrary network domain, client-supplied service unit, or arbitrary service restart authority, the task violates the closed typed-job boundary.
- If policy acceptance depends on Remote Agent credentials, host grants, reverse-WS transport, file-proxy status, or Remote Agent permission fallback, rail separation is broken.
- If sandbox permissions are broader than local policy eligibility without a documented fail-closed reason, host authority is too broad.
- If docs describe this task as Helper poll/lease/result transport, OpenClaw action, service lifecycle restart, terminal UI closure, or sudo/privileged service behavior, the scope is too broad.

## 4. Out Of Scope

- Helper outbound poll/long-poll, lease acquisition, ack, result upload, retry/backoff, idempotency, cancellation settlement, and transport stale-credential handling owned by task 6.
- OpenClaw install, agent config, Borgee plugin connection/channel binding, or any actual host-management action.
- Service lifecycle start/stop/restart, boot/crash recovery, arbitrary service manager operation, sudo cache, or persistent privileged daemon behavior.
- Configure OpenClaw UI, terminal status rendering, bounded logs UI, or OpenClaw connected/failed closure.
- Remote Agent rail changes, shared credentials/grants/enforcement, or new user-facing privacy/compliance product surfaces.
