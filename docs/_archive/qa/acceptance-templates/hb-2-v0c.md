# Acceptance Template — HB-2 v0(C) ✅ Go daemon binary

> HB stack 4 步路径第 3 步 · HB-2 v0(C) host-bridge daemon 真启 (UDS POSIX
> + handshake + ACL + audit + sandbox stub). v0(D) 留 follow-up (真
> landlock/sandbox-exec/Windows Named Pipe + systemd unit). content-lock
> 不需 (CI infra + helper package 无 UI).

## 验收清单

### §1 HB-2 v0(C).2 reasons 8-dict 字典分立

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 1.1 8-dict 字面 byte-identical 跟 hb-2-spec §3.3 | unit | `TestHB2_Reason8DictByteIdentical` PASS |
| 1.2 反向断 HB-1 7-dict + AL-1a 6-dict 字面 0 hit (3 字典分立) | unit | `TestHB2_NoSeventhDictBleed` PASS |

### §2 HB-2 v0(C).3 audit 5-field SSOT

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 2.1 audit.Event 5 字段 byte-identical 跟 HB-1 audit log schema | unit | `TestHB22_AuditEvent5FieldSchemaByteIdentical` PASS |
| 2.2 5-field 单测断字段集 exact (反第 6 字段污染) | unit | `TestHB22_AuditEvent5FieldSetExact` PASS |
| 2.3 When=0 自动填 unix millis (forward-only) | unit | `TestHB22_WhenAutoFillIfZero` PASS |

### §3 HB-2 v0(C).4 grants Consumer interface (HB-3 接入点)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 3.1 MemoryConsumer Lookup happy path | unit | `TestHB23_GrantLookupHappyPath` PASS |
| 3.2 grant_not_found vs grant_expired 字面区分 | unit | `TestHB23_GrantNotFound` + `TestHB23_GrantExpired` PASS |
| 3.3 撤销 < 100ms 生效 (反 grantsCache 反约束 #3) | unit | `TestHB23_RevocationImmediate` PASS |

### §4 HB-2 v0(C).5 ACL gate (cross-agent + path traversal + 写类 reject)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 4.1 read_file happy path | unit | `TestHB24_ReadFileHappyPath` PASS |
| 4.2 path traversal 100% reject (../, NUL byte, relative path, empty) | unit | `TestHB24_PathTraversalRejected` PASS (5 边界) |
| 4.3 cross-agent ACL reject (handshake != request agent_id) | unit | `TestHB24_CrossAgentRejected` + `TestHB24_HandshakeAgentEmpty` PASS |
| 4.4 grant_not_found / grant_expired reason 字面对 | unit | `TestHB24_GrantNotFound` + `TestHB24_GrantExpired` PASS |
| 4.5 写类 100% reject 反向枚举 (write_file/delete_file/chmod/chown/mkdir/rmdir/mv/cp/exec/shell/rename/truncate) 全 reject | unit | `TestHB24_WriteActions100PercentRejected` PASS (12 写类) |
| 4.6 network_egress happy path | unit | `TestHB24_NetworkEgressHappyPath` PASS |

### §5 HB-2 v0(C).6 IPC server (handshake + multiplex + audit on reject)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 5.1 handshake + read_file happy round-trip | unit | `TestHB25_HandshakeAndReadFileHappyPath` PASS |
| 5.2 cross-agent reject 走 ACL gate | unit | `TestHB25_CrossAgentRejected` PASS |
| 5.3 reject 也写 audit (反约束 #5) | unit | `TestHB25_AuditWrittenForReject` PASS |
| 5.4 handshake missing agent_id 拒连 | unit | `TestHB25_HandshakeMissingAgentRejected` PASS |
| 5.5 单连接 request_id 多路复用 | unit | `TestHB25_MultipleRequestsMultiplexed` PASS (r1+r2+r3) |

### §6 HB-2 v0(C).7 sandbox build tag 拆

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 6.1 Platform 字面 build tag 选对 (linux / darwin / other) | unit | `TestHB26_PlatformLabelMatchesBuildTag` PASS |
| 6.2 Apply v0(C) no-op (留 v0(D) 真 landlock/sandbox-exec) | unit | `TestHB26_ApplyNoOpV0C` PASS |

### §7 HB-2 v0(C).8 main daemon (UDS POSIX + signal shutdown)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 7.1 cmd/borgee-helper Linux build green | build | `go build ./cmd/borgee-helper` PASS |
| 7.2 main_other.go (build !linux && !darwin) 不接 main daemon (留 v0(D)) | inspect | `//go:build !linux && !darwin` build tag |
| 7.3 UDS listener default `/run/borgee-helper/borgee-helper.sock` (HB-2.0 #605 IPCEndpointDefault byte-identical) | inspect | `cmd/borgee-helper/main.go` flag default |

### §8 closure

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 8.1 反向 grep 4 锚全 0 hit (`package server` / `grantsCache` / `exec.Cmd` / HB-1 7-dict + AL-1a 6-dict 在 reasons/) | grep | 4 锚 0 hit |
| 8.2 REG-HB2-001..006 6 行 🟢 | regression-registry.md | 6 行 |
| 8.3 PROGRESS [x] 加行 (phase-4.md changelog) | PROGRESS | 1 行 |
| 8.4 acceptance template ✅ closed | 本文件 | 关闭区块加日期 |

## 边界

- HB stack 4 步路径第 3 步 / HB-2.0 #605 prereq / HB-3 host_grants schema (HB-2 v0(C) Consumer interface 是接入点) / HB-1 install-butler audit log schema (HB-2 audit.Event 同 SSOT)
- AL-1a 6-dict + HB-1 7-dict + HB-2 8-dict 三字典分立 / ADM-0 §1.3 + HB-2 spec §1.3 红线 admin god-mode 不挂 host IPC

## 退出条件

- §1+§2+§3+§4+§5+§6+§7+§8 全绿
- 6 包 27 unit PASS (race + non-race)
- ~623 行 production / ~461 行 test
- 反向 grep 锚 4 锚 0 hit

## 关闭

✅ 2026-04-30 战马E — `packages/borgee-helper` 独立 Go module 6 internal 包 (reasons/audit/grants/acl/ipc/sandbox) + cmd/borgee-helper UDS daemon (POSIX); 27 unit PASS race + non-race; 反向 grep 4 锚 0 hit; REG-HB2-001..006 6 🟢. v0(D) 留 follow-up — 真 landlock/sandbox-exec/Windows Named Pipe + systemd unit + 真文件 IO + 真网络出站 outbound proxy + HB-3 SQLite consumer 真接.
