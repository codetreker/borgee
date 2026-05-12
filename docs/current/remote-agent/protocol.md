# Remote Agent Protocol

The Remote Agent protocol is a two-plane design. The user API is the control plane for nodes and bindings. The reverse WebSocket is the data plane for live filesystem requests. The two planes meet at the server hub: REST handlers decide whether a request is authorized and online, while the hub carries a short request/response exchange to the connected agent.

## Overview

**Role**
The protocol turns user-scoped HTTP requests into live agent work without giving the browser direct access to the local machine. It also lets the server treat remote files as a best-effort live resource: available when the node is connected, unavailable when it is offline.

**Boundary**
The control plane is authenticated as the user. The data plane is authenticated as the remote node. The server never treats a remote node token as a user session, and it never treats a user session as proof that a remote node is connected.

**Collaborators**
The protocol collaborates with user auth, remote node storage, the WebSocket hub, channel bindings, and the local filesystem boundary. It intentionally does not use the plugin WebSocket or helper IPC protocol.

**Internal Architecture**

- Management resources: nodes, bindings, channel-to-path associations, online status.
- Tunnel resources: a single live connection per node in the hub map.
- Request broker: server-side adapter constructs action requests and waits for one matching response.
- Agent dispatcher: local process routes action names to filesystem operations.

**Key Flows**

```text
Node lifecycle:
  create node -> store token -> start agent -> WebSocket authenticate -> online

Read/list request:
  user request -> owner check -> online check -> broker request -> agent action
  -> response -> HTTP response or mapped error

Disconnect:
  WebSocket close -> hub unregisters node -> later REST status reports offline
```

**Invariants**

- Every management request is scoped to the authenticated user before it can reach a remote node.
- Every data-plane request is correlated by request id and has a bounded wait time.
- The agent can be asked to list, read, or stat, but the server protocol does not include write or execute actions.
- Remote errors are translated at the server edge so clients see HTTP semantics rather than raw agent transport state.

## Management Plane

The management plane owns remote node inventory and channel bindings. It is designed as ordinary user API surface: authenticated user in, owner-scoped resources out. Bindings connect a channel to a remote node path, but they do not themselves grant filesystem access; the local agent's directory allowlist remains the final local boundary.

## Data Plane

The data plane is a reverse connection from the agent to the server. The server does not dial into the user's machine. This keeps NAT/firewall behavior simple and makes online state explicit: if no remote connection is registered, the node is offline for proxy operations.

The envelope is intentionally small: ping/pong for liveness, request for server-to-agent work, response for agent-to-server results. There is no streaming, chunking, or background job protocol in the current design.

## Error Model

Protocol failures are mapped at the server boundary:

- offline node becomes service unavailable;
- request timeout becomes gateway timeout;
- local path denial becomes forbidden;
- missing file and file-size limit are normalized to client-readable HTTP errors;
- unknown remote errors become upstream failures.

This keeps the remote filesystem path from leaking implementation-specific local exceptions as stable public API.

## Out Of Scope

The protocol does not cover helper daemon IPC, plugin-to-server API tunneling, admin override, offline sync, or remote writes.

## Known Gaps

- Token delivery for newly created nodes is not clearly represented in the current API shape.
- The current request payload shape is inconsistent between the server adapter and the TypeScript agent.
- Directory missing errors are not fully normalized across list/stat/read actions.

## Implementation Anchors

- `packages/server-go/internal/api/remote.go` (`RemoteHandler`)
- `packages/server-go/internal/ws/remote.go` (`RemoteConn.SendRequest`, `HandleRemote`)
- `packages/server-go/internal/server/server.go` (`hubRemoteAdapter`)
- `packages/server-go/internal/ws/hub.go` (`Hub` remote registry)
- `packages/server-go/internal/store/models.go` (`RemoteNode`, `RemoteBinding`)
- `packages/remote-agent/src/agent.ts` (`RemoteAgent`)
