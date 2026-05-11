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

| Principle | Contract |
|---|---|
| **HB-1 + HB-2 v0(D) stay byte-identical** | The installer is a deployment tool. It does not modify the server endpoint, daemon binary, or schema. |
| **Three-platform split, Windows deferred to v2** | Linux `.deb` and macOS `.pkg` are v1. Windows `.msi` remains deferred per blueprint §1.4 literal "Windows: v2 才支持, 需重新设计"; `cmd/borgee-installer-windows/` is absent in v1 (grep check 0 hit). |
| **First-install ed25519 manifest verify** | Before install, fetch the HB-1 endpoint and verify the signature using the existing HB-1 `PluginManifestEntries` const slice plus ed25519 detached signature. Verify failure blocks install; there is no silent fallback. |
| **Permission popup UX uses the HB-3 #520 grant_type list** | The install/exec/filesystem/network 4-value enum stays byte-identical with the HB-3 host_grants CHECK constraint. Changing the popup enum means changing the HB-3 schema and this doc. |
| **Service units come from borgee-helper byte-identical** | The installer does not duplicate `.service` / `.plist` bytes. The sudo install command uses the existing `packages/borgee-helper/install/{borgee-helper.service, cloud.borgee.host-bridge.plist}` as the HB-2 v0(D) #617 single source. |
| **No server-go or borgee-helper changes** | PR diff is limited to the independent `packages/borgee-installer/` Go module, the GitHub Actions matrix workflow, and the uninstall script. |
| **admin god-mode 永久不挂** | Per ADM-0 §1.3 red line, the installer uses user sudo, and grep check `admin.*installer|/admin-api/.*installer` returns 0 hit. |

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

OS user/group: Linux `borgee-helper` / macOS `_borgee-helper` (blueprint §1.4
literal "独立 OS user/group, 反 root 跑 host-bridge").

## ed25519 verify gate

```
installer → GET /api/v1/plugin-manifest (HB-1 #491 endpoint, 字面不动)
         → ed25519.Verify(pubKey, manifestBytes, sig)
         → fail → block install (no silent fallback)
         → pass → show permission popup (4 grant_type rows)
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

The literal 4-value enum is byte-identical with the host_grants CHECK
constraint (HB-3 #520 migration v=27). Changing the frontend means changing
the schema CHECK and content-lock §1.①.

## Uninstall (信任底线, 蓝图 §1.2 字面 6 项)

`borgee-installer-uninstall` (deferred to a follow-up PR after the v1 release)
tracks these uninstall responsibilities:

1. 二进制 (`/usr/local/bin/borgee-helper` / `/Applications/Borgee Helper.app`)
2. 配置 / 状态 (`~/.config/borgee-helper/`)
3. Installed runtimes
4. Borgee server registration records (stamp `revoked_at` on matching host_grants rows)
5. OS user/group (`borgee-helper` / `_borgee-helper`)
6. service unit (`borgee-helper.service` / `cloud.borgee.host-bridge.plist`)

This list mirrors blueprint §1.2. Any change should update both the blueprint
and this doc.

## CI build matrix

`.github/workflows/installer.yml` (HB1B.2) cross-compiles linux/darwin,
produces `.deb` / `.pkg` artifacts, writes checksums, and ed25519-signs the
installer binary itself, matching the HB-1 manifest signing model. Windows row
remains deferred to v2.

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

| Test | Coverage |
|---|---|
| `packages/borgee-installer/internal/manifest/fetcher_test.go` | HB-1 endpoint fetch plus ed25519 verify gate; verify failure blocks install. |
| `packages/borgee-installer/internal/dialog/dialog_test.go` | 4 grant_type popup values stay byte-identical with the HB-3 schema CHECK. |
| `packages/borgee-installer/internal/deploy/deploy_test.go` | Per-platform `LinuxPlan` / `DarwinPlan` use the existing borgee-helper service unit bytes instead of duplicating them. |

Regression rows: `REG-HB1B-001..010` in
[`docs/qa/regression-registry.md`](../qa/regression-registry.md).
