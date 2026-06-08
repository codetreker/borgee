# Remote Agent

Remote Node is a working, read-only filesystem browser. A signed-in user attaches one machine to their Borgee account, binds selected directories to channels, and browses and reads those files through the server. The server proxies each request over one reverse WebSocket to a single Go daemon running on that machine as the install user. There is no second rail: the daemon performs bounded read-only filesystem operations and nothing else.

## Overview

**Role**
Remote Node gives a signed-in user a way to attach a machine to their Borgee account, bind selected paths to channels, and use the server as a proxy for directory listing and file reads to that machine. The server never mounts the filesystem directly; it asks the connected daemon to perform bounded read-only operations. That is the contract, not an intended one.

**Boundary**
The boundary is the remote node. A node belongs to one user, authenticates with its own connection token, and is only reachable through that user's remote API requests. Local filesystem access is constrained by the daemon's startup directory allowlist; server-triggered ls/read/stat is the real filesystem boundary for that node.

**Collaborators**
Remote Node collaborates with the user API control plane, the WebSocket hub data plane, the remote node store, channel remote bindings, and the local filesystem boundary.

**Internal Architecture**
The design splits into three layers:

- Control plane: node and binding lifecycle, owned by the server and scoped to the user.
- Data plane: a long-lived reverse WebSocket connection keyed by the node connection token.
- Local executor: the Go daemon dispatching read-only filesystem actions against its startup allowlist.

**Key Flows**

```text
create node -> create response returns the connection token once (UI shows it)
run `npx @codetreker/borgee-remote-agent install --server <wss://host> --token <id>.<secret> --dirs <paths>` on the target
daemon installs under the user's home/XDG service -> opens one reverse WebSocket authenticated by the node token
server marks node online -> user issues ls/read/stat
server checks owner + online state -> flat {action, path} request frame to the daemon
daemon runs the read-only op -> response frame -> server HTTP response or mapped error
```

**Invariants**

- Remote nodes are user-owned resources; cross-user node access is rejected.
- The remote WebSocket token is node-specific and not the same credential as the user cookie, API key, or plugin API key.
- Remote reads are online-only; no offline queue or cached filesystem snapshot is part of this path.
- Directory exposure is selected at install/startup and enforced locally by the daemon process.
- Remote Node is not an execution channel, installer, or network egress broker.

## Module Map

- [protocol.md](protocol.md) describes the control plane/data plane split, message contract, timeout behavior, and protocol-level invariants.
- [filesystem-boundary.md](filesystem-boundary.md) describes the local directory allowlist, read limits, and read-only behavior.
- [ui/](ui/) keeps a combined Remote Explorer ASCII reference sketch as Interaction And Layout Reference. It maps to the user SPA's remote nodes sidepane and channel remote tab; protocol details and filesystem boundary rules are defined in [protocol.md](protocol.md) and [filesystem-boundary.md](filesystem-boundary.md).

## Out Of Scope

Remote Node does not provide host-wide privileges, OS sandboxing, package installation, or command execution. These are simply not part of v1; there is no other rail that supplies them.

## Implementation Anchors

- `packages/borgee/cmd/borgee` (daemon entrypoint)
- `packages/borgee/internal/fsops` (`Ls`, `Read`, `Stat`, allowlist check)
- `packages/borgee/internal/remotews` (flat `{action, path}` frame; reverse-WebSocket client)
- `packages/borgee/internal/cli/install`, `internal/cli/daemon`, `internal/cli/uninstall`
- `packages/borgee/internal/tokenstore`
- `packages/server-go/internal/api/remote.go` (`RemoteHandler`, `handleNodeStat`)
- `packages/server-go/internal/ws/remote.go` (`RemoteConn`, `HandleRemote`, `SendRequest`)
- `packages/server-go/internal/ws/hub.go` (`Hub.RegisterRemote`, `Hub.GetRemote`)
- `packages/server-go/internal/server/server.go` (`hubRemoteAdapter.ProxyRequest`)
- `packages/server-go/internal/store/models.go` (`RemoteNode`, `RemoteBinding`)
- `packages/server-go/internal/store/queries_phase2b.go` (remote node and binding queries)
- `packages/server-go/internal/store/queries_phase3.go` (remote token lookup and last-seen update)
