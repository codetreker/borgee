# Security And Privacy Boundaries

本文描述管理员、用户、agent、plugin、remote、helper、installer 的权限边界、token/cookie、host grants、remote dirs、audit，以及当前 best-effort/persistence 风险。每条重要断言后给出代码路径证据。

## Boundary Matrix

| Actor | Identity material | Can do | Cannot do / boundary | Evidence |
| --- | --- | --- | --- | --- |
| Human user | `borgee_token` JWT cookie or user API key | Use `/api/v1/*`, own channels/files/agents/remote nodes/host grants by handler ACL | Does not enter `/admin-api/*`; cannot use admin session | `packages/server-go/internal/auth/middleware.go`, `packages/server-go/internal/server/server.go`, `packages/server-go/internal/api/*` |
| Admin | `borgee_admin_session` opaque cookie backed by `admin_sessions` | Use `/admin-api/v1/*`, manage user/admin metadata surfaces, read admin audit | Does not authenticate as user; no admin capability in `auth.ALL`; metadata-only read endpoints omit private raw fields | `packages/server-go/internal/admin/*`, `packages/server-go/internal/auth/capabilities.go`, `packages/server-go/internal/api/runtimes.go` |
| Agent | `users` row with `role="agent"`, owner id, API key | Connect plugin WS, call user API as bearer, receive explicit grants | Does not receive wildcard `(*,*)` in `HasCapability`; owner-only checks still apply | `packages/server-go/internal/auth/abac.go`, `packages/server-go/internal/store/queries.go`, `packages/server-go/internal/ws/plugin.go` |
| Plugin process | agent API key on `/ws/plugin` | Proxy API requests through server with bearer key, send BPP frames | No cookie auth on plugin WS; downstream API remains user/capability gated | `packages/server-go/internal/ws/plugin.go` |
| Remote node | `remote_nodes.connection_token` | Connect `/ws/remote`, answer `ls/read/stat` style requests | Not a user session; REST ownership remains current user-owned | `packages/server-go/internal/ws/remote.go`, `packages/server-go/internal/api/remote.go` |
| Remote-agent CLI | `--token`, local `--dirs` | Expose local reads under allowed dirs | No OS sandbox; allowed dirs are process-level path-prefix checks | `packages/remote-agent/src/index.ts`, `packages/remote-agent/src/fs-ops.ts` |
| borgee-helper | local daemon, UDS handshake `agent_id`, SQLite host grants | Read-only IPC actions after grant lookup and sandbox | Rejects write-class IPC; cannot mint grants; SQLite consumer is read-only by design | `packages/borgee-helper/internal/acl/acl.go`, `packages/borgee-helper/internal/grants/sqlite_consumer.go` |
| Installer | local user + sudo after prompt | Fetch/verify manifest, install helper service | No installer admin API; local sudo is outside server authority | `packages/borgee-installer/cmd/*`, `packages/borgee-installer/internal/*` |

```mermaid
flowchart TD
  User[User cookie/API key] --> UserRail[/api/v1 user rail]
  Admin[Admin cookie] --> AdminRail[/admin-api admin rail]
  Agent[Agent API key] --> PluginRail[/ws/plugin]
  RemoteToken[Remote node token] --> RemoteRail[/ws/remote]
  Helper[borgee-helper UDS] --> Grants[(host_grants SQLite)]

  UserRail -. no .-> AdminRail
  AdminRail -. no .-> UserRail
  PluginRail --> UserRail
  RemoteRail -. token only .-> UserRail
  Helper --> HostFS[host filesystem read-only]
```

## Token And Cookie Boundaries

User cookies are named `borgee_token`. They carry an HS256 JWT signed with `JWT_SECRET`, include `userId` and `email`, expire after 7 days, and are set HTTP-only and SameSite=Lax. Production requires `JWT_SECRET`; development fills `dev-secret` if missing. Evidence: `packages/server-go/internal/auth/middleware.go`, `packages/server-go/internal/api/auth.go`, `packages/server-go/internal/config/config.go`.

User bearer tokens are API keys stored on users and resolved by `Store.GetUserByAPIKey`. HTTP user middleware and user WebSocket auth reject deleted or disabled users. Evidence: `packages/server-go/internal/auth/middleware.go`, `packages/server-go/internal/ws/client.go`.

Development bypass is deliberately scoped to development mode plus `DEV_AUTH_BYPASS=true`. HTTP middleware accepts `X-Dev-User-Id` and then first member fallback; user WebSocket additionally accepts `?user_id=`. Evidence: `packages/server-go/internal/auth/middleware.go`, `packages/server-go/internal/ws/client.go`, `packages/server-go/internal/config/config.go`.

