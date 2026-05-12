# Remote Agent Filesystem Boundary

Remote Agent's filesystem boundary is a process-level allowlist chosen by the user at startup. It is intentionally lighter than Host Bridge: there is no grant table, no helper daemon, no Unix socket, and no OS sandbox profile. The trust model is that the user starts a local process and explicitly names the directories that process may expose.

## Overview

**Role**
The filesystem boundary limits what the remote protocol can read once a node is connected. It gives the remote tunnel a local guardrail: a request can only resolve inside a startup directory, and successful operations are read-only.

**Boundary**
The boundary is path containment after local path resolution. A request path must resolve to an allowed directory or one of its descendants. This boundary is enforced inside the Node.js process, not by the operating system.

**Collaborators**
The boundary collaborates with the remote protocol dispatcher and server error mapping. It does not collaborate with host grants, helper ACL, or platform sandboxing.

**Internal Architecture**

- Startup configuration: the user provides a comma-separated directory allowlist.
- Path gate: each request path is resolved and compared with the allowlist.
- Read executor: allowed paths are listed, read, or statted synchronously.
- Result normalizer: filesystem details are returned as JSON-friendly metadata and content.

**Key Flows**

```text
agent start -> parse allowed directories
remote request -> resolve target path -> containment check
allowed -> read/list/stat -> structured result
denied -> stable remote error -> server HTTP mapping
```

**Invariants**

- No filesystem operation is attempted before the allowlist check passes.
- The exposed operation set is read-only: list, read, and stat.
- File reads are bounded by a small maximum size.
- The server does not enforce local path containment; the connected agent does.
- Host Bridge grants do not expand or shrink this allowlist.

## Read Semantics

Directory listing returns entry names and lightweight metadata. File reads return text content, MIME classification, and size. Stat returns size, modification time, and directory status. The model is optimized for inspection and context gathering rather than bulk transfer.

The current read path is text-oriented. Files are read as UTF-8 content after size validation. Binary MIME labels may be produced, but the transport still returns the content through the same text-shaped response path.

## Boundary Difference From Host Bridge

Remote Agent and Host Bridge both touch local files, but they solve different problems:

| Aspect | Remote Agent | Host Bridge Helper |
| --- | --- | --- |
| User intent | user starts agent with directories | user grants host capability stored server-side |
| Transport | reverse WebSocket | local UDS IPC |
| Authorization | remote node token + owner API check | agent id + host grant lookup |
| Local boundary | process allowlist | ACL plus platform sandbox where available |
| Audit | no helper JSONL audit | per-request local JSONL audit |

## Out Of Scope

The filesystem boundary does not provide write operations, symlink-realpath policy, OS-level sandboxing, grant revocation, or persistent audit.

## Known Gaps

- The boundary uses resolved path containment, not realpath-based symlink containment.
- Large directory listing is not separately capped by the agent boundary.
- Binary reads are not modeled as a separate binary transfer path.

## Implementation Anchors

- `packages/remote-agent/src/index.ts` (CLI startup and directory parsing)
- `packages/remote-agent/src/agent.ts` (`RemoteAgent` request dispatcher)
- `packages/remote-agent/src/fs-ops.ts` (`isPathAllowed`, `ls`, `readFile`, `stat`)
- `packages/server-go/internal/api/remote.go` (remote error mapping)
