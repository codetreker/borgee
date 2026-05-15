# Acceptance: Service Lifecycle Boot Crash

## Segment A: Helper Boot/Crash Restart

- Linux Helper systemd service keeps `User=borgee-helper`, `Group=borgee-helper`, `NoNewPrivileges=yes`, sandbox hardening, and bounded Helper-owned write paths.
- Linux Helper service uses failure restart with bounded delay and start-limit settings.
- macOS Helper launchd plist keeps `_borgee-helper` user/group, `RunAtLoad`, failure-only `KeepAlive`, sandbox wrapper, and a bounded throttle interval.
- Neither asset adds sudo, Remote Agent flags, raw command hooks, service unit input, boot-time installer behavior, or persistent privileged installer behavior.

## Segment B: Server-Owned Service Lifecycle Job

- `service.lifecycle` is enabled only as an `openclaw_lifecycle` Helper job.
- The accepted client payload is closed to `target=openclaw` and `operation=restart`.
- The stored/leased effective payload contains only the operation needed by helper policy and action wiring; it does not include target, service ID, service unit, command, shell, argv, path, domain, credential, or TTL authority.
- The manifest binding includes the declared logical service ID `openclaw-user` and does not grant paths, domains, or artifacts for service lifecycle work.
- Fresh owner/org/enrollment/category gates, idempotency, and public serializer redaction continue to apply.

## Segment C: Helper Local Policy Service Boundary

- Service lifecycle policy requires a signed manifest service declaration, server binding, and sandbox/profile service ID affordance to agree on logical service ID.
- Unknown, duplicate, path-like, or sandbox-missing service IDs are denied.
- Linux service declarations require `systemd` and a safe `.service` unit name; Darwin service declarations require `launchd` and a safe label shape.
- Service payload schema remains strict and rejects client-supplied service unit or extra authority fields.

## Segment D: Scope Guards

- `install-butler` remains short-lived and visible; no autostart, supervised restart loop, sudo cache, or silent escalation is introduced.
- No Helper daemon service-manager call, OpenClaw action execution, policy-to-action wiring, raw log upload, service lifecycle UI, or Configure OpenClaw terminal closure is introduced.
- Remote Agent files, credentials, grants, and WebSocket transport are not modified.

## Verification Required

- Focused helper tests cover service assets and local service policy.
- Focused server store/API tests cover service lifecycle enqueue, poll serialization, closed payloads, category delegation, and forbidden authority.
- Broader helper and server package tests pass with the repo-required executable `GOTMPDIR` and `sqlite_fts5` tag where needed.
- `git diff --check` passes.
