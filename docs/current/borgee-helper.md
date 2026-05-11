# borgee-helper — HB-2 host-bridge daemon (Go, packages/borgee-helper/)

> 落地: PR #617 (v0(D), 5-01 merged) + #622 (e2e closure) — `packages/borgee-helper/` 独立 Go module (跟 server-go 分离, HB stack Go spec patch §5.5)
> 蓝图锚: [`host-bridge.md`](../blueprint/current/host-bridge.md) §1.1 (内部双 daemon, host-bridge 常驻无 sudo + 攻击面减半) + §1.5 (release 硬指标含撤销 < 100ms) + §2 (信任五支柱)
> 设计沿用: install-butler 短命特权 (HB-1 留后续 PR) ↔ host-bridge 长命无 sudo (HB-2 已落), DELETE 不真删 stamp `revoked_at` forward-only revoke

## 1. 包路径 + 模块

| 路径 | 角色 |
|---|---|
| `packages/borgee-helper/go.mod` | Go module `borgee-helper` (go 1.25.0; 独立 module, 不在 server-go go.mod 内) |
| `packages/borgee-helper/cmd/borgee-helper/main.go` | daemon entrypoint (常驻进程, IPC server 监听 unix socket / windows named pipe) |
| `packages/borgee-helper/cmd/borgee-helper/main_other.go` | 非 darwin/linux/windows 平台 stub |
| `packages/borgee-helper/install/borgee-helper.service` | systemd unit (Linux 独立 OS user `borgee`) |
| `packages/borgee-helper/install/cloud.borgee.host-bridge.plist` | launchd plist (macOS LaunchDaemon) |
| `packages/borgee-helper/install/borgee-helper.sb` | macOS sandbox profile (Seatbelt) |

## 2. internal/ 子包 (7)

| 子包 | 路径 | 角色 |
|---|---|---|
| `acl` | `internal/acl/acl.go` (128) + `_test.go` (118) | 路径白名单 + grant_type 校验 (HB-3 grant_type enum 4-list 同源) |
| `audit` | `internal/audit/audit.go` (48) + `_test.go` (61) | 审计日志 5 字段 (`actor / action / target / when / scope`) byte-identical 跟 HB-1 + BPP-4 #499 + HB-3 同源对齐链 |
| `fileio` | `internal/fileio/file_actions.go` (119) + `_test.go` (105) | 文件读/写代理 actions (受 sandbox + acl 双门把守) |
| `grants` | `internal/grants/grants.go` (94) + `sqlite_consumer.go` (110) + 2 `_test.go` | SQLite consumer (HB-3 `host_grants` 表 read-only, 单 SELECT WHERE `revoked_at IS NULL AND (expires_at IS NULL OR expires_at > now)`) |
| `ipc` | `internal/ipc/ipc.go` (181) + `_test.go` (147) | IPC server (unix socket on Linux/macOS, named pipe on Windows; handshake + frame routing) |
| `reasons` | `internal/reasons/reasons.go` (27) + `_test.go` (51) | 拒绝原因码 enum (`grant_revoked` / `grant_expired` / `path_outside_whitelist` 等; UI 文案锁同源) |
| `sandbox` | `internal/sandbox/sandbox_{linux,darwin,windows,other}.go` + `_test.go` | 三平台 sandbox apply (Linux landlock + macOS Seatbelt + Windows AppContainer 占位) |

## 3. e2e (3 真测)

| 文件 | 行 | 测试场景 |
|---|---|---|
| `e2e/daemon_startup_test.go` | 171 | daemon 启动 + IPC socket bind + clean shutdown (信号路径) |
| `e2e/ipc_handshake_test.go` | 167 | client 连 IPC + handshake + 拒绝非授权 caller |
| `e2e/sandbox_apply_test.go` | 75 | sandbox 真 apply (跨平台 build tag) |

## 4. 关键产品原则字面 (跟蓝图 byte-identical)

- **常驻无 sudo** (蓝图 §1.3) — daemon 跑在独立 OS user/group `borgee`, install/`borgee-helper.service` 字面 `User=borgee Group=borgee`
- **forward-only revoke** (蓝图 §2 信任五支柱第 3 条 "可逆卸载"; HB-3 #520 server-go 唯一写路径 stamp `revoked_at` forward-only) — `internal/grants/sqlite_consumer.go` 单 SELECT, daemon 不 INSERT/UPDATE/DELETE `host_grants` (server-go `internal/api/host_grants.go` 唯一写路径; 跟 [`server/api/host-grants.md`](server/api/host-grants.md) §1 byte-identical)
- **撤销 < 100ms** (蓝图 host-bridge §1.5 第 5 行 + HB-4 §1.5 release gate 第 5 行) — 不缓存 grant 状态, 每次 file action 真查 SQLite (e2e `sandbox_apply_test.go` 验)
- **审计 5 字段同源** (HB-4 §1.5 release gate 第 4 行) — `internal/audit/audit.go` JSON schema 跟 HB-1 install audit + BPP-4 dead-letter audit + HB-3 host-IPC audit 字段 byte-identical, 改 = 改四处单测对齐链

## 5. grep 守门 (CI lint)

| 检查项 | 期望值 | 含义 |
|---|---|---|
| `grep -rE 'host_grants.*INSERT\|host_grants.*UPDATE\|host_grants.*DELETE' packages/borgee-helper/` | 0 hit | daemon 不写 (server-go 唯一写路径) |
| `grep -rE 'admin\|is_admin\|god_mode' packages/borgee-helper/internal/grants/` | 0 hit | admin god-mode 不挂 (蓝图 §1.3 + ADM-0 §1.3 红线) |
| `grep -rE 'cache\|Cache' packages/borgee-helper/internal/grants/sqlite_consumer.go` | 0 hit | 不缓存 grant 状态 (撤销 < 100ms 守门) |

## 6. 留账 (透明)

- HB-1 install-butler 短命特权 daemon 留后续 PR (acceptance `hb-1.md` 5 ⚪ pending)
- Windows AppContainer sandbox 真实施留后续 PR (`sandbox_windows.go` 当前是占位)
- HB-2 e2e 跨平台 CI matrix 已落 (`hb20-ipc-prereq` ubuntu/macos/windows 三平台)
