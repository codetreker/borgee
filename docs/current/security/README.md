# Security And Privacy Boundaries

本页横向描述管理员、用户、agent/plugin、remote-agent、host helper、installer 的权限边界。核心原则是 rail 分离：user API、admin API、plugin WS、remote WS、host helper IPC 使用不同凭据和不同授权表，不能把某一条 rail 的凭据当作另一条 rail 的 god-mode。

```text
browser/user API     -> borgee_token cookie / Bearer API key / dev bypass
admin API            -> borgee_admin_session cookie only
plugin WS            -> user API key, agent identity
remote WS            -> remote node connection_token
host helper IPC      -> local UDS handshake agent_id + host_grants SQLite lookup
installer            -> optional Bearer for manifest fetch + ed25519 pubkey verify
```

## 负责什么

security boundary 负责说明现有代码已经实际执行的权限切分。user rail 由 `auth.AuthMiddleware` 解析 `borgee_token` JWT cookie、Bearer API key 或开发模式 bypass；admin rail 由 `admin.RequireAdmin` 解析 `borgee_admin_session` opaque session cookie；plugin rail 和 remote rail 各自在 WebSocket handler 内独立认证。证据：`packages/server-go/internal/auth/middleware.go`、`packages/server-go/internal/admin/middleware.go`、`packages/server-go/internal/ws/plugin.go`、`packages/server-go/internal/ws/remote.go`。

privacy boundary 负责说明哪些信息被显式隐藏。用户 API 和 admin API 的 serializer 不返回 `users.api_key`、`password_hash`、`org_id`；remote node token 在 JSON 中隐藏；admin runtime endpoint 只返回 metadata whitelist，不返回 `last_error_reason`。证据：`packages/server-go/internal/store/models.go`、`packages/server-go/internal/api/auth.go`、`packages/server-go/internal/api/admin.go`、`packages/server-go/internal/api/runtimes.go`。

audit boundary 负责说明持久 audit 和 best-effort audit 的差异。admin actions 写入 `admin_actions`/`audit_events` 路径，helper IPC 写 JSONL，host grant REST 目前只写 logger，admin multi-source audit 对 host_bridge source 当前返回 placeholder 0 行。证据：`packages/server-go/internal/store/admin_actions.go`、`packages/server-go/internal/migrations/admin_audit_events_rename.go`、`packages/borgee-helper/internal/audit/audit.go`、`packages/server-go/internal/api/admin_audit_query.go`。

## 不负责什么

本页不定义未来态权限模型；只记录当前代码。代码里没有出现的 token delivery、admin override、host bridge audit table 等能力写入 known risk / unknown，而不当作现有行为。

本页不把 remote-agent 和 host helper 合并。remote-agent 是 WebSocket + `--dirs` 进程内 allowlist；host helper 是 UDS + `host_grants` + ACL + sandbox。证据：`packages/remote-agent/src/*`、`packages/borgee-helper/internal/*`。

## 和其他模块的接口

| Boundary | Auth material | Authorization source | Can do | Cannot do | Evidence |
| --- | --- | --- | --- | --- | --- |
| User API | `borgee_token` cookie, Bearer API key, dev bypass in development | user row + route-specific owner/capability checks | normal app API, remote node REST, host grant REST | admin API unless separately logged in as admin | `packages/server-go/internal/auth/middleware.go`, `packages/server-go/internal/server/server.go` |
| Admin API | `borgee_admin_session` opaque cookie | `admin_sessions` row joined to `admins` | `/admin-api/v1/*` routes behind `admin.RequireAdmin` | user API auth by cookie/Bearer; host grant bypass | `packages/server-go/internal/admin/auth.go`, `packages/server-go/internal/admin/middleware.go` |
| Capabilities | user row in context | `user_permissions(permission, scope)` | channel/artifact/DM capabilities; non-agent wildcard `(*,*)` in `HasCapability` | admin god-mode; host grants | `packages/server-go/internal/auth/capabilities.go`, `packages/server-go/internal/auth/abac.go` |
| Legacy permission middleware | user row in context | `ListUserPermissions` direct scan | route-level permission middleware | cross-org gate and agent wildcard restriction are not in this helper | `packages/server-go/internal/auth/permissions.go` |
| Plugin WS | Bearer API key or `?apiKey=` | `GetUserByAPIKey`, user not deleted/disabled | register plugin conn; internal `api_request` with same API key | admin rail; remote token rail | `packages/server-go/internal/ws/plugin.go` |
| Remote WS | Bearer remote token or `?token=` | `remote_nodes.connection_token` | register remote node; serve `ls/read/stat` | host grants; plugin API bridge | `packages/server-go/internal/ws/remote.go`, `packages/server-go/internal/store/models.go` |
| Host helper | UDS first-line `{agent_id}` plus request `agent_id` | `host_grants` read-only SQLite query | `read_file/list_files/network_egress` after ACL | write files; create grants; remote WS | `packages/borgee-helper/internal/ipc/ipc.go`, `packages/borgee-helper/internal/acl/acl.go` |
| Installer | optional Bearer for fetch, ed25519 public key for verify | manifest signature and local sudo confirmation | install helper artifact | runtime grant decisions | `packages/borgee-installer/internal/manifest/fetcher.go`, `packages/borgee-installer/internal/deploy/deploy.go` |

