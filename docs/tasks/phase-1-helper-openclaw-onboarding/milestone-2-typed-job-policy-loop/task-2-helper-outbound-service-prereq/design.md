# Dev Design: Helper Outbound Service Prereq

## 1. Boundary

This task prepares the long-lived Helper service for later outbound poll or long-poll work. It owns the service, sandbox, configuration, and local state prerequisites only.

It must keep these guardrails intact:

- Helper transport remains outbound-only; the server never dials the host.
- The existing local UDS remains the only inbound listener.
- The long-lived Helper remains non-sudo and does not gain sudo cache or privileged daemon behavior.
- Helper outbound service authority remains separate from Remote Agent credentials, reverse WebSocket transport, host grants, file-proxy status, and permission fallbacks.
- No poll loop, lease, result upload, ack upload, local policy execution, OpenClaw action, service lifecycle restart, or installer trust change is implemented in this task.

The expected runtime behavior change is limited to validated startup prerequisites: bounded outbound configuration and explicit Helper-owned local state roots. No HTTP request is made by this task.

## 2. Linux Service Shape

The Linux systemd unit currently restricts the long-lived service to `AF_UNIX`, which blocks future outbound HTTPS polling. This task should widen only the required address families:

```text
RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6
```

Do not add packet, raw, netlink, broad address-family, service-manager, sudo, or listener permissions. The unit must continue to run as `borgee-helper`, keep `NoNewPrivileges=yes`, preserve strict filesystem protections, and expose only the existing UDS socket locally.

Linux write permission should be expressed through systemd-owned path boundaries. The unit should name the existing socket/audit paths plus explicit Helper-owned state roots needed by later queue/status work. The existing grants database stays read-only.

## 3. macOS Launchd And Sandbox Shape

macOS launchd should pass the same bounded outbound configuration and explicit state roots as Linux while continuing to run as `_borgee-helper` under the existing `sandbox-exec` wrapper.

The macOS sandbox profile may allow remote TCP for outbound Helper polling prerequisites:

```scheme
(allow network-outbound (remote tcp))
```

That sandbox permission is not the destination allowlist. `sandbox-exec` cannot enforce the required Borgee domain allowlist by itself, so Helper config and client construction must reject unknown origins before any later outbound request can be built. Review should treat remote TCP without code-level origin validation as incomplete.

The macOS profile should also name the Helper-owned write roots for queue/status/audit-handoff prerequisites rather than broad application-support writes. UDS bind/outbound permissions remain local unix only.

## 4. Helper Daemon Configuration

Add Helper-owned startup configuration for outbound prerequisites. The exact flag names can follow local CLI conventions, but the design expects these concepts:

- Borgee API server origin or base URL.
- Allowed Borgee API domains/origins.
- Helper-owned state directory for future queue cursor or poll state.
- Helper-owned state directory for bounded status material.
- Helper-owned audit handoff directory if later upload/handoff work needs a file boundary distinct from the existing JSONL audit log.
- Credential path only if the current enrollment credential storage model requires an explicit Helper-owned filesystem boundary in this service layer.

Configured outbound mode must fail closed. Missing config may leave outbound prerequisites disabled for local/manual startup, but malformed configured values must fail startup. Service assets for installed production mode should provide explicit values rather than relying on defaults that could drift into arbitrary network or filesystem access.

## 5. Origin Validation Boundary

Destination control belongs in Helper config/client code, not in job payloads, local policy manifests, Remote Agent state, host grants, or the macOS sandbox profile.

Validation rules should establish these boundaries:

- Production origins use HTTPS and have a non-empty normalized host.
- The configured server origin must match the configured allowlist at startup.
- Allowlist matching is exact-host by default; any wildcard support must be explicit and suffix-safe, not substring or prefix matching.
- Later outbound URL construction can use only fixed Helper-owned relative endpoint paths.
- Client-supplied, job-payload-supplied, or manifest-supplied full URLs are rejected.
- Unknown hosts, malformed hosts, non-HTTP(S) schemes, userinfo, fragments, and path traversal fail closed.
- Local loopback exceptions, if needed for tests, stay test/dev-only and cannot appear in production service assets.

This task may introduce the validation surface and tests needed to lock the boundary, but it does not implement the poll/lease/result endpoint contract.

## 6. Write-Path Model

Helper write authority should be explicit service state, not general file authority.

Allowed write roots for this prerequisite are limited to:

