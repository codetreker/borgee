# Host Bridge

Host Bridge is the local host capability path. It is designed for actions that need stronger host mediation than Remote Agent provides: user-granted capabilities, helper-side ACL decisions, local IPC, local audit, and platform sandboxing where available. It is not the remote filesystem WebSocket path.

## Overview

**Role**
Host Bridge lets Borgee-controlled agents request limited host capabilities through a local helper. The server owns user consent records, the helper owns local enforcement, and the installer owns deployment of the helper runtime.

**Boundary**
The boundary is a grant-backed helper request. A request must identify the agent, match the connection's agent identity, normalize to a grant scope, pass grant lookup, and then pass local OS/process constraints before host IO is attempted.

**Collaborators**
Host Bridge collaborates with the user API for grants, server storage for grant state, the helper daemon for enforcement, the installer for deployment, and admin audit views for limited visibility. It does not collaborate with the remote-agent WebSocket token path.

**Internal Architecture**

- Grant control plane: user-owned rows describing host capability consent.
- Helper data plane: local UDS IPC carrying agent-scoped requests.
- Enforcement stack: handshake identity, action allowlist, path/scope normalization, grant lookup, read-only IO, audit, sandbox.
- Installer path: signed manifest verification and platform service deployment.

**Key Flows**

```text
Grant flow:
  user grants capability -> server stores host grant -> helper sees it on next lookup

Helper request flow:
  local client connects -> handshake agent id -> request action/target
  -> ACL decision -> SQLite grant lookup -> IO or rejection -> local audit

Install flow:
  installer fetches signed manifest -> verifies signature -> user confirms
  -> package manager installs helper -> platform service starts daemon
```

**Invariants**

- User consent is represented as host grants, not as generic user API capabilities.
- Helper enforcement is per request; grant state is not cached in the helper decision path.
- Helper filesystem IO is read-only in the current capability set.
- Remote Agent and Host Bridge are separate capabilities with separate credentials, transports, and boundaries.
- Server-side host grant ownership does not imply admin-wide override.

## Submodules

- `helper-daemon.md` defines local enforcement: UDS IPC, ACL, SQLite grant lookup, audit, sandbox, and read-only IO.
- `host-grants.md` defines the server-side consent model and its invariants.
- `installer.md` defines package installation, manifest verification, and deployment responsibilities.

## Out Of Scope

Host Bridge does not provide Remote Agent browsing, plugin WebSocket API tunneling, unrestricted command execution, or admin-owned host consent.

## Known Gaps

- Helper sandbox paths are static at daemon start, while grant lookup is dynamic per request.
- Host Bridge audit is split between local helper JSONL and server-side logs; server admin multi-source audit does not yet ingest helper JSONL rows.

## Implementation Anchors

- `packages/server-go/internal/api/host_grants.go` (`HostGrantsHandler`)
- `packages/server-go/internal/migrations/host_grants.go` (`host_grants` schema)
- `packages/borgee-helper/cmd/borgee-helper/main.go`
- `packages/borgee-helper/internal/ipc` (`Request`, `Response`, `Handler`)
- `packages/borgee-helper/internal/acl` (`Gate`, `Decision`)
- `packages/borgee-helper/internal/grants` (`SQLiteConsumer`)
- `packages/borgee-helper/internal/fileio`
- `packages/borgee-helper/internal/audit`
- `packages/borgee-helper/internal/sandbox`
- `packages/borgee-installer/cmd`
- `packages/borgee-installer/internal`
