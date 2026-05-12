# Helper Daemon

`borgee-helper` 是 Linux/macOS 宿主机 daemon，入口限定在 `//go:build linux || darwin`。它把本机能力收敛到 UDS JSON-line IPC、SQLite host grant lookup、ACL、audit 和平台 sandbox。

```text
daemon start
  -> open audit log or stderr fallback
  -> require --grants-db and open SQLite consumer
  -> build ACL gate
  -> apply sandbox with --read-paths
  -> listen on UDS
  -> per connection: handshake {agent_id}
  -> per request: ACL -> IO/decision -> audit -> response
```

## 负责什么

daemon startup 负责绑定运行时依赖。默认 socket 是 `/run/borgee-helper/borgee-helper.sock`，默认 audit log 是 `/var/log/borgee-helper/audit.log.jsonl`，`--grants-db` 是生产必填 SQLite DSN，`--read-paths` 是静态 sandbox read allowlist。证据：`packages/borgee-helper/cmd/borgee-helper/main.go`。

IPC 层负责 wire protocol。第一行必须是 handshake `{agent_id}`；后续每行是 `Request{request_id, action, agent_id, params}`，响应是 `Response{request_id,status,reason,data,audit_log_id}`。当前 handler 识别 `read_file`、`list_files`、`network_egress`。证据：`packages/borgee-helper/internal/ipc/ipc.go`。

ACL 层负责拒绝写类动作、cross-agent mismatch、非法路径和无效 grant。文件 action 要求绝对路径、非空、无 NUL、原始 path 不含 `..` segment，并归一化为 `fs:<clean path>`；network action 归一为 `egress:<target>`。证据：`packages/borgee-helper/internal/acl/acl.go`。

SQLite consumer 负责每次 IPC 查询最新 grant，不做 grant-state cache。查询条件包含 `agent_id = ?`、`scope = ?`、`revoked_at IS NULL`，并区分 not found 与 expired。证据：`packages/borgee-helper/internal/grants/sqlite_consumer.go`。

file IO 层负责只读操作。`ReadFile` 使用 `os.Open`，默认/最大读取 16 MiB，并返回 bytes、truncated、size；`ListFiles` 使用 `os.ReadDir`，最多返回 1000 entries。证据：`packages/borgee-helper/internal/fileio/file_actions.go`。

audit 层负责为每个 IPC 请求写 JSONL，包括 rejected 请求；写失败由调用者忽略，因此不阻断 IPC 响应。事件字段是 actor/action/target/when/scope。证据：`packages/borgee-helper/internal/ipc/ipc.go`、`packages/borgee-helper/internal/audit/audit.go`。

## 不负责什么

helper 不写 `host_grants`。它只通过 read-only SQLite consumer 查询 server DB；grant 创建和撤销由 server REST handler 执行。证据：`packages/borgee-helper/internal/grants/sqlite_consumer.go`、`packages/server-go/internal/api/host_grants.go`。

helper 不执行远程 WebSocket proxy。它不连接 `/ws/remote`，也不处理 remote node token；remote-agent 是独立进程和协议。证据：`packages/server-go/internal/ws/remote.go`、`packages/remote-agent/src/agent.ts`。

helper 不提供写文件 action。ACL allowlist 只有 `list_files`、`read_file`、`network_egress`；write/delete/chmod 等写类 action 不在 allowlist。证据：`packages/borgee-helper/internal/acl/acl.go`。

## 和其他模块的接口

| 模块 | 接口 | 说明 | 证据 |
| --- | --- | --- | --- |
| server host-grants | SQLite DB file | helper 每次 IPC 查 `host_grants`，server 负责写 | `packages/server-go/internal/migrations/host_grants.go`, `packages/borgee-helper/internal/grants/sqlite_consumer.go` |
| local plugin/agent client | UDS JSON line | handshake agent identity, request carries same agent id | `packages/borgee-helper/internal/ipc/ipc.go` |
| OS sandbox | Landlock / sandbox-exec profile | Linux self-apply Landlock; macOS wrapper-only | `packages/borgee-helper/internal/sandbox/sandbox_linux.go`, `packages/borgee-helper/internal/sandbox/sandbox_darwin.go` |
| audit readers | JSONL file | helper writes local audit, not server audit_events | `packages/borgee-helper/internal/audit/audit.go`, `packages/server-go/internal/api/admin_audit_query.go` |

## Sandbox

Linux 使用 Landlock read-file/read-dir 权限；没有 read paths 时创建空 ruleset，形成 deny-by-default。内核不支持 Landlock 时，`ENOSYS` 返回 nil，等价于文档化 no-op fallback。证据：`packages/borgee-helper/internal/sandbox/sandbox_linux.go`。

macOS 的 `Apply` 是 no-op，因为 Go 进程无法自调用私有 `sandbox_init`；代码提供 `GenerateProfile`，由 wrapper 通过 `sandbox-exec -f profile.sb borgee-helper` 启动。profile 默认 deny，允许 read subpaths、audit/tmp 写和 Unix socket 相关操作。证据：`packages/borgee-helper/internal/sandbox/sandbox_darwin.go`。

## Known risk / unknown

- `--read-paths` 注释写明当前是 v0(D) static；运行中新增 grants 不会自动扩大 OS sandbox。ACL 会查到新 grant，但 OS sandbox 仍可能拒绝。证据：`packages/borgee-helper/cmd/borgee-helper/main.go`、`packages/borgee-helper/internal/sandbox/sandbox_linux.go`。
- Linux Landlock `ENOSYS` fallback 是 no-op；老内核上边界退化为 ACL + OS permission。证据：`packages/borgee-helper/internal/sandbox/sandbox_linux.go`。
- macOS sandbox 依赖外部 wrapper 正确启动；daemon 进程内 `Apply` 不验证当前是否真的在 sandbox-exec profile 中。证据：`packages/borgee-helper/internal/sandbox/sandbox_darwin.go`。
