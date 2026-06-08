# Remote Agent Filesystem Boundary

Remote Node's filesystem boundary is a process-level read-only allowlist the Go daemon applies, chosen by the user at `install`/startup. It is an allowlist, not an OS sandbox: a request can only resolve inside a startup directory, and successful operations are read-only.

## Overview

**Role**
The filesystem boundary limits what the daemon process can read when it receives a filesystem action. It gives the remote tunnel a local guardrail: a request can only resolve inside a startup directory, and successful operations are read-only.

**Boundary**
The boundary is path containment after local path resolution. A request path must resolve to an allowed directory or one of its descendants. This boundary is enforced inside the Go daemon, not by the operating system.

**Collaborators**
The boundary collaborates with the request dispatcher and server error mapping.

**Internal Architecture**

- Startup configuration: the user provides a comma-separated directory allowlist.
- Path gate: each request path is resolved and compared with the allowlist.
- Read executor: allowed paths are listed, read, or statted synchronously.
- Result normalizer: filesystem details are returned as JSON-friendly metadata and content.

**Key Flows**

```text
daemon start -> parse allowed directories
remote request -> resolve target path -> containment check
allowed -> read/list/stat -> structured result
denied -> stable remote error -> server HTTP mapping
```

**Invariants**

- No filesystem operation is attempted before the allowlist check passes inside the daemon dispatcher.
- The exposed operation set is read-only: ls, read, and stat.
- File reads are bounded by a small maximum size.
- The server does not enforce local path containment; the connected daemon does.

## Read Semantics

Directory listing returns entry names and lightweight metadata. File reads return text content, MIME classification, and size. Stat returns size, modification time, and directory status, and it is exposed via the server stat route (`GET /api/v1/remote/nodes/{nodeId}/stat`). The model is optimized for inspection and context gathering rather than bulk transfer.

The read path is text-oriented. Files are read as UTF-8 content after size validation. Binary MIME labels may be produced, but the transport still returns the content through the same text-shaped response path.

## Out Of Scope

The filesystem boundary does not provide write operations, symlink-realpath policy, or OS-level sandboxing.

## Known Gaps

- The boundary uses resolved path containment, not realpath-based symlink containment.
- Large directory listing is not separately capped by the daemon boundary.
- Binary reads are not modeled as a separate binary transfer path.

## Implementation Anchors

- `packages/borgee/internal/cli/install`, `internal/cli/daemon` (startup and directory parsing)
- `packages/borgee/internal/fsops` (`Ls`, `Read`, `Stat`, allowlist check)
- `packages/server-go/internal/api/remote.go` (remote error mapping)
