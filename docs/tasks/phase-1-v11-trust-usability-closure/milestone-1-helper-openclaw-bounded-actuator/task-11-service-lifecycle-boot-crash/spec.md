# Spec Brief: Service Lifecycle Boot Crash

Task contract: make the current Helper/OpenClaw service-lifecycle slice reliable and bounded without turning the privileged installer into a persistent authority. This task owns non-sudo boot/crash restart settings for the installed Helper service, server-owned `service.lifecycle` enqueue for OpenClaw restart intent, and helper local-policy validation for declared logical service IDs.

Blueprint anchors:
- `HB-RA-1A` (`remote-actuator-design.md` section 1.2): long-lived Helper/OpenClaw services stay non-sudo; `install-butler` stays short-lived, visible, and without sudo cache; server enqueue and helper local policy validate service IDs.
- `HB-RA-1B` (`remote-actuator-design.md` sections 9 and 12): service permissions cover allowed service operations, restart/crash-recovery boundaries, bounded restart/backoff, and install-time privilege handoff.

## Accepted Scope

Segment A: Helper service boot/crash reliability.
The installed Linux systemd and macOS launchd Helper assets restart the long-lived Helper on boot and crash/failure, with bounded restart/backoff settings. The service continues to run as the dedicated non-root Helper user/group and keeps existing sandbox/outbound prerequisite boundaries.

Segment B: OpenClaw service lifecycle enqueue authority.
`service.lifecycle` becomes an enabled user-rail Helper job only for `openclaw_lifecycle` enrollment delegation. The client may request the closed intent `target=openclaw` and `operation=restart`; the server derives the effective payload and binds the job to the declared logical service ID `openclaw-user`. Clients cannot supply service IDs, service units, commands, shell, paths, domains, manifest digests, TTL, credentials, or config authority.

Segment C: Helper local-policy service ID boundary.
The pure local policy evaluator accepts service lifecycle jobs only when signed manifest declarations, server binding, and sandbox/profile service affordances all name the same logical service IDs. Service declarations must use platform-compatible managers and safe unit/label shapes, and duplicate or path-like IDs are denied.

Segment D: Privilege boundary stays unchanged.
`install-butler` remains outside boot/crash restart and service lifecycle jobs. This task does not add sudo cache, a persistent privileged installer, arbitrary service restart, or local service-manager execution inside the daemon.

## Rejected Scope

- No raw shell, argv, executable path, script, arbitrary service unit, or arbitrary local service restart.
- No Remote Agent rail merge or use of Remote Agent credentials/grants for Helper lifecycle work.
- No local policy-to-action execution, service-manager calls, OpenClaw success claim, raw/bulk log upload, or Configure OpenClaw terminal UI.
- No boot-time installer, sudo cache, persistent privileged daemon, or silent escalation.

## Review Boundary

A reviewer should be able to verify the slice from static assets, server enqueue tests, and helper policy tests: boot/crash restart is bounded for the Helper service, `service.lifecycle` carries only server-owned OpenClaw restart intent and declared service IDs, and the privileged installer remains short-lived and visible.
