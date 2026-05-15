# Acceptance: Helper Pull / Lease / Result

## Source Alignment

- Task: `task-6-helper-pull-lease-result`
- Milestone: `phase-1-v11-trust-usability-closure/milestone-1-helper-openclaw-bounded-actuator`
- Blueprint anchors: `remote-actuator-design.md` section 1.2, section 6, section 8, and section 10
- Dependency: task 5 accepted and merged through PR #939 (`96dc0dca19c243bfc53c8e8a4af56dbd33214a26`).

## Segment A: Outbound Retrieval Loop

Acceptance checks:

- Enrolled Helper retrieves work only through Helper-initiated outbound poll or long-poll.
- Polling uses Helper credential/enrollment authority and fails closed for missing, stale, revoked, wrong-owner, or wrong-org authority.
- The server does not dial the host and no inbound host-control listener is introduced.

Negative checks:

- No Remote Agent reverse-WS transport, file-proxy credential, host grant, or status rail authorizes Helper job retrieval.
- No job payload can supply arbitrary network destinations or turn the loop into a general command channel.

## Segment B: Lease And Ack

Acceptance checks:

- A job can be leased by at most one valid enrolled Helper at a time.
- Lease token, expiry, idempotency, and retry behavior prevent duplicate successful execution after repeated polls, lost responses, or helper restarts.
- Ack records receipt/basic envelope validation without granting local execution authority.

Negative checks:

- Duplicate helpers cannot both successfully execute the same job lease.
- Ack cannot bypass task 7 local policy, manifest/artifact, allowlist, revoke, stale credential, TTL, owner, or org checks.

## Segment C: Result Upload And Terminal Settlement

Acceptance checks:

- Helper can upload terminal result status with bounded failure reason, bounded log/audit references, and lease/cancellation context.
- Terminal failures include representable outcomes for schema invalid, unknown job type, policy handoff failure, revoked, stale credential, wrong owner, wrong org, TTL expired, lease lost, cancelled, and execution failure.
- Result upload is idempotent and does not expose tokens, secrets, private file content, private message content, or full environment dumps.

Negative checks:

- Failed, denied, stale, revoked, cancelled, expired, or lease-lost jobs cannot be recorded as successful.
- Jobs cannot spin indefinitely without a terminal or recoverable retry state.

## Segment D: Cancellation, Revoke, And Stale Authority

Acceptance checks:

- Future jobs are not pulled after Helper authority is revoked, uninstalled, stale, or owner/org mismatched.
- Queued jobs are not claimed when cancellation/revoke/TTL has already won.
- Leased-before-action jobs settle deterministically when revoke, uninstall, stale credential, cancellation, TTL expiry, or lease loss wins before local policy/action.

Negative checks:

- Revoke or stale credential cannot be reduced to offline ambiguity.
- Cancellation cannot leave a job in a success-looking or indefinitely running state.

## Segment E: Task Boundary And Handoff

Acceptance checks:

- The implementation exposes a narrow handoff for task 7 local policy/manifest/sandbox work and later task 8 terminal status/log settlement.
- Design and evidence keep local policy execution, OpenClaw actions, sandbox allowlists, service lifecycle, and Configure OpenClaw UI closure outside this task.
- Helper/Remote Agent rail separation is explicit in code, docs, and review evidence.

Negative checks:

- No local policy execution, manifest/artifact enforcement, path/domain/service allowlist decision, OpenClaw install/config action, service lifecycle restart, sudo cache, or Remote Agent rail reuse is introduced.

## Segment F: Task-Start Completion

Acceptance checks for this task-start commit:

- `spec.md`, `stance.md`, `acceptance.md`, and `progress.md` exist and match the task/milestone boundary.
- Shared state marks task 6 as TASKING after accepted PR #934/#936/#937/#938/#939 and does not modify task 7 files.
- `content-lock.md` is recorded as N/A in `progress.md` unless exact UI copy/selectors become part of a later implementation design.
- No product code is implemented in the task-start commit.
