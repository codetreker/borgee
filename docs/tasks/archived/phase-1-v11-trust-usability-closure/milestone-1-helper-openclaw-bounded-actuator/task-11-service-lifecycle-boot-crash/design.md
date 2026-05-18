# Dev Design: Service Lifecycle Boot Crash

## Approach

Task11 should close the lifecycle boundary in three small layers rather than adding a service executor. The service assets provide boot/crash reliability for the installed long-lived Helper. The server enables a closed `service.lifecycle` job only for OpenClaw restart intent and derives the effective payload plus logical service binding. The helper policy evaluator tightens service declaration validation so later action wiring can rely on service IDs rather than raw unit names.

This keeps the task inside the accepted milestone boundary: reliable non-sudo services and declared service IDs, without sudo cache, privileged persistence, arbitrary local service restart, or Configure OpenClaw success claims.

## Service Assets

Linux systemd:
- Keep `Type=simple`, `User=borgee-helper`, `Group=borgee-helper`, `NoNewPrivileges=yes`, address-family restrictions, and existing state/read/write boundaries.
- Use `Restart=on-failure`, `RestartSec=10s`, `StartLimitIntervalSec=5min`, and `StartLimitBurst=5` so crashes restart but not infinitely tight-loop.

macOS launchd:
- Keep the sandbox-exec wrapper, `_borgee-helper` user/group, `RunAtLoad`, and failure-only `KeepAlive` shape.
- Add `ThrottleInterval=10` to bound restart churn.

The assets must not mention `install-butler`, sudo, Remote Agent, reverse WebSocket, raw commands, or service unit arguments.

## Server Enqueue

Enable `service.lifecycle` in `helperJobTaxonomy` with category `openclaw_lifecycle` and manifest binding required. The client payload schema is intentionally narrow:

```json
{"target":"openclaw","operation":"restart"}
```

The server stores an effective payload with only:

```json
{"operation":"restart"}
```

The server binding includes only `service_ids:["openclaw-user"]`. This prevents client-supplied service IDs, service units, command strings, paths, domains, artifacts, TTL, or manifest authority while still giving local policy the logical service identity it needs.

## Helper Policy

Extend `validateServices` so a service lifecycle allow decision requires:
- manifest service declarations with valid logical IDs;
- no duplicate manifest or binding service IDs;
- binding service IDs present in the signed manifest;
- binding service IDs present in the supplied sandbox/profile service affordances;
- platform-compatible manager/unit shape (`systemd` plus `.service` on Linux, `launchd` plus label on Darwin).

The policy still returns allow/deny only. It does not call systemd, launchd, OpenClaw, HTTP, filesystem IO, or result settlement.

## Tests

- Asset tests check Linux and macOS bounded boot/crash settings and forbidden privileged/Remote Agent strings.
- Helper policy tests check allowed declared service ID plus denials for unknown IDs, missing sandbox affordance, manager mismatch, unsafe unit shape, path-like IDs, duplicate IDs, and Darwin label mismatch.
- Server store/API tests check successful service lifecycle enqueue and poll, server-owned effective payload, `openclaw-user` service binding, public redaction, lifecycle category delegation, and rejection of client service/unit/command authority.

## Docs Sync

Update current host-bridge, helper-daemon, server, security, and known-gaps docs to state that service lifecycle enqueue and service ID policy are current, Helper service boot/crash restart is bounded, and local service-manager execution remains future work.
