# Host Grants

`host_grants` 是 host-bridge 的 server-side 授权记录。它与 `user_permissions` 和 remote-agent `RemoteNode` 都是分开的：host grant 描述宿主能力，helper daemon 每次 IPC 读取它做 ACL 决策。

```text
POST /api/v1/host-grants
  -> validate grant_type + ttl_kind + scope
  -> insert host_grants row

helper IPC request
  -> normalize target to fs:<path> or egress:<target>
  -> SELECT host_grants WHERE agent_id=? AND scope=? AND revoked_at IS NULL
  -> reject not_found / expired, allow otherwise

DELETE /api/v1/host-grants/{id}
  -> owner check
  -> stamp revoked_at
  -> next helper IPC lookup sees not_found
```

## 负责什么

REST handler 负责 owner-only grant lifecycle。`POST /api/v1/host-grants` 创建当前用户自己的 grant；`GET /api/v1/host-grants` 只列出当前用户 active grants；`DELETE /api/v1/host-grants/{id}` 先读取 row，再检查 `row.UserID == user.ID`，然后 stamp `revoked_at`。证据：`packages/server-go/internal/api/host_grants.go`。

schema 负责 host grant 字典和状态字段。`host_grants` 有 `id/user_id/agent_id/grant_type/scope/ttl_kind/granted_at/expires_at/revoked_at`，`grant_type` CHECK 是 `install/exec/filesystem/network`，`ttl_kind` CHECK 是 `one_shot/always`。证据：`packages/server-go/internal/migrations/host_grants.go`。

TTL 规则由 server 写入。`one_shot` 设置 `expires_at = now + 1h`，`always` 使用 nil `expires_at`；list 路径过滤 `revoked_at IS NULL AND (expires_at IS NULL OR expires_at > now)`。证据：`packages/server-go/internal/api/host_grants.go`。

helper lookup 负责执行 grant。SQLite consumer 查询 `agent_id` 和 `scope`，过滤 `revoked_at IS NULL`，并在返回前检查 `expires_at <= now` 为 expired。证据：`packages/borgee-helper/internal/grants/sqlite_consumer.go`。

## 不负责什么

host grants 不授权 remote-agent。remote-agent 使用 `remote_nodes.connection_token` 和 `--dirs`；helper 使用 `host_grants` 和 `agent_id/scope`。证据：`packages/server-go/internal/store/models.go`、`packages/remote-agent/src/index.ts`、`packages/borgee-helper/internal/grants/sqlite_consumer.go`。

host grants 不替代 user API capabilities。user API capabilities 在 `user_permissions` 上检查，capability allowlist 来自 `internal/auth/capabilities.go`；host grant 类型是 `install/exec/filesystem/network`，字典不同。证据：`packages/server-go/internal/auth/capabilities.go`、`packages/server-go/internal/auth/abac.go`、`packages/server-go/internal/migrations/host_grants.go`。

host grants 不提供 admin-wide override path。server handler 注释和路由只挂 user `authMw`，没有 `/admin-api` host grant route。证据：`packages/server-go/internal/api/host_grants.go`、`packages/server-go/internal/server/server.go`。

## 和其他模块的接口

| 模块 | host_grants 接口 | 说明 | 证据 |
| --- | --- | --- | --- |
| user auth rail | `authMw` | cookie/Bearer/dev bypass 后写当前用户 grants | `packages/server-go/internal/auth/middleware.go`, `packages/server-go/internal/api/host_grants.go` |
| helper ACL | `scope` | file scope 是 `fs:<clean path>`，egress scope 是 `egress:<target>` | `packages/borgee-helper/internal/acl/acl.go` |
| helper SQLite consumer | read-only DB query | 每次 IPC 查库，revoked row 被过滤 | `packages/borgee-helper/internal/grants/sqlite_consumer.go` |
| audit | logger + helper JSONL | server grant/revoke 写 slog；helper IPC 写 JSONL | `packages/server-go/internal/api/host_grants.go`, `packages/borgee-helper/internal/audit/audit.go` |

## 权限矩阵

| Actor | Can create/list/revoke server grants | Can consume grants | Can bypass owner | Evidence |
| --- | --- | --- | --- | --- |
| current user | yes, own rows only | no direct helper access by API | no | `packages/server-go/internal/api/host_grants.go` |
| admin rail | no route found | no | no | `packages/server-go/internal/server/server.go` |
| helper daemon | no write path | yes, read-only SQLite | no, matches `agent_id + scope` | `packages/borgee-helper/internal/grants/sqlite_consumer.go` |
| remote-agent | no | no | no | `packages/remote-agent/src/*` |

## Known risk / unknown

- server grant/revoke audit 是 structured logger 调用，不是 `audit_events` 持久 row；admin multi-source audit 的 `host_bridge` source 当前保留为 0 行 placeholder。证据：`packages/server-go/internal/api/host_grants.go`、`packages/server-go/internal/api/admin_audit_query.go`。
- helper ACL 的 scope 要求 `fs:<absolute clean path>`，server REST 对 `scope` 只检查非空和 enum，不校验 filesystem scope 格式。证据：`packages/server-go/internal/api/host_grants.go`、`packages/borgee-helper/internal/acl/acl.go`。
- `install/exec` grants 允许 `agent_id` 为空，但 helper SQLite lookup 总是按 `agent_id` 查询；这些 user-level grant 当前如何被 helper consume 未在已核对代码中确定。证据：`packages/server-go/internal/migrations/host_grants.go`、`packages/borgee-helper/internal/grants/sqlite_consumer.go`。
