# Helper Daemon

The helper daemon is the local enforcement component of Host Bridge. It receives local IPC requests, checks identity and grants, records audit events, and performs the narrow host operation set that is currently allowed.

## Overview

**Role**
The helper is a host-resident mediator. It prevents server or plugin code from directly touching host resources by forcing requests through a local decision path.

**Boundary**
The boundary is the IPC request. A request is not trusted because it arrived over the local socket; it must match the handshake agent identity, use an allowed action, normalize to a supported scope, and match an active grant.

**Collaborators**
The helper collaborates with server-created host grants through read-only SQLite access, with local clients through UDS JSON lines, with local audit through JSONL append, and with the operating system through sandbox primitives. It now has validated outbound service prerequisites for later Helper job polling, but this release does not start a poll loop or make job HTTP requests. It does not talk to Remote Agent or the remote WebSocket hub.

**Internal Architecture**

- Startup layer: opens audit output, requires grant DB configuration, validates any configured outbound origin/state prerequisites, applies platform sandbox, then listens on UDS.
- IPC layer: validates the handshake and frames request/response JSON lines.
- ACL layer: enforces action allowlist, cross-agent identity, path normalization, and grant lookup.
- Execution layer: performs read-only file actions or accepts a network-egress decision.
- Audit layer: appends one record per request, including rejected requests.

**Key Flows**

```text
daemon boot -> audit sink -> read-only grant DB -> outbound prerequisite validation -> ACL gate -> sandbox -> UDS listen
connection -> handshake agent id -> request -> ACL decision
allowed -> read/list/egress decision -> response -> audit
rejected -> rejection response -> audit
```

**Invariants**

- The helper is a consumer of grants, not the writer of grants.
- Grant lookup is fresh per request; revocation is visible at the next lookup.
- The request agent id must match the connection handshake agent id.
- File actions require absolute normalized paths and are represented as filesystem scopes.
- The helper's file IO surface is read-only.
- Configured outbound prerequisites fail closed: the server origin must be an allowed exact HTTPS origin, and state roots must normalize under Helper-owned state directories.
- The local UDS remains the only inbound listener.

## Sandbox Model

Linux applies a Landlock read-only ruleset when supported by the kernel. The installed systemd service permits only `AF_UNIX`, `AF_INET`, and `AF_INET6` address families so later Helper polling can use outbound HTTPS while the daemon still exposes only the local UDS inbound path. With no configured read paths, the intended shape is deny-by-default. If the kernel lacks Landlock support, the sandbox layer falls back without aborting startup, so ACL, systemd hardening, and OS permissions become the effective boundary.

macOS uses a wrapper model. The helper process itself does not self-apply a sandbox; a generated profile is intended to be applied by `sandbox-exec` before the daemon starts. The installed sandbox profile keeps local Unix socket bind/outbound permissions for UDS and permits remote TCP only as an outbound prerequisite; destination allowlisting is enforced by Helper startup config validation, not by `sandbox-exec`. The helper keeps the same internal ACL path on both platforms so platform sandboxing is defense in depth, not the only enforcement layer.

## Outbound Prerequisite Model

The daemon accepts optional startup flags for a Borgee server origin, an exact allowed-origin list, and three Helper-owned state directories: queue cursor state, bounded status state, and audit handoff state. If none of those flags are set, local/manual startup leaves outbound prerequisites disabled. If any of them are set, all are required and malformed values abort startup.

The installed Linux and macOS service assets set the production origin to `https://app.borgee.io`, allow only that exact origin, and name platform-specific Helper-owned state roots. The daemon creates configured state directories with owner-only permissions. These paths are service state only; clients, job payloads, Remote Agent state, and host grants do not choose them.

## Audit Model

Helper audit is local JSONL. It records the actor, action, target, timestamp, and matched scope for both allowed and rejected requests. Audit write failure is not allowed to block the IPC path, so helper audit is evidence-oriented rather than a transactional commit log.

## Out Of Scope

The helper does not create grants, write files, expose Remote Agent directories, install itself, provide an admin API, poll for jobs, lease jobs, upload results, upload acks, run local policy, execute OpenClaw actions, or restart services.

## Known Gaps

- Sandbox read paths are fixed at daemon start; dynamic grants can change ACL outcomes without changing the already-applied platform sandbox.
- The macOS sandbox depends on correct wrapper deployment.
- Local JSONL audit is not currently a first-class server audit source.
- Helper outbound prerequisites are configured and validated, but Helper pull, lease, result, ack, bounded log upload, and local policy execution remain future work.

## Implementation Anchors

- `packages/borgee-helper/cmd/borgee-helper/main.go`
- `packages/borgee-helper/internal/ipc` (`Handler`, `Request`, `Response`)
- `packages/borgee-helper/internal/acl` (`Gate`, `Action`, `Decision`)
- `packages/borgee-helper/internal/grants` (`Consumer`, `SQLiteConsumer`)
- `packages/borgee-helper/internal/fileio` (`ReadFile`, `ListFiles`)
- `packages/borgee-helper/internal/audit` (`Logger`, `Event`)
- `packages/borgee-helper/internal/sandbox` (`Profile`, platform `Apply`)
- `packages/borgee-helper/internal/outbound` (`PrereqConfig`, `ValidateAndPrepare`)
- `packages/borgee-helper/install` (systemd, launchd, and macOS sandbox assets)
