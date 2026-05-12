# API Auth And Admin Rails

本文按当前代码描述用户 API、管理员 API、plugin/remote rails 与 capability 授权边界。结论只来自本 worktree；未在代码里闭合的部分标为“未确定/未接线”。

## Entry Map

| Rail | Entry | Credential | Principal | Primary gate | Evidence |
| --- | --- | --- | --- | --- | --- |
| User REST | `/api/v1/*` | `borgee_token` cookie, `Authorization: Bearer <api_key>`, dev bypass | `store.User` | `auth.AuthMiddleware` + handler ACL/capability checks | `packages/server-go/internal/server/server.go`, `packages/server-go/internal/auth/middleware.go`, `packages/server-go/internal/api/auth_helpers.go` |
| User WebSocket | `/ws` | WebSocket subprotocol `Bearer,<api_key>`, bearer header, `?token=`, `borgee_token`, dev bypass | `store.User` | `authenticateWS` | `packages/server-go/internal/ws/client.go` |
| Admin auth | `/admin-api/auth/*`, `/admin-api/v1/auth/*` | `borgee_admin_session` cookie | `admin.Admin` | `admin.RequireAdmin` after login | `packages/server-go/internal/admin/auth.go`, `packages/server-go/internal/admin/middleware.go` |
| Admin REST | `/admin-api/v1/*` | `borgee_admin_session` cookie | `admin.Admin` | `admin.RequireAdmin` | `packages/server-go/internal/server/server.go`, `packages/server-go/internal/api/admin.go`, `packages/server-go/internal/api/admin_endpoints.go` |
| Plugin WS | `/ws/plugin` | bearer API key or `?apiKey=` | agent user row | `GetUserByAPIKey` | `packages/server-go/internal/ws/plugin.go` |
| Plugin manifest | `GET /api/v1/plugin-manifest` | current `authMw` accepts cookie, bearer API key, dev bypass | `store.User` | `auth.AuthMiddleware` + `mustUser` | `packages/server-go/internal/api/host_manifest.go`, `packages/server-go/internal/server/server.go` |
| Remote REST | `/api/v1/remote/*` | current `authMw` accepts cookie, bearer API key, dev bypass | owner `store.User` | owner check against `RemoteNode.UserID` | `packages/server-go/internal/api/remote.go` |
| Remote WS | `/ws/remote` | remote node connection token via bearer header or `?token=` | `RemoteNode` | `GetRemoteNodeByToken` | `packages/server-go/internal/ws/remote.go`, `packages/server-go/internal/store/queries_phase3.go` |

```mermaid
flowchart LR
  UserCookie[borgee_token JWT] --> AuthMw[auth.AuthMiddleware]
  Bearer[Bearer API key] --> AuthMw
  Dev[DEV_AUTH_BYPASS] --> AuthMw
  AuthMw --> UserAPI[/api/v1/*]

  AdminLogin[/admin-api/v1/auth/login] --> AdminSession[admin_sessions opaque token]
  AdminSession --> AdminMw[admin.RequireAdmin]
  AdminMw --> AdminAPI[/admin-api/v1/*]

  PluginKey[agent API key] --> PluginWS[/ws/plugin]
  RemoteToken[remote node token] --> RemoteWS[/ws/remote]
```

## User Auth

User sessions use cookie name `borgee_token`. Login and registration mint an HS256 JWT with `userId`, `email`, `iat`, and a 7-day `exp`, then set an HTTP-only, SameSite=Lax cookie with `MaxAge=604800`; the cookie is marked Secure outside development unless the request host is localhost/127.0.0.1. Logout clears the same cookie. Evidence: `packages/server-go/internal/auth/middleware.go`, `packages/server-go/internal/api/auth.go`.

`auth.AuthMiddleware` authenticates in this order: session cookie, `Authorization: Bearer <api_key>`, then development bypass. Bearer auth uses `Store.GetUserByAPIKey` and rejects deleted or disabled users. Development bypass is only active when `NODE_ENV=development` and `DEV_AUTH_BYPASS=true`; it first honors `X-Dev-User-Id`, then falls back to the first `role == "member"` user for HTTP middleware. Evidence: `packages/server-go/internal/auth/middleware.go`, `packages/server-go/internal/config/config.go`.

