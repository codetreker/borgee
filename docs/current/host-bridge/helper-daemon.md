# Helper Daemon

The helper daemon is the local enforcement component of Host Bridge. It receives local IPC requests, checks identity and grants, records audit events, and performs the narrow host operation set that is currently allowed.

## Privilege Separation (two-process model)

The helper now splits into two long-lived processes from the same `borgee` binary:

- `borgee daemon` (User=`borgee`, the process this doc otherwise describes) — holds the outbound WebSocket to the server, parses untrusted server payloads, and runs no-root executors. This is the high-attack-surface side; it intentionally does not hold root.
- `borgee rootd` (User=`root`, NEW) — listens on a local Unix-domain socket (`/run/borgee/borgee-rootd.sock` on Linux, `/Users/Shared/Borgee/borgee-rootd.sock` on macOS), accepts only a hardcoded command whitelist, and executes. The main daemon forwards root-requiring jobs to rootd via this IPC. Peer-cred checks (SO_PEERCRED on Linux, getpeereid on macOS) gate the socket to members of the `borgee` group.

See [`docs/blueprint/current/host-bridge.md`](../../blueprint/current/host-bridge.md) §1.1 for the rationale: an attacker who compromises the WS-facing main daemon gets the `borgee` user (constrained by systemd hardening), not root. Root operations are gated by rootd's narrow whitelist plus the IPC peer-cred check.

Skeleton PR-1 ships only the rootd binary, systemd unit, and a placeholder `ping` whitelist entry to validate the IPC + auth + audit pattern. PR-4 adds the three real root commands (`install_plugin`, `service_lifecycle`, `delegation_revoke`).

## Overview

**Role**
The helper is a host-resident mediator. It prevents server or plugin code from directly touching host resources by forcing requests through a local decision path.

**Boundary**
The boundary is the IPC request. A request is not trusted because it arrived over the local socket; it must match the handshake agent identity, use an allowed action, normalize to a supported scope, and match an active grant.

**Collaborators**
The helper collaborates with server-created host grants through read-only SQLite access, with local clients through UDS JSON lines, with local audit through JSONL append, with the operating system through sandbox primitives, and with the server through a narrow outbound Helper job client. It has validated outbound service prerequisites, bounded boot/crash restart settings in the installed service assets, a pure local job-policy evaluator for delivered server-owned job views, and a fixed-path outbound client that can poll Helper job endpoints, ack receipt, and upload bounded terminal metadata when a daemon loop supplies current Helper credentials. It does not talk to Remote Agent or the remote WebSocket hub.

**Internal Architecture**

- Startup layer: opens audit output, requires grant DB configuration, validates any configured outbound origin/state prerequisites, applies platform sandbox, then listens on UDS.
- IPC layer: validates the handshake and frames request/response JSON lines.
- ACL layer: enforces action allowlist, cross-agent identity, path normalization, and grant lookup.
- Job policy layer: validates a delivered Helper job candidate before any future host-management action can start. It returns deterministic allow/deny decisions with reasons such as `schema_invalid`, `unknown_job_type`, `manifest_invalid`, `artifact_invalid`, `path_denied`, `domain_denied`, `service_denied`, `revoked`, `stale_credential`, `wrong_owner`, `wrong_org`, and `policy_denied`.
- Outbound job client: uses the prepared server origin plus fixed relative poll/ack/result paths, sends the current Helper credential and `helper_device_id`, and maps no-work, retry, stale credential, revoked, and uninstalled responses into daemon-loop directives.
- Execution layer: performs read-only file actions or accepts a network-egress decision.
- Audit layer: appends one record per request, including rejected requests.

**Key Flows**

```text
daemon boot -> audit sink -> read-only grant DB -> outbound prerequisite validation -> ACL gate -> sandbox -> UDS listen
connection -> handshake agent id -> request -> ACL decision
allowed -> read/list/egress decision -> response -> audit
rejected -> rejection response -> audit
future Helper job gate:
delivered server-owned job view -> strict schema validation -> local enrollment/state recheck
-> signed manifest and artifact digest binding where required
-> path/domain/service binding against sandbox profile -> allow/deny reason only
outbound job client -> poll fixed server path -> no_work retry or leased typed job
leased job -> ack receipt -> later policy/action handoff -> bounded terminal result metadata
```

