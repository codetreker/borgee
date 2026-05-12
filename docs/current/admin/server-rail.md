# Admin Server Rail

admin server rail 是独立于 user API 的 `/admin-api/*` 路径。它使用 `admins` 表、`admin_sessions` 表和 `borgee_admin_session` cookie；它不复用 `users.role == admin`，也不把 user cookie 当作 admin god-mode。

```text
POST /admin-api/v1/auth/login
  -> admins.login + bcrypt password check
  -> admin_sessions opaque token row
  -> Set-Cookie borgee_admin_session

/admin-api/v1/*
  -> admin.RequireAdmin
  -> ResolveSession(token)
  -> admin context
  -> admin handlers
```

## 负责什么

admin auth 负责 bootstrap、login、logout、me 和 session resolution。bootstrap 要求 `BORGEE_ADMIN_LOGIN`，并要求 `BORGEE_ADMIN_PASSWORD_HASH` 与 `BORGEE_ADMIN_PASSWORD` 二选一；hash 必须是 bcrypt 且 cost 至少 10，plain path 会在启动时用 bcrypt cost 10 生成 hash。证据：`packages/server-go/internal/admin/auth.go`。

admin session 负责把 cookie value 变成不透明 token。`CreateSession` 生成 32 字节随机 token 的 hex 字符串，写入 `admin_sessions`，TTL 是 7 天；`ResolveSession` 只通过 DB lookup 解析，不从 token 中解析 admin id。证据：`packages/server-go/internal/admin/auth.go`、`packages/server-go/internal/migrations/admin_sessions.go`。

admin middleware 负责保护 admin routes。`RequireAdmin` 只读取 `borgee_admin_session` cookie，查 `admin_sessions` 和 `admins`，成功后把 admin 放入 request context；没有 Bearer 或 dev bypass 分支。证据：`packages/server-go/internal/admin/middleware.go`。

server wiring 负责把 admin routes 和 user routes 分开。`SetupRoutes` 调用 `admin.Bootstrap`、注册 admin auth handler、创建 `admin.RequireAdmin`，再把 admin handlers 挂到 `/admin-api/v1/*`；注释明确 legacy `/api/v1/admin/*` user-rail god-mode mount 未接线。证据：`packages/server-go/internal/server/server.go`。

## 不负责什么

admin rail 不负责 user API authentication。user API 走 `borgee_token` cookie、Bearer API key 或 development bypass；admin middleware 不读取这些凭据。证据：`packages/server-go/internal/auth/middleware.go`、`packages/server-go/internal/admin/middleware.go`。

admin rail 不负责 host grant override。`/api/v1/host-grants` 挂在 user `authMw` 后，代码中未看到 `/admin-api` host grant route。证据：`packages/server-go/internal/api/host_grants.go`、`packages/server-go/internal/server/server.go`。

admin rail 不负责 remote-agent token 或 WebSocket 连接。remote node REST 是 user rail，remote WebSocket 使用 remote node connection token。证据：`packages/server-go/internal/api/remote.go`、`packages/server-go/internal/ws/remote.go`。

## 和其他模块的接口

| Admin interface | Route | Behavior | Evidence |
| --- | --- | --- | --- |
| auth login/logout/me | `/admin-api/auth/*`, `/admin-api/v1/auth/*` | opaque session cookie lifecycle | `packages/server-go/internal/admin/auth.go` |
| main admin API | `/admin-api/v1/stats`, users, permissions, invites, channels | user/admin management operations | `packages/server-go/internal/api/admin.go` |
| runtime metadata | `GET /admin-api/v1/runtimes` | read-only whitelist, no raw error reason | `packages/server-go/internal/api/runtimes.go` |
| audit log | `GET /admin-api/v1/audit-log` | admin-visible audit_events/admin_actions list with filters | `packages/server-go/internal/api/admin_endpoints.go` |
| multi-source audit | `GET /admin-api/v1/audit/multi-source` | server/plugin/agent plus host_bridge placeholder | `packages/server-go/internal/api/admin_audit_query.go` |

## Admin Handler Surface

| Capability | Route group | Write/read | Audit behavior | Evidence |
| --- | --- | --- | --- | --- |
| stats | `GET /admin-api/v1/stats` | read | no audit row | `packages/server-go/internal/api/admin.go` |
| users | `GET/POST/PATCH/DELETE /admin-api/v1/users` | read/write | `PATCH` audits suspend/change_role/reset_password when those fields change; delete user does not visibly call audit helper | `packages/server-go/internal/api/admin.go`, `packages/server-go/internal/store/admin_actions.go` |
| user agents | `GET /admin-api/v1/users/{id}/agents` | read | no audit row | `packages/server-go/internal/api/admin.go` |
| API key | `POST/DELETE /admin-api/v1/users/{id}/api-key` | write | no visible audit helper call | `packages/server-go/internal/api/admin.go` |
| permissions | `GET/POST/DELETE /admin-api/v1/users/{id}/permissions` | read/write | grant validates capability with `auth.IsValidCapability`; no visible audit helper call | `packages/server-go/internal/api/admin.go`, `packages/server-go/internal/auth/capabilities.go` |
| invites | `POST/GET/DELETE /admin-api/v1/invites` | read/write | no visible audit helper call | `packages/server-go/internal/api/admin.go` |
| channels | `GET /admin-api/v1/channels`, `DELETE /admin-api/v1/channels/{id}/force` | read/write | force delete emits delete_channel audit after commit | `packages/server-go/internal/api/admin.go` |
| runtimes | `GET /admin-api/v1/runtimes` | read-only | no raw `last_error_reason` in response | `packages/server-go/internal/api/runtimes.go` |

## Metadata-only Admin Runtime

Admin runtime rail reads `agent_runtimes` and returns only `id`、`agent_id`、`endpoint_url`、`process_kind`、`status`、`last_heartbeat_at`。It intentionally omits `last_error_reason`, while owner user runtime GET can include that reason. Evidence: `packages/server-go/internal/api/runtimes.go`。

## Known risk / unknown

- `RequireImpersonationGrant` helper exists and tests cover it, but current production callsites were not found; admin write handlers do not appear to enforce active user impersonation grants before writes. Evidence: `packages/server-go/internal/api/admin_grant_check.go`, `packages/server-go/internal/api/admin.go`。
- Audit coverage is not uniform across admin writes: `PATCH users` selected fields and force channel delete call audit helpers; API key, permission, invite, create user, delete user paths do not visibly call the same audit helper in current code. Evidence: `packages/server-go/internal/api/admin.go`, `packages/server-go/internal/store/admin_actions.go`。