- Existing local UDS socket directory.
- Existing local JSONL audit log path.
- Future queue/poll cursor state owned by Helper.
- Future bounded status state owned by Helper.
- Future local audit handoff state owned by Helper, if needed.
- Helper credential storage path only if this task must make that boundary explicit for outbound service startup.

These paths are configured by installation/service assets, not by clients, job payloads, Remote Agent state, or host grants. They must be absolute, narrow, Helper-owned, and created with owner-only or tighter permissions where the daemon owns creation.

State paths must not store raw Helper credentials in logs, private file content, private message content, full environment dumps, arbitrary path exports, unbounded logs, or Remote Agent data. The existing grants database remains read-only and must not be reused as a writable queue/status store.

## 7. Verification Strategy

Static verification should cover the service and sandbox boundary without needing root, launchd, systemd, or a live remote server:

- Source or asset checks lock Linux `RestrictAddressFamilies` to exactly `AF_UNIX AF_INET AF_INET6`.
- Asset checks prove no raw/packet/broad network families, sudo cache, Remote Agent WebSocket/token flags, poll loop flags, result/lease flags, service restart target flags, or arbitrary host-control settings were added.
- Plist and systemd asset checks prove installed startup passes the bounded outbound config and explicit state roots.
- macOS sandbox generation/static profile checks prove remote TCP is present only with the paired code-level allowlist boundary, and write roots remain explicit.
- Config validation tests prove allowed origins pass and unknown, malformed, client-supplied, or job-supplied destinations fail closed.
- State-path tests prove only explicit Helper-owned absolute paths are accepted and created with narrow permissions.

Runtime verification should stay at prerequisite level:

- Existing daemon startup/listen tests may be updated to pass temporary state roots and test-only loopback outbound config.
- Runtime tests should verify startup validation, sandbox/profile shape where supported, UDS listen behavior, audit setup, and clean shutdown.
- Do not add a fake poll server, lease/result fixtures, local policy execution, OpenClaw action, or service lifecycle restart assertions in this task.

Expected implementation verification after code exists is a focused Helper package test run plus the existing daemon startup integration slice. If platform sandbox support or privileges are unavailable, record the exact skip/fallback reason in progress and rely on static asset/config tests for default PR evidence.

## 8. Docs/Current Sync Targets

After implementation, sync current docs where the accepted behavior changes:

- `docs/current/host-bridge/helper-daemon.md`: outbound prerequisite config, explicit state roots, unchanged UDS-only inbound boundary, and poll loop still out of scope until later tasks.
- `docs/current/host-bridge/README.md`: Host Bridge capability summary and known Helper outbound prerequisite status.
- `docs/current/security/README.md`: Helper outbound origin validation, rail separation, non-sudo boundary, and Remote Agent non-reuse.
- `docs/current/known-gaps.md`: any remaining poll/lease/result/local-policy/service-lifecycle gaps or sandbox verification limitations.

Docs should not describe this task as Configure OpenClaw closure or as implemented job polling.

## 9. Explicit Non-Goals

This task must not implement or widen into:

- Helper poll loop, long-poll loop, lease, ack, result upload, retry/backoff, cancellation, or bounded log upload.
- Local policy manifest evaluation, artifact verification, path/domain/service allowlist execution, or OpenClaw action.
- Service lifecycle restart, boot restart, crash restart, arbitrary service unit control, or service-manager execution.
- Sudo cache, silent escalation, persistent privileged helper, or installer trust changes.
- Inbound server dial, new TCP listener, generic host-control listener, shell, argv, script, executable path, arbitrary local path, or arbitrary network domain authority.
- Remote Agent rail reuse, including credentials, reverse WebSocket transport, host grants, file-proxy status, remote-node rows, or Remote Agent permission fallbacks.

## 10. Acceptance Mapping

- Linux prerequisite: systemd allows only UDS plus IPv4/IPv6 families, keeps non-sudo hardening, and names explicit write paths.
- macOS prerequisite: launchd and sandbox profile allow outbound TCP only as a prerequisite, with Helper code-level origin validation carrying destination control.
- Helper config: startup has fail-closed server origin/domain validation and explicit state path configuration.
- Write paths: queue/status/audit-handoff or credential paths are Helper-owned and narrow, with no arbitrary writes.
- Rail separation: no Remote Agent credential, transport, host grant, file-proxy status, or permission rail configures Helper outbound service authority.
- Task status: this design is ready for review; product implementation remains blocked until design review accepts it.
