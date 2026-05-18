# PM Stance: Helper Pull / Lease / Result

## Scope Position

This task makes the enrolled Helper able to retrieve and settle server-authorized jobs through outbound-only transport. It should complete the job transport loop just far enough for later local policy and OpenClaw action tasks to consume a leased typed job and return truthful terminal state.

## Stances

1. Outbound pull is the only transport.
   - Constraint: the Helper initiates poll or long-poll to Borgee queue/status authority; the server never dials the host.
   - Reviewer signal: there is still no inbound listener, reverse-WS reuse, generic host-control channel, arbitrary command surface, or job-payload-supplied network destination.

2. Leasing prevents duplicate execution.
   - Constraint: a job must have one active Helper lease with explicit expiry and idempotent claim/result behavior.
   - Reviewer signal: duplicate pollers, retries, lost responses, and result replays converge on one accepted terminal result or a safe non-success outcome.

3. Ack is receipt, not authorization to act.
   - Constraint: Helper ack can confirm receipt and basic envelope validation, but local policy remains a later enforcement step.
   - Reviewer signal: ack does not bypass owner/org, manifest, artifact, path, domain, service ID, revoke, stale credential, or task 7 policy checks.

4. Terminal results must be truthful and bounded.
   - Constraint: Helper reports succeeded, failed, cancelled, expired, revoked, stale credential, lease lost, or policy handoff failure states without unbounded logs or secret exposure.
   - Reviewer signal: failed, denied, stale, revoked, cancelled, or expired work cannot look successful or spin indefinitely.

5. Revoke and stale authority win over queued or leased work.
   - Constraint: polling, leasing, ack, and result paths must stop or settle deterministically when enrollment, credential, owner/org, TTL, revoke, uninstall, or cancellation state invalidates authority.
   - Reviewer signal: future jobs are not pulled, queued jobs are not claimed, and leased-before-action jobs settle as revoked/cancelled/stale rather than executing.

6. Task 7 owns local policy.
   - Constraint: this task may define the handoff shape, but it must not implement manifest/artifact validation, allowlist enforcement, sandbox policy, path/domain/service decisions, or OpenClaw execution.
   - Reviewer signal: policy-denied and schema-invalid states are representable without broadening this task into local enforcement.

7. Helper and Remote Agent rails remain separate.
   - Constraint: no Remote Agent credential, reverse-WS transport, host grant, file-proxy status, or Remote Agent permission fallback authorizes Helper polling, leasing, ack, or result upload.

## Out-Of-Scope Locks

- No local policy execution or manifest/artifact enforcement beyond the handoff/interface boundary.
- No OpenClaw action.
- No Configure OpenClaw UI closure.
- No service lifecycle restart, boot restart, crash restart, sudo cache, sandbox expansion, or privileged long-lived behavior.
- No Remote Agent rail reuse.
- No arbitrary host command, service unit, file path, executable, script, argv, or network domain authority.
