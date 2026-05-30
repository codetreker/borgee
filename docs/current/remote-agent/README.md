# Remote Agent

Remote Agent is the user-owned path intended to make selected local directories visible to Borgee while the agent process is running. It is a reverse WebSocket bridge with a local read-only executor, but current protocol caveats mean the filesystem proxy should be treated as a partially wired capability. It is separate from Host Bridge and Helper enrollment: Remote Agent is for user-selected remote filesystem browsing; Host Bridge is for host capabilities mediated by grants, helper IPC, audit, Helper enrollment/status identity, and sandboxing.

## Overview

**Role**
Remote Agent gives a signed-in user a way to attach a machine to their Borgee account, bind selected paths to channels, and use the server as an intended proxy for directory listing and file reads to that machine. The server never mounts the filesystem directly; in the intended contract it asks the connected agent to perform bounded local operations.

**Boundary**
The boundary is the remote node plus the current protocol caveat. A node belongs to one user, authenticates with its own connection token, and is only reachable through that user's remote API requests. Local filesystem access is intended to be constrained by the agent's startup directory allowlist, but maintainers should account for the protocol caveats before treating server-triggered list/read as a reliable filesystem boundary.

**Collaborators**
Remote Agent collaborates with the user API control plane, the WebSocket hub data plane, the remote node store, channel remote bindings, and the local filesystem boundary. It does not collaborate with host grants, Helper enrollment credentials, or helper daemon IPC.

**Internal Architecture**
The design splits into three layers:

- Control plane: node and binding lifecycle, owned by the server and scoped to the user.
- Data plane: a long-lived reverse WebSocket connection keyed by the node token.
- Local executor: a small agent process that dispatches filesystem actions against the startup allowlist.

**Key Flows**

```text
intended contract:
  create node -> obtain connection token -> start agent with server/token/dirs
  agent connects -> server marks node online -> user requests ls/read
  server checks owner + online state -> WebSocket request -> agent filesystem executor
  agent response -> server HTTP response or mapped error

current caveat:
  server and TypeScript agent disagree on where request path is carried
```

**Invariants**

- Remote nodes are user-owned resources; cross-user node access is rejected.
- The remote WebSocket token is node-specific and not the same credential as the user cookie, API key, plugin API key, Helper enrollment credential, or helper grant.
- Remote reads are online-only; no offline queue or cached filesystem snapshot is part of this path.
- Directory exposure is selected at agent startup and enforced locally by the agent process once the agent receives a correctly shaped filesystem request.
- Remote Agent is not an execution channel, installer, network egress broker, Helper enrollment authority, or host grant consumer.

## Module Map

- [protocol.md](protocol.md) describes the control plane/data plane split, message contract, timeout behavior, and protocol-level invariants.
- [filesystem-boundary.md](filesystem-boundary.md) describes the local directory allowlist, read limits, read-only behavior, and how this differs from helper sandboxing.
- [ui/](ui/) keeps a combined Remote Explorer ASCII reference sketch as Interaction And Layout Reference. It maps to the user SPA's remote nodes sidepane and channel remote tab; protocol caveats and filesystem boundary rules remain defined in [protocol.md](protocol.md) and [filesystem-boundary.md](filesystem-boundary.md).

## Out Of Scope

Remote Agent does not provide host-wide privileges, OS sandboxing, package installation, command execution, Helper enrollment status, or helper audit integration. Those belong to the Host Bridge capability path.

## Current Status And Boundary Caveats

- The current filesystem proxy is an intended capability with protocol caveats; [protocol.md](protocol.md) owns the connection setup and request-contract details.
- Treat Remote Agent's boundary as node ownership plus local allowlist intent until those protocol caveats are resolved.
- Remote Agent tokens do not authenticate Helper enrollment claim, heartbeat, credential rotation, or helper-originated uninstall. Helper enrollment rows and credentials live in the server Helper enrollment rail.
- PR-2 #1038 bundled the host-bridge daemon's WebSocket transport into the `@codetreker/borgee-remote-agent` npm package as part of the single-binary distribution chain. The daemon now connects to the server via `wss://<server>/ws/helper/<enrollmentId>` instead of the prior HTTP long-poll. This does not affect the Remote Agent reverse WebSocket data plane; the bumped tarball version (0.2.0) reflects the new bundled binary capability set.
- The package now exposes one public npm bin, `borgee-remote-agent`. Host-bridge subcommands such as `install` are dispatched by that default CLI to the embedded platform binary under `bin/platforms/<plat>-<arch>/borgee`; there is no separate public npm `borgee` bin or `bin/borgee.js` shim. Direct Node remote-agent startup through `--server ... --dirs ...` remains as a deprecated compatibility path.

## Implementation Anchors

- `packages/server-go/internal/api/remote.go` (`RemoteHandler`, `RemoteProxy`)
- `packages/server-go/internal/ws/remote.go` (`RemoteConn`, `HandleRemote`)
- `packages/server-go/internal/ws/hub.go` (`Hub.RegisterRemote`, `Hub.GetRemote`)
- `packages/server-go/internal/server/server.go` (`hubRemoteAdapter`)
- `packages/server-go/internal/store/models.go` (`RemoteNode`, `RemoteBinding`)
- `packages/server-go/internal/store/queries_phase2b.go` (remote node and binding queries)
- `packages/server-go/internal/store/queries_phase3.go` (remote token lookup and last-seen update)
- `packages/server-go/internal/api/helper_enrollments.go` (`HelperEnrollmentHandler`, separate rail)
- `packages/server-go/internal/store/helper_enrollment_queries.go` (separate rail)
- `packages/remote-agent/src/index.ts`
- `packages/remote-agent/src/cli.ts` (default CLI dispatcher)
- `packages/remote-agent/src/platform-binary.ts` (embedded platform binary resolver)
- `packages/remote-agent/src/agent.ts` (`RemoteAgent`)
- `packages/remote-agent/src/fs-ops.ts`
