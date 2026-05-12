# API Auth Admin Rails

当前 server-go 有两条明确分离的 HTTP rail：user rail 是 `/api/v1/*`，使用 `borgee_token` JWT cookie、Bearer API key 和 `users` / `user_permissions`；admin rail 是 `/admin-api/*`，使用 `borgee_admin_session` opaque cookie、`admins` / `admin_sessions`。route mount 和 middleware 都在 `Server.SetupRoutes()` 中完成。证据：`packages/server-go/internal/server/server.go`、`packages/server-go/internal/auth/middleware.go`、`packages/server-go/internal/auth/permissions.go`、`packages/server-go/internal/admin/auth.go`、`packages/server-go/internal/admin/middleware.go`。

```mermaid
flowchart LR
  req[HTTP request]
  user[/api/v1/* user rail]
  admin[/admin-api/* admin rail]
  userAuth[auth.AuthMiddleware\nborgee_token / Bearer api_key]
  perms[user_permissions\nRequirePermission / HasCapability]
  adminAuth[admin.RequireAdmin\nborgee_admin_session]
  adminDB[admins + admin_sessions]
  handlers[api handlers]

  req --> user --> userAuth --> perms --> handlers
  req --> admin --> adminAuth --> adminDB --> handlers
```

## 负责什么

user rail 负责已登录 user/agent 的业务 API：channel/message/DM/agent/runtime/artifact/workspace/remote/push/poll 等 `/api/v1/*` routes。它通过 `auth.AuthMiddleware` 放入 `*store.User` context，再由 handler 或 permission middleware 做资源校验。证据：`packages/server-go/internal/server/server.go`、`packages/server-go/internal/auth/middleware.go`、`packages/server-go/internal/api/channels.go`、`packages/server-go/internal/api/messages.go`、`packages/server-go/internal/api/agents.go`、`packages/server-go/internal/api/artifacts.go`。

admin rail 负责 admin auth、admin metadata views、admin user/invite/channel operations、audit、runtime metadata、retention override 和 heartbeat lag 等 `/admin-api/*` routes。它通过 `admin.RequireAdmin` 放入 `*admin.Admin` context。证据：`packages/server-go/internal/server/server.go`、`packages/server-go/internal/admin/auth.go`、`packages/server-go/internal/admin/middleware.go`、`packages/server-go/internal/api/admin.go`、`packages/server-go/internal/api/admin_endpoints.go`、`packages/server-go/internal/api/runtimes.go`、`packages/server-go/internal/api/host_lag.go`。

capability 层负责 user rail 的 row-based 授权。`RequirePermission` 查 `user_permissions`，`HasCapability` 在权限匹配外还对 `channel:` / `artifact:` scope 做 org boundary。证据：`packages/server-go/internal/auth/permissions.go`、`packages/server-go/internal/auth/abac.go`、`packages/server-go/internal/store/queries.go`。

admin privacy boundary 负责 admin response 的 metadata-only 约束。代码中 admin runtime list 明确省略 `last_error_reason`，admin read endpoint 有 forbidden key reflect scan 测试防止泄露 content-bearing fields。证据：`packages/server-go/internal/api/runtimes.go`、`packages/server-go/internal/admin/handlers_field_whitelist_test.go`。

## 不负责什么

user rail 不提供 admin god-mode shortcut。`RequirePermission` 明确移除了 `users.role == "admin"` shortcut，admin authority 只走 `/admin-api/*`。证据：`packages/server-go/internal/auth/permissions.go`、`packages/server-go/internal/server/server.go`。

admin rail 不使用 `users` 表表达 admin 身份。admin bootstrap、login、session resolve 都围绕 `admins` 与 `admin_sessions`；`internal/admin` 注释也要求不 import `internal/auth`。证据：`packages/server-go/internal/admin/auth.go`、`packages/server-go/internal/admin/middleware.go`、`packages/server-go/internal/migrations/admin_admins.go`、`packages/server-go/internal/migrations/admin_sessions.go`。

admin rail 不自动获得 user rail 的 owner 身份。owner-only runtime、channel preference、message/artifact write 等路径仍在 user rail；admin endpoints 只在被挂载的 `/admin-api/*` route 上生效。证据：`packages/server-go/internal/server/server.go`、`packages/server-go/internal/api/runtimes.go`、`packages/server-go/internal/api/channel_pin.go`、`packages/server-go/internal/api/channel_mute.go`、`packages/server-go/internal/api/channel_description.go`。

WebSocket routes 不通过 HTTP `authMw` 或 `adminMw`。`/ws`、`/ws/plugin`、`/ws/remote` 分别在 handler 内做 token/cookie 校验。证据：`packages/server-go/internal/server/server.go`、`packages/server-go/internal/ws/client.go`、`packages/server-go/internal/ws/plugin.go`、`packages/server-go/internal/ws/remote.go`。

## 和其他模块的接口

