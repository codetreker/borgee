# Host Bridge

host-bridge 是宿主机权限通道，由三部分组成：server 上的 `host_grants` 用户授权记录、宿主机上的 `borgee-helper` daemon、以及 `borgee-installer` 安装器。它与 remote-agent 是不同机制：host-bridge 使用本机 UDS IPC、SQLite grant lookup、ACL、audit 和平台 sandbox；remote-agent 使用 WebSocket 反连和 `--dirs` 进程内 allowlist。

```text
user API /api/v1/host-grants
  |
  | writes host_grants in server DB
  v
SQLite server DB, read-only DSN for helper
  |
  | per IPC lookup, no grant cache
  v
borgee-helper daemon
  |
  | UDS JSON-line IPC + ACL + audit + sandbox
  v
read_file / list_files / network_egress decision path

borgee-installer
  |
  | fetch + ed25519 verify manifest, prompt, sudo deploy
  v
systemd / launchd helper service
```

## 负责什么

server host-grants 负责用户创建、列举、撤销宿主授权。它提供 `POST /api/v1/host-grants`、`GET /api/v1/host-grants`、`DELETE /api/v1/host-grants/{id}`，所有路由都在 user `authMw` 后面，写入 `host_grants` 表，且没有 admin-wide grant path。证据：`packages/server-go/internal/api/host_grants.go`、`packages/server-go/internal/migrations/host_grants.go`、`packages/server-go/internal/server/server.go`。

`borgee-helper` 负责在宿主机上执行受限能力。daemon 读取 `--grants-db` SQLite DSN、监听 Unix Domain Socket、为每个 IPC 请求做 cross-agent ACL、scope lookup、JSONL audit，并在通过后执行只读 file IO 或 network egress allow decision。证据：`packages/borgee-helper/cmd/borgee-helper/main.go`、`packages/borgee-helper/internal/ipc/ipc.go`、`packages/borgee-helper/internal/acl/acl.go`、`packages/borgee-helper/internal/fileio/file_actions.go`。

installer 负责安装 helper artifact。Linux CLI 安装 `.deb` 并部署 systemd，macOS CLI 安装 `.pkg` 并部署 launchd；两者都要求 manifest URL、公钥、artifact 路径，可选 Bearer token 和 dry-run。证据：`packages/borgee-installer/cmd/borgee-installer-linux/main.go`、`packages/borgee-installer/cmd/borgee-installer-darwin/main.go`、`packages/borgee-installer/internal/deploy/deploy.go`。

## 不负责什么

host-bridge 不负责 remote-agent 的 `/ws/remote` 反连，也不读取 remote node `connection_token`。remote node 和 binding 存储在 `remote_nodes` / `remote_bindings`，host grant 存储在 `host_grants`，两套 rail 没有共享 token 或 scope 字典。证据：`packages/server-go/internal/store/models.go`、`packages/server-go/internal/api/remote.go`、`packages/server-go/internal/api/host_grants.go`。

host-bridge 不负责 plugin WebSocket API bridge。plugin rail 使用 `/ws/plugin` 和 API key；helper IPC 使用 UDS JSON line 和 `{agent_id}` handshake。证据：`packages/server-go/internal/ws/plugin.go`、`packages/borgee-helper/internal/ipc/ipc.go`。

server host-grants 不直接执行宿主 IO。server 只写/列出/撤销 grants；helper 是 read-only SQLite consumer 和宿主 IO 执行者。证据：`packages/server-go/internal/api/host_grants.go`、`packages/borgee-helper/internal/grants/sqlite_consumer.go`。

## 和其他模块的接口

| 接口 | 方向 | 身份/权限 | 负责数据 | 证据 |
| --- | --- | --- | --- | --- |
| `/api/v1/host-grants` | user -> server | user cookie/Bearer/dev bypass | create/list/revoke grants | `packages/server-go/internal/api/host_grants.go` |
| SQLite `host_grants` | helper -> server DB file | read-only DSN by convention | `agent_id`, `scope`, `expires_at`, `revoked_at` | `packages/borgee-helper/internal/grants/sqlite_consumer.go` |
| UDS IPC | plugin/agent local client -> helper | handshake `agent_id` plus per-request `agent_id` | `read_file`, `list_files`, `network_egress` | `packages/borgee-helper/internal/ipc/ipc.go` |
| ACL gate | helper internal | cross-agent and grant lookup | `fs:<path>` / `egress:<target>` scope | `packages/borgee-helper/internal/acl/acl.go` |
| audit JSONL | helper -> file/stderr | best-effort writer | actor/action/target/when/scope | `packages/borgee-helper/internal/audit/audit.go` |
| installer manifest | installer -> server/CDN endpoint | optional Bearer + ed25519 pubkey verify | manifest entries and signature | `packages/borgee-installer/internal/manifest/fetcher.go` |

## 子模块

- `helper-daemon.md` 展开 daemon startup、UDS IPC、ACL、SQLite grants、audit、sandbox、read-only IO。
- `host-grants.md` 展开 server REST、schema、ttl/revoke、helper lookup scope 和当前 audit 风险。
- `installer.md` 展开 Linux/macOS CLI、manifest verify、deploy plan、权限确认和当前 shape mismatch。

## Known risk / unknown

- helper 的 OS sandbox read paths 在 daemon 启动时由 `--read-paths` 静态传入；grant revoke 会影响 ACL lookup，但不会动态重写已应用的 Landlock/sandbox profile。证据：`packages/borgee-helper/cmd/borgee-helper/main.go`、`packages/borgee-helper/internal/sandbox/sandbox_linux.go`、`packages/borgee-helper/internal/sandbox/sandbox_darwin.go`。
- server `host_grants` 的 grant/revoke audit 目前是 logger 输出，不是插入 `audit_events` 的持久 audit row；helper 有 JSONL audit，但 admin multi-source audit 的 host_bridge source 当前是 placeholder 0 行。证据：`packages/server-go/internal/api/host_grants.go`、`packages/borgee-helper/internal/audit/audit.go`、`packages/server-go/internal/api/admin_audit_query.go`。