## Token And Cookie Boundaries

| Token/cookie | Storage/format | Accepted by | Notes | Evidence |
| --- | --- | --- | --- | --- |
| `borgee_token` | JWT cookie, 7-day max age, HS256 with userId/email claims | user `authMw`, after cookie check in normal middleware | cookie checked before Bearer in `AuthMiddleware`; `AuthenticateFlexible` checks Bearer first | `packages/server-go/internal/api/auth.go`, `packages/server-go/internal/auth/middleware.go` |
| Bearer API key | `users.api_key`, generated as `bgr_` + 32 random bytes hex in admin handler | user `authMw`, plugin WS, manifest endpoint when behind authMw | not returned in user serializers; admin generate endpoint returns only `{ok:true}` | `packages/server-go/internal/api/admin.go`, `packages/server-go/internal/ws/plugin.go` |
| dev bypass | `X-Dev-User-Id` or first member fallback | user `authMw` only when development + `DevAuthBypass` | not in admin middleware | `packages/server-go/internal/auth/middleware.go` |
| `borgee_admin_session` | 32 random bytes hex opaque token in `admin_sessions`, 7-day TTL | admin middleware only | not parsed as admin id; DB lookup required | `packages/server-go/internal/admin/auth.go`, `packages/server-go/internal/admin/middleware.go` |
| remote connection token | 32 random bytes hex in `remote_nodes.connection_token`, JSON hidden | `/ws/remote` only | token delivery endpoint not found | `packages/server-go/internal/store/queries_phase2b.go`, `packages/server-go/internal/ws/remote.go` |

## Privacy Matrix

| Data | User-visible | Admin-visible | Hidden/limited behavior | Evidence |
| --- | --- | --- | --- | --- |
| `org_id` | not serialized by user sanitizer | not serialized by admin user sanitizer | model uses `json:"-"`; serializers hand-build maps | `packages/server-go/internal/store/models.go`, `packages/server-go/internal/api/auth.go`, `packages/server-go/internal/api/admin.go` |
| password hash | never returned | never returned | auth/admin handlers hash/check only | `packages/server-go/internal/api/auth.go`, `packages/server-go/internal/admin/auth.go` |
| API key | not returned | generation returns `{ok:true}` only | stored on user row as `json:"-"` | `packages/server-go/internal/store/models.go`, `packages/server-go/internal/api/admin.go` |
| remote node token | not returned via node JSON | no admin route found | `ConnectionToken json:"-"` | `packages/server-go/internal/store/models.go`, `packages/server-go/internal/api/remote.go` |
| runtime error reason | owner runtime GET returns it | admin runtime list omits it | admin metadata-only whitelist | `packages/server-go/internal/api/runtimes.go` |
| admin audit actor id | user `/api/v1/me/admin-actions` omits actor_id | admin audit includes actor_id | user view scoped to target user | `packages/server-go/internal/api/admin_endpoints.go` |

## Known risk / unknown

- `RequireImpersonationGrant` exists but current production callsites were not found; admin write handlers emit some audit rows but do not appear gated by active impersonation grants in current code. Evidence: `packages/server-go/internal/api/admin_grant_check.go`, `packages/server-go/internal/api/admin.go`.
- `auth.HasCapability` prevents agent wildcard `(*,*)`; legacy `RequirePermission` does not. Routes using legacy middleware may grant agents broader access if an agent has wildcard rows. Evidence: `packages/server-go/internal/auth/abac.go`, `packages/server-go/internal/auth/permissions.go`.
- helper audit JSONL write is best-effort and local; server admin multi-source audit currently treats host_bridge as 0-row placeholder. Evidence: `packages/borgee-helper/internal/ipc/ipc.go`, `packages/server-go/internal/api/admin_audit_query.go`.
- remote node token delivery remains unknown because the token is generated and stored but hidden from JSON responses. Evidence: `packages/server-go/internal/store/queries_phase2b.go`, `packages/server-go/internal/store/models.go`, `packages/server-go/internal/api/remote.go`.
