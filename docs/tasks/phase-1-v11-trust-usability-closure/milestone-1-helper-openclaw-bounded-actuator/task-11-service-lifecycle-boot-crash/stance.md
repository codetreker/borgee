# PM Stance: Service Lifecycle Boot Crash

This task closes the reliability and authority gap for long-lived Helper/OpenClaw lifecycle handling without changing the product promise into a general host control channel.

## Direction

1. Long-lived service reliability is expected for the Configure OpenClaw value path.
   - Constraint: boot and crash restart belong to long-lived non-sudo services, not to `install-butler`.
   - Reject if: `install-butler` gains autostart, a supervised loop, sudo cache, or silent escalation.

2. Service lifecycle authority is logical-service based.
   - Constraint: user and job payloads cannot name service units or service IDs directly. The server binds allowed lifecycle work to declared logical service IDs, and local policy rechecks those IDs against signed manifest and sandbox/profile affordances.
   - Reject if: a client can enqueue `systemctl`, `launchctl`, a raw unit/label, shell, argv, or an arbitrary service target.

3. Reliability is not Configure OpenClaw success.
   - Constraint: service restart/backoff settings and `service.lifecycle` enqueue do not prove OpenClaw installed, configured, connected, or successfully restarted.
   - Reject if: docs or UI copy turn Helper service boot/crash reliability into OpenClaw connected status or terminal Configure OpenClaw closure.

4. Remote Agent remains separate.
   - Constraint: this task does not modify Remote Agent packaging, credentials, grants, or file-proxy behavior.
   - Reject if: Helper lifecycle work reuses Remote Agent token, reverse WebSocket, node status, or filesystem proxy authority.

## Out Of Scope

- Local service-manager execution inside the Helper daemon.
- Raw/bulk log upload or service status UI.
- Helper uninstall execution.
- Configure OpenClaw terminal UI states.
- New product promise around Teamlead cron concepts.