`internal/server` 是 rail 的挂载点。它创建 `authMw := auth.AuthMiddleware(s.store, s.cfg)`，把 user handler 包进 `authMw`；同时创建 `adminMw := admin.RequireAdmin(s.store.DB(), nil)`，把 admin handler 包进 `adminMw`。证据：`packages/server-go/internal/server/server.go`。

`internal/api` 通过 `auth.UserFromContext`、`auth.HasCapability`、`auth.RequirePermission`、`admin.AdminFromContext` 读取认证/授权结果，而不是自己解析 cookie。证据：`packages/server-go/internal/auth/middleware.go`、`packages/server-go/internal/auth/abac.go`、`packages/server-go/internal/admin/middleware.go`、`packages/server-go/internal/api/admin_endpoints.go`。

`internal/store` 为 auth/admin 提供用户、API key、permission、admin action、session 等数据查询；admin package 直接用 `*gorm.DB` 访问 `admins` / `admin_sessions`。证据：`packages/server-go/internal/store/queries.go`、`packages/server-go/internal/store/admin_actions.go`、`packages/server-go/internal/admin/auth.go`。

## User Rail Authentication

`auth.AuthMiddleware` 的第一优先级是 `borgee_token` cookie。它用 `ValidateJWT(s, cfg.JWTSecret, cookie.Value)` 解析 user id，并拒绝 missing/deleted/disabled user。证据：`packages/server-go/internal/auth/middleware.go`。

第二优先级是 `Authorization: Bearer <api_key>`。middleware 用 `Store.GetUserByAPIKey` 查用户，并同样拒绝 deleted/disabled user。证据：`packages/server-go/internal/auth/middleware.go`、`packages/server-go/internal/store/queries.go`。

development 且 `DevAuthBypass` 开启时，middleware 接受 `X-Dev-User-Id`；如果没有该 header，会选择第一个 `role == "member"` 的 user。证据：`packages/server-go/internal/auth/middleware.go`。

`AuthenticateFlexible`、`AuthenticateFromAPIKey`、`AuthenticateFromQuery` 是 user rail 的辅助认证函数，用于不直接套 `AuthMiddleware` 的路径。证据：`packages/server-go/internal/auth/middleware.go`。

`api.AuthHandler.RegisterRoutes` 挂 `POST /api/v1/auth/login`、`POST /api/v1/auth/register`、`POST /api/v1/auth/logout`；`GET /api/v1/users/me` 在 server 层显式用 `authMw` 包起来。证据：`packages/server-go/internal/api/auth.go`、`packages/server-go/internal/server/server.go`。

## User Authorization And Capability

`RequirePermission` 从 request context 取 user，读取 `ListUserPermissions(user.ID)`，允许 `(*,*)`、指定 permission + `*` scope、指定 permission + exact scope。证据：`packages/server-go/internal/auth/permissions.go`、`packages/server-go/internal/store/queries.go`。

`RequirePermission` 不看 `users.role == admin`。这意味着 user rail 中的 admin-like 能力必须表现为 permission row，而不是 role shortcut。证据：`packages/server-go/internal/auth/permissions.go`。

`HasCapability(ctx, store, permission, scope)` 是 capability helper。它支持 `*`、`channel:<id>`、`artifact:<id>` scope，并先做 cross-org gate：当 user org 和资源 org 都非空且不一致时返回 false。证据：`packages/server-go/internal/auth/abac.go`、`packages/server-go/internal/store/models.go`。

agent 不享受 `(*,*)` wildcard shortcut。`HasCapability` 在 `user.Role == "agent"` 时跳过 wildcard grant，只接受具体 permission/scope 匹配。证据：`packages/server-go/internal/auth/abac.go`。

当前 route 既有 middleware-style permission，也有 handler 内 owner/member check。消息发送/读取使用 `RequirePermission`；runtime control 使用 owner check；private channel/message/artifact access 使用 channel membership 或 org/member predicate。证据：`packages/server-go/internal/server/server.go`、`packages/server-go/internal/api/messages.go`、`packages/server-go/internal/api/runtimes.go`、`packages/server-go/internal/api/channels.go`、`packages/server-go/internal/api/artifacts.go`。

## Admin Authentication And Session

admin bootstrap 读取 `BORGEE_ADMIN_LOGIN` 与 `BORGEE_ADMIN_PASSWORD_HASH` 或 `BORGEE_ADMIN_PASSWORD`，写入 `admins`；生产入口在 `server.New` 之前 fail loud 执行一次，`SetupRoutes` 再执行一次并记录错误。证据：`packages/server-go/cmd/collab/main.go`、`packages/server-go/internal/server/server.go`、`packages/server-go/internal/admin/auth.go`。

admin login routes 是 `POST /admin-api/auth/login` 与 `POST /admin-api/v1/auth/login`；成功后创建 `admin_sessions` row，并设置 `borgee_admin_session` HttpOnly SameSite=Lax cookie。证据：`packages/server-go/internal/admin/auth.go`。

