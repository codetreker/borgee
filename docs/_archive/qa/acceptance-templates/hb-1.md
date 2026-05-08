# HB-1 install-butler — acceptance template v0 (Go binary, HB stack Go 重审)

> Anchor: `docs/implementation/modules/hb-1-spec.md` v0 §3-§4 + §5.5 Go 包锁 + §6.5 一键安装脚本立场
> Mode: docs-only acceptance v0 — 真测在 HB-1.1..HB-1.5 实施 follow-up PR 落 (DL-4 ship 后启).
> 跟 HB-2 acceptance template 同模式 (字面 byte-identical 跟 HB-1 spec drift 防御).

## §1 IPC contract (drift-防御; spec §3)

- **§1.1** Request schema 4 字段锁 (`request_id / action / params / agent_id`) — byte-identical
- **§1.2** Response schema 5 字段锁 (`request_id / status / reason / data / audit_log_id`) — byte-identical
- **§1.3** action 枚举 v1 = 3 项 (`install / uninstall / list_installed`) — 反向枚举写类 0 hit (HB-1 仅装/卸/列, 不直接 exec 业务命令)

## §2 reason 7-dict (spec §3.3, 字典分立)

- **§2.1** 7 reason 字面: `manifest_signature_invalid / manifest_url_not_whitelisted / sha256_mismatch / binary_gpg_invalid / disk_full / install_path_not_writable / ok` — byte-identical
- **§2.2** 跟 HB-2 8-dict 字典分立 — HB-1 7 reason 字面在 `packages/borgee-helper/host-bridge/` 0 hit
- **§2.3** 跟 AL-1a 6-dict 字典分立 — HB-1 7 reason 字面在 `packages/server-go/internal/...runtime audit` 0 hit

## §3 反约束 (spec §4)

- **§3.1** 零写命令 — `exec.Command|os/exec.Cmd|sh -c` 在 `packages/borgee-helper/install-butler/src` 0 hit (除 systemd/launchd unit 启动)
- **§3.2** GPG 双签 fail-closed — manifest signature OR binary signature 任一失败 → reject + 不安装
- **§3.3** SHA256 单源 (反 GPG 单签) — `sha256_mismatch` reason 真挂 unit
- **§3.4** install 路径白名单 — 仅装到 `~/.borgee/plugins/<plugin_id>/` (反向 grep `os.WriteFile.*\\$HOME[^/]\\|os.WriteFile.*Documents` 0 hit)
- **§3.5** audit log 5 字段 byte-identical 跟 HB-2/BPP-4/HB-3 同源
- **§3.6** 沙箱 user — daemon 跑 `borgee-helper` (跟 HB-2 同 OS user)
- **§3.7** 一键安装脚本域名锁 — `borgee.cloud` (反向 grep `borgee\.io` 0 hit, 跟 spec §6.5 同源)

## §4 Go 三方包锁 (spec §5.5, HB stack Go 重审)

- **§4.1** GPG verify: `github.com/ProtonMail/go-crypto/openpgp` (反 deprecated `golang.org/x/crypto/openpgp`)
- **§4.2** Linux sandbox: `github.com/landlock-lsm/go-landlock` + AppArmor fallback (kernel <5.13)
- **§4.3** Windows IPC pipe: `github.com/Microsoft/go-winio` (反 raw syscall)
- **§4.4** build tag 拆: `sandbox_linux.go` + `sandbox_darwin.go` + `sandbox_other.go`
- **§4.5** 包结构: `packages/borgee-helper/install-butler/` 独立 Go module (separate go.mod 防 server-go binary bloat)

## §5 跨 milestone byte-identical 链 (spec §5)

- **§5.1** DL-4 manifest contract byte-identical (HB-1 消费方 + DL-4 提供方)
- **§5.2** HB-2/HB-3/HB-4/BPP-4 audit log schema 5 字段 byte-identical
- **§5.3** HB-4 §1.5 release gate 第 3 行 (签名校验失败率 0%) + 第 4 行 (audit schema 锁定)

