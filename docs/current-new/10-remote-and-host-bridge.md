# Remote And Host Bridge

本文覆盖当前 remote-agent、RemoteNode/RemoteBinding、server `/ws/remote` proxy、borgee-helper daemon、host grants SQLite、UDS IPC、ACL/audit/sandbox/read-only IO、以及 borgee-installer manifest/verify/deploy。事实以当前代码为准；实现不一致处直接列出。

## Entry And Ownership Matrix

| Surface | Owner/Auth | Data source | Operation class | Evidence |
| --- | --- | --- | --- | --- |
| Remote REST | user cookie/API key/dev bypass through `authMw` | `remote_nodes`, `remote_bindings` | CRUD + online/proxy reads | `packages/server-go/internal/api/remote.go`, `packages/server-go/internal/store/queries_phase2b.go` |
| Remote WS | remote node connection token | `remote_nodes.connection_token` | request/response tunnel | `packages/server-go/internal/ws/remote.go`, `packages/server-go/internal/store/queries_phase3.go` |
| TypeScript remote-agent | CLI `--server --token --dirs` | local filesystem under `--dirs` | `ls`, `read`, `stat` | `packages/remote-agent/src/index.ts`, `packages/remote-agent/src/agent.ts`, `packages/remote-agent/src/fs-ops.ts` |
| Host grants REST | user auth; owner-only | `host_grants` | grant/list/revoke | `packages/server-go/internal/api/host_grants.go`, `packages/server-go/internal/migrations/host_grants.go` |
| borgee-helper daemon | local UDS client handshake `agent_id` | read-only SQLite consumer over `host_grants` | read-only IPC actions | `packages/borgee-helper/cmd/borgee-helper/main.go`, `packages/borgee-helper/internal/ipc/ipc.go`, `packages/borgee-helper/internal/grants/sqlite_consumer.go` |
| Installer | local user + sudo | server plugin manifest + local artifact | verify + deploy service | `packages/borgee-installer/cmd/*`, `packages/borgee-installer/internal/*` |

```mermaid
flowchart LR
  Owner[owner user] -->|authMw| RemoteREST[/api/v1/remote/*]
  RemoteREST --> Store[(remote_nodes / remote_bindings)]
  RemoteAgent[borgee-remote-agent] -->|node token| RemoteWS[/ws/remote]
  RemoteREST -->|ProxyRequest 10s| RemoteWS
  RemoteWS --> RemoteAgent
  RemoteAgent --> LocalDirs[allowed --dirs]

  Owner -->|host grant REST| HostGrants[(host_grants)]
  Plugin[plugin/runtime] -->|UDS JSON lines| Helper[borgee-helper]
  Helper -->|SELECT per IPC| HostGrants
  Helper -->|read-only IO| HostFS[host filesystem]
```

## RemoteNode And RemoteBinding

`RemoteNode` stores `id`, `user_id`, `machine_name`, a hidden `connection_token`, optional `last_seen_at`, `created_at`, and `org_id`. `RemoteBinding` stores `id`, `node_id`, `channel_id`, `path`, `label`, and `created_at`, with a database uniqueness constraint on `(node_id, channel_id, path)` in the base schema. Evidence: `packages/server-go/internal/store/models.go`, `packages/server-go/internal/store/migrations.go`.

`CreateRemoteNode` generates a random 32-byte token as hex, stamps the registrant's org id, and persists the row. Because `ConnectionToken` is tagged `json:"-"`, the inspected REST create response does not expose the token. That leaves connection-token delivery to the UI/agent unresolved in the inspected code. Evidence: `packages/server-go/internal/store/queries_phase2b.go`, `packages/server-go/internal/store/models.go`, `packages/server-go/internal/api/remote.go`.

Remote REST endpoints are owner-scoped. Node list filters by current `user.ID`; node delete, binding list/create/delete, status, `ls`, and `read` load the node and reject when `node.UserID != user.ID`. Channel binding list joins `remote_bindings` to `remote_nodes` and filters by both `channel_id` and current user. Evidence: `packages/server-go/internal/api/remote.go`, `packages/server-go/internal/store/queries_phase2b.go`.