admin cookie value 是 32-byte random hex 的 opaque session token，不是 admin id。`ResolveSession` 必须查询 `admin_sessions`，检查 `expires_at`，再读取 `admins` row。证据：`packages/server-go/internal/admin/auth.go`、`packages/server-go/internal/migrations/admin_sessions.go`。

`admin.RequireAdmin` 只接受 `borgee_admin_session` cookie，解析失败或过期都返回 401；成功时把 `*admin.Admin` 放入 context。证据：`packages/server-go/internal/admin/middleware.go`。

logout routes 删除当前 session token 并清空 cookie。证据：`packages/server-go/internal/admin/auth.go`。

## Admin Handler Surface

核心 admin handler 挂 `/admin-api/v1/stats`、`/users`、`/invites`、`/channels` 及 permission/API key 管理 endpoints；所有这些 route 都通过 `adminMw` 包装。证据：`packages/server-go/internal/api/admin.go`、`packages/server-go/internal/server/server.go`。

admin audit endpoints 分两面：user rail 有 `/api/v1/me/admin-actions` 和 `/api/v1/me/impersonation-grant`，只看或管理当前 user 自己的授权；admin rail 有 `/admin-api/v1/audit-log`，要求 `admin.AdminFromContext`。证据：`packages/server-go/internal/api/admin_endpoints.go`。

multi-source audit 只挂 admin rail 的 `GET /admin-api/v1/audit/multi-source`，读取 server/plugin/host_bridge/agent source 并返回 source/action/actor/target/payload/created_at shape。证据：`packages/server-go/internal/api/admin_audit_query.go`、`packages/server-go/internal/server/server.go`。

runtime metadata admin endpoint 只挂 `GET /admin-api/v1/runtimes`，不挂 admin start/stop/error 写路径；响应白名单包含 id、agent_id、endpoint_url、process_kind、status、last_heartbeat_at，并省略 `last_error_reason`。证据：`packages/server-go/internal/api/runtimes.go`、`packages/server-go/internal/server/server.go`。

部分 admin read endpoints 是 readonly mirror，例如 archived channels、message edit history、channel description history、comment edit history；server 没有给这些 handler 挂 admin PATCH/PUT/DELETE routes。证据：`packages/server-go/internal/server/server.go`、`packages/server-go/internal/api/channel_archived.go`、`packages/server-go/internal/api/message_history.go`、`packages/server-go/internal/api/channel_history.go`、`packages/server-go/internal/api/canvas_edit_history.go`。

## Admin Metadata And Privacy Boundary

admin user list 使用 `sanitizeUserAdmin` 显式输出 id、display_name、role、avatar_url、require_mention、disabled、created_at 及可选 email/owner/deleted/last_seen；它不直接 dump `store.User`。证据：`packages/server-go/internal/api/admin.go`。

admin action serialization 在 admin view 下包含 `actor_id`，user view 下省略 raw actor id；两者都输出 action metadata，而 user rail 只返回 target_user_id 等当前用户相关数据。证据：`packages/server-go/internal/api/admin_endpoints.go`。

admin read privacy 有测试守门：`handlers_field_whitelist_test.go` 对当前 mounted admin read endpoints 扫描 forbidden keys `body`、`content`、`text`、`artifact`。证据：`packages/server-go/internal/admin/handlers_field_whitelist_test.go`。

admin audit hook 也有 metadata-only 约束测试，要求 audit metadata 不包含 channel content、DM body、artifact content 等内容字段。证据：`packages/server-go/internal/api/admin_audit_hook_test.go`。

## WebSocket Auth Boundary

浏览器 `/ws` 认证顺序包括 `Sec-WebSocket-Protocol: Bearer,<api_key>`、`Authorization: Bearer <api_key>`、query `token=<api_key>`、`borgee_token` JWT cookie，以及 development bypass 的 query `user_id` / `X-Dev-User-Id` / first member fallback。证据：`packages/server-go/internal/ws/client.go`。

`/ws/plugin` 接受 agent API key，来源是 `Authorization: Bearer <api_key>` 或 query `apiKey`；连接成功后注册为 plugin connection，并把非 RPC BPP frame 交给 `PluginFrameDispatcher`。证据：`packages/server-go/internal/ws/plugin.go`、`packages/server-go/internal/server/server.go`、`packages/server-go/internal/bpp/plugin_frame_dispatcher.go`。

`/ws/remote` 接受 remote node connection token，来源是 `Authorization: Bearer <token>` 或 query `token`；成功后查 `remote_nodes` 并更新 last seen。证据：`packages/server-go/internal/ws/remote.go`、`packages/server-go/internal/store/models.go`、`packages/server-go/internal/api/remote.go`。

WebSocket auth 不使用 admin cookie。admin rail 的 `borgee_admin_session` 只由 `internal/admin` HTTP middleware 解析。证据：`packages/server-go/internal/admin/middleware.go`、`packages/server-go/internal/ws/client.go`、`packages/server-go/internal/ws/plugin.go`、`packages/server-go/internal/ws/remote.go`。
