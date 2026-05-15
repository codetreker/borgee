# PM Stance: Helper Enrollment Model And Status

## Scope Position

This task establishes the product foundation that Helper exists as a distinct enrolled host-management authority before any typed host-management job can run. It must create enough enrollment, identity, owner/org/host binding, allowed-category, and status shape for later tasks to build on, while staying below execution design detail.

## Stances

1. Helper enrollment is its own authority, not an extension of Remote Agent.
   - Anchors: `remote-actuator-design.md` §1.1-§1.2 and §5; `migration-analysis.md` §2.1.
   - Constraint: the model must expose a server-side `enrollment_id` and a host-side `helper_device_id` that are separate from Remote Agent file-proxy credentials and runtime grants.
   - Blacklist grep: `Remote Agent token`, `remote_agent.*helper`, `file-proxy.*helper`, `helper.*file-proxy`, `shared token`, `shared grant`, `merged rail`.

2. Owner, org, and host binding are part of enrollment identity, not optional metadata.
   - Anchors: `remote-actuator-design.md` §1.2 and §5; `migration-analysis.md` §2.1.
   - Constraint: Helper visibility and future enqueue authority must be traceable to `owner_user_id`, `org_id`, and host/device identity from the enrollment record. Client-only or label-only status cannot become the source of truth.
   - Blacklist grep: `client-only status`, `label-only host`, `owner optional`, `org optional`, `unscoped helper`, `global helper`.

3. Allowed job categories are an enrollment capability boundary, not a job execution contract.
   - Anchors: `remote-actuator-design.md` §1.2, §2.1, and §5.
   - Constraint: this task may record or expose the closed v1 category shape needed to prove Helper is bounded, but it must not define queue records, leases, result schemas, job payload schemas, manifest binding, retry behavior, or service execution semantics.
   - Blacklist grep: `job queue`, `lease`, `result schema`, `job payload`, `retry rule`, `manifest_digest`, `service manager`, `execute job`, `run job`.

4. Helper status is truthful enrollment/device visibility, not Configure OpenClaw completion.
   - Anchors: `remote-actuator-design.md` §1.2 and §5; milestone acceptance boundary.
   - Constraint: status may distinguish the enrollment/device foundation later needed for connected, offline, revoked, and uninstalled states, but it must not imply OpenClaw was configured, jobs ran, or lifecycle actions completed.
   - Blacklist grep: `Configure OpenClaw succeeded`, `OpenClaw connected`, `configured successfully`, `job succeeded`, `queued`, `running`, `failed job`, `lifecycle action`.

5. Server-side visibility authority must preserve the two-gate product promise.
   - Anchors: `remote-actuator-design.md` §1.2 and §5; `migration-analysis.md` §2.1.
   - Constraint: the server must be able to decide whether an enrollment is visible and eligible for later authority checks by owner, org, enrollment, delegation/category, and revocation state. This task does not implement the helper local policy or execution-time revalidation.
   - Blacklist grep: `server dials host`, `inbound helper`, `local policy implementation`, `execution-time policy`, `sudo cache`, `blanket preauthorization`.

6. Privacy and compliance remain internal safety boundaries, not new product surfaces.
   - Anchors: `migration-analysis.md` §6.1; `remote-actuator-design.md` §1.2.
   - Constraint: preserve rail separation, backend enforcement, and data-minimization intent, but do not add a privacy dashboard, compliance center, audit UI, legal-promise copy, or other user-facing privacy/compliance product surface in this task.
   - Blacklist grep: `GDPR`, `DPA`, `compliance center`, `privacy dashboard`, `audit UI`, `admin impact`, `legal agreement`, `privacy promise`.

## Out-Of-Scope Locks

- No job execution, queue, lease, result, retry, manifest, service-manager, sandbox, or local-policy implementation.
- No Configure OpenClaw closure or claim that OpenClaw has been installed, configured, connected, or validated.
- No credential rotation/revoke lifecycle beyond the enrollment/status foundation needed for later tasks.
- No merged Helper and Remote Agent credential, grant, or enforcement rail.
- No new user-facing privacy/compliance product surface.