**Invariants**

- The helper is a consumer of grants, not the writer of grants.
- Grant lookup is fresh per request; revocation is visible at the next lookup.
- The request agent id must match the connection handshake agent id.
- File actions require absolute normalized paths and are represented as filesystem scopes.
- The helper's file IO surface is read-only.
- Helper job policy is a pre-action gate only. It does not execute OpenClaw actions, write config, call a service manager, poll, lease, ack, or upload results.
- Job payload fields cannot add shell, argv, executable path, script, arbitrary service unit, local path, network domain, credential, environment dump, or raw file authority.
- Manifest-required jobs must bind to a verified Ed25519-signed runtime manifest digest, server-owned binding JSON, artifact cache bytes matching signed SHA-256 digests where artifacts are needed, and declared path/domain/service IDs before policy can allow. `openclaw.configure_agent` requires a signed manifest plus approved config path binding; `openclaw.install_from_manifest` requires signed manifest, artifact, approved paths, and approved artifact origin binding; `borgee_plugin.configure_connection` requires a server-owned connection/channel payload plus approved Borgee plugin config path binding; `service.lifecycle` requires a server-owned lifecycle payload plus declared logical service IDs that also appear in supplied sandbox/profile service affordances.
- Local policy rechecks owner, org, enrollment id, Helper device id, credential generation, active enrollment status, category delegation, revocation, stale credential state, and job expiry.
- Configured outbound prerequisites fail closed for literal origins: the server origin must be an allowed exact public HTTPS origin, literal host/IP input is classified with `netip`, localhost/private/link-local/metadata literal origins are rejected even over HTTPS, and state roots must normalize under Helper-owned state directories.
- Helper job HTTP is outbound-only and fixed-path. Job payloads, manifests, Remote Agent state, host grants, and user input cannot supply a URL override.
- The local UDS remains the only inbound listener.

## Sandbox Model

Linux applies a Landlock read-only ruleset when supported by the kernel. The installed systemd service permits only `AF_UNIX`, `AF_INET`, and `AF_INET6` address families so later Helper polling can use outbound HTTPS while the daemon still exposes only the local UDS inbound path. The service restarts on failure with `RestartSec=10s`, `StartLimitIntervalSec=5min`, and `StartLimitBurst=5`, while continuing to run as the non-root Helper user. With no configured read paths, the intended shape is deny-by-default. If the kernel lacks Landlock support, the sandbox layer falls back without aborting startup, so ACL, systemd hardening, and OS permissions become the effective boundary.

macOS uses a wrapper model. The helper process itself does not self-apply a sandbox; a generated profile is intended to be applied by `sandbox-exec` before the daemon starts. The installed launchd plist runs at load, restarts after unsuccessful exit through failure-only `KeepAlive`, and uses `ThrottleInterval=10` while continuing to run as `_borgee`. The installed sandbox profile keeps local Unix socket bind/outbound permissions for UDS and permits remote TCP only as an outbound prerequisite; destination allowlisting is enforced by Helper startup config validation, not by `sandbox-exec`. The helper keeps the same internal ACL path on both platforms so platform sandboxing is defense in depth, not the only enforcement layer.

## Outbound Prerequisite Model

The daemon accepts optional startup flags for a Borgee server origin, an exact allowed-origin list, and three Helper-owned state directories: queue cursor state, bounded status state, and audit handoff state. If none of those flags are set, local/manual startup leaves outbound prerequisites disabled. If any of them are set, all are required and malformed values abort startup. Default validation classifies literal host/IP input with `netip` and rejects localhost, loopback, RFC1918, link-local, metadata, and IPv6 local/private literal origins even when the scheme is HTTPS; the only local exception is an explicit test/development option for HTTP loopback.

