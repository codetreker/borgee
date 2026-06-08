# Remote Agent Protocol

The Remote Agent protocol is a two-plane design. The user API is the control plane for nodes and bindings. The reverse WebSocket is the data plane for live read-only filesystem requests. The two planes meet at the server hub: REST handlers decide whether a request is authorized and online, while the hub carries a short request/response exchange to the connected daemon. The server exposes ls, read, and stat over HTTP; each maps to one flat request frame to the daemon.

## Overview

**Role**
The protocol turns user-scoped HTTP requests into bounded read-only daemon work without giving the browser direct access to the local machine: ls, read, and stat.

**Boundary**
The control plane is authenticated as the user. The data plane is authenticated as the remote node. The server never treats a remote node token as a user session, and it never treats a user session as proof that a remote node is connected.

**Collaborators**
The protocol collaborates with user auth, remote node storage, the WebSocket hub, channel bindings, and the local filesystem boundary. It intentionally does not use the plugin WebSocket.

**Internal Architecture**

- Management resources: nodes, bindings, channel-to-path associations, online status.
- Tunnel resources: a single live connection per node in the hub map.
- Request broker: server-side adapter constructs action requests and waits for one matching response.
- Daemon dispatcher: the Go daemon routes action names to read-only filesystem operations.

**Key Flows**

```text
Node lifecycle:
  create node -> store token -> start daemon -> WebSocket authenticate -> online

ls/read/stat request:
  user request -> owner check -> online check -> broker request -> daemon action
  -> response -> HTTP response or mapped error

Disconnect:
  WebSocket close -> hub unregisters node -> later REST status reports offline
```

**Invariants**

- Every management request is scoped to the authenticated user before it can reach a remote node.
- Every data-plane request is correlated by request id and has a bounded wait time.
- The server HTTP surface exposes ls, read, and stat; each is a read-only operation.
- The request frame is flat `{action, path}`, identical on the server and the daemon.
- Remote errors are translated at the server edge so clients see HTTP semantics rather than raw daemon transport state.

## Management Plane

The management plane owns remote node inventory and channel bindings. It is ordinary user API surface: authenticated user in, owner-scoped node resources out. Binding creation is scoped to the node owner, but the submitted `channel_id` is not separately checked for channel membership or capability. Bindings connect a channel to a remote node path, but they do not themselves grant filesystem access; the daemon's directory allowlist is the final local boundary.

## Server Surface

The server-exposed remote filesystem API is ls, read, and stat. `GET /api/v1/remote/nodes/{nodeId}/stat` is the stat route; ls and read sit at the same handler. Each one maps to a single flat request frame proxied to the daemon.

## Data Plane

The data plane is a reverse connection from the daemon to the server. The server does not dial into the user's machine. This keeps NAT/firewall behavior simple and makes online state explicit: if no remote connection is registered, the node is offline for proxy operations.

The envelope is small and settled: `ping`/`pong` for liveness, `request` (flat `{action, path}`) for server-to-daemon work, and `response` for daemon-to-server results. There is no streaming, chunking, or background job protocol.

## Error Model

For the ls/read/stat flow, protocol failures are mapped at the server boundary (`api/remote.go`):

- offline node becomes service unavailable (`node_offline` → `503`);
- request timeout becomes gateway timeout (`timeout` → `504`);
- local path denial becomes forbidden (parsed `forbidden` → `403`);
- missing file and file-size limit are normalized to client-readable HTTP errors (`not found` → `404`);
- unknown remote errors become upstream failures.

This keeps the remote filesystem path from leaking implementation-specific local exceptions as stable public API.

## Out Of Scope

The protocol does not cover plugin-to-server API tunneling, admin override, offline sync, or remote writes.

## Current Limitations

- Binding creation is node-owner scoped but does not independently verify the supplied channel id against channel membership or capability.
- Directory missing errors are not fully normalized across ls/read/stat actions.

## Implementation Anchors

- `packages/server-go/internal/api/remote.go` (`RemoteHandler`, `handleNodeStat`)
- `packages/server-go/internal/ws/remote.go` (`RemoteConn.SendRequest`, `HandleRemote`)
- `packages/server-go/internal/server/server.go` (`hubRemoteAdapter`)
- `packages/server-go/internal/ws/hub.go` (`Hub` remote registry)
- `packages/server-go/internal/store/models.go` (`RemoteNode`, `RemoteBinding`)
- `packages/borgee/internal/remotews` (`RequestData`, reverse-WebSocket client)
- `packages/borgee/internal/fsops` (`Ls`, `Read`, `Stat`)