`AuthenticateFlexible` has a different order and narrower bypass: bearer API key first, then cookie, then `X-Dev-User-Id`; it does not implement the first-member fallback. Use it as a distinct helper, not as identical behavior to `AuthMiddleware`. Evidence: `packages/server-go/internal/auth/middleware.go`.

WebSocket user auth has its own resolver. `/ws` accepts `Sec-WebSocket-Protocol: Bearer,<api_key>`, `Authorization: Bearer <api_key>`, `?token=<api_key>`, then the `borgee_token` cookie, then dev bypass via `?user_id=`, `X-Dev-User-Id`, or first member. Evidence: `packages/server-go/internal/ws/client.go`.

Registration requires an invite code, lowercases email, validates password length 8-72, creates a `role="member"` user, creates an org, consumes the invite, grants default permissions, adds the user to public channels, creates a welcome/system channel best-effort, and signs the user cookie. Evidence: `packages/server-go/internal/api/auth.go`, `packages/server-go/internal/store/queries.go`.

## User Permissions And Capabilities

The canonical capability allowlist is `auth.ALL`/`auth.Capabilities`, currently 14 dot-notation literals: channel read/write/delete, artifact read/write/commit/iterate/rollback, mention/DM read/send, and channel member/invite/role management. Admin god-mode is intentionally absent from this allowlist. Evidence: `packages/server-go/internal/auth/capabilities.go`.

`user_permissions` is the row source for authorization: `(user_id, permission, scope, granted_by, granted_at, expires_at, revoked_at)`. `ListUserPermissions` filters `revoked_at IS NULL`; the AP-2 sweeper soft-revokes expired rows by setting `revoked_at` and emits an `admin_actions` audit row with actor `system`. Evidence: `packages/server-go/internal/store/models.go`, `packages/server-go/internal/store/queries.go`, `packages/server-go/internal/auth/expires_sweeper.go`, `packages/server-go/internal/migrations/permission_user_permissions_expires.go`, `packages/server-go/internal/migrations/permission_user_permissions_revoked.go`.

`auth.HasCapability` is the ABAC helper for capability-aware handlers. It reads the authenticated user from context, rejects cross-org access when it can resolve a channel/artifact scope org, allows explicit `(permission, scope)`, explicit `(permission, *)`, and for non-agent users only the wildcard `(*, *)`. Agents do not get wildcard god-mode even if a row exists. Evidence: `packages/server-go/internal/auth/abac.go`.

Default grants are role based at creation time: `member` receives one `(*, *)` row; `agent` receives `message.send:*` and `message.read:*`; all other roles receive no default grants. Evidence: `packages/server-go/internal/store/queries.go`, `packages/server-go/internal/api/auth.go`, `packages/server-go/internal/api/admin.go`.

Important current mismatch: the 14-value `auth.ALL` allowlist does not contain several literals used by older `RequirePermission` callsites, including `message.send`, `message.read`, `channel.create`, `channel.manage_visibility`, and `agent.runtime.control`. `RequirePermission` itself does not call `auth.IsValidCapability`; it scans rows and wildcard grants directly. Admin grant creation, owner one-click grants, and capability-grant DM validation do call `auth.IsValidCapability`. Evidence: `packages/server-go/internal/auth/capabilities.go`, `packages/server-go/internal/auth/permissions.go`, `packages/server-go/internal/server/server.go`, `packages/server-go/internal/api/admin.go`, `packages/server-go/internal/api/me_grants.go`, `packages/server-go/internal/api/capability_grant.go`.

`GET /api/v1/me/permissions` returns `permissions`, row `details`, and a derived `capabilities` array. For `role="member"` it returns `permissions=["*"]` and all 14 allowlisted capabilities without querying rows; agents/bundle-narrowed accounts derive capabilities from explicit permission strings and drop unknown tokens. Evidence: `packages/server-go/internal/api/users.go`.

## Admin Auth

Admins live in a separate `admins` table, not `users`. Bootstrap requires `BORGEE_ADMIN_LOGIN` and exactly one of `BORGEE_ADMIN_PASSWORD_HASH` or `BORGEE_ADMIN_PASSWORD`; plaintext is bcrypt-hashed in memory before insert, and configured hashes must be bcrypt cost >= 10. Evidence: `packages/server-go/internal/admin/auth.go`, `packages/server-go/internal/migrations/admin_admins.go`.

