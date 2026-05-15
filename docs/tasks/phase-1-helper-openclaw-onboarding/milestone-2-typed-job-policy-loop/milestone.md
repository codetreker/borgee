# Milestone 2: Typed Job Policy Loop

> Remapped history. This milestone remains the detailed task home for typed job policy-loop tasks, but the authoritative coarse grouping is now `docs/tasks/phase-1-v11-trust-usability-closure/milestone-1-helper-openclaw-bounded-actuator/`.

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
| Helper enrollment/status | ACCEPTED | Supplied by Phase 1 milestone 1 through PR #934, PR #936, and PR #937 |
| Typed job enqueue authority | ACCEPTED | Supplied by task 1 through PR #938 (`64d56f1`) |
| Signed manifest/artifact authority | PLANNED | Task design must bind install/config jobs to signed artifacts before execution |
| Linux outbound poll permission | ACCEPTED | Supplied by task 2 through PR #939 (`96dc0dc`) |

## Task-Split Trigger

Run milestone breakdown after enrollment/status task skeletons are accepted and the job/policy loop can be split into server, helper, and policy/result tasks.

## Task Index

| Task | Status | Purpose | Depends on | Parallel? | First ready? |
|---|---|---|---|---|---|
| `task-1-job-envelope-and-enqueue-authority` | ACCEPTED | Define server-authorized typed job envelope and enqueue authority | Phase 1 milestone 1 task set accepted | no | yes |
| `task-2-helper-outbound-service-prereq` | ACCEPTED | Resolve Helper service permission/sandbox prerequisites for outbound poll/long-poll | `task-1-job-envelope-and-enqueue-authority` | no | complete |
| `task-3-helper-pull-lease-result` | READY | Add Helper outbound pull, lease, ack, result, retry, and cancellation loop | `task-2-helper-outbound-service-prereq` | yes, with task 4 if file ownership is clean | yes |
| `task-4-local-policy-manifest-and-sandbox-profile` | READY | Add local policy checks, manifest/artifact binding, allowlists, and declared service ID checks | `task-2-helper-outbound-service-prereq` | yes, with task 3 if file ownership is clean | yes |
| `task-5-bounded-status-logs-and-revoke-settlement` | BLOCKED | Make terminal status, bounded logs, and revoke/uninstall race settlement truthful | `task-3-helper-pull-lease-result`, `task-4-local-policy-manifest-and-sandbox-profile` | no | no |

Dependency order: Helper enrollment/status contracts are accepted, task 1 is accepted through PR #938 (`64d56f1`), and task 2 is accepted through PR #939 (`96dc0dc`). Tasks 3 and 4 are now ready and may run in parallel if Teamlead confirms disjoint file ownership. Terminal status/log/revoke settlement still waits for both pull/lease/result and local policy/manifest/sandbox work.

## Breakdown Review

| Role | Decision | Notes |
|---|---|---|
| Architect | LGTM | Server envelope, service prerequisite, helper pull, local policy, and terminal settlement form an executable dependency graph. |
| PM | LGTM | The Configure OpenClaw path is supported through typed jobs without promising full closure before later milestones. |
| QA | LGTM | Negative checks are scoped around enqueue authority, helper revalidation, policy denial, revoke, and bounded logs. |
| Dev | LGTM | The service prerequisite split keeps outbound/sandbox permission work separate from pull/policy implementation. |
| Security | LGTM | Credentials, host action authority, sandbox, revocation, and log redaction are marked as sensitive execution paths. |