This prerequisite validation does not resolve allowed hostnames and does not inspect DNS answers or CNAME chains. Production service assets use the exact `https://app.borgee.io` allowlist, but DNS resolution or rebinding to private, link-local, or metadata addresses remains outside this startup validator and should be handled by future hardening or runtime network policy.

The installed Linux and macOS service assets set the production origin to `https://app.borgee.io`, allow only that exact origin, and name platform-specific Helper-owned state roots. The daemon creates configured state directories with owner-only permissions. These paths are service state only; clients, job payloads, Remote Agent state, and host grants do not choose them.

## Local Job Policy Model

`internal/jobpolicy` is a pure evaluator for the Helper job boundary that later transport/action tasks can call. It validates only inputs it is given: the server-owned job view, current Helper enrollment state, explicit Ed25519 trust roots, artifact cache bytes, and sandbox/profile affordances for paths, origins, and logical service IDs. It returns a decision and reason; it does not perform IO, HTTP, service-manager calls, OpenClaw execution, result upload, or settlement.

The policy manifest is a runtime Helper contract, separate from the existing installer manifest path. The evaluator verifies canonical manifest bytes with Ed25519, compares the canonical SHA-256 digest to the job `manifest_digest`, verifies the server-owned binding references only declared artifact/path/domain/service IDs, and hashes local artifact bytes before allowing manifest-required work. Paths must be absolute, traversal-free, non-root, and supported by the supplied sandbox write/read roots. Domains must normalize to exact public HTTPS origins and also appear in the supplied allowed-origin profile. Service lifecycle policy accepts only fixed operations and logical manifest service IDs that are also present in the supplied service capability list; duplicate IDs, path-like IDs, platform manager mismatches, unsafe systemd unit names, and unsafe launchd labels are denied.

The evaluator preserves the documented DNS limitation from the outbound prerequisite model: it rejects unsafe literal origins but does not resolve allowed hostnames or inspect DNS answers/CNAME chains.

## Helper Job Transport

The helper outbound package now has a typed client for the server Helper job rail. It builds only these fixed relative paths from the prepared origin:

- `POST /api/v1/helper/enrollments/{enrollmentId}/jobs/poll`
- `POST /api/v1/helper/enrollments/{enrollmentId}/jobs/{jobId}/ack`
- `POST /api/v1/helper/enrollments/{enrollmentId}/jobs/{jobId}/result`

Every request sends `Authorization: Bearer <helper credential>` and `helper_device_id`. Poll can return `no_work` with bounded retry metadata or one leased typed job with a lease token and lease expiry. Ack records receipt only. Result upload sends terminal status, closed failure codes, a bounded redacted failure message, and small opaque audit/log references. Non-success terminal statuses require a reason code; matching terminal replays are idempotent and conflicting terminal replays are rejected. `401`, `stale_credential`, `revoked`, and `uninstalled` are stop directives for the daemon loop.

This is transport and settlement plumbing. It does not implement local policy, manifest/artifact verification, sandbox allowlist decisions, OpenClaw execution, service-manager calls, raw log upload, or Configure OpenClaw success. Result summaries are references only; raw tokens, credentials, private file/message content, full environment dumps, command text, scripts, arbitrary paths, URLs, and service unit names are not accepted as result metadata.

## Transport (PR-2 #1038, WebSocket)

The daemon's outbound transport is a persistent WebSocket connection to
`wss://<server>/ws/helper/<enrollmentId>`. The blueprint at
[`docs/blueprint/current/host-bridge.md`](../../blueprint/current/host-bridge.md)
locks the high-level architecture; this section is the operational
reference for the wire protocol.