Admin login verifies bcrypt using `bcrypt.CompareHashAndPassword` plus `subtle.ConstantTimeCompare` on the success signal. It creates an opaque 32-byte random hex token in `admin_sessions`; the `borgee_admin_session` cookie value is that token, never an admin id or JWT. Session TTL is 7 days. Evidence: `packages/server-go/internal/admin/auth.go`, `packages/server-go/internal/migrations/admin_sessions.go`.

`admin.RequireAdmin` only accepts the `borgee_admin_session` cookie, resolves it through `admin_sessions`, checks expiry, loads the `admins` row, and injects a private admin context value. User cookies and user API keys are not accepted on this rail. Evidence: `packages/server-go/internal/admin/middleware.go`, `packages/server-go/internal/admin/auth.go`.

The admin auth handler mounts both non-versioned and versioned auth aliases: `POST /admin-api/auth/login`, `POST /admin-api/auth/logout`, `GET /admin-api/auth/me`, and the same under `/admin-api/v1/auth/*`. Evidence: `packages/server-go/internal/admin/auth.go`.

## Admin API Rail

Admin REST endpoints are mounted only under `/admin-api/v1/*` behind `admin.RequireAdmin`; the legacy `/api/v1/admin/*` user-rail god-mode mount is intentionally absent from `server.go`. Evidence: `packages/server-go/internal/server/server.go`, `packages/server-go/internal/api/admin.go`.

Core admin endpoints include stats, users CRUD, user agents, API key generate/delete, user permission list/grant/revoke, invites, channel list, and force-delete channel. Permission grants reject capability names not present in `auth.IsValidCapability`, while revocation only requires a provided literal and matching row. Evidence: `packages/server-go/internal/api/admin.go`, `packages/server-go/internal/auth/capabilities.go`.

Admin metadata-only read surfaces are mounted separately. `GET /admin-api/v1/runtimes` returns only `id`, `agent_id`, `endpoint_url`, `process_kind`, `status`, and `last_heartbeat_at`; it deliberately omits raw `last_error_reason` and does not expose admin start/stop/heartbeat/error runtime writes. Evidence: `packages/server-go/internal/server/server.go`, `packages/server-go/internal/api/runtimes.go`, `packages/server-go/internal/admin/handlers_field_whitelist_test.go`.

Admin audit surfaces are split by audience. Users can read their own affected admin actions via `GET /api/v1/me/admin-actions`, which ignores any target-user query injection and filters by current user. Admins read cross-user audit via `GET /admin-api/v1/audit-log` with filters, and a multi-source view via `GET /admin-api/v1/audit/multi-source`. Evidence: `packages/server-go/internal/api/admin_endpoints.go`, `packages/server-go/internal/store/admin_actions.go`, `packages/server-go/internal/api/admin_audit_query.go`.

`admin_actions` was introduced as the forward-only admin audit table and later renamed to `audit_events` with an `admin_actions` compatibility view and triggers. The Go store still uses `AdminAction.TableName() == "admin_actions"`, so writes route through the view when the migration has run. Evidence: `packages/server-go/internal/migrations/admin_actions.go`, `packages/server-go/internal/migrations/admin_audit_events_rename.go`, `packages/server-go/internal/store/admin_actions.go`.

Admin write audit coverage is partial by code inspection. `handleUpdateUser` emits audit rows for suspend, role change, and password reset; `handleForceDeleteChannel` emits delete-channel audit. API-key changes, invite CRUD, and permission grant/revoke do not visibly call `EmitAdminActionAudit` in the inspected code. Evidence: `packages/server-go/internal/api/admin.go`, `packages/server-go/internal/store/admin_actions.go`.

Impersonation grants are user-created via `/api/v1/me/impersonation-grant` and stored for 24 hours with optional owner revocation. There is a helper `RequireImpersonationGrant` intended to reject admin writes without an active target-user grant, but this helper has no production callsites in `internal/api` in the inspected worktree. Treat admin-write impersonation gating as not currently wired. Evidence: `packages/server-go/internal/api/admin_endpoints.go`, `packages/server-go/internal/api/admin_grant_check.go`, `packages/server-go/internal/store/admin_actions.go`, `packages/server-go/internal/migrations/admin_impersonation_grants.go`.

## Plugin Rail