`/ws/remote` authenticates by node connection token, not by user cookie. It accepts `Authorization: Bearer <token>` or `?token=`, resolves `GetRemoteNodeByToken`, updates `last_seen_at`, registers the connection by node id, and handles inbound `ping`, `pong`, and `response` messages. Evidence: `packages/server-go/internal/ws/remote.go`, `packages/server-go/internal/store/queries_phase3.go`.

Server-side `ProxyRequest` sends a request envelope to the connected remote node and waits up to 10 seconds before returning `context.DeadlineExceeded`. Remote REST maps that timeout to HTTP 504 and maps remote errors such as `path_not_allowed`, `file_not_found`, and `file_too_large` to 403, 404, and 413 respectively. Evidence: `packages/server-go/internal/server/server.go`, `packages/server-go/internal/ws/remote.go`, `packages/server-go/internal/api/remote.go`.

Current mismatch: Go `hubRemoteAdapter.ProxyRequest` sends `data: {"action": action, "params": {"path": path}}`, but the TypeScript remote-agent reads `data.action` and `data.path`. As written, `ls/read` requests do not pass the target path in the shape the agent expects. Evidence: `packages/server-go/internal/server/server.go`, `packages/server-go/internal/ws/remote.go`, `packages/remote-agent/src/agent.ts`.

## Remote-Agent CLI

The CLI binary is `borgee-remote-agent`. It requires `--server <url>`, `--token <token>`, and `--dirs <comma-separated dirs>`. It refuses to start when no directory is provided, logs the allowed directories, constructs `new RemoteAgent(server, token, dirs)`, connects, and handles SIGINT/SIGTERM by closing the WebSocket. Evidence: `packages/remote-agent/src/index.ts`.

`RemoteAgent.connect` appends `/ws/remote?token=<encoded token>` to the configured server URL. It reconnects with exponential backoff from 1s to 30s after unintentional close, and sends a WebSocket `ping` every 30 seconds while connected. Evidence: `packages/remote-agent/src/agent.ts`.

The agent handles three request actions: `ls`, `read`, and `stat`. Unknown actions return `{error: "Unknown action: ..."}`. Evidence: `packages/remote-agent/src/agent.ts`.

Allowed directories are enforced in the TypeScript agent by resolving the target path and requiring it to equal an allowed dir or start with `allowedDir + path.sep`. This is path-prefix based after `path.resolve`; it is not an OS sandbox. Evidence: `packages/remote-agent/src/fs-ops.ts`.

Remote-agent read size is capped at 2 MiB (`MAX_FILE_SIZE = 2 * 1024 * 1024`). Files over that return `file_too_large`; directories return `is_directory`; missing files return `file_not_found`. `ls` returns directory entries with name, directory flag, size, and mtime, but missing directories return `path_not_found`, which the Go REST mapper currently treats as a generic bad gateway rather than 404. Evidence: `packages/remote-agent/src/fs-ops.ts`, `packages/server-go/internal/api/remote.go`.

## Host Grants REST

Host grants are separate from `user_permissions`. The schema has `id`, `user_id`, optional `agent_id`, `grant_type`, `scope`, `ttl_kind`, `granted_at`, optional `expires_at`, and optional `revoked_at`; `grant_type` is constrained to `install`, `exec`, `filesystem`, and `network`; `ttl_kind` is constrained to `one_shot` and `always`. Evidence: `packages/server-go/internal/migrations/host_grants.go`, `packages/server-go/internal/api/host_grants.go`.

`POST /api/v1/host-grants` is user-authenticated and writes a grant owned by the current user. `one_shot` grants set `expires_at = now + 1h`; `always` grants leave `expires_at` null. `GET /api/v1/host-grants` returns only active grants for the current user: `revoked_at IS NULL` and not expired. `DELETE /api/v1/host-grants/{id}` is owner-only and stamps `revoked_at` rather than deleting the row. Evidence: `packages/server-go/internal/api/host_grants.go`.

Host-grant audit in this handler is currently process logging (`host_grants.granted` / `host_grants.revoked`) with five fields, not a persistent `audit_events` insert in the inspected code. Evidence: `packages/server-go/internal/api/host_grants.go`.

