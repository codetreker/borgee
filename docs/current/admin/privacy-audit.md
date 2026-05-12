# Admin Privacy Audit

admin privacy/audit rail 负责记录和展示管理员影响用户的行为，同时限制普通用户只能看到与自己相关的 admin actions。当前实现有持久 audit、best-effort system DM、用户自授权 impersonation grant，以及多源 audit 查询。

```text
admin write handler
  -> business write commits
  -> EmitAdminActionAudit / InsertAdminAction where wired
  -> audit row in admin_actions view / audit_events table
  -> best-effort system DM to target user's system channel

user privacy view
  -> GET /api/v1/me/admin-actions
  -> WHERE target_user_id = current user

admin audit view
  -> GET /admin-api/v1/audit-log
  -> filters over audit rows
```

## 负责什么

audit storage 负责持久记录 admin-facing audit rows。`AdminAction` 映射到 `admin_actions`，后续 migration 将 `admin_actions` rename 为 `audit_events`，再创建 `admin_actions` view 和 trigger 做兼容。证据：`packages/server-go/internal/store/admin_actions.go`、`packages/server-go/internal/migrations/admin_actions.go`、`packages/server-go/internal/migrations/admin_audit_events_rename.go`。

user privacy rail 负责让用户只看到自己的 admin actions。`GET /api/v1/me/admin-actions` 使用当前认证用户 ID 调 `ListAdminActionsForTargetUser`，handler 注释明确忽略 query 中的 `target_user_id` 注入。证据：`packages/server-go/internal/api/admin_endpoints.go`、`packages/server-go/internal/store/admin_actions.go`。

admin audit rail 负责让 admin 查询 audit log。`GET /admin-api/v1/audit-log` 走 `admin.RequireAdmin`，支持 actor/action/target_user_id 以及 since/until/archived/actions filters；admin view sanitizer 会包含 `actor_id`。证据：`packages/server-go/internal/api/admin_endpoints.go`。

impersonation grant 负责由用户自签 24h 授权。`POST /api/v1/me/impersonation-grant` 调 `GrantImpersonation`，固定 24h，重复 active grant 返回 conflict；`DELETE` stamp `revoked_at`；`GET` 返回 active grant 或 null。证据：`packages/server-go/internal/api/admin_endpoints.go`、`packages/server-go/internal/store/admin_actions.go`、`packages/server-go/internal/migrations/admin_impersonation_grants.go`。

system DM 负责用户可见通知，但它是 best-effort。`EmitAdminActionAudit` 先插 audit row，再调用 `EmitAdminActionSystemDM`；system DM 失败不会回滚 audit row。证据：`packages/server-go/internal/store/admin_actions.go`。

## 不负责什么

privacy audit 不保证所有 admin write 都已经接入持久 audit helper。当前代码里 `PATCH users` 的 suspend/change_role/reset_password 和 force channel delete 明确调用 audit helper；API key、permission、invite、create/delete user 等路径未看到同样调用。证据：`packages/server-go/internal/api/admin.go`。

privacy audit 不保证 impersonation grant helper 已接入生产写路径。`RequireImpersonationGrant` 存在，但已核对代码中只看到定义和测试。证据：`packages/server-go/internal/api/admin_grant_check.go`。

privacy audit 不纳入 helper JSONL 本地 audit。admin multi-source audit 当前把 host_bridge source 保留为 0 行 placeholder；helper daemon 的 JSONL audit 不进入 server `audit_events`。证据：`packages/server-go/internal/api/admin_audit_query.go`、`packages/borgee-helper/internal/audit/audit.go`。

## 和其他模块的接口

| Module | Interface | Data | Privacy behavior | Evidence |
| --- | --- | --- | --- | --- |
| admin server rail | `/admin-api/v1/audit-log` | `AdminAction` rows | admin can see actor_id and filters | `packages/server-go/internal/api/admin_endpoints.go` |
| user settings/privacy | `/api/v1/me/admin-actions` | same rows scoped to target user | user sanitizer omits raw `actor_id` | `packages/server-go/internal/api/admin_endpoints.go` |
| impersonation banner | `/api/v1/me/impersonation-grant` | active grant row | user can create/revoke own 24h grant | `packages/server-go/internal/api/admin_endpoints.go` |
| admin writes | `EmitAdminActionAudit` where wired | audit row + best-effort DM | audit row is persisted before DM attempt | `packages/server-go/internal/api/admin.go`, `packages/server-go/internal/store/admin_actions.go` |
| multi-source audit | `/admin-api/v1/audit/multi-source` | server/plugin/agent/host_bridge merged view | host_bridge currently placeholder | `packages/server-go/internal/api/admin_audit_query.go` |

## Flow: audited admin write

```text
admin handler executes write
  -> loads admin from request context
  -> writes business state
  -> Store.EmitAdminActionAudit(actorID, actorLogin, targetUserID, action, metadata, context)
      -> InsertAdminAction(...)
      -> EmitAdminActionSystemDM(...), best-effort
  -> response to admin

affected user later calls /api/v1/me/admin-actions
  -> authMw current user
  -> WHERE target_user_id = current user
```

## Audit Action Coverage

| Action family | Current audit path | Evidence |
| --- | --- | --- |
| `suspend_user` | `PATCH /admin-api/v1/users/{id}` when `disabled=true` calls `EmitAdminActionAudit` | `packages/server-go/internal/api/admin.go` |
| `change_role` | `PATCH /admin-api/v1/users/{id}` when role changed calls `EmitAdminActionAudit` | `packages/server-go/internal/api/admin.go` |
| `reset_password` | `PATCH /admin-api/v1/users/{id}` when password present calls `EmitAdminActionAudit` | `packages/server-go/internal/api/admin.go` |
| `delete_channel` | force delete channel calls `EmitAdminActionAudit` after `ForceDeleteChannel` | `packages/server-go/internal/api/admin.go` |
| `start_impersonation` | user creates grant and handler calls `InsertAdminAction` best-effort | `packages/server-go/internal/api/admin_endpoints.go` |
| plugin lifecycle | migration expands audit action enum; multi-source query classifies `plugin_*` from `audit_events` | `packages/server-go/internal/migrations/plugin_admin_actions_plugin_actions.go`, `packages/server-go/internal/api/admin_audit_query.go` |

## Known risk / unknown

- `InsertAdminAction` is persistent, but `EmitAdminActionSystemDM` is best-effort and silently degrades when no system channel exists or DM write fails. Evidence: `packages/server-go/internal/store/admin_actions.go`。
- `handleCreateMyImpersonateGrant` commits the grant and then best-effort inserts `start_impersonation` audit; audit insert failure is logged but grant remains active. Evidence: `packages/server-go/internal/api/admin_endpoints.go`。
- `RequireImpersonationGrant` is not wired into the audited admin write handlers found in current code, so user grants currently appear observable/manageable but not enforced before admin writes. Evidence: `packages/server-go/internal/api/admin_grant_check.go`, `packages/server-go/internal/api/admin.go`。
- `audit_events` rename preserves `admin_actions` as view/trigger alias, so code still using `TableName() == "admin_actions"` writes through compatibility triggers. Evidence: `packages/server-go/internal/migrations/admin_audit_events_rename.go`, `packages/server-go/internal/store/admin_actions.go`。