**Upgrade authentication.** The daemon sends `Authorization: Bearer
<helper_credential>`, `X-Helper-Device-Id: <device_id>`, and (PR-4
final amend) `X-Helper-Platform: <runtime.GOOS>` headers on the
upgrade request. The server validates the credential digest +
device id (same `HelperEnrollmentRepository.UpdateLastSeen` call the
prior REST `/status` route used) and bumps `last_seen_at` in one DB
write. On failure the server returns 401/403/404 and the daemon's
reconnect backoff applies. The daemon does not send an Origin header
(it is not a browser); the server rejects any upgrade with a non-empty
Origin as a defense against confused-deputy attacks.

**One session per enrollment.** A second WS connect for the same
enrollment displaces the older session with close code 4001
("displaced"). The daemon's outbound client treats a 4001 close as a
normal reconnect signal — the operator may have re-claimed on another
device, in which case the older daemon should yield.

**Frame protocol** (text frames, JSON):

- Server → daemon: `{"type":"job","job":{...}}` — push a leased job.
  The `job` object matches the prior REST `/jobs/poll` lease shape.
- Server → daemon: `{"type":"directive","code":"revoked"|"stale_credential"|"uninstalled"|"unauthorized"|"displaced"}` — tell the daemon to stop. Stop codes cause the daemon to exit; systemd `Restart=on-failure` rebounds under StartLimit.
- Daemon → server: `{"type":"ack","job_id":"...","lease_token":"..."}` — server marks the lease delivered + extends TTL. Equivalent to the prior REST `POST /jobs/<id>/ack`.
- Daemon → server: `{"type":"result","job_id":"...","lease_token":"...","status":"succeeded"|"failed","failure_code":"...","failure_message":"...","summary":{...}}` — terminal job result. Equivalent to `POST /jobs/<id>/result`.

**Heartbeat.** WS `Ping`/`Pong` control frames every 30s. The server's
pong handler updates `last_seen_at`; the freshness window stays 5
minutes. Three consecutive ping failures tear the connection down and
the outbound client redials with exponential backoff (1s base, 30s
cap, ±20% jitter). The previous POST `/status` producer is removed.

**Backward compatibility.** The REST endpoints `POST /api/v1/helper/enrollments/{id}/jobs/poll`, `/jobs/{id}/ack`, `/jobs/{id}/result`, and `/status` stay mounted but are marked Deprecated. The shared `ProcessHelperAck` / `ProcessHelperResult` mutations are reused by both the WS read loop and the legacy REST handlers so there is one source of truth for the store mutation. The Deprecated REST `/jobs/poll` body now requires a `helper_platform` field (`linux` or `darwin`) — missing or unknown returns 400 `helper_platform_required`. Production daemons since the PR-2 WS rewrite never call REST poll; the field is the same selector the WS upgrade carries via `X-Helper-Platform`.

**Platform header (PR-4 final amend).** The WS upgrade carries `X-Helper-Platform: <runtime.GOOS>` (`linux` or `darwin`). The server rejects a missing or unknown value with HTTP 400 before the WS handshake completes. The session stores the parsed platform so every pushed job frame carries the matching signed canonical manifest body (Linux paths + systemd vs. macOS paths + launchd). The platform-to-manifest mapping lives in `packages/server-go/internal/helpermanifest`.

**Push wiring (PR-4 final amend).** Jobs land on the daemon via two paths now:

1. **Push-on-enqueue.** When a user enqueues a helper job via `POST /api/v1/helper/enrollments/{enrollmentId}/jobs`, the server checks whether a daemon WS session is connected for that enrollment. If yes, it immediately leases the next queued job for that enrollment+device (same `PollAndLeaseForHelper` mutation REST poll uses) and pushes a `{"type":"job","job":...}` frame onto the WS write pump. Sub-second delivery.
2. **Connect-hook drain.** When a daemon reconnects after a transient drop, the hub's `OnHelperConnected` hook fires and the helper-jobs handler leases + pushes one queued job to the freshly-connected session. The store's lease semantics are one-at-a-time per enrollment, so subsequent jobs follow on each ack/result frame.

Push is best-effort. If the WS send buffer is full or the lease conflicts with a concurrent poll, the job stays queued and either REST poll fallback or the next connect-hook drain delivers it. No double-execution: the lease mutation is idempotent at the store layer.