The helper-side grant lookup expects exact scopes such as `fs:/absolute/path` for file actions and `egress:<host>` for network egress. The server REST handler accepts arbitrary non-empty `scope` and does not normalize it to helper scope strings. Producers must therefore write scopes that match the helper's exact lookup format. Evidence: `packages/server-go/internal/api/host_grants.go`, `packages/borgee-helper/internal/acl/acl.go`, `packages/borgee-helper/internal/grants/sqlite_consumer.go`.

## borgee-helper Daemon

`borgee-helper` currently has a real daemon entrypoint only for Linux and macOS. It listens on a POSIX Unix domain socket, default `/run/borgee-helper/borgee-helper.sock`, writes JSONL audit to `/var/log/borgee-helper/audit.log.jsonl` with stderr fallback, requires `--grants-db`, and accepts `--read-paths` as a comma-separated static sandbox allowlist. Non-Linux/Darwin builds log unsupported and do not start the daemon. Evidence: `packages/borgee-helper/cmd/borgee-helper/main.go`, `packages/borgee-helper/cmd/borgee-helper/main_other.go`.

Production startup requires a SQLite DSN for the server database, for example `file:/var/lib/borgee/server.db?mode=ro&_busy_timeout=5000`. `NewSQLiteConsumer` opens the DB through `github.com/mattn/go-sqlite3`, constrains the pool, and performs a ping. Evidence: `packages/borgee-helper/cmd/borgee-helper/main.go`, `packages/borgee-helper/internal/grants/sqlite_consumer.go`.

The SQLite consumer is read-only by convention and per DSN; every grant check performs a fresh `SELECT ... FROM host_grants WHERE agent_id = ? AND scope = ? AND revoked_at IS NULL ORDER BY granted_at DESC LIMIT 1`. It distinguishes not-found from expired via `LookupRaw`, and does not cache grants. Evidence: `packages/borgee-helper/internal/grants/sqlite_consumer.go`, `packages/borgee-helper/internal/grants/grants.go`.

UDS IPC is JSON-lines. The first line is a handshake `{"agent_id":"..."}`. Each later request has `request_id`, `action`, `agent_id`, and `params`; each response has `request_id`, `status` (`ok`, `rejected`, or `failed`), `reason`, optional `data`, and optional `audit_log_id`. Evidence: `packages/borgee-helper/internal/ipc/ipc.go`.

The ACL gate allows only `list_files`, `read_file`, and `network_egress`. All other actions, including write/delete/chmod-style classes, are rejected. The gate also rejects cross-agent requests when the per-request `agent_id` differs from the handshake `agent_id`. Evidence: `packages/borgee-helper/internal/acl/acl.go`.

File actions require absolute paths, reject NUL bytes and any original `..` segment, and normalize with `filepath.Clean`. The code does not resolve symlinks in the ACL layer; comments rely on OS sandboxing and permissions for that. Evidence: `packages/borgee-helper/internal/acl/acl.go`.

After ACL allow, `read_file` and `list_files` perform real read-only IO. `read_file` caps each call at 16 MiB (`MaxReadBytes`) and reports `truncated`; `list_files` caps directory entries at 1000 (`MaxListEntries`). There are no write APIs in `fileio`. Evidence: `packages/borgee-helper/internal/fileio/file_actions.go`.

Every IPC request, including rejects, attempts to append a JSON-line audit event with `actor`, `action`, `target`, `when`, and `scope`. The IPC handler ignores the returned audit write error, so audit logging is best-effort on this path. Evidence: `packages/borgee-helper/internal/ipc/ipc.go`, `packages/borgee-helper/internal/audit/audit.go`.

## Helper Sandbox

Linux sandboxing uses raw Landlock syscalls. If `ReadPaths` is empty it applies an empty ruleset for deny-by-default. If the kernel lacks Landlock (`ENOSYS`, e.g. older than 5.13), `Apply` returns nil as a documented no-op fallback. Allowed access is read-file/read-dir only. Evidence: `packages/borgee-helper/internal/sandbox/sandbox_linux.go`.

