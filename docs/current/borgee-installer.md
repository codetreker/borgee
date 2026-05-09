# borgee-installer — Borgee Helper installer (Linux .deb + macOS .pkg)

> **Source-of-truth pointer.** Code at `packages/borgee-installer/`
> (independent Go module, separate from `server-go` and `borgee-helper`).
> Spec brief at
> [`docs/implementation/modules/hb-1b-installer-spec.md`](../implementation/modules/hb-1b-installer-spec.md).
> Acceptance at
> [`docs/qa/acceptance-templates/hb-1b-installer.md`](../qa/acceptance-templates/hb-1b-installer.md).

## Why

Blueprint
[`host-bridge.md`](../blueprint/current/host-bridge.md) §1.1 + §1.2 + §1.4
require a real installer that ships a sandboxed `Borgee Helper` daemon
with a single-name UX (one icon, one install package, one log target).
HB-1 (#491) shipped only the server `/api/v1/plugin-manifest` endpoint
and HB-2 v0(D) (#617) shipped only the `borgee-helper` Go daemon binary
— neither covered the **first-install UX**. HB-1B-INSTALLER closes that
gap as a deploy-only tool: it fetches the HB-1 manifest, verifies the
ed25519 signature, prompts the user for `host_grants` permissions, and
deploys the existing `borgee-helper` binary plus its platform service
unit. **No `server-go` or `borgee-helper` bytes change.**

## Stance (蓝图 host-bridge.md §1.1 + §1.2 + §1.4 字面)

- **HB-1 + HB-2 v0(D) byte-identical 不破** — installer 是部署工具,
  不动 server endpoint, 不动 daemon binary, 不动 schema.
- **3 平台拆分, Windows 留 v2** — Linux `.deb` + macOS `.pkg` v1
  实施; Windows `.msi` per blueprint §1.4 字面 "Windows: v2 才支持,
  需重新设计" 留账, `cmd/borgee-installer-windows/` 在 v1 不存在
  (grep 检查 0 hit).
- **首次 install ed25519 manifest verify** — 安装前 fetch HB-1 endpoint
  + 验签 (复用 HB-1 既有 `PluginManifestEntries` const slice + ed25519
  detached signature). verify 失败 → 安装阻塞 (反 silent fallback).
- **permission popup UX 4 grant_type byte-identical 跟 HB-3 #520** —
  install/exec/filesystem/network 4 enum 字面承袭 HB-3 host_grants
  CHECK 约束, 改 = 改 HB-3 schema = 改本文.
- **service unit 复用 borgee-helper byte-identical** — installer 不
  duplicate `.service` / `.plist` 字节; sudo install 命令调既有
  `packages/borgee-helper/install/{borgee-helper.service,
  cloud.borgee.host-bridge.plist}` (HB-2 v0(D) #617 SSOT).
- **0 server-go 改 + 0 borgee-helper 改** — PR diff 仅
  `packages/borgee-installer/` 独立 Go module + GitHub Actions matrix
  workflow + uninstall 脚本.
- **admin god-mode 永久不挂** (ADM-0 §1.3 红线) — installer 走用户
  sudo, grep 检查 `admin.*installer|/admin-api/.*installer` 0 hit.

## Module layout

```
packages/borgee-installer/
├── go.mod                                        # independent module
├── cmd/
│   ├── borgee-installer-linux/main.go            # .deb installer
│   └── borgee-installer-darwin/main.go           # .pkg installer
│   (Windows v2 留账; grep 检查 0 hit)
├── internal/
│   ├── manifest/   # HB-1 endpoint fetch + ed25519 verify
│   ├── dialog/     # 4 grant_type permission popup
│   └── deploy/     # per-platform service unit deployment
└── install/
    └── README.md   # 反向 pointer to HB-2 v0(D) byte-identical units
```

## Per-platform deploy contract

| 平台   | 安装命令                                    | service unit (byte-identical from `packages/borgee-helper/install/`) |
|--------|---------------------------------------------|----------------------------------------------------------------------|
| Linux  | `sudo apt install ./borgee-helper.deb`      | `/lib/systemd/system/borgee-helper.service`                          |
| macOS  | `sudo /usr/sbin/installer -pkg ... -target /` | `/Library/LaunchDaemons/cloud.borgee.host-bridge.plist`              |
| Windows | (留 v2)                                    | (留 v2)                                                              |

OS user/group: Linux `borgee-helper` / macOS `_borgee` (蓝图 §1.4
字面 "独立 OS user/group, 反 root 跑 host-bridge").

## ed25519 verify gate

```
installer → GET /api/v1/plugin-manifest (HB-1 #491 endpoint, 字面不动)
         → ed25519.Verify(pubKey, manifestBytes, sig)
         → fail → block install (反 silent fallback)
         → pass → 弹 permission popup (4 grant_type 列)
         → user confirm → sudo install (apt / installer)
         → 写 host_grants 4 enum 行 (HB-3 #520 schema)
```

## Permission popup contract (HB-3 #520 4 grant_type byte-identical)

| grant_type   | 触发                      | scope JSON 例                  |
|--------------|---------------------------|--------------------------------|
| `install`    | 装/卸 runtime 二进制       | `{}`                           |
| `exec`       | 启动 runtime 进程          | `{}`                           |
| `filesystem` | agent 第一次读写用户目录   | `{"path": "/home/user/code"}`  |
| `network`    | agent 第一次发请求到外部   | `{"host": "api.example.com"}`  |

字面 4 enum 跟 host_grants CHECK 约束 byte-identical (HB-3 #520
migration v=27). 改前端 = 改 schema CHECK = 改 content-lock §1.①.

## Uninstall (信任底线, 蓝图 §1.2 字面 6 项)

`borgee-installer-uninstall` (留 v1 release 后续 PR) 真挂 6 项删除:

1. 二进制 (`/usr/local/bin/borgee-helper` / `/Applications/Borgee Helper.app`)
2. 配置 / 状态 (`~/.config/borgee-helper/`)
3. 安装的 runtime 们
4. Borgee server 注册记录 (DELETE host_grants WHERE user_id=…)
5. OS user/group (`borgee-helper` / `_borgee`)
6. service unit (`borgee-helper.service` / `cloud.borgee.host-bridge.plist`)

字面 6 项 byte-identical 跟蓝图 §1.2 — 改 = 改两处 (蓝图 + 本文).

## CI build matrix

`.github/workflows/installer.yml` (HB1B.2) cross-compile linux/darwin
× `.deb` / `.pkg` artifact + checksum + ed25519 sign installer binary
自身 (跟 HB-1 manifest signing 同精神). Windows row 留 v2.

## Reverse-grep 锚

```
test -f packages/borgee-installer/cmd/borgee-installer-linux/main.go     # exists
test -f packages/borgee-installer/cmd/borgee-installer-darwin/main.go    # exists
test ! -d packages/borgee-installer/cmd/borgee-installer-windows         # absent (留 v2)
git diff origin/main -- packages/server-go/internal/api/hb_1_plugin_manifest.go | wc -l   # 0
git diff origin/main -- packages/borgee-helper/                          | wc -l   # 0
grep -rE 'admin.*installer|/admin-api/.*installer' packages/borgee-installer/  # 0 hit (ADM-0 §1.3)
```

## Tests

- `packages/borgee-installer/internal/manifest/fetcher_test.go` — HB-1
  endpoint fetch + ed25519 verify gate (verify 失败 block install).
- `packages/borgee-installer/internal/dialog/dialog_test.go` — 4
  grant_type popup 字面 byte-identical 跟 HB-3 schema CHECK.
- `packages/borgee-installer/internal/deploy/deploy_test.go` — per-
  platform `LinuxPlan` / `DarwinPlan` 调既有 borgee-helper service unit
  byte-identical (反 duplicate bytes).

Regression rows: `REG-HB1B-001..010` in
[`docs/qa/regression-registry.md`](../qa/regression-registry.md).
