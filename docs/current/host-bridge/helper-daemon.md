# Helper Daemon

The helper daemon is the local enforcement component of Host Bridge. It receives local IPC requests, checks identity and grants, records audit events, and performs the narrow host operation set that is currently allowed.

## Overview

**Role**
The helper is a host-resident mediator. It prevents server or plugin code from directly touching host resources by forcing requests through a local decision path.

**Boundary**
The boundary is the IPC request. A request is not trusted because it arrived over the local socket; it must match the handshake agent identity, use an allowed action, normalize to a supported scope, and match an active grant.

**Collaborators**
The helper collaborates with server-created host grants through read-only SQLite access, with local clients through UDS JSON lines, with local audit through JSONL append, with the operating system through sandbox primitives, and with the server through a narrow outbound Helper job client. It has validated outbound service prerequisites, a pure local job-policy evaluator for delivered server-owned job views, and a fixed-path outbound client that can poll Helper job endpoints, ack receipt, and upload bounded terminal metadata when a daemon loop supplies current Helper credentials. It does not talk to Remote Agent or the remote WebSocket hub.

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
- Manifest-required jobs must bind to a verified Ed25519-signed runtime manifest digest, server-owned binding JSON, artifact cache bytes matching signed SHA-256 digests, and declared path/domain/service IDs before policy can allow.
- Local policy rechecks owner, org, enrollment id, Helper device id, credential generation, active enrollment status, category delegation, revocation, stale credential state, and job expiry.
- Configured outbound prerequisites fail closed for literal origins: the server origin must be an allowed exact public HTTPS origin, literal host/IP input is classified with `netip`, localhost/private/link-local/metadata literal origins are rejected even over HTTPS, and state roots must normalize under Helper-owned state directories.
- Helper job HTTP is outbound-only and fixed-path. Job payloads, manifests, Remote Agent state, host grants, and user input cannot supply a URL override.
- The local UDS remains the only inbound listener.

## Sandbox Model

Linux applies a Landlock read-only ruleset when supported by the kernel. The installed systemd service permits only `AF_UNIX`, `AF_INET`, and `AF_INET6` address families so later Helper polling can use outbound HTTPS while the daemon still exposes only the local UDS inbound path. With no configured read paths, the intended shape is deny-by-default. If the kernel lacks Landlock support, the sandbox layer falls back without aborting startup, so ACL, systemd hardening, and OS permissions become the effective boundary.

macOS uses a wrapper model. The helper process itself does not self-apply a sandbox; a generated profile is intended to be applied by `sandbox-exec` before the daemon starts. The installed sandbox profile keeps local Unix socket bind/outbound permissions for UDS and permits remote TCP only as an outbound prerequisite; destination allowlisting is enforced by Helper startup config validation, not by `sandbox-exec`. The helper keeps the same internal ACL path on both platforms so platform sandboxing is defense in depth, not the only enforcement layer.

## Outbound Prerequisite Model

The daemon accepts optional startup flags for a Borgee server origin, an exact allowed-origin list, and three Helper-owned state directories: queue cursor state, bounded status state, and audit handoff state. If none of those flags are set, local/manual startup leaves outbound prerequisites disabled. If any of them are set, all are required and malformed values abort startup. Default validation classifies literal host/IP input with `netip` and rejects localhost, loopback, RFC1918, link-local, metadata, and IPv6 local/private literal origins even when the scheme is HTTPS; the only local exception is an explicit test/development option for HTTP loopback.

This prerequisite validation does not resolve allowed hostnames and does not inspect DNS answers or CNAME chains. Production service assets use the exact `https://app.borgee.io` allowlist, but DNS resolution or rebinding to private, link-local, or metadata addresses remains outside this startup validator and should be handled by future hardening or runtime network policy.

The installed Linux and macOS service assets set the production origin to `https://app.borgee.io`, allow only that exact origin, and name platform-specific Helper-owned state roots. The daemon creates configured state directories with owner-only permissions. These paths are service state only; clients, job payloads, Remote Agent state, and host grants do not choose them.

## Local Job Policy Model

`internal/jobpolicy` is a pure evaluator for the Helper job boundary that later transport/action tasks can call. It validates only inputs it is given: the server-owned job view, current Helper enrollment state, explicit Ed25519 trust roots, artifact cache bytes, and sandbox/profile affordances for paths, origins, and logical service IDs. It returns a decision and reason; it does not perform IO, HTTP, service-manager calls, OpenClaw execution, result upload, or settlement.

