# PM Stance: Helper Outbound Service Prereq

## Scope Position

This task clears the service and sandbox prerequisite that lets a long-lived Helper perform outbound polling later. It should make the minimum host-service permission change needed for outbound Helper transport while preserving the non-sudo, outbound-only, rail-separated trust model.

## Stances

1. Outbound poll is a service capability, not a command channel.
   - Constraint: the Helper service may gain only the network permission needed to reach Borgee queue/status endpoints.
   - Reviewer signal: there is still no inbound server dial, generic host-control listener, arbitrary network domain, shell, argv, script, or executable path surface.

2. Linux AF_UNIX-only restriction must be resolved narrowly.
   - Constraint: the long-lived Linux Helper service must be able to perform outbound HTTPS poll/long-poll, but the fix cannot broaden into general host networking or privileged daemon behavior.
   - Reviewer signal: Dev design can point to the exact service/sandbox permission delta and why it is the minimum needed.

3. Write paths are service state, not file authority.
   - Constraint: local writes are limited to Helper credential/state, queue/status prerequisites, bounded local audit, and later status handoff needs.
   - Reviewer signal: arbitrary local paths, private content dumps, full environment dumps, and client-supplied paths remain rejected.

4. Long-lived Helper stays non-sudo.
   - Constraint: this task cannot add sudo cache, persistent privileged service behavior, silent escalation, or restart/boot/crash lifecycle features.
   - Reviewer signal: any privileged installer or service-manager work remains later, bounded, visible, and separate.

5. Task 1 is input, not reopened scope.
   - Constraint: PR #938 supplies the server-authorized typed job enqueue boundary. This task should not redesign enqueue authority or implement lease/result semantics.
   - Reviewer signal: service prerequisite work leaves job lease, ack, result, retry, cancellation, and terminal settlement to later tasks.

6. Helper and Remote Agent rails remain separate.
   - Constraint: no Remote Agent credential, reverse-WS transport, host grant, file-proxy status, or Remote Agent permission fallback authorizes Helper outbound polling.
   - Reviewer signal: code and docs keep Helper service permissions on the Helper rail only.

7. Privacy scope stays internal and bounded.
   - Constraint: keep data minimization and rail separation; do not add a user-facing privacy dashboard, compliance center, legal promise copy, or new admin-impact surface.

## Out-Of-Scope Locks

- No job lease/result/poll contract implementation beyond service prerequisite.
- No local policy execution.
- No OpenClaw action.
- No service lifecycle restart, boot restart, or crash restart feature.
- No sudo cache or persistent privileged long-lived service.
- No Remote Agent rail reuse.
- No arbitrary host command, service unit, file path, or network domain authority.
