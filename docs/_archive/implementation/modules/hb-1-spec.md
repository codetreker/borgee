# HB-1 install-butler — spec brief v1 (server-side endpoint, scope [A])

> Owner: 战马A v0 (#491 锚) → 战马D v1 升级 / 飞马 review / 烈马 acceptance.
> Blueprint锚: `host-bridge.md` §1.1+§1.2. Status: **🟢 v1 server endpoint scope**;
> Rust client (install-butler daemon) 留 HB-1b 单 milestone (BPP-2 server先模式).

## 1. 一句话定义

`GET /api/v1/plugin-manifest` 返 ed25519 签名的 plugin binary 清单, 供 Rust
install-butler (HB-1b) 拉后双签校验. v0 schema 0 改 (manifest 走 const slice).

## 2. 范围 / 不在范围

### 在范围 (HB-1 v0 [A])

1. server endpoint Bearer api-key 鉴权 (admin god-mode 不挂)
2. manifest hardcoded const (0 schema)
3. ed25519 detached signature over canonical JSON
4. 7-reason 字典字面锁 byte-identical
5. pwa_manifest_test.go 反向锚转正向 (HB-1 真返 200)
6. 审计日志 5 字段 (HB-4 §1.5 release gate 第 4 行 byte-identical)

### 不在范围

- install-butler Rust daemon (HB-1b 后续 — Rust crate / IPC / GPG verify /
  install/uninstall / audit 留独立 milestone)
- v1 不自动更新, 多源 registry, Windows, shell exec

## 3. 接口边界

### 3.1 server endpoint (HB-1 v0 实施)

```
GET /api/v1/plugin-manifest    Authorization: Bearer <user-api-key>
200 {manifest_version:1, issued_at, expires_at, signature, plugins:[
  {id, version, binary_url, sha256, signature, platforms}]}
```

### 3.2 reason 字典 (跟 AL-1a 6-dict 拆死, 7 字面 const)

`HB1ReasonOK / HB1ReasonManifestSignatureInvalid / HB1ReasonBinarySHA256Mismatch
/ HB1ReasonBinaryGPGInvalid / HB1ReasonManifestFetchFailed / HB1ReasonDiskWriteFailed
/ HB1ReasonUnknownPlugin` byte-identical 跟 战马A v0 spec §3.3.

## 4. 立场 (3 + 3 边界)

- **①** server endpoint owner-only Bearer api-key (admin god-mode 不挂 ADM-0 §1.3 红线)
- **②** manifest data const slice (0 schema, 跟 RT-4 / DM-9 同模式)
- **③** 7-reason 字典字面锁 byte-identical 跟 §3.2 + spec brief v0 #491 同源
- **④** ed25519 detached signature non-empty 单测 verify 通过 (双签 v1 升级到 sequoia/openpgp 留 HB-1b)
- **⑤** AL-1a reason 锁链不漂 — HB-1 7-dict 跟 runtime AL-1a 6-dict 反向 grep 拆死, 锁链停在 HB-6 #19
- **⑥** AST 锁链延伸第 23 处 forbidden 3 token (`pendingPluginManifest / pluginManifestQueue / deadLetterPluginManifest`) 0 hit

## 5. 反约束 grep 锚

- 0 schema: `migrations/hb_1_\d+|ALTER.*plugin` 0 hit
- admin god-mode 不挂: `admin-api/v[0-9]+/.*plugin-manifest` 0 hit
- 7-reason 字面锁: const 字符串 byte-identical 跟 §3.2
- pwa_manifest_test.go 既有反向锚不破 + 新加 `TestHB1_PluginManifest_Returns200`
- AST 锁链延伸第 23 处: 3 forbidden token 0 hit
- AL-1a 拆死: `hb1.*reason|plugin.*reason` 在 internal/agent/reasons/ 0 hit

## 6. 跨 milestone byte-identical 锁

- DL-4 命名拆死: DL-4 PWA manifest `/api/v1/manifest/plugins` 跟 HB-1 plugin
  manifest `/api/v1/plugin-manifest` 不混 (反向锚 `pwa_manifest_test.go::
  TestDL44_PWAManifest_NameNotPluginManifest` 不破)
- HB-1b Rust client 消费 HB-1 v0 endpoint, manifest JSON shape byte-identical
- HB-4 ⭐ release gate 第 3 行 (签名校验失败率 0%) + 第 4 行 (audit log
  schema) byte-identical 跟此 spec
- AL-1a reason 字典分立 (HB-1 install 路径 vs AL-1a runtime 路径)

## 7. 不在范围 (HB-1 v0 [A] vs 留 HB-1b)

- `manifest_signature_invalid` — manifest GPG 签名校验失败
- `binary_sha256_mismatch`     — plugin 二进制 SHA256 不匹配 manifest
- `binary_gpg_invalid`         — plugin 二进制 GPG 签名校验失败
- `manifest_fetch_failed`      — manifest endpoint HTTP 失败
- `disk_write_failed`          — 安装路径写失败 (权限 / 磁盘满)
- `unknown_plugin`             — manifest 不含请求的 plugin_id
- `ok`                         — 成功

## 4. 反约束 (acceptance v0 锚)

1. **fail-closed double-check**: SHA256 单独 PASS 且 GPG 单独 PASS, 二者
   缺一 reject. 单签不接受 (蓝图 §1.2 ① "双签校验")
2. **manifest 不缓存**: 每次 install 重新拉 + 重新校验 expires_at; 反向
   grep `manifestCache\|cachedManifest` 0 hit
3. **daemon 不常驻**: 反向 grep `for.*loop\|select.*Done\|常驻` 0 hit (短命
   生命周期, 任务完即退)
4. **无写命令路径**: 反向 grep `exec.Cmd\|Command(.*shell` 0 hit (除
   binary 自己 chmod +x), v1 不跑用户命令
5. **未签 binary 100% reject**: 合约测试 — 篡改 binary 1 byte → reject;
   伪造 signature → reject. CI lock-in.
6. **卸载完整**: 卸载后反向断言 `installed_path` 不存在 + 注册表清; 信任
   底线 (蓝图 §1.2 ④ "一键完全卸载")
7. **审计日志 schema 锁定**: 每条 audit log 含 `actor / action / target /
   when / scope`, schema 文件 + 校验单测 (HB-4 §1.5 release gate 第 4 行)

## 5. 跨 milestone byte-identical 链

| Milestone   | 关系                                                  | 字面承袭                          |
|-------------|-------------------------------------------------------|-----------------------------------|
| **DL-4**    | server 端 manifest API 提供方; HB-1 消费, 不开新写路径 | manifest JSON schema byte-identical |
| **HB-2**    | 装好 plugin 后, host-bridge 启 plugin 走 BPP 接 server | install-path SSOT 路径 byte-identical |
| **HB-3**    | install/uninstall 都走情境化授权 (一次性 sudo)         | 授权 scope 跟 host_grants 同源     |
| **HB-4** ⭐ | 5 行 release gate 第 3/4 行 (签名校验失败率 / 审计日志) | gate 数字字面 byte-identical       |
| **AL-1a**   | reason 字典分立 (HB-1 7-dict 是 install 路径, AL-1a 6-dict 是 runtime 路径); 不混 | 字典分立反约束 |

## 5.5 Go 三方包锁 (HB stack Go 重审 飞马 #1+#4+#5 必修)

| 用途 | 包名 | 立场 |
|---|---|---|
| GPG 签名 verify | `github.com/ProtonMail/go-crypto/openpgp` | 反 `golang.org/x/crypto/openpgp` (deprecated 2022 + 不维护). 反向 grep `golang.org/x/crypto/openpgp` 0 hit (CI 守门待 HB-1 真实施加) |
| Linux sandbox | `github.com/landlock-lsm/go-landlock` | landlock LSM (kernel ≥5.13); 反 cgroups (cgroups 不限 mmap/exec 路径). fallback: AppArmor profile (kernel <5.13 或非 landlock 内核) |
| Windows IPC pipe | `github.com/Microsoft/go-winio` | Named Pipe SSOT, 反 syscall raw CreateNamedPipe (跨版本 ABI 漂移) |

build tag 拆 (HB-2 sandbox 同模式): `sandbox_linux.go` + `sandbox_darwin.go` + `sandbox_other.go` (Windows + 其他, 跟 install-butler 同模式).

## 6. 实施切入路线 (DL-4 落地后)

1. DL-4 ship `/api/v1/plugin-manifest` endpoint (interface §3.2 byte-
   identical) + GPG signing 服务.
2. HB-1.1 daemon skeleton: Go binary `packages/borgee-helper/install-butler/`
   (独立 Go module, separate go.mod 防 server-go binary bloat; 跟 packages/server-go 同 mono-repo 但模块拆死).
3. HB-1.2 IPC contract impl (UDS server + serde JSON).
4. HB-1.3 manifest fetch + GPG verify + SHA256.
5. HB-1.4 install/uninstall + audit log.
6. HB-1.5 acceptance (反约束 grep + 合约测试 + e2e 7 reason × 3 plugin).

## 6.5 一键安装脚本 (PM #10 必修立场)

**用户路径**: `curl -fsSL borgee.cloud/install.sh | bash` 一键装 borgee-helper daemon (Go binary + systemd/launchd unit + 默认 grants schema 创建).

- **域名锁**: `borgee.cloud` (PM 拍板, **非 borgee.io** — 跟 manifest_url `api.borgee.cloud` 同源, 反向 grep `borgee\.io` count==0 立场守门)
- **install.sh 内容**: bash 脚本下载 binary + verify GPG 签名 + 注册 systemd unit (Linux) / launchd plist (macOS) + 启 daemon
- **反约束**: 反向 grep `borgee\.io|github\.com/codetreker/borgee/releases` 在 install 文档 0 hit (域名单源 borgee.cloud)
- **CI 守门**: release-gate.yml 加 step `install-script-domain-lock` (待 HB-1 真实施 PR 加)

## 7. 退出条件 (HB-1 v0 close)

- §3 IPC contract + DL-4 manifest contract 双向 byte-identical (HB-1
  消费方 + DL-4 提供方 review 闭环)
- §4 反约束 7 项全绿
- HB-4 release gate 第 3 行 (签名校验失败率 0%) 真测 PASS
- HB-4 release gate 第 4 行 (audit log schema) 锁定 + 单测
- 审计日志 e2e (install → log line written; uninstall → log line written)

## 8. 现状 (本 stub v0)

- ✅ spec brief 锁
- ✅ §3 IPC contract + DL-4 manifest contract 字面锁 (drift 防御)
- ✅ §4 反约束 7 项 acceptance v0
- ⏸️ Go binary skeleton — 等 DL-4 落地真启 (HB stack Go 重审拍板, 撤 Rust crate 路径)
- ⏸️ acceptance template `docs/qa/acceptance-templates/hb-1.md` v0 —
  本 brief §4 升级为正式 acceptance template 时同步起