Admin cookies are named `borgee_admin_session`. The value is an opaque random token stored in `admin_sessions`; `RequireAdmin` resolves the token server-side and rejects missing, expired, or deleted admin rows. It does not parse JWTs and does not accept user API keys. Evidence: `packages/server-go/internal/admin/auth.go`, `packages/server-go/internal/admin/middleware.go`, `packages/server-go/internal/migrations/admin_sessions.go`.

Remote node tokens are separate from both user API keys and admin sessions. `/ws/remote` accepts only a bearer/query remote token and resolves `remote_nodes.connection_token`; it updates `last_seen_at` on successful connection. Evidence: `packages/server-go/internal/ws/remote.go`, `packages/server-go/internal/store/queries_phase3.go`.

Plugin WebSocket tokens are user API keys, typically for agent users. `/ws/plugin` accepts `Authorization: Bearer` or `?apiKey=`, not cookies, and stores the API key on the connection so plugin `api_request` can re-enter the HTTP handler with the same bearer identity. Evidence: `packages/server-go/internal/ws/plugin.go`.

## Admin Privacy Boundary

Admins are not users. The admin schema is `admins(id, login, password_hash, created_at)` and has no org, role, email, or owner fields. Bootstrap inserts admins from env credentials and enforces bcrypt hash constraints. Evidence: `packages/server-go/internal/migrations/admin_admins.go`, `packages/server-go/internal/admin/auth.go`.

Admin authority lives on `/admin-api/*` behind `admin.RequireAdmin`; user rail auth and user capabilities do not include admin god-mode. The server explicitly does not mount legacy `/api/v1/admin/*` user-rail admin routes. Evidence: `packages/server-go/internal/server/server.go`, `packages/server-go/internal/api/admin.go`, `packages/server-go/internal/auth/capabilities.go`.

Admin read access is intended to be metadata-only where mounted as god-mode. The runtime admin endpoint returns a whitelist and omits `last_error_reason`; tests also assert admin read endpoints do not leak forbidden fields. Evidence: `packages/server-go/internal/api/runtimes.go`, `packages/server-go/internal/admin/handlers_field_whitelist_test.go`.

Admin write audit is not uniform. User disable/role/password changes and force-delete channel call audit helpers; API-key changes, invite CRUD, and permission grant/revoke do not visibly emit persistent admin audit rows in the inspected handlers. Treat audit coverage as endpoint-specific. Evidence: `packages/server-go/internal/api/admin.go`, `packages/server-go/internal/store/admin_actions.go`.

Impersonation grant privacy is only partially implemented. Users can create/revoke 24h grants and read current grant state. The helper `RequireImpersonationGrant` would reject admin writes without an active target-user grant, but no production handler calls it in the inspected worktree. Evidence: `packages/server-go/internal/api/admin_endpoints.go`, `packages/server-go/internal/api/admin_grant_check.go`, `packages/server-go/internal/store/admin_actions.go`.

## User, Agent, And Plugin Boundary

`user_permissions` rows are the primary user/agent authorization store. Active rows are `revoked_at IS NULL`; expired rows are soft-revoked by a sweeper and audited with actor `system`. Evidence: `packages/server-go/internal/store/models.go`, `packages/server-go/internal/store/queries.go`, `packages/server-go/internal/auth/expires_sweeper.go`.

Humans created as `member` receive a wildcard `(*, *)` default grant; agents receive only `message.send:*` and `message.read:*` by default. `HasCapability` refuses to let `role="agent"` use the wildcard shortcut even if a row exists. Evidence: `packages/server-go/internal/store/queries.go`, `packages/server-go/internal/auth/abac.go`.

Capability grants from plugin to owner are explicit. A plugin request is validated against `auth.Capabilities`, translated into an owner system DM with quick action, and the owner completes the write through `/api/v1/me/grants`. Reject/snooze outcomes are only logged in v1; no deny-list state is persisted. Evidence: `packages/server-go/internal/api/capability_grant.go`, `packages/server-go/internal/api/me_grants.go`.

Plugin `api_request` is not a privileged backdoor. It re-enters the server HTTP handler with the plugin connection's API key, so endpoint-specific owner checks, membership checks, and capability checks still apply. Evidence: `packages/server-go/internal/ws/plugin.go`, `packages/server-go/internal/server/server.go`.

