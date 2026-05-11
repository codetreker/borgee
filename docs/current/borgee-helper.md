# borgee-helper — HB-2 host-bridge daemon (Go, `packages/borgee-helper/`)

> `packages/borgee-helper/` is an independent Go module, separate from
> `server-go`, per the HB stack Go spec patch §5.5.
>
> Blueprint anchors: [`host-bridge.md`](../blueprint/current/host-bridge.md)
> §1.1 (internal two-daemon model, long-lived host-bridge without sudo,
> reduced attack surface), §1.5 (release gate including revoke < 100ms),
> and §2 (five trust pillars).
>
> Design split: install-butler remains the short-lived privileged component
> (HB-1 follow-up), while host-bridge is the long-lived non-sudo component
> delivered by HB-2. DELETE does not hard-delete grants; it stamps
> `revoked_at` for forward-only revoke.

## 1. 包路径 + 模块

| 路径 | 角色 |
|---|---|
| `packages/borgee-helper/go.mod` | Go module `borgee-helper` (go 1.25.0; independent module, outside `server-go/go.mod`) |
| `packages/borgee-helper/cmd/borgee-helper/main.go` | Daemon entrypoint for linux/darwin; long-lived IPC server on Unix socket |
| `packages/borgee-helper/cmd/borgee-helper/main_other.go` | Unsupported-platform fallback for non-linux/darwin builds, including Windows |
| `packages/borgee-helper/install/borgee-helper.service` | systemd unit (Linux dedicated OS user `borgee-helper`) |
| `packages/borgee-helper/install/cloud.borgee.host-bridge.plist` | launchd plist (macOS LaunchDaemon) |
| `packages/borgee-helper/install/borgee-helper.sb` | macOS sandbox profile (Seatbelt) |

## 2. internal/ 子包 (7)

| 子包 | 路径 | 角色 |
|---|---|---|
| `acl` | `internal/acl/acl.go` (135) + `_test.go` (118) | Gates action, agent IDs, normalized scope, and grant lookup; grant_type selection stays outside helper ACL |
| `audit` | `internal/audit/audit.go` (48) + `_test.go` (61) | Five-field audit log (`actor / action / target / when / scope`), aligned with HB-1, BPP-4 #499, and HB-3 |
| `fileio` | `internal/fileio/file_actions.go` (119) + `_test.go` (105) | File read/list proxy actions guarded by both sandbox and acl; write-class IPC is rejected by ACL |
| `grants` | `internal/grants/grants.go` (97) + `sqlite_consumer.go` (110) + 2 `_test.go` | Read-only SQLite consumer for HB-3 `host_grants`, using exact helper scope values such as `fs:<path>` / `egress:<host>` with SELECT WHERE `agent_id = ? AND scope = ? AND revoked_at IS NULL`; expiration is checked afterward in Go, and revoked rows are treated as not found |
| `ipc` | `internal/ipc/ipc.go` (181) + `_test.go` (147) | IPC frame server and handshake/routing layer; runtime listener is Unix socket on Linux/macOS |
| `reasons` | `internal/reasons/reasons.go` (27) + `_test.go` (51) | Denial reason enum (`path_outside_grants` / `grant_expired` / `grant_not_found` 等) shared with UI copy |
| `sandbox` | `internal/sandbox/sandbox_{linux,darwin,windows,other}.go` + `_test.go` | Platform sandbox apply path: Linux Landlock, macOS Seatbelt, Windows AppContainer placeholder |

## 3. e2e (3 真测)

| 文件 | 行 | 测试场景 |
|---|---|---|
| `e2e/daemon_startup_test.go` | 171 | Daemon startup, IPC socket bind, and clean shutdown through the signal path |
| `e2e/ipc_handshake_test.go` | 167 | Client IPC connection, handshake, and unauthorized caller rejection |
| `e2e/sandbox_apply_test.go` | 75 | Real sandbox apply coverage through platform build tags |

## 4. 关键产品原则

- **常驻无 sudo** (蓝图 §1.3) — The daemon runs as the dedicated OS user/group `borgee-helper`; install/`borgee-helper.service` keeps the literal `User=borgee-helper Group=borgee-helper`.
- **forward-only revoke** (蓝图 §2 信任五支柱第 3 条 "可逆卸载"; HB-3 #520 server-go 唯一写路径 stamp `revoked_at` forward-only) — `internal/grants/sqlite_consumer.go` uses one SELECT, and the daemon does not INSERT/UPDATE/DELETE `host_grants`. The only write path is server-go `internal/api/host_grants.go`, documented with [`server/api/host-grants.md`](server/api/host-grants.md) §1.
- **撤销 < 100ms** (蓝图 host-bridge §1.5 第 5 行 + HB-4 §1.5 release gate 第 5 行) — Grant state is not cached; each file action reads SQLite directly, covered by e2e `sandbox_apply_test.go`.
- **审计 5 字段同源** (HB-4 §1.5 release gate 第 4 行) — `internal/audit/audit.go` JSON schema fields stay aligned with HB-1 install audit, BPP-4 dead-letter audit, and HB-3 host-IPC audit. Any schema change must update the four-test alignment chain.

## 5. CI Checks

| 检查项 | 期望值 | 含义 |
|---|---|---|
| `grep -rE 'host_grants.*INSERT\|host_grants.*UPDATE\|host_grants.*DELETE' packages/borgee-helper/` | no matches | Daemon does not write; server-go remains the only write path |
| `grep -rE 'admin\|is_admin\|god_mode' packages/borgee-helper/internal/grants/` | no matches | No admin-wide grant path, per 蓝图 §1.3 and ADM-0 §1.3 red line |
| `grep -rE 'cache\|Cache' packages/borgee-helper/internal/grants/sqlite_consumer.go` | no matches | Grant state is not cached; this protects the revoke < 100ms gate |

## 6. 留账 (透明)

- HB-1 install-butler short-lived privileged daemon is tracked separately (acceptance `hb-1.md` item 5).
- Real Windows AppContainer sandbox implementation is tracked separately; `sandbox_windows.go` is currently a placeholder.
- HB-2 e2e cross-platform CI runs on ubuntu/macos/windows.