`/ws/plugin` authenticates only by agent/user API key, supplied as `Authorization: Bearer <api_key>` or `?apiKey=`. It rejects missing, deleted, or disabled users, registers a `PluginConn` keyed by `user.ID`, updates plugin liveness on inbound frames, handles RPC `api_request`/`api_response`, and routes non-RPC BPP frames to the plugin frame router when present. Evidence: `packages/server-go/internal/ws/plugin.go`, `packages/server-go/internal/server/server.go`.

Plugin `api_request` frames are re-entered into the same HTTP server handler through an internal `httptest.NewRequest`; the plugin connection's API key is injected as `Authorization: Bearer <api_key>`. This means plugin API reach is constrained by the same user API auth and downstream ACL/capability logic as direct bearer requests. Evidence: `packages/server-go/internal/ws/plugin.go`.

`GET /api/v1/plugin-manifest` is mounted behind `authMw`, so the live middleware accepts cookie, bearer API key, or dev bypass even though comments describe owner bearer API-key auth. The handler logs a `plugin_manifest.fetch` line and returns a signed manifest payload from `PluginManifestEntries`; admin rail does not mount this endpoint. Evidence: `packages/server-go/internal/api/host_manifest.go`, `packages/server-go/internal/server/server.go`, `packages/server-go/internal/auth/middleware.go`.

Capability request flow is BPP-mediated: a plugin can trigger `request_capability_grant`; server validates the requested capability against `auth.Capabilities`, writes a system DM with quick-action JSON to the owner, and the owner grants/rejects/snoozes through `POST /api/v1/me/grants`. Grant writes insert `user_permissions` for the agent; reject/snooze are log-only in v1. Evidence: `packages/server-go/internal/api/capability_grant.go`, `packages/server-go/internal/api/me_grants.go`, `packages/server-go/internal/store/queries.go`.

## Remote Rail

Remote REST nodes and bindings are user-owned. Node CRUD, binding CRUD, channel binding list, node status, node `ls`, and node `read` all mount under user `authMw`; node-specific handlers load `RemoteNode` and require `node.UserID == current user.ID`. Evidence: `packages/server-go/internal/api/remote.go`, `packages/server-go/internal/store/queries_phase2b.go`, `packages/server-go/internal/server/server.go`.

Creating a remote node generates a random 32-byte connection token and stores it in `remote_nodes.connection_token`, but `RemoteNode.ConnectionToken` has `json:"-"`. The inspected REST response returns the node struct, so the token is not exposed by that response; no separate token-returning endpoint was found in the required source set. Connection-token provisioning is therefore unresolved by current code inspection. Evidence: `packages/server-go/internal/store/models.go`, `packages/server-go/internal/store/queries_phase2b.go`, `packages/server-go/internal/api/remote.go`.

`/ws/remote` is not a user-cookie rail. It accepts a remote node connection token via bearer header or `?token=`, resolves the `remote_nodes` row, updates `last_seen_at`, registers the connection, and supports server-initiated request/response with a 10 second timeout. Evidence: `packages/server-go/internal/ws/remote.go`, `packages/server-go/internal/store/queries_phase3.go`, `packages/server-go/internal/server/server.go`.

Current remote proxy request shape is not aligned with the TypeScript remote agent: server sends `data: { action, params: { path } }`, while `packages/remote-agent/src/agent.ts` reads `data.action` and `data.path`. Until code changes or an adapter proves otherwise, treat remote REST `ls/read` proxying as currently mismatched. Evidence: `packages/server-go/internal/server/server.go`, `packages/server-go/internal/api/remote.go`, `packages/server-go/internal/ws/remote.go`, `packages/remote-agent/src/agent.ts`.

## Open Questions / Not Wired

- Admin impersonation-gate helper exists but has no production callsites in inspected code: `packages/server-go/internal/api/admin_grant_check.go`.
- Remote node connection token generation exists, but inspected REST JSON hides it and no token retrieval endpoint was found: `packages/server-go/internal/store/models.go`, `packages/server-go/internal/api/remote.go`.
- Remote proxy request shape is mismatched between Go server and TypeScript remote agent: `packages/server-go/internal/server/server.go`, `packages/remote-agent/src/agent.ts`.
- Capability vocabulary is split between the 14-value allowlist and older `RequirePermission` literals: `packages/server-go/internal/auth/capabilities.go`, `packages/server-go/internal/server/server.go`.
- Plugin manifest handler response shape differs from the installer manifest fetcher; see `10-remote-and-host-bridge.md` for details.