Current capability vocabulary is inconsistent. The `auth.ALL` allowlist contains 14 dot-notation capabilities, but some active `RequirePermission` middleware uses legacy or extra literals such as `message.send`, `message.read`, `channel.create`, and `agent.runtime.control`. Admin/user grant validators use the 14-value allowlist, while those middleware callsites scan rows directly. Evidence: `packages/server-go/internal/auth/capabilities.go`, `packages/server-go/internal/auth/permissions.go`, `packages/server-go/internal/server/server.go`.

## Remote Boundary

Remote nodes are owned by a user id. REST management and proxy endpoints are user-authenticated and verify node ownership before acting. Evidence: `packages/server-go/internal/api/remote.go`, `packages/server-go/internal/store/queries_phase2b.go`.

Remote WebSocket connections use the node connection token, not the user cookie. A connected remote node can answer server requests for paths on the machine where the TypeScript agent runs. Evidence: `packages/server-go/internal/ws/remote.go`, `packages/remote-agent/src/agent.ts`.

Remote-agent directory boundaries are local process checks. The agent resolves target path and allowed dirs and accepts paths equal to or below an allowed dir; it does not use Landlock, sandbox-exec, or host grants. Evidence: `packages/remote-agent/src/fs-ops.ts`.

Remote-agent read is capped at 2 MiB, whereas borgee-helper read is capped at 16 MiB. These are separate surfaces and should not be treated as one policy. Evidence: `packages/remote-agent/src/fs-ops.ts`, `packages/borgee-helper/internal/fileio/file_actions.go`.

Current remote privacy risk: connection-token delivery is unresolved because the token is hidden from JSON, and Go/TypeScript request shapes are mismatched for proxy path forwarding. Evidence: `packages/server-go/internal/store/models.go`, `packages/server-go/internal/api/remote.go`, `packages/server-go/internal/server/server.go`, `packages/remote-agent/src/agent.ts`.

## Host Grants And Helper Boundary

Host grants are user-owned and separate from user permissions. Server grants support `grant_type` values `install`, `exec`, `filesystem`, and `network`, TTL values `one_shot` and `always`, and forward-only revocation through `revoked_at`. Evidence: `packages/server-go/internal/migrations/host_grants.go`, `packages/server-go/internal/api/host_grants.go`.

The helper consumes grants read-only from SQLite. It re-queries on every IPC request and filters out revoked rows; expired rows are distinguished and rejected. This protects revocation visibility better than a helper-side cache. Evidence: `packages/borgee-helper/internal/grants/sqlite_consumer.go`, `packages/borgee-helper/internal/grants/grants.go`.

Helper IPC has a cross-agent boundary: the first handshake binds an `agent_id`, and every request must carry the same `agent_id`. Mismatches return `cross_agent_reject`. Evidence: `packages/borgee-helper/internal/ipc/ipc.go`, `packages/borgee-helper/internal/acl/acl.go`, `packages/borgee-helper/internal/reasons/reasons.go`.

Helper IO is read-only by protocol and by implementation. Only `list_files`, `read_file`, and `network_egress` are accepted; file reads use `os.Open`, directory lists use `os.ReadDir`, and write-class actions are rejected before IO. Evidence: `packages/borgee-helper/internal/acl/acl.go`, `packages/borgee-helper/internal/fileio/file_actions.go`.

Linux sandboxing uses Landlock read-file/read-dir rules over static `--read-paths`; empty read paths fail closed with an empty ruleset. On kernels without Landlock support, current code returns nil as a no-op fallback. Evidence: `packages/borgee-helper/internal/sandbox/sandbox_linux.go`, `packages/borgee-helper/cmd/borgee-helper/main.go`.

macOS sandboxing depends on being launched under a generated `sandbox-exec` profile; `Apply` itself is a no-op. The profile allows configured read subpaths, audit/tmp writes, and local Unix socket operations. Evidence: `packages/borgee-helper/internal/sandbox/sandbox_darwin.go`.

Current host-grant/sandbox risk: grants can be revoked in SQLite, but the OS sandbox allowlist is static after daemon startup. ACL checks may reject revoked paths, but newly granted paths will not be added to Landlock/sandbox-exec until restart/relaunch with updated `--read-paths`/profile. Evidence: `packages/borgee-helper/cmd/borgee-helper/main.go`, `packages/borgee-helper/internal/sandbox/sandbox_linux.go`, `packages/borgee-helper/internal/sandbox/sandbox_darwin.go`.

## Audit And Persistence

Admin actions are persisted through `admin_actions`/`audit_events` depending on migration state. The code keeps a backward-compatible `admin_actions` view after renaming to `audit_events`, and store helpers still write through the `admin_actions` table name. Evidence: `packages/server-go/internal/migrations/admin_actions.go`, `packages/server-go/internal/migrations/admin_audit_events_rename.go`, `packages/server-go/internal/store/admin_actions.go`.

