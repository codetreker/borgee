# Milestone 2: Typed Job Policy Loop

## Capability Goal

Make Web-side Configure OpenClaw requests flow through server-authorized, Helper-revalidated, schema-bound jobs with truthful terminal status and bounded logs.

## Acceptance Boundary

Accepted by this milestone:

- Server enqueue gate validates owner, org, enrollment, delegation, job type, and revocation before creating a job.
- Helper pulls outbound, leases work, revalidates fixed schema and local policy, and reports terminal result/failure.
- Logs and status are bounded, redacted, and cannot make failed or revoked work look successful.

Rejected by this milestone:

- Client-supplied shell, argv, executable path, script, arbitrary service unit, arbitrary path, or arbitrary network domain.
- OpenClaw full closure UI if job/policy/result semantics are not in place.

## Dependencies

| Dependency | Status | Handling |
|---|---|---|
| Helper enrollment/status | PLANNED | Must supply helper identity, credential, and revoke state |
| Signed manifest/artifact authority | PLANNED | Task design must bind install/config jobs to signed artifacts before execution |
| Linux outbound poll permission | PLANNED | Task design must resolve current AF_UNIX-only long-lived service restriction |

## Task-Split Trigger

Run milestone breakdown after enrollment/status task skeletons are accepted and the job/policy loop can be split into server, helper, and policy/result tasks.