## REG 占号 (HB-1 真实施落地后翻 ⚪→🟢)

| Reg ID | Source | Test path | Owner | Status |
|---|---|---|---|---|
| REG-HB1-001 | spec §4 ① + acceptance §3.1 — 零写命令路径 (Go binary) | `install-butler/internal/no_exec_test.go` (反向 grep) | 战马 / 烈马 | ⚪ pending impl |
| REG-HB1-002 | spec §4 ② + acceptance §3.2 — GPG 双签 fail-closed | `install-butler/internal/gpg_double_sig_test.go` | 战马 / 飞马 / 烈马 | ⚪ pending impl |
| REG-HB1-003 | spec §4 ③ + acceptance §3.4 — install 路径白名单 | `install-butler/internal/path_whitelist_test.go` (≥10 case) | 战马 / 飞马 / 烈马 | ⚪ pending impl |
| REG-HB1-004 | acceptance §4.1 — GPG 包锁 ProtonMail/go-crypto/openpgp | `go.mod` grep `ProtonMail/go-crypto` ≥1 + 反向 `golang.org/x/crypto/openpgp` 0 hit | 战马 / 飞马 | ⚪ pending impl |
| REG-HB1-005 | acceptance §4.4 — sandbox build tag 拆 (Linux/Darwin/Other 3 文件) | `find install-butler -name 'sandbox_*.go'` ≥3 + go test 跨平台 PASS | 战马 / 飞马 | ⚪ pending impl |
| REG-HB1-006 | acceptance §3.7 + spec §6.5 — 一键安装脚本域名锁 borgee.cloud | install.sh grep `borgee.cloud` + 反向 `borgee\.io` 0 hit | 战马 / 野马 | ⚪ pending impl |

## 退出条件 (HB-1 docs PR close)

- §1-§5 全锁定字面
- REG-HB1-001..006 占号 ⚪ pending impl

## 退出条件 (HB-1 实施 PR — DL-4 ship 后启)

- §3 反约束 7 项全绿
- HB-4 §1.5 release gate 第 3+4 行 真测 PASS
- 合约测试: GPG 双签 fail-closed + path 白名单 + SHA256 单源
- REG-HB1-001..006 翻 🟢 active

---

## §6 HB-1 v0 [A] server endpoint (此 PR 真实施 — server-side only, daemon 留 HB-1b)

# Acceptance Template — HB-1: install-butler v0 [A] server endpoint