User-facing admin audit is scoped to the affected user through `ListAdminActionsForTargetUser`; admin-facing audit can query across actors/actions/targets and archive state. Multi-source audit currently merges server/plugin audit events and agent event tables, while `host_bridge` is a placeholder that returns no rows. Evidence: `packages/server-go/internal/api/admin_endpoints.go`, `packages/server-go/internal/store/admin_actions.go`, `packages/server-go/internal/api/admin_audit_query.go`.

System DM notification for admin actions is best-effort. `EmitAdminActionAudit` persists the audit row first, then ignores system-DM delivery failure. Missing target system channel degrades silently. Evidence: `packages/server-go/internal/store/admin_actions.go`.

Helper audit is JSONL and best-effort. The daemon falls back to stderr if the audit log file cannot be opened, and IPC ignores `Audit.Write` errors after attempting to write every request. Evidence: `packages/borgee-helper/cmd/borgee-helper/main.go`, `packages/borgee-helper/internal/ipc/ipc.go`, `packages/borgee-helper/internal/audit/audit.go`.

Host grants REST currently logs grant/revoke actions through the process logger rather than inserting persistent `audit_events` rows. Evidence: `packages/server-go/internal/api/host_grants.go`.

Remote REST/WS proxy activity has no inspected persistent audit path. The remote code updates `last_seen_at` on WS connect and returns proxied results/errors, but no audit insert/log helper is visible in `remote.go` or `ws/remote.go`. Evidence: `packages/server-go/internal/api/remote.go`, `packages/server-go/internal/ws/remote.go`, `packages/server-go/internal/store/queries_phase3.go`.

## Installer Boundary

The installer is a local privileged deployment tool, not a server-side authority. Linux and macOS CLIs fetch a manifest, verify ed25519 with a caller-supplied public key, ask for confirmation, then run sudo deployment commands. Evidence: `packages/borgee-installer/cmd/borgee-installer-linux/main.go`, `packages/borgee-installer/cmd/borgee-installer-darwin/main.go`, `packages/borgee-installer/internal/manifest/fetcher.go`, `packages/borgee-installer/internal/deploy/deploy.go`.

Current installer risk: the installer fetcher expects `{entries, signed_at, signature}`, while the server plugin-manifest handler returns `{manifest_version, issued_at, expires_at, signature, plugins}` and signs a different payload. Verification against the current server endpoint is therefore not aligned. Evidence: `packages/borgee-installer/internal/manifest/fetcher.go`, `packages/server-go/internal/api/host_manifest.go`.

Current installer dialog risk: dialog grant types are `read/write/exec/network`, but server `host_grants.grant_type` allows `install/exec/filesystem/network`. This can mislead users or produce grants that do not match server validation unless translated elsewhere. Evidence: `packages/borgee-installer/internal/dialog/dialog.go`, `packages/server-go/internal/migrations/host_grants.go`, `packages/server-go/internal/api/host_grants.go`.

## Best-Effort And Persistence Risks

- Admin system DMs are best-effort after persistent audit row creation; missing system channel is not fatal. Evidence: `packages/server-go/internal/store/admin_actions.go`.
- Admin impersonation grants exist, but admin-write enforcement helper is not wired into production handlers. Evidence: `packages/server-go/internal/api/admin_grant_check.go`, `packages/server-go/internal/api/admin.go`.
- Helper audit writes can fail without blocking IPC; daemon may fall back to stderr. Evidence: `packages/borgee-helper/internal/audit/audit.go`, `packages/borgee-helper/internal/ipc/ipc.go`, `packages/borgee-helper/cmd/borgee-helper/main.go`.
- Linux Landlock has a no-op fallback on unsupported kernels; macOS sandboxing depends on external `sandbox-exec` launch. Evidence: `packages/borgee-helper/internal/sandbox/sandbox_linux.go`, `packages/borgee-helper/internal/sandbox/sandbox_darwin.go`.
- Remote-agent uses process-level allowed-dir checks, not host grants or OS sandboxing. Evidence: `packages/remote-agent/src/fs-ops.ts`.
- Remote proxy and installer manifest paths have current code-shape mismatches and should not be presented as fully operational until fixed. Evidence: `packages/server-go/internal/server/server.go`, `packages/remote-agent/src/agent.ts`, `packages/server-go/internal/api/host_manifest.go`, `packages/borgee-installer/internal/manifest/fetcher.go`.
