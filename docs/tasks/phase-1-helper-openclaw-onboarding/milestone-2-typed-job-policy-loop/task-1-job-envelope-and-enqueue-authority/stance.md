# PM Stance: Job Envelope And Enqueue Authority

## Scope Position

This task starts the Typed Job Policy Loop by making server enqueue authority explicit. It should let Web request bounded Helper jobs only through a server-owned typed envelope, while keeping execution, local policy, service lifecycle, and OpenClaw closure for later tasks.

## Stances

1. Server enqueue is the authority boundary.
   - Constraint: the server decides whether a Helper job record can be created after checking owner, org, enrollment, delegation/category, job type, and revocation state.
   - Reviewer signal: job creation cannot be a thin pass-through of client payload.

2. The job envelope is typed, not executable text.
   - Constraint: Web requests must map into fixed schemas for closed job types. Client-supplied shell, argv, executable paths, scripts, service unit names, arbitrary paths, and arbitrary domains are rejected.
   - Blacklist grep candidates for product code review: `shell`, `argv`, `script`, `executable_path`, `service_unit`, `command`, `raw_command` near Helper enqueue code unless they are negative tests or rejection reasons.

3. Closed taxonomy starts narrow.
   - Constraint: use the blueprint v1 job taxonomy as the upper bound, and narrow at implementation time if required by available authority. Do not invent a generic host action job.
   - Reviewer signal: unknown job types and extra fields fail closed.

4. Milestone 1 is input, not reopened scope.
   - Constraint: accepted Helper enrollment identity, credential/revoke authority, and Helper status UI from PRs #934/#936/#937 are dependencies. This task should consume those contracts without changing enrollment/status UI behavior unless Dev design proves a narrow server enqueue need.

5. Queue lifecycle starts at enqueue only.
   - Constraint: idempotency, TTL, initial queued state, and enqueue-time terminal failure shape are in scope. Helper polling, leases, result upload, retry execution, cancellation settlement, and bounded logs stay out until later tasks.

6. Helper and Remote Agent rails remain separate.
   - Constraint: no Remote Agent token, connection token, host grant, user permission fallback, or file-proxy status should authorize Helper host-management jobs.

7. Privacy scope stays internal and bounded.
   - Constraint: keep data minimization and rail separation; do not add a user-facing privacy dashboard, compliance center, audit UI, legal promise copy, or admin-impact surface.

## Out-Of-Scope Locks

- No Helper polling/lease execution.
- No Linux service lifecycle or outbound service permission repair.
- No local policy manifest/sandbox profile implementation.
- No OpenClaw closure UI or job progress/log UI.
- No arbitrary host command channel and no merged Helper/Remote Agent authority rail.