**Sequence (enqueue → execute → ack):**

```
owner (browser) ──POST /jobs──▶ server (api.HelperJobsHandler)
                                  │ store.EnqueueForUser (persisted)
                                  │ hub.GetHelper(enrollmentID) → session
                                  │ store.PollAndLeaseForHelper (leased)
                                  │ serializeHelperJobLease(lease, platform)
                                  └─▶ session.SendJob(payload)  ◀── ws write pump
                                                                       │
                                                                       ▼
                                                            daemon outbound.Receive
                                                                       │
                                                            jobpolicy.Evaluate (manifest sig + binding)
                                                                       │
                                                            dispatcher.routeByJobType
                                                                       │
                                                                       ▼
                                                            executor.Execute (5 no-root) /
                                                            rootd IPC (3 root)
                                                                       │
                                                            daemon ──{type:"ack"}──▶ server
                                                            daemon ──{type:"result"}──▶ server
                                                            server.ProcessHelperResult (terminal)
```

## Audit Model

Helper audit is local JSONL for the current IPC path. It records the actor, action, target, timestamp, and matched scope for both allowed and rejected requests. Audit write failure is not allowed to block the IPC path, so helper audit is evidence-oriented rather than a transactional commit log. Helper job policy decisions, including Borgee plugin connection/channel binding decisions, are shaped for future daemon-loop action wiring and bounded status handoff, but the current daemon does not upload or settle local policy decisions.

## Out Of Scope

The helper does not create grants, write files, expose Remote Agent directories, install itself, provide an admin API, execute OpenClaw actions, write Borgee plugin config, upload bounded logs, or restart services. The local job-policy evaluator is present as a pure pre-action decision package; the outbound client is transport only and is not a host action loop.

## Executors (all 8 JobTypes)

The dispatcher registers per-`JobType` executors. Five run **in-process inside the `borgee daemon`** (`User=borgee`, no root) and only touch borgee-owned paths or read-only system state. The three root-requiring ones delegate the privileged operation to the `borgee rootd` companion (`User=root`) via local UDS IPC; rootd's command whitelist is the narrow security boundary. See `helper-daemon.md` § Privilege Separation for the rationale.

The 5 no-root executors (`helper.uninstall`, `status.collect`, `state.write`, `openclaw.configure_agent`, `borgee_plugin.configure_connection`) resolve their target paths from the signed manifest carried in each leased job, NOT from any daemon-startup flag. The manifest is signed by Borgee server's ed25519 trust root; daemon-side `jobpolicy.Evaluate` validates the signature plus the binding's PathIDs are subset of manifest's allowed paths. The executor then re-parses the same manifest+binding via `internal/executors/manifestpath.Resolve(<PathID>)` to look up the concrete absolute root and writes there. No daemon-startup flag controls remote-write paths. The systemd unit's `ReadWritePaths` must align with the manifest-declared roots; misalignment fails loud at write time (the executor does NOT invent fallback paths).

The 3 root-requiring executors (`openclaw.install_from_manifest`, `service.lifecycle`, `delegation.revoke`) follow the same manifest-binding pattern but forward the actual privileged syscall into rootd. The daemon-side executor builds the rootd request from the leased job's payload + manifest binding; rootd re-validates every field at the IPC boundary (defense-in-depth) before invoking install-butler / systemctl / launchctl / file removal. No new daemon-startup flag carries paths or unit names — manifest is authority.

