# Spec Brief: Helper Outbound Service Prereq

## 0. Constraints

Task contract: start the Helper outbound service prerequisite task by defining the service, sandbox, and permission boundary required before a long-lived Helper can poll outbound for jobs. This task owns the prerequisite shape only. It must not implement job lease/result/poll contract behavior beyond the service prerequisite, local policy execution, OpenClaw action, service lifecycle restart, sudo cache, or Remote Agent rail reuse.

Blueprint anchors:

- `HB-RA-1A` (`remote-actuator-design.md` section 1.2): Helper job transport is outbound poll/long-poll only; the server never dials the host. Long-lived Helper/OpenClaw services stay non-sudo, and Helper actuator credentials/grants/enforcement remain separate from Remote Agent rails.
- `HB-RA-1B` (`remote-actuator-design.md` sections 8, 9, and 14): task design must resolve exact sandbox write paths, allowed network domains, service permissions, and the Linux AF_UNIX-only long-lived service restriction before outbound polling can be accepted.
- `PS-1` (`migration-analysis.md` section 6.1): preserve rail separation, data minimization, and existing security/admin/privacy controls without adding user-facing privacy/compliance product scope.

Dependency base:

- Canonical tasks 1-3 are accepted through PR #934, PR #936, and PR #937.
- Canonical task 4 is accepted and merged through PR #938 (`64d56f1d6b326bc3ceabd93412717c85aa0e0506`). Task 5 consumes the server-authorized typed job enqueue boundary and unlocks Helper-side pull/policy tasks by resolving service prerequisites.

## 1. Segmentation

Segment A: Linux outbound service permission shape.
The accepted task records and later implements the minimum Linux long-lived Helper service changes needed to permit outbound HTTPS polling while resolving the current AF_UNIX-only restriction. The result must preserve outbound-only transport and non-sudo long-lived service behavior.

Segment B: Allowed outbound domains.
The accepted task defines how the Helper service can reach only Borgee job queue/status endpoints needed for poll or long-poll. It does not introduce arbitrary client-supplied network domains or a generic host network action channel.

Segment C: Queue/status write paths.
The accepted task identifies the local write surfaces the Helper service needs for credential state, queue cursor or poll state, bounded status material, and local audit handoff. These paths must stay narrow and must not become arbitrary file writes.

Segment D: Service permission boundaries.
The accepted task keeps the long-lived Helper service non-sudo and records the boundary between prerequisite permissions and later service lifecycle jobs. It may prepare permission profiles required for later controlled service operations, but it does not implement restart/crash recovery or service lifecycle actions.

Segment E: Rail separation.
The accepted task proves outbound Helper service permissions do not reuse Remote Agent credentials, grants, reverse-WS transport, host grants, file-proxy authority, or status rails.

## 2. Carry-Over

Carry into later Dev design, but do not solve in this task-start package:

- Exact systemd/launchd unit changes, sandbox profile syntax, platform-specific file locations, endpoint constants, and test fixtures.
- Exact job lease, ack, result upload, retry/backoff, cancellation, bounded log upload, and poll contract behavior.
- Exact local policy manifest checks, artifact signing validation, path/domain/service allowlist enforcement, and OpenClaw execution actions.
- Service lifecycle restart/boot/crash behavior and any privileged installer handoff.
- UI copy, terminal job status presentation, and Configure OpenClaw closure.

## 3. Reverse Checks

- If the server can dial the host or the Helper accepts inbound host-control traffic, the task violates the outbound-only guardrail.
- If the Helper service gains sudo, sudo cache, silent escalation, or privileged long-lived behavior, the task violates the privilege boundary.
- If outbound networking accepts arbitrary domains or client-supplied domains, the task violates the sandbox boundary.
- If queue/status writes become arbitrary file writes or expose tokens, secrets, private file content, private message content, or full environment dumps, the task violates data minimization.
- If implementation reuses Remote Agent credentials, reverse-WS transport, file-proxy status, host grants, or Remote Agent authority rails, the task violates rail separation.
- If docs describe this task as lease/result implementation, local policy execution, OpenClaw action, service lifecycle restart, sudo cache, or Configure OpenClaw closure, the scope is too broad.

## 4. Out Of Scope

- No job lease/result/poll contract implementation beyond the service prerequisite.
- No local policy execution.
- No OpenClaw action.
- No service lifecycle restart, boot restart, or crash restart feature.
- No sudo cache, persistent privileged helper, or silent escalation.
- No Remote Agent rail reuse.
- No arbitrary host command, shell, argv, executable path, script, service unit, local path, or network domain authority.