macOS sandboxing is wrapper-based. `Apply` is a no-op because Go cannot self-apply `sandbox_init`; `GenerateProfile` produces a `sandbox-exec` profile with deny-default, read subpaths from `ReadPaths`, audit/tmp write paths, and local Unix socket permissions. The daemon depends on install-butler/launcher starting it under `sandbox-exec`. Evidence: `packages/borgee-helper/internal/sandbox/sandbox_darwin.go`, `packages/borgee-helper/cmd/borgee-helper/main.go`.

Current sandbox gap: the daemon accepts static `--read-paths`; despite comments about v1+ pulling live grants, current `main.go` does not derive Landlock/sandbox read paths from SQLite `host_grants`. Revoked or newly granted paths affect ACL lookup, but not the already-applied OS sandbox allowlist. Evidence: `packages/borgee-helper/cmd/borgee-helper/main.go`, `packages/borgee-helper/internal/sandbox/sandbox_linux.go`, `packages/borgee-helper/internal/sandbox/sandbox_darwin.go`.

## Installer Manifest, Verify, Deploy

Linux installer CLI requires `--manifest-url`, `--pubkey-base64`, and `--deb`; it optionally accepts `--bearer-token` and `--dry-run`. It fetches the manifest with a 60s parent context and 30s HTTP timeout, verifies ed25519, asks for confirmation, then runs a deploy plan with `sudo apt install`, `systemctl daemon-reload`, `systemctl enable`, and `systemctl start`. Evidence: `packages/borgee-installer/cmd/borgee-installer-linux/main.go`, `packages/borgee-installer/internal/deploy/deploy.go`.

macOS installer CLI mirrors Linux but requires `--pkg` and deploys with `sudo /usr/sbin/installer -pkg ... -target /` followed by `sudo launchctl load /Library/LaunchDaemons/cloud.borgee.host-bridge.plist`. Evidence: `packages/borgee-installer/cmd/borgee-installer-darwin/main.go`, `packages/borgee-installer/internal/deploy/deploy.go`.

The installer manifest fetcher sends `Authorization: Bearer <token>` when provided, limits response reads to 8 MiB, and verifies an ed25519 detached signature over canonical JSON of `{entries, signed_at}`. Bad or empty signatures return `manifest_signature_invalid`; transport/decode errors return `manifest_fetch_failed`. Evidence: `packages/borgee-installer/internal/manifest/fetcher.go`.

Current manifest mismatch: server `GET /api/v1/plugin-manifest` returns `manifest_version`, `issued_at`, `expires_at`, `signature`, and `plugins`, and signs the `PluginManifestPayload` shape. The installer fetcher expects `entries`, `signed_at`, and `signature`, and verifies a different canonical byte sequence. Treat installer verification against the current server endpoint as not yet aligned. Evidence: `packages/server-go/internal/api/host_manifest.go`, `packages/server-go/internal/api/host_manifest_test.go`, `packages/borgee-installer/internal/manifest/fetcher.go`.

Current installer dialog mismatch: installer `dialog.GrantTypes` lists `read`, `write`, `exec`, and `network`, while server `host_grants.grant_type` accepts `install`, `exec`, `filesystem`, and `network`. Treat the permission dialog vocabulary as not aligned with the current server schema. Evidence: `packages/borgee-installer/internal/dialog/dialog.go`, `packages/server-go/internal/migrations/host_grants.go`, `packages/server-go/internal/api/host_grants.go`.

## Open Questions / Not Aligned

- Remote REST `ls/read` proxy request shape does not match the TypeScript remote-agent path field: `packages/server-go/internal/server/server.go`, `packages/remote-agent/src/agent.ts`.
- Remote node token delivery is unresolved because the stored token is hidden from JSON and no token retrieval endpoint was found: `packages/server-go/internal/store/models.go`, `packages/server-go/internal/api/remote.go`.
- Helper OS sandbox read paths are static at daemon start and are not recalculated from live `host_grants`: `packages/borgee-helper/cmd/borgee-helper/main.go`.
- Installer manifest envelope/signature format does not match the current server plugin-manifest handler: `packages/server-go/internal/api/host_manifest.go`, `packages/borgee-installer/internal/manifest/fetcher.go`.
- Installer permission dialog grant types do not match the server `host_grants` enum: `packages/borgee-installer/internal/dialog/dialog.go`, `packages/server-go/internal/migrations/host_grants.go`.
