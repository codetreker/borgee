# Acceptance: Helper Outbound Service Prereq

## Source Alignment

- Task: `task-2-helper-outbound-service-prereq`
- Milestone: `phase-1-helper-openclaw-onboarding/milestone-2-typed-job-policy-loop`
- Blueprint anchors: `remote-actuator-design.md` section 1.2, section 8, section 9, and section 14; `migration-analysis.md` section 6.1
- Dependency: task 1 accepted and merged through PR #938 (`64d56f1d6b326bc3ceabd93412717c85aa0e0506`).

## Segment A: Linux Outbound Service Permission

Acceptance checks:

- The Helper long-lived service can perform outbound HTTPS polling or long-polling needed by later Helper pull work.
- The Linux AF_UNIX-only restriction is explicitly resolved or replaced by a documented minimum permission shape.
- The service remains outbound-only; the server still never dials the host.

Negative checks:

- No inbound server dial, local host-control listener, Remote Agent reverse-WS reuse, or arbitrary network listener is introduced.
- No job lease, ack, result upload, retry, cancellation, or terminal status contract is implemented beyond prerequisite affordances.

## Segment B: Allowed Domains

Acceptance checks:

- Outbound network access is limited to Borgee queue/status endpoints or their environment-derived authority needed for Helper polling.
- Unknown, client-supplied, or job-payload-supplied domains fail closed.
- Review evidence names the boundary without relying on broad host network access as the security model.

Negative checks:

- The Helper service cannot call arbitrary network domains as part of this task.
- Configure OpenClaw job payloads cannot add domains through this prerequisite.

## Segment C: Queue And Status Write Paths

Acceptance checks:

- Required local write paths for Helper credential/state, queue cursor or poll prerequisite state, bounded status material, and local audit handoff are explicit and narrow.
- Write paths avoid raw tokens in logs, private file content, private message content, full local paths beyond operational necessity, and full environment dumps.

Negative checks:

- No arbitrary file write, client-supplied path write, private content dump, or generic local state export is accepted.

## Segment D: Service Permission Boundary

Acceptance checks:

- The long-lived Helper service remains non-sudo.
- Any service-manager permissions are described as prerequisites only and do not execute restart/boot/crash behavior in this task.
- Privileged installer handoff, if referenced for later work, remains short-lived, visible, and separate from the long-lived Helper service.

Negative checks:

- No sudo cache, persistent privileged daemon, silent escalation, arbitrary service unit control, or service lifecycle restart feature is introduced.

## Segment E: Rail Separation Evidence

Acceptance checks:

- Helper outbound service prerequisites use Helper identity and Helper service authority only.
- Review evidence shows Remote Agent credentials, reverse-WS transport, host grants, file-proxy status, and Remote Agent permission fallbacks are not reused.

Negative checks:

- No merged Helper/Remote Agent credential, endpoint, status rail, grant, or transport is introduced.

## Segment F: Task-Start Completion

Acceptance checks for this task-start PR:

- `spec.md`, `stance.md`, `acceptance.md`, and `progress.md` exist and match the task/milestone boundary.
- Shared state marks task 1 accepted/merged through PR #938 and task 2 unlocked for task-start/four-piece review.
- `content-lock.md` is recorded as N/A in `progress.md` unless exact UI copy/selectors become part of a later implementation design.
- No product code is implemented in the task-start commit.
