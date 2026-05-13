# Helper Daemon

The helper daemon is the local enforcement component of Host Bridge. It receives local IPC requests, checks identity and grants, records audit events, and performs the narrow host operation set that is currently allowed.

## Overview

**Role**
The helper is a host-resident mediator. It prevents server or plugin code from directly touching host resources by forcing requests through a local decision path.

**Boundary**
The boundary is the IPC request. A request is not trusted because it arrived over the local socket; it must match the handshake agent identity, use an allowed action, normalize to a supported scope, and match an active grant.

**Collaborators**
The helper collaborates with server-created host grants through read-only SQLite access, with local clients through UDS JSON lines, with local audit through JSONL append, and with the operating system through sandbox primitives. It does not talk to Remote Agent or the remote WebSocket hub.

**Internal Architecture**

- Startup layer: opens audit output, requires grant DB configuration, applies platform sandbox, then listens on UDS.
- IPC layer: validates the handshake and frames request/response JSON lines.
- ACL layer: enforces action allowlist, cross-agent identity, path normalization, and grant lookup.
- Execution layer: performs read-only file actions or accepts a network-egress decision.
- Audit layer: appends one record per request, including rejected requests.

**Key Flows**

```text
daemon boot -> audit sink -> read-only grant DB -> ACL gate -> sandbox -> UDS listen
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

## Sandbox Model

Linux applies a Landlock read-only ruleset when supported by the kernel. With no configured read paths, the intended shape is deny-by-default. If the kernel lacks Landlock support, the sandbox layer falls back without aborting startup, so ACL and OS permissions become the effective boundary.

macOS uses a wrapper model. The helper process itself does not self-apply a sandbox; a generated profile is intended to be applied by `sandbox-exec` before the daemon starts. The helper keeps the same internal ACL path on both platforms so platform sandboxing is defense in depth, not the only enforcement layer.

## Audit Model

Helper audit is local JSONL. It records the actor, action, target, timestamp, and matched scope for both allowed and rejected requests. Audit write failure is not allowed to block the IPC path, so helper audit is evidence-oriented rather than a transactional commit log.

## Out Of Scope

The helper does not create grants, write files, expose Remote Agent directories, install itself, or provide an admin API.

## Known Gaps

- Sandbox read paths are fixed at daemon start; dynamic grants can change ACL outcomes without changing the already-applied platform sandbox.
- The macOS sandbox depends on correct wrapper deployment.
- Local JSONL audit is not currently a first-class server audit source.

## Implementation Anchors

- `packages/borgee-helper/cmd/borgee-helper/main.go`
- `packages/borgee-helper/internal/ipc` (`Handler`, `Request`, `Response`)
- `packages/borgee-helper/internal/acl` (`Gate`, `Action`, `Decision`)
- `packages/borgee-helper/internal/grants` (`Consumer`, `SQLiteConsumer`)
- `packages/borgee-helper/internal/fileio` (`ReadFile`, `ListFiles`)
- `packages/borgee-helper/internal/audit` (`Logger`, `Event`)
- `packages/borgee-helper/internal/sandbox` (`Profile`, platform `Apply`)