- `helper.uninstall` — one-key self-teardown of the helper footprint. See `packages/borgee/internal/executors/uninstall/README.md`.
- `status.collect` — gather machine + helper + plugin status and return the snapshot in the terminal result summary (no filesystem write). See `packages/borgee/internal/executors/statuscollect/README.md`.
- `state.write` — write an attested state record under the manifest-declared `borgee_state_config` path. See `packages/borgee/internal/executors/statewrite/README.md`.
- `openclaw.configure_agent` — record the server-attested per-agent config metadata under the manifest-declared `openclaw_agent_config` path. See `packages/borgee/internal/executors/openclawconfigure/README.md`.
- `borgee_plugin.configure_connection` — record the server-attested plugin connection metadata under the manifest-declared `borgee_plugin_config` path. See `packages/borgee/internal/executors/pluginconfigure/README.md`.
- `openclaw.install_from_manifest` — fetch + verify + place a signed runtime plugin binary under the manifest-declared `openclaw_install` path. Daemon executor calls rootd's `install_plugin` whitelist entry which invokes install-butler in-process. See `packages/borgee/internal/executors/installplugin/README.md`.
- `service.lifecycle` — start / stop / restart / reload / enable / disable a manifest-declared service unit. Daemon executor resolves the binding's ServiceID against the manifest's `ServiceDeclaration` to get `(manager, unit)`, then calls rootd's `service_lifecycle` whitelist entry which exec's systemctl / launchctl. See `packages/borgee/internal/executors/servicelifecycle/README.md`.
- `delegation.revoke` — disable `borgee.service` (so systemd does not respawn) + wipe the helper credential / enrollment-id / device-id files. Daemon executor drains the dispatcher then calls rootd's `delegation_revoke` whitelist entry. The daemon process exits naturally after the WS Result frame is sent (no self-stop signal, mirroring `helper.uninstall`). See `packages/borgee/internal/executors/delegationrevoke/README.md`.

### rootd whitelist (PR-4 #1033)

`borgee rootd` (`User=root`) listens on a local UDS and accepts exactly four commands:

| cmd | purpose | called by |
|---|---|---|
| `ping` | smoke / connectivity check | health probe (PR-1) |
| `install_plugin` | fetch + verify + place a signed plugin binary | `executors/installplugin` |
| `service_lifecycle` | exec systemctl / launchctl | `executors/servicelifecycle` |
| `delegation_revoke` | disable service + wipe credential files | `executors/delegationrevoke` |

Every command type-checks its params with a fixed schema, rejects unknown fields, audit-logs the request envelope, and is allow-listed at the daemon's `DefaultHandlers()` map. There is no eval / shell / arbitrary-exec command. See `packages/borgee/internal/cli/rootd/README.md` (if present) or the package doc comment for the threat model.

## Known Gaps

- Sandbox read paths are fixed at daemon start; dynamic grants can change ACL outcomes without changing the already-applied platform sandbox.
- The macOS sandbox depends on correct wrapper deployment.
- Local JSONL audit is not currently a first-class server audit source.
- Outbound origin validation rejects unsafe literal origins but does not resolve allowed hostnames or guard against DNS answers/CNAMEs resolving to private, link-local, or metadata addresses.
- Helper outbound prerequisites, bounded Helper boot/crash service restart settings, the fixed-path poll/ack/result client, bounded terminal settlement, server-bound service lifecycle enqueue, and the local job-policy evaluator exist, but daemon-loop credential persistence, raw/bulk log upload, policy-to-action wiring, local Borgee plugin config writes, OpenClaw execution, and service-manager action execution remain future work.

## Implementation Anchors

- `packages/borgee/cmd/borgee/main.go`
- `packages/borgee/internal/ipc` (`Handler`, `Request`, `Response`)
- `packages/borgee/internal/acl` (`Gate`, `Action`, `Decision`)
- `packages/borgee/internal/grants` (`Consumer`, `SQLiteConsumer`)
- `packages/borgee/internal/fileio` (`ReadFile`, `ListFiles`)
- `packages/borgee/internal/audit` (`Logger`, `Event`)
- `packages/borgee/internal/sandbox` (`Profile`, platform `Apply`)
- `packages/borgee/internal/jobpolicy` (`Evaluate`, `Decision`, runtime policy manifest and binding types)
- `packages/borgee/internal/outbound` (`PrereqConfig`, `ValidateAndPrepare`, `Client`)
- `packages/borgee/install` (systemd, launchd, and macOS sandbox assets)