The policy manifest is a runtime Helper contract, separate from the existing installer manifest path. The evaluator verifies canonical manifest bytes with Ed25519, compares the canonical SHA-256 digest to the job `manifest_digest`, verifies the server-owned binding references only declared artifact/path/domain/service IDs, and hashes local artifact bytes before allowing manifest-required work. Paths must be absolute, traversal-free, non-root, and supported by the supplied sandbox write/read roots. Domains must normalize to exact public HTTPS origins and also appear in the supplied allowed-origin profile. Service lifecycle policy accepts only fixed operations and logical manifest service IDs that are also present in the supplied service capability list.

The evaluator preserves the documented DNS limitation from the outbound prerequisite model: it rejects unsafe literal origins but does not resolve allowed hostnames or inspect DNS answers/CNAME chains.

## Helper Job Transport

The helper outbound package now has a typed client for the server Helper job rail. It builds only these fixed relative paths from the prepared origin:

- `POST /api/v1/helper/enrollments/{enrollmentId}/jobs/poll`
- `POST /api/v1/helper/enrollments/{enrollmentId}/jobs/{jobId}/ack`
- `POST /api/v1/helper/enrollments/{enrollmentId}/jobs/{jobId}/result`

Every request sends `Authorization: Bearer <helper credential>` and `helper_device_id`. Poll can return `no_work` with bounded retry metadata or one leased typed job with a lease token and lease expiry. Ack records receipt only. Result upload sends terminal status, closed failure codes, a bounded failure message, and small opaque audit/log references. `401`, `stale_credential`, `revoked`, and `uninstalled` are stop directives for the daemon loop.

This is transport and settlement plumbing. It does not implement local policy, manifest/artifact verification, sandbox allowlist decisions, OpenClaw execution, service lifecycle restart, bounded log upload, or Configure OpenClaw success.

## Audit Model

Helper audit is local JSONL for the current IPC path. It records the actor, action, target, timestamp, and matched scope for both allowed and rejected requests. Audit write failure is not allowed to block the IPC path, so helper audit is evidence-oriented rather than a transactional commit log. Helper job policy decisions are shaped for later task 6/task 8 transport and settlement, but this release does not upload or settle those decisions.

## Out Of Scope

The helper does not create grants, write files, expose Remote Agent directories, install itself, provide an admin API, execute OpenClaw actions, upload bounded logs, or restart services. The local job-policy evaluator is present as a pure pre-action decision package; the outbound client is transport only and is not a host action loop.

## Known Gaps

- Sandbox read paths are fixed at daemon start; dynamic grants can change ACL outcomes without changing the already-applied platform sandbox.
- The macOS sandbox depends on correct wrapper deployment.
- Local JSONL audit is not currently a first-class server audit source.
- Outbound origin validation rejects unsafe literal origins but does not resolve allowed hostnames or guard against DNS answers/CNAMEs resolving to private, link-local, or metadata addresses.
- Helper outbound prerequisites, the fixed-path poll/ack/result client, and the local job-policy evaluator exist, but daemon-loop credential persistence, bounded log upload, policy-to-action wiring, OpenClaw execution, and service lifecycle remain future work.

## Implementation Anchors

- `packages/borgee-helper/cmd/borgee-helper/main.go`
- `packages/borgee-helper/internal/ipc` (`Handler`, `Request`, `Response`)
- `packages/borgee-helper/internal/acl` (`Gate`, `Action`, `Decision`)
- `packages/borgee-helper/internal/grants` (`Consumer`, `SQLiteConsumer`)
- `packages/borgee-helper/internal/fileio` (`ReadFile`, `ListFiles`)
- `packages/borgee-helper/internal/audit` (`Logger`, `Event`)
- `packages/borgee-helper/internal/sandbox` (`Profile`, platform `Apply`)
- `packages/borgee-helper/internal/jobpolicy` (`Evaluate`, `Decision`, runtime policy manifest and binding types)
- `packages/borgee-helper/internal/outbound` (`PrereqConfig`, `ValidateAndPrepare`, `Client`)
- `packages/borgee-helper/install` (systemd, launchd, and macOS sandbox assets)