> 蓝图 host-bridge.md §1.1+§1.2 + spec brief v1 (战马D 升级 战马A v0 #491).
> Scope [A] = server-side `GET /api/v1/plugin-manifest` 单做; Rust client
> install-butler daemon 留 HB-1b. Owner: 战马D 实施 / 飞马 review / 烈马 验收.

## 验收清单

### §1 HB-1 v0 [A] server endpoint

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 1.1 server endpoint `GET /api/v1/plugin-manifest` Bearer api-key 鉴权 (no auth → 401); response shape byte-identical 跟 spec §3.1 (manifest_version=1 + issued_at + expires_at + signature + plugins[]) | unit (3 sub-case) | 战马D / 烈马 | `internal/api/hb_1_plugin_manifest_test.go::TestHB1_PluginManifest_Returns200_WithShape` (Bearer + 200 + shape) + `_Unauthorized_NoToken_401` + `_PluginEntriesNonEmpty` |
| 1.2 manifest data const slice (PluginManifestEntries) 单源 + 0 schema | unit + grep | 战马D / 烈马 | `TestHB1_PluginEntriesConstNonEmpty` + `TestHB1_NoSchemaChange` (filepath.Walk migrations/ 反向断 hb_1_* 0 hit) |
| 1.3 ed25519 detached signature non-empty + verify 通过 (HB-1 v0 简化 ed25519, sequoia/openpgp 留 HB-1b) | unit (verify roundtrip) | 战马D / 飞马 / 烈马 | `TestHB1_ManifestSignatureVerify` (signature non-empty + ed25519.Verify 通过 canonical JSON) |
| 1.4 7-reason 字典字面锁 byte-identical (`HB1ReasonOK / ManifestSignatureInvalid / BinarySHA256Mismatch / BinaryGPGInvalid / ManifestFetchFailed / DiskWriteFailed / UnknownPlugin`) | unit | 战马D / 飞马 / 烈马 | `TestHB1_ReasonsByteIdentical` (反向断 7 const string 字面 byte-identical 跟 spec §3.2) |
| 1.5 admin god-mode 不挂 PATCH/POST/PUT/DELETE 在 admin-api/v[0-9]+/.*plugin-manifest 反向 grep 0 hit (ADM-0 §1.3 红线) | filepath.Walk + regex grep | 战马D / 飞马 / 烈马 | `TestHB1_NoAdminPluginManifestPath` (filepath.Walk + regex 反向 0 hit) |

### §2 跨 milestone 锁链

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 2.1 DL-4 命名拆死锚不破 — 既有 `pwa_manifest_test.go::TestDL44_PWAManifest_NameNotPluginManifest` 反向锚不变 + 新加正向 `TestHB1_PluginManifest_Returns200` (HB-1 真挂 + 不 squat DL-4 字面) | unit | 战马D / 飞马 / 烈马 | DL-4 既有反向锚不破 + HB-1 正向 200 |
| 2.2 AL-1a reason 锁链不漂 (停在 HB-6 #19) — HB-1 7-dict 跟 runtime AL-1a 6-dict 反向 grep 拆死 | grep | 飞马 / 烈马 | `_NoAL1aDriftIntoHB1` 反向 grep `hb1.*reason\|plugin.*reason` 在 internal/agent/reasons/ 0 hit |

### §3 HB-1.4 closure + AST 锁链延伸第 23 处

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 3.1 立场 ⑥ AST 锁链延伸第 23 处 forbidden 3 token (`pendingPluginManifest / pluginManifestQueue / deadLetterPluginManifest`) 在 internal/api 0 hit | AST scan | 飞马 / 烈马 | `TestHB1_NoPluginManifestQueue` (filepath.Walk + 反向 grep 3 forbidden 0 hit) |

## 边界

- 战马A v0 #491 spec brief lock 承袭 (§3.1 IPC contract + §3.2 manifest
  contract + §3.3 7-reason 字典) byte-identical
- DL-4 命名拆死: `/api/v1/manifest/plugins` (DL-4 PWA) 跟 `/api/v1/plugin-
  manifest` (HB-1) 拆开 (反向锚 `pwa_manifest_test.go::TestDL44_*`)
- HB-1b: Rust client (install-butler daemon) 消费此 endpoint, manifest JSON
  shape byte-identical (留 HB-1b 后续 milestone)
- HB-4 ⭐ release gate 第 3 行 (签名校验失败率) + 第 4 行 (audit log schema)
  byte-identical 跟此 spec
- ADM-0 §1.3 admin god-mode 不挂
- AL-1a reason 字典分立 (HB-1 install 7-dict vs AL-1a runtime 6-dict)
- AST 锁链延伸第 23 处 (BPP-4..8 + HB-3 v2 + AL-7+AL-8 + HB-5 + CHN-5..14
  + DM-6/7 + HB-6 + RT-4 + HB-1 forbidden tokens 全 0 hit)

## 退出条件

- §1 (5) + §2 (2) + §3 (1) 全绿 — 一票否决
- 0 schema (反向 grep migrations/hb_1_* 0 hit)
- DL-4 命名拆死锚不破
- AL-1a reason 锁链停在 HB-6 #19
- AST 锁链延伸第 23 处
- 7-reason 字典字面 byte-identical 跟 spec
- 登记 REG-HB1-001..006
