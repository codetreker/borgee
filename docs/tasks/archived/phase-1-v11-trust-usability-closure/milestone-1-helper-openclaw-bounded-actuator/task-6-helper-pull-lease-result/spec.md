# Spec Brief: Helper Pull / Lease / Result

## 0. Constraints

Task contract: start the Helper pull / lease / result task by defining the outbound-only job retrieval and terminal settlement boundary for enrolled Helpers. This task owns the Helper loop that polls or long-polls for server-authorized jobs, claims a lease once, reports ack/result state, and stops on stale or revoked authority. It must not implement local policy execution, manifest or sandbox enforcement beyond interfaces, OpenClaw action, Configure OpenClaw UI closure, or service lifecycle restart behavior.

Blueprint anchors:

- `HB-RA-1A` (`remote-actuator-design.md` section 1.2): Helper transport is outbound poll/long-poll only; the server never dials the host. Jobs remain bounded, typed, pre-authorized, non-sudo, redacted, and rail-separated from Remote Agent credentials and grants.
- `HB-RA-1B` (`remote-actuator-design.md` sections 6, 8, and 10): task design must resolve the queue/lease/result contract shape, Helper-side policy handoff boundary, and revoke/stale authority settlement behavior at task-design granularity before implementation.

Dependency base:

- Canonical tasks 1-3 are accepted through PR #934, PR #936, and PR #937.
- Canonical task 4 is accepted through PR #938 (`64d56f1d6b326bc3ceabd93412717c85aa0e0506`) and supplies the server-authorized typed job envelope/enqueue boundary.
- Canonical task 5 is accepted through PR #939 (`96dc0dca19c243bfc53c8e8a4af56dbd33214a26`) and supplies the outbound service prerequisite needed before a long-lived Helper can poll.

## 1. Segmentation

Segment A: Outbound retrieval loop.
The accepted task records and later implements Helper outbound poll or long-poll behavior against Borgee job queue authority. The server still never dials the host, and the loop must fail closed when Helper authority is missing, stale, revoked, or mismatched.

Segment B: Lease and ack contract.
The accepted task defines how one enrolled Helper claims one job lease, acknowledges receipt after basic envelope validation, and avoids duplicate execution through lease token, expiry, idempotency, and retry boundaries. This is transport and settlement plumbing, not local policy execution.

Segment C: Result upload and terminal settlement.
The accepted task defines how Helper uploads terminal result state, failure reason, bounded log/audit references, cancellation, and lease-loss or expiry outcomes. Failed, denied, stale, revoked, cancelled, or expired work must not look successful or spin indefinitely.

Segment D: Cancellation, revoke, and stale credential behavior.
The accepted task records how poll, lease, ack, and result flows stop or settle when authority changes. Queued or leased-before-action work must settle deterministically when revoke, uninstall, stale credential, wrong owner, wrong org, TTL expiry, or cancellation wins.

Segment E: Handoff to local policy and later tasks.
The accepted task exposes the interface boundary needed by task 7 local policy/manifest/sandbox work and task 8 terminal status/log settlement. It does not authorize paths, domains, service IDs, artifacts, OpenClaw actions, or UI closure inside this task-start scope.

## 2. Carry-Over

Carry into later Dev design, but do not solve in this task-start package:

- Exact API route names, store schema details, endpoint constants, worker loop structure, and test fixtures.
- Exact lease duration, renewal cadence, retry/backoff values, heartbeat timing, and clock authority.
- Exact local policy manifest validation, artifact signing checks, path/domain/service allowlists, sandbox profile changes, and OpenClaw execution actions.
- Exact bounded log storage, status presentation, Configure OpenClaw terminal UI states, service lifecycle restart/boot/crash behavior, and privileged installer handoff.

## 3. Reverse Checks

- If the server can dial the host or Helper accepts inbound host-control traffic, the task violates the outbound-only guardrail.
- If a job can be leased or settled without Helper enrollment, owner/org match, Helper credential authority, or revocation/stale-state checks, the task violates the authority boundary.
- If duplicate Helpers can execute the same leased job successfully, the task violates the lease/idempotency boundary.
- If failure, denial, revocation, cancellation, lease loss, stale credential, or expiry can appear successful or remain indefinite, the task violates terminal truthfulness.
- If implementation executes local policy, installs/configures OpenClaw, changes sandbox allowlists, performs service lifecycle actions, or opens UI closure, the scope is too broad.
- If implementation reuses Remote Agent credentials, reverse-WS transport, host grants, file-proxy status, or Remote Agent authority rails, the task violates rail separation.

## 4. Out Of Scope

- No local policy execution or manifest/artifact enforcement beyond the handoff/interface boundary.
- No OpenClaw install/config action.
- No Configure OpenClaw terminal UI closure.
- No sandbox profile, service lifecycle restart, boot restart, crash restart, sudo cache, or privileged long-lived service behavior.
- No Remote Agent rail reuse.
- No arbitrary host command, shell, argv, executable path, script, service unit, local path, or network domain authority.
