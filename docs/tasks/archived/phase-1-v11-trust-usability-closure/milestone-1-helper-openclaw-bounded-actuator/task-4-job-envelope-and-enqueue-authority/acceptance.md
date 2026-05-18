# Acceptance: Job Envelope And Enqueue Authority

## Source Alignment

- Task: `task-4-job-envelope-and-enqueue-authority`
- Milestone: `phase-1-v11-trust-usability-closure/milestone-1-helper-openclaw-bounded-actuator`
- Blueprint anchors: `remote-actuator-design.md` section 1.2, section 6, and section 7; `migration-analysis.md` section 6.1
- Dependency: canonical tasks 1-3 accepted through PR #934, PR #936, and PR #937.

## Segment A: Typed Job Envelope

Acceptance checks:

- Server-owned Helper job records have a typed envelope boundary with enrollment identity, job type, schema version, payload, idempotency, TTL, and initial/terminal status fields sufficient for enqueue-time truthfulness.
- Payload validation rejects unknown fields and records a deterministic failure shape for invalid input.
- Envelope fields do not expose raw Helper credentials, Remote Agent credentials, private file content, private message content, or full local paths.

Negative checks:

- The client cannot enqueue shell, argv, executable path, script body, arbitrary service unit, arbitrary path, arbitrary network domain, or generic host command payloads.
- The envelope is not authorized by Remote Agent node status, Remote Agent connection tokens, host grants, or user permission fallback.

## Segment B: Enqueue Authority

Acceptance checks:

- Enqueue checks owner, org, Helper enrollment, delegation/category, job type, and revocation state before creating a queued job.
- Revoked or uninstalled enrollment state prevents future job creation.
- Repeated Configure OpenClaw-style requests use an idempotency boundary rather than creating uncontrolled duplicate jobs.

Negative checks:

- A user cannot enqueue for another owner/org, a missing enrollment, a stale/revoked enrollment, or an undelegated category.
- Invalid enqueue attempts do not create jobs that later appear queued, running, or successful.

## Segment C: Closed Taxonomy

Acceptance checks:

- Job type handling is closed and reviewer-visible. Unknown job types fail closed.
- The task may initially enable only a safe subset of the v1 taxonomy if later Dev design records why the rest needs later manifest/policy/service work.
- Job type names map to schema-bound host-management intent, not arbitrary local execution.

Negative checks:

- There is no generic `command`, `shell`, `script`, `service unit`, `path write`, or `network call` job type accepted from clients.

## Segment D: Status And Failure Truthfulness At Enqueue

Acceptance checks:

- Accepted jobs begin in a truthful queued state with TTL/idempotency metadata.
- Enqueue-time denials have deterministic error/failure reasons such as unknown job type, schema invalid, revoked, wrong owner/org, or delegation denied.
- Failed enqueue attempts cannot look successful or spin indefinitely.

Negative checks:

- This task does not claim Helper execution, OpenClaw install/config success, bounded logs, lease results, or service lifecycle completion.

## Segment E: Scope And Rail Separation Evidence

Acceptance checks:

- Design and later implementation evidence explicitly show Helper polling/lease execution, Linux service lifecycle, local policy manifest/sandbox profile, and OpenClaw closure UI stayed out of this task.
- Review evidence includes negative tests or equivalent proof for unknown job types, extra fields, revoked enrollment, wrong owner/org, and client-supplied command/service/path/domain authority.
- `docs/current` sync is handled later only if product code lands in this task; task-start docs alone do not promote behavior to current.

Negative checks:

- No new user-facing privacy/compliance product surface is introduced.
- No shared Helper/Remote Agent credential, grant, endpoint, or status rail is introduced.

## Segment F: Task-Start Completion

Acceptance checks for this task-start PR:

- `spec.md`, `stance.md`, `acceptance.md`, and `progress.md` exist and match the task/milestone boundary.
- Shared state marks canonical tasks 1-3 accepted through PR #934/#936/#937 and task 4 unlocked.
- `content-lock.md` is recorded as N/A in `progress.md` unless exact UI copy/selectors become part of a later implementation design.
- No product code is implemented in the task-start commit.
