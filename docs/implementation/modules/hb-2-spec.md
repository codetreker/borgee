# HB-2 host-bridge daemon — spec brief v1 (read-only IPC contract lock)

> Owner: 战马 起 v0 spec → v1 升级 (本 PR) / 飞马 review 安全模型 / 野马 PM 立场 / 烈马 acceptance
> Blueprint锚: `docs/blueprint/current/host-bridge.md` §1.4 + §2 信任五支柱
> Module ref: `docs/architecture/host-bridge.md` §HB-2
> 依赖: **HB-1 install-butler** (装好 plugin runtime 后才有 host-bridge
> 跑) + plugin (BPP) 提需求方
> Status: **✅ spec brief v1 + 4 件套** (本 PR) — Rust crate 真实施 stub
> deferred until HB-1 #589 lands (HB-2.1..HB-2.6 follow-up PR).
> 跟 HB-1 spec brief (#491 81a41fa) 同模式 — 锁住 IPC contract + 反约束防
> drift, 真 Go binary 实施等 HB-1 落 (HB stack Go 重审拍板, 撤 Rust crate 路径).

## v0 → v1 升级笔记 (本 PR)

- v0 = 仅 spec brief stub.
- v1 = spec brief + PM stance checklist (`docs/qa/hb-2-stance-checklist.md`)
  + acceptance template (`docs/qa/acceptance-templates/hb-2.md`) + REG-HB2-001..007 占号 ⚪ pending impl.
- content-lock 不适用 (HB-2 无用户 UI; 仅 daemon IPC contract).
- 实施 (Rust crate) 拆 follow-up PR HB-2.1..HB-2.6, 待 HB-1 #589 ship 后启.

## 1. 一句话定义

host-bridge 是 Borgee Helper 内部**常驻**无 sudo daemon, 跑在独立 OS
user/group, 给 plugin (OpenClaw 等 runtime) 提供受控 host 资源访问 (v1
仅文件读 + 网络出站白名单, **零写命令**). 跟 install-butler (短命 + 需
sudo) 拆开两进程, UI 合一为 "Borgee Helper".

## 2. 范围 / 不在范围

### 在范围 (HB-2 v0)

1. **daemon lifecycle**: 常驻进程 (systemd unit Linux / launchd unit
   macOS), 启停由 install-butler 在装/卸时拉起 (蓝图 §1.1)
2. **IPC server**: plugin (BPP runtime) ↔ host-bridge UDS / Named Pipe
   server. 仅接 "读类" request — list_files / read_file / network_egress.
3. **文件读路径**: 走授权目录 (HB-3 grants 的 scope), 反向断言路径在
   grants 白名单内, 越界 reject (蓝图 §1.4 v1 表第 1 行)
4. **网络出站白名单**: HTTP/HTTPS 出站走 outbound proxy 控制 (蓝图
   §1.4 v1 表第 2 行)
5. **审计日志**: 每次 IPC call (含 reject) 写本地 JSON line, 跟 HB-1
   audit log schema byte-identical (蓝图 §2 信任五支柱第 3 条 + HB-4
   §1.5 release gate 第 4 行)
6. **沙箱守**: Linux cgroups + macOS sandbox-exec profile, 限制 daemon
   只能写 audit log + 临时缓存路径, 不能写用户家目录任何处 (蓝图
   §1.2 ② "进程沙箱")

### 不在范围

- **写命令 / shell exec ❌** (v1 立场承袭, 蓝图 §1.4 第 3 行 "Borgee
  不直接跑命令; 命令走 OpenClaw runtime 自带 shell tool 沙箱内")
- 进程查看 ❌ (v2+, 蓝图 §1.4 v1 表第 4 行)
- 屏幕/键盘 ❌ (不在路线图, 蓝图 §1.4 v1 表第 5 行)
- Windows ❌ (v2 重新设计 sandbox-exec equivalent, 跟 HB-1 同模式)
- HB-3 授权弹窗 + grants 持久化 ❌ (HB-3 范围, HB-2 仅消费 grants)
- BPP 协议本身 ❌ (BPP 模块负责; HB-2 仅提供受控 host 资源)
- v2 命令通道 ❌ (蓝图 §1.4 "v2 路径" 单独立项)

## 3. 接口边界 (interface stub, lock-in 防 drift)

### 3.1 plugin → host-bridge IPC contract

常驻 daemon, 长连接 IPC. JSON line request/response, 单连接多路复用
(request_id correlate).

#### Request (plugin → host-bridge)

```json
{
  "request_id": "<uuid>",
  "action": "list_files" | "read_file" | "network_egress",
  "agent_id": "<uuid>",      // 调用 agent (cross-agent ACL 守)
  "params": {
    // list_files:    {"path": "/abs/path/to/dir"}
    // read_file:     {"path": "/abs/path/to/file", "max_bytes": 1048576}
    // network_egress: {"url": "https://api.example.com/..."}
  }
}
```

#### Response (host-bridge → plugin)

```json
{
  "request_id": "<uuid>",
  "status": "ok" | "rejected" | "failed",
  "reason": "path_outside_grants" |
            "grant_expired" |
            "grant_not_found" |
            "host_exceeds_max_bytes" |
            "egress_domain_not_whitelisted" |
            "cross_agent_reject" |
            "io_failed" |
            "ok",
  "data": <action-specific>,    // when status=ok
  "audit_log_id": "<uuid>"
}
```

### 3.2 host-bridge → HB-3 grants store contract (read-only consumer)

host-bridge **不写 grants** — 只读. HB-3 持有 `host_grants` 表 SSOT;
host-bridge 在每次 IPC call 时按 `(agent_id, scope)` 查询并校验 TTL.

```sql
-- HB-3 owns this schema; HB-2 reads via host-bridge daemon process.
SELECT scope, ttl_until FROM host_grants
WHERE agent_id = ? AND scope = ? AND ttl_until > strftime('%s', 'now') * 1000
LIMIT 1;
```

### 3.3 reason 字典 (跟 HB-1 7-dict + AL-1a 6-dict 字典分立, 不混)

8 字典 (HB-2 host-bridge 专属, install/runtime 路径不混):

- `path_outside_grants`         — 请求 path 不在 HB-3 grants 白名单 scope
- `grant_expired`               — grants 行存在但 TTL 过期 (HB-3 §1.3)
- `grant_not_found`             — `(agent_id, scope)` 无 grant 行
- `host_exceeds_max_bytes`      — read_file 超过 caller `max_bytes` 限
- `egress_domain_not_whitelisted` — 出站 URL domain 不在 outbound 白名单
- `cross_agent_reject`          — agent_id 跟 IPC 来源 plugin 注册 agent_id 不匹配
- `io_failed`                   — host 端 OS 错误 (read failed / network connect failed)
- `ok`                          — 成功

## 4. 反约束 (acceptance v0 锚)

1. **零写命令路径**: 反向 grep `exec\.Cmd\|process::Command\|Command::new\|sh -c` 0 hit (除 daemon 自身的 systemd/launchd unit 启动). 蓝图 §1.4 第 3 行 "Borgee 不直接跑命令" 字面立场守.
2. **路径越界 100% reject**: 任何 path 不在 grants scope 内 → reject. 反向断言: `../` 路径分量 + 符号链接遍历 + Unicode normalization bypass 全 reject (合约测试).
3. **grants 不缓存**: 每次 IPC call 重新查 host_grants 表 (HB-3 SSOT 单源, 跟 HB-1 manifest 不缓存同模式). 反向 grep `grantsCache\|cachedGrants` 0 hit. 撤销 grant → 下次 IPC call < 100ms 内拒绝 (HB-4 §1.5 release gate 第 5 行)
4. **cross-agent ACL**: IPC call 携带 `agent_id`, daemon 校验跟 plugin 连接时握手 agent_id 一致, 不一致 reject (REG-INV-002 fail-closed 同模式, anchor #360 owner-only 立场延伸到 host 层).
5. **审计日志 schema 锁定**: 跟 HB-1 audit log schema byte-identical (`actor / action / target / when / scope`), 改 = 改两处 (HB-1 install + HB-2 host-bridge IPC) 单测锁. HB-4 §1.5 release gate 第 4 行守.
6. **沙箱守**: 反向断言 daemon 跑用户 = `borgee-helper` (独立 OS user), 不是 root, 不是当前 login user. systemd unit + launchd unit 启动配置自动校验 (合约测试).
7. **写类 IPC 100% reject**: HB-4 §1.5 release gate 第 6 行 "任何写类 IPC 调用一律拒绝 (v1 仅读)". 单测覆盖每种写法尝试 — `write_file / delete_file / chmod / chown / mkdir / rmdir / mv / cp` action 全 reject (反向枚举).

## 5. 跨 milestone byte-identical 链

| Milestone   | 关系                                                           | 字面承袭                              |
|-------------|----------------------------------------------------------------|---------------------------------------|
| **HB-1**    | install-butler 装好 plugin runtime 后, 拉起 host-bridge daemon | install-butler audit log schema byte-identical (HB-2 复用) |
| **HB-3**    | host-bridge 消费 host_grants 表 (read-only); HB-3 持 schema + 弹窗写路径 | `host_grants(agent_id, scope, ttl_until, granted_at)` schema 字面 |
| **HB-4** ⭐ | release gate 第 5 行 (撤销 < 100ms) + 第 6 行 (写类 100% reject) | gate 数字 + 反向枚举 byte-identical |
| **AL-1a**   | reason 字典分立 (HB-2 8-dict 是 host IPC 路径; HB-1 7-dict 是 install 路径; AL-1a 6-dict 是 runtime 路径; 不混) | 字典分立反约束 |
| **BPP**     | host-bridge 仅给 BPP runtime (plugin) 用; 不给 server-go 直接调 | 单源 plugin → host-bridge IPC, server 不绕过 |
| **anchor #360 owner-only** | cross-agent ACL 立场承袭到 host 层 — agent 持的 grants 是 owner 授的, 跨 agent 调用 reject | 反向断言同源 |

## 5.5 Go 包结构 + sandbox build tag 拆 (HB stack Go 重审 飞马 #2+#3 必修)

**包结构** (项目布局):
```
packages/borgee-helper/         # 独立 Go module (separate go.mod, 防 server-go binary bloat)
├── go.mod                      # module borgee-helper
├── install-butler/             # HB-1 daemon (短命)
│   └── main.go
└── host-bridge/                # HB-2 daemon (常驻)
    ├── main.go
    ├── sandbox_linux.go        # //go:build linux  — landlock LSM (go-landlock + AppArmor fallback)
    ├── sandbox_darwin.go       # //go:build darwin — sandbox-exec profile
    └── sandbox_other.go        # //go:build !linux && !darwin — Windows / 其他 (no-op + 警告日志, v1 不挂)
```

**反约束**:
- 反向 grep `package server` 在 `packages/borgee-helper/` 0 hit (模块拆死, 不混 server-go binary)
- 反向 grep `cgroups` 在 sandbox_linux.go 0 hit (改用 landlock LSM, cgroups 不限 mmap/exec 路径)
- 反向 grep `syscall.CreateNamedPipe` 0 hit (Windows IPC 走 `github.com/Microsoft/go-winio` SSOT)

**Borgee Helper 命名拆死** (yema #9 必修):
- **daemon binary**: `borgee-helper` (CLI/系统服务命名, OS user `borgee-helper`, systemd unit `borgee-helper.service`)
- **PWA UI**: 不复用 "Borgee Helper" 文案 (避混淆 — PWA = "Borgee" 主品牌 SPA, daemon = OS-level 后台服务)
- **install.sh**: `curl -fsSL borgee.cloud/install.sh | bash` 装 daemon + 注册系统服务 (跟 HB-1 §6.5 一键安装脚本立场承袭)

## 5.6 HB-2.0 prerequisite — CI matrix + 3 IPC unit (HB stack Go 重审 飞马 #7 必修)

**HB-2.0** (HB-2.1..HB-2.6 之前必跑) — CI matrix 跨平台 verify, 真挂 macOS + Windows runner:

```yaml
# .github/workflows/hb-stack-go.yml (HB-2.0 真实施 PR 加)
strategy:
  matrix:
    os: [ubuntu-latest, macos-latest, windows-latest]
runs-on: ${{ matrix.os }}
steps:
  - uses: actions/setup-go@v5
    with: { go-version: '1.23' }
  - run: cd packages/borgee-helper && go test -tags ${{ matrix.os == 'ubuntu-latest' && 'sandbox_linux' || matrix.os == 'macos-latest' && 'sandbox_darwin' || 'sandbox_other' }} ./...
```

**3 IPC unit** (HB-2.0 真实施 PR 加, 跨平台 byte-identical 反断):
- `ipc_uds_test.go` (Linux + macOS UDS server start + JSON envelope round-trip)
- `ipc_winpipe_test.go` (Windows Named Pipe via go-winio, 同 JSON envelope, byte-identical 跟 UDS 路径)
- `ipc_dispatch_test.go` (跨平台 request_id 多路复用 + 11 reason 8-dict 字面 byte-identical)

反约束: HB-2.1..HB-2.6 任一 PR merge 前, HB-2.0 CI matrix 跑过 3 OS × sandbox build tag 全 PASS (release-gate.yml step `hb-stack-go-matrix` 守门, 待 HB-2.0 PR 加).

## 6. 实施切入路线 (HB-1 落地后)

> **HB-1 #589 cross-check (战马D 1 行 verify)**: HB-1 #589 PR 真落 Go binary skeleton 后, HB-2 §3 IPC contract + §5.5 包结构跟 HB-1 真实施 1:1 verify (Go module 共享 audit log JSON schema + sandbox build tag 命名跟 install-butler 同模式). 反向 grep `package install_butler` 在 HB-2 路径 0 hit (模块拆死), 但 audit log JSON 字段顺序 byte-identical (跟 HB-4 §1.5 第 4 行守门).


1. HB-1 ship install-butler Go binary + audit log schema 锁.
2. HB-2.1 daemon skeleton: Go binary `packages/borgee-helper/host-bridge/`
   (独立 Go module, separate go.mod 防 server-go binary bloat; 跟 install-butler 同 mono-repo 但模块拆死, 复用 audit log JSON schema).
3. HB-2.2 IPC server (UDS + serde JSON, request_id 多路复用).
4. HB-2.3 文件读路径 + grants 校验 + 路径 normalization (anti-traversal).
5. HB-2.4 网络出站白名单 + outbound proxy.
6. HB-2.5 沙箱配置 (systemd unit + launchd unit + sandbox-exec profile).
7. HB-2.6 acceptance (反约束 grep + 合约测试 + e2e 8 reason × 3 action).

## 7. 退出条件 (HB-2 v0 close)

- §3 IPC contract + HB-3 grants store contract 双向 byte-identical
- §4 反约束 7 项全绿 (零写命令 + 路径越界 reject + grants 不缓存 +
  cross-agent ACL + audit schema 锁 + 沙箱守 + 写类 IPC 100% reject)
- HB-4 release gate 第 5 行 (撤销 < 100ms) 真测 PASS
- HB-4 release gate 第 6 行 (写类 IPC 100% reject) 反向枚举单测 PASS
- 合约测试: 路径 traversal (../, symlink, Unicode normalization) 全 reject

## 8. 现状 (v1 本 PR)

- ✅ spec brief v1 锁
- ✅ §3 IPC contract + HB-3 grants store contract 字面锁 (drift 防御)
- ✅ §4 反约束 7 项 acceptance v0 → v1 升级 (acceptance template 落地)
- ✅ PM stance checklist `docs/qa/hb-2-stance-checklist.md` (7 立场 + 黑名单 grep)
- ✅ acceptance template `docs/qa/acceptance-templates/hb-2.md` (REG-HB2-001..007 占号 ⚪ pending impl)
- ⏸️ Rust crate skeleton `packages/host-bridge/host-bridge/` — 等 HB-1 #589 ship 后真启 (HB-2.1..HB-2.6 follow-up)
- ⏸️ HB-3 grants schema 真定义 — HB-3 spec brief 待起 (跟 HB-1/HB-2 同模式)

## 9. 跨 spec 反查 (跟 HB-1 + DL-4 spec 锁互校)

- HB-1 §3.3 reason 7-dict (install) ≠ HB-2 §3.3 reason 8-dict (host IPC) —
  字典分立反约束, 不混 (跟 AL-1a 6-dict runtime 也分立)
- HB-1 audit log schema = HB-2 audit log schema (复用同 SSOT —
  `actor / action / target / when / scope`, 改一处改两处)
- DL-4 ↔ HB-1 drift anchor (8a35589) 模式 — 命名相近 endpoint 必须
  cross-PR review 锁; 本 doc 跟 HB-1 spec 是兄弟范围, 已分立, 不需 anchor.
