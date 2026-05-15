# Host Bridge

Host Bridge is the local host capability path. It is designed for actions that need stronger host mediation than Remote Agent provides: user-granted capabilities, helper-side ACL decisions, local IPC, local audit, Helper enrollment/status identity, and platform sandboxing where available. It is not the remote filesystem WebSocket path.

## Overview

**Role**
Host Bridge lets Borgee-controlled agents request limited host capabilities through a local helper. The server owns user consent records and Helper enrollment/status rows, the helper owns local enforcement, and the installer owns deployment of the helper runtime.

**Boundary**
The current request boundary is a grant-backed helper request. A request must identify the agent, match the connection's agent identity, normalize to a grant scope, pass grant lookup, and then pass local OS/process constraints before host IO is attempted. Helper enrollment is a separate server-side identity/status boundary: it binds owner, org, enrollment id, helper device id, host label, allowed categories, and terminal revoke/uninstall state before later host-management work can rely on a Helper identity.

**Collaborators**
Host Bridge collaborates with the user API for grants and Helper enrollment management, server storage for grant and enrollment state, the helper daemon for enforcement, the installer for deployment, and admin audit views for limited visibility. It does not collaborate with the remote-agent WebSocket token path.

**Internal Architecture**

- Grant control plane: user-owned rows describing host capability consent.
- Helper enrollment control plane: owner/org-scoped rows describing enrolled Helper identity, allowed category visibility, device id, last seen, revoke, and helper-originated uninstall status.
- Helper data plane: local UDS IPC carrying agent-scoped requests.
- Enforcement stack: handshake identity, action allowlist, path/scope normalization, grant lookup, read-only IO, audit, sandbox.
- Installer path: current manifest verifier path, local operator confirmation, and platform service deployment.

**Key Flows**

```text
Grant flow:
  user grants capability -> server stores host grant -> helper sees it on next lookup

Helper enrollment flow:
  user creates enrollment -> local helper claims with one-time secret/device id
  -> server returns persistent Helper credential once -> helper heartbeat updates last seen
  -> user revoke or helper-originated uninstall makes the enrollment terminal

Helper request flow:
  local client connects -> handshake agent id -> request action/target
  -> ACL decision -> SQLite grant lookup -> IO or rejection -> local audit

Install flow:
  installer fetches manifest -> runs current verifier path -> user confirms
  -> package manager installs the local artifact path -> platform service starts daemon
```

**Invariants**

- User consent is represented as host grants, not as generic user API capabilities.
- Helper enrollment is represented as `helper_enrollments`, not as Remote Agent nodes, host grants, or user permissions.
- Helper enforcement is per request; grant state is not cached in the helper decision path.
- Helper enrollment status is identity/status only; it does not execute jobs or prove Configure OpenClaw success.
- Helper filesystem IO is read-only in the current capability set.
- Remote Agent and Host Bridge are separate capabilities with separate credentials, transports, and boundaries.
- Server-side host grant ownership does not imply admin-wide override.

## Submodules

- [helper-daemon.md](helper-daemon.md) defines local enforcement: UDS IPC, ACL, SQLite grant lookup, audit, sandbox, and read-only IO.
- [host-grants.md](host-grants.md) defines the server-side consent model and its invariants.
- [installer.md](installer.md) defines package installation, the manifest verifier path, and deployment responsibilities.

## Out Of Scope

Host Bridge does not provide Remote Agent browsing, plugin WebSocket API tunneling, unrestricted command execution, job queue/lease/result handling, Configure OpenClaw execution status, or admin-owned host consent.

## Known Gaps

- Runtime authorization and platform sandboxing do not have identical update lifecycles; [helper-daemon.md](helper-daemon.md) owns the daemon-level details.
- Deployment trust and runtime authorization are separate boundaries; [installer.md](installer.md) owns installer trust details.
- Helper enrollment has identity/status and minimal credential handling only. Rotation, pull queues, service lifecycle, local uninstall action execution, and local policy execution are not current behavior.

## Implementation Anchors

- `packages/server-go/internal/api/host_grants.go` (`HostGrantsHandler`)
- `packages/server-go/internal/api/helper_enrollments.go` (`HelperEnrollmentHandler`)
- `packages/server-go/internal/migrations/host_grants.go` (`host_grants` schema)
- `packages/server-go/internal/migrations/helper_enrollments.go` (`helper_enrollments` schema)
- `packages/server-go/internal/store/helper_enrollment_queries.go`
- `packages/borgee-helper/cmd/borgee-helper/main.go`
- `packages/borgee-helper/internal/ipc` (`Request`, `Response`, `Handler`)
- `packages/borgee-helper/internal/acl` (`Gate`, `Decision`)
- `packages/borgee-helper/internal/grants` (`SQLiteConsumer`)
- `packages/borgee-helper/internal/fileio`
- `packages/borgee-helper/internal/audit`
- `packages/borgee-helper/internal/sandbox`
- `packages/borgee-installer/cmd`
- `packages/borgee-installer/internal`
