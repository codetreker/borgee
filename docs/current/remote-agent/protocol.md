# Remote Agent Protocol

The Remote Agent protocol is a two-plane design. The user API is the control plane for nodes and bindings. The reverse WebSocket is the data plane intended for live filesystem requests. The two planes meet at the server hub: REST handlers decide whether a request is authorized and online, while the hub carries a short request/response exchange to the connected agent. Current implementation status matters here: the server exposes HTTP proxy operations for list and read, while the agent also has a stat dispatcher that is not wired to the server HTTP surface.

## Overview

**Role**
The protocol is intended to turn user-scoped HTTP requests into live agent work without giving the browser direct access to the local machine. In the current server-exposed surface, that means list and read requests only. The agent supports stat internally, but that support is not an exposed server API contract today.

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

Intended list/read request:
  user request -> owner check -> online check -> broker request -> agent action
  -> response -> HTTP response or mapped error

Current contract caveat:
  server sends action plus params; the TypeScript agent currently expects path at top level

Disconnect:
  WebSocket close -> hub unregisters node -> later REST status reports offline
```

**Invariants**

- Every management request is scoped to the authenticated user before it can reach a remote node.
- Every data-plane request is correlated by request id and has a bounded wait time.
- The current server HTTP surface exposes list and read; stat exists in the agent dispatcher but is not part of the server-exposed remote API.
- The intended server/agent envelope must carry the same path shape on both sides before list/read can be treated as an operationally reliable proxy.
- Remote errors are translated at the server edge so clients see HTTP semantics rather than raw agent transport state.

## Management Plane

The management plane owns remote node inventory and channel bindings. It is designed as ordinary user API surface: authenticated user in, owner-scoped node resources out. Binding creation is currently scoped to the node owner, but the submitted `channel_id` is not separately checked for channel ownership or capability at binding creation. Bindings connect a channel to a remote node path, but they do not themselves grant filesystem access; the local agent's directory allowlist remains the intended final local boundary once the request envelope is aligned.

## Current Server Surface

The current server-exposed remote filesystem API consists of list and read operations. The TypeScript agent dispatcher has a stat action, but no corresponding server HTTP route is currently exposed for stat. Treat stat as agent-internal capability until a server route and authorization shape are added.

## Data Plane

The data plane is a reverse connection from the agent to the server. The server does not dial into the user's machine. This keeps NAT/firewall behavior simple and makes online state explicit: if no remote connection is registered, the node is offline for proxy operations.

The intended envelope is small: ping/pong for liveness, request for server-to-agent work, response for agent-to-server results. The current server adapter and TypeScript agent disagree on the path field location, so the envelope is a boundary caveat rather than a settled contract. There is no streaming, chunking, or background job protocol in the current design.

## Error Model

For the intended list/read flow, protocol failures are mapped at the server boundary:

- offline node becomes service unavailable;
- request timeout becomes gateway timeout;
- local path denial becomes forbidden;
- missing file and file-size limit are normalized to client-readable HTTP errors;
- unknown remote errors become upstream failures.

This keeps the remote filesystem path from leaking implementation-specific local exceptions as stable public API.

## Out Of Scope

The protocol does not cover helper daemon IPC, plugin-to-server API tunneling, admin override, offline sync, or remote writes.

## Current Limitations

- Token delivery for newly created nodes is not clearly represented in the current API shape.
- The current request payload shape is inconsistent between the server adapter and the TypeScript agent.
- Binding creation is node-owner scoped but does not independently verify the supplied channel id against channel ownership or capability.
- Server-exposed filesystem proxy operations are list and read only; agent stat support is not wired to HTTP.
- Directory missing errors are not fully normalized across list/stat/read actions.

## Implementation Anchors

- `packages/server-go/internal/api/remote.go` (`RemoteHandler`)
- `packages/server-go/internal/ws/remote.go` (`RemoteConn.SendRequest`, `HandleRemote`)
- `packages/server-go/internal/server/server.go` (`hubRemoteAdapter`)
- `packages/server-go/internal/ws/hub.go` (`Hub` remote registry)
- `packages/server-go/internal/store/models.go` (`RemoteNode`, `RemoteBinding`)
- `packages/remote-agent/src/agent.ts` (`RemoteAgent`)
