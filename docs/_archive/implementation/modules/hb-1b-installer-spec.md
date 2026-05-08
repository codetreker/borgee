# HB-1B-INSTALLER spec brief — 真 installer Go binary (≤80 行)

> 飞马 · 2026-05-01 · 第三方 audit P1 真未实现兑现 (HB-1 #491 仅 endpoint, HB-2 v0(D) #617 仅 daemon, installer 未做)
> **关联**: HB-1 #491 plugin manifest endpoint · HB-2 v0(D) #617 Borgee Helper Go daemon · HB-3 #520 host_grants schema · CS-3 #598 PWA install (web 路径) · 蓝图 host-bridge.md / client-shape.md §1.1
> **命名**: HB-1B-INSTALLER = HB-1b 续作真 installer binary (跟 HB-1 #491 spec 留账之 HB-1b 同精神, BPP-2 server-first 模式承袭)

> ⚠️ Installer binary milestone — Go binary cross-compile (linux .deb + macOS .pkg + windows .msi) — **0 server-go 改 / 0 schema 改 / 0 endpoint URL 改**.
> 跟 HB-2 v0(D) #617 daemon 配套: installer 部署 daemon (首次安装) + 信任 manifest signature (HB-1 ed25519 verify) + permission popup UX.

## 0. 关键约束 (3 条立场)

1. **HB-1 + HB-2 v0(D) byte-identical 不破** (HB stack 锁链承袭): HB-1 #491 server endpoint `GET /api/v1/plugin-manifest` 字面不动, HB-2 v0(D) #617 daemon binary cmd/borgee-helper/* 字面不动. installer 是**部署工具**, 不是 daemon/server 改. 反约束: `git diff origin/main -- packages/server-go/internal/api/hb_1_plugin_manifest.go packages/borgee-helper/cmd/ packages/borgee-helper/internal/` 0 hit.

2. **真 installer 跨 3 平台 + ed25519 manifest 验证 + permission popup UX**:
   - **linux .deb + .rpm**: `packages/borgee-installer/cmd/borgee-installer-linux/main.go` 新, 走 `os/exec` sudo apt install + systemd unit 部署 (跟 HB-2 v0(D) install/borgee-helper.service 既有 systemd unit byte-identical 承袭)
   - **macOS .pkg**: `packages/borgee-installer/cmd/borgee-installer-darwin/main.go` 新, 走 sudo /usr/sbin/installer + launchd unit 部署 (跟 borgee-helper.plist byte-identical 承袭)
   - **windows .msi**: `packages/borgee-installer/cmd/borgee-installer-windows/main.go` 新, 走 PowerShell Start-Process -Verb RunAs + Windows Service 部署
   - **ed25519 manifest verify**: 安装前 fetch HB-1 endpoint /api/v1/plugin-manifest + 验签 (走 HB-1 既有 PluginManifestEntries const slice + 既有 ed25519 detached signature)
   - **permission popup UX**: 安装前显 native dialog 列 host_grants 4 grant_type (read/write/exec/network) + 用户 explicit confirm (跟 HB-3 #520 弹窗 4-enum byte-identical 承袭)
   反约束: 反向 grep `os/exec.*sudo|installer\.exec|RunAs` per-platform 各 ≥1 hit + ed25519 verify ≥1 hit per installer.

3. **0 server-go 改 + 0 schema 改 + 独立 Go module + admin god-mode 永久不挂**: PR diff 仅 (a) `packages/borgee-installer/` 独立 Go module (跟 packages/borgee-helper/ 同精神 不污染 server-go) (b) 3 platform installer cmd/ binary (c) GitHub Actions workflow 加 build matrix 出 .deb/.pkg/.msi artifact. 反约束: 0 server-go 改 + 0 borgee-helper 改 + 0 schema column / 0 migration v 号 + admin god-mode 永久不挂 (反向 grep `admin.*installer|/admin-api/.*installer` 0 hit ADM-0 §1.3 红线).

## 1. 拆段实施 (3 段, 一 milestone 一 PR)

| 段 | 文件 | 范围 |
|---|---|---|
| **HB1B.1 installer 三平台 binary** | `packages/borgee-installer/go.mod` 独立 module + 3 platform main.go (cmd/borgee-installer-{linux,darwin,windows}/main.go 各 ~120 行 ed25519 verify + sudo dialog + service unit 部署); shared internal/ (manifest.go ed25519 verify + dialog.go permission popup + deploy.go service unit 部署) ~200 行 | 战马 / 飞马 review |
| **HB1B.2 CI build matrix + artifact** | `.github/workflows/installer.yml` 新 build matrix linux/darwin/windows × .deb/.pkg/.msi cross-compile + GitHub Release artifact 上传 + checksum + ed25519 sign installer binary 自身 (跟 HB-1 #491 manifest signing 同精神) | 战马 / 飞马 review |
| **HB1B.3 closure** | REG-HB1B-001..010 (10 反向 grep + 三平台真 build pass + ed25519 verify ≥1 hit per installer + permission popup UX 真 dialog + 0 server-go 改 + admin god-mode 永久不挂 + post-#621 haystack 三轨过 + 既有 test 全 PASS) + acceptance + content-lock §1 (4 grant_type 字面 byte-identical 跟 HB-3 #520) + 4 件套 spec 第一件 | 战马 / 烈马 |

## 2. 反向 grep 锚 (10 反约束)

```bash
# 1) installer binary 三平台 build pass
for plat in linux darwin windows; do
  test -f packages/borgee-installer/cmd/borgee-installer-$plat/main.go  # exists per-platform
done

# 2) 0 server-go / borgee-helper 改 (post-HB-1 / HB-2 v0(D) byte-identical)
git diff origin/main -- packages/server-go/internal/api/hb_1_plugin_manifest.go | grep -cE '^\+|^-'  # 0 hit
git diff origin/main -- packages/borgee-helper/ | grep -cE '^\+|^-'  # 0 hit

# 3) ed25519 manifest verify 真挂
grep -rE 'ed25519\.Verify|crypto/ed25519' packages/borgee-installer/internal/manifest.go  # ≥1 hit
grep -rE 'PluginManifestEntries|ed25519 verify' packages/borgee-installer/  | wc -l  # ≥3 hit (per platform)

# 4) sudo / RunAs 真挂 (admin elevation)
grep -rE 'os/exec.*sudo|sudo apt install|sudo /usr/sbin/installer|RunAs' packages/borgee-installer/cmd/  | wc -l  # ≥3 hit (per platform)

# 5) permission popup UX 真挂 (4 grant_type 字面跟 HB-3 #520 byte-identical)
grep -rE 'grant_type.*read|grant_type.*write|grant_type.*exec|grant_type.*network' packages/borgee-installer/internal/dialog.go  # ≥4 hit

# 6) systemd / launchd / Windows Service 部署
grep -rE 'borgee-helper\.service|borgee-helper\.plist|New-Service.*BorgeeHelper' packages/borgee-installer/internal/deploy.go  | wc -l  # ≥3 hit

# 7) admin god-mode 永久不挂 (ADM-0 §1.3 红线)
grep -rE 'admin.*installer|/admin-api/.*installer' packages/borgee-installer/ packages/server-go/internal/  # 0 hit

# 8) installer build matrix CI
test -f .github/workflows/installer.yml  # exists
grep -nE '\.deb|\.pkg|\.msi' .github/workflows/installer.yml  # ≥3 hit (3 artifact)

# 9) installer 自签 ed25519 (跟 HB-1 manifest signing 同精神)
grep -rE 'ed25519.*Sign.*installer|signed installer|installer\.sig' packages/borgee-installer/internal/  | wc -l  # ≥1 hit

# 10) post-#621 haystack gate + 既有 test
THRESHOLD_FUNC=50 THRESHOLD_PACKAGE=70 THRESHOLD_TOTAL=85 BUILD_TAGS="sqlite_fts5" go run ./scripts/lib/coverage/  # ALL ≥阈值
cd packages/borgee-installer && go test ./...  # ALL PASS
```

## 3. 不在范围 (留账)

- ❌ **macOS notarization** (Apple sign + Apple Notary Service) — 留 HB-1B v1.x release 前 (需 Apple Developer ID, 有外部依赖)
- ❌ **Windows code-sign** (Authenticode + EV cert) — 留 v1.x release 前 (有外部依赖)
- ❌ **upgrade in-place** (existing daemon → new daemon hot upgrade) — 留 v2+ (本 v1 仅 first install)
- ❌ **uninstall flow** (sudo apt remove / pkgutil --forget) — 留 v1.x follow-up (uninstaller 是 v1 release 前补)
- ❌ **iOS / Android installer** — 永远不做 (蓝图 §1.4 字面 desktop only)
- ❌ **silent install (CI/CD pipeline 用)** — 留 v2+ (本 v1 必 user-explicit confirm)

## 4. 跨 milestone byte-identical 锁

- HB-1 #491 PluginManifestEntries const slice + ed25519 detached signature byte-identical
- HB-2 v0(D) #617 cmd/borgee-helper/ binary + install/borgee-helper.service / .plist + sandbox 三平台 byte-identical
- HB-3 #520 host_grants 4 grant_type CHECK enum byte-identical (permission popup UX)
- ADM-0 §1.3 admin god-mode 永久不挂 (installer / installer 安装记录 不入 admin /admin-api/*)
- BPP-2 server-first 模式 (跟 HB-1 #491 endpoint-first / HB-2 v0(D) daemon 同精神, installer 是 client 部署工具最后一步)
