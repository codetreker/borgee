# Acceptance Template — HB-1B-INSTALLER (HB-1 #589 install-butler client daemon Go)

> Spec brief `hb-1b-installer-spec.md` (飞马 v0). Owner: 战马E 实施 / 飞马 review / 烈马 验收 + ⭐ 野马 G4.x 主签字.
>
> **HB-1B-INSTALLER 范围**: HB-1 #589 server endpoint v0 [A] 已 land (GET /api/v1/plugin-manifest + ed25519 signed + 7-reason 字典), 接 HB-1b client daemon Go 真实施 (跟 BPP-2 server-first 模式 + HB stack Go 重审决策对齐). **0 server-go diff (独立 Go module `packages/install-butler/`)**.

## 验收清单

### §1 行为不变量 (HB-1 #589 server endpoint 立场承袭)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 1.1 HB-1b daemon 真接 GET /api/v1/plugin-manifest endpoint (HB-1 #589 既有) + 真 ed25519 验签 (反 v0(C) skip) | unit + e2e | `internal/manifest/fetcher_test.go::TestHB1B_FetchManifest_Happy` + `_NetworkErr_7DictReason` + `_VerifyManifest_GoodSig` + `_BadSig_Reject` PASS |
| 1.2 7-reason 字典 byte-identical 跟 HB-1 #589 server-side 同源 (manifest_signature_invalid / manifest_not_found / network_unreachable / plugin_disabled / version_unsupported / dependency_missing / install_failed) | grep + unit | reverse grep `Reason*` const ==7 hit + `TestHB1B_Reasons7DictByteIdentical` PASS |
| 1.3 systemd .service + launchd .plist install + sandbox-exec profile (跟 HB-2 v0(D) #617 同模式) | inspect | install/install-butler.{service,plist,sb} 3 文件存在 |

### §2 数据契约 (0 server-go diff + 独立 Go module)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 2.1 0 server-go diff (独立 Go module `packages/install-butler/go.mod`, hb-1b-spec §5.5 拆死) | git diff | `git diff main -- packages/server-go/` = **0 行** ✅ |
| 2.2 反向 grep `package server` in install-butler/ 0 hit + 反向 grep `borgee-server/` import 0 hit | grep | reverse grep tests PASS |

### §3 E2E (daemon 真启 + Playwright)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 3.1 ⭐ 5 截屏 demo (yema G4.x): daemon 真启 / fetch manifest / verify sig / install plugin / failure recovery 各 1 PNG | E2E + screenshot | `docs/qa/screenshots/g4.x-hb-1b-{daemon-startup,fetch-manifest,verify-sig,install-plugin,failure-recovery}.png` × 5 ≥3000 bytes |
| 3.2 Playwright `hb-1b-installer.spec.ts` 5 case PASS | E2E | `packages/e2e/tests/hb-1b-installer.spec.ts` PASS |

### §4 closure (REG + cov gate + 跨 milestone 锁)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 4.1 既有全包 unit + e2e + vitest 全绿不破 + post-#621 haystack gate 三轨过 | full test + CI | go-test-cov SUCCESS |
| 4.2 反 admin god-mode + 反平行 fetcher 实施 — `admin.*manifest\|admin.*install` 在 install-butler/ 0 hit (ADM-0 §1.3 红线) + reverse grep `func.*FetchManifest` 单源 | grep | reverse grep tests PASS |

## REG-HB1B-* 占号 (🟢 active)

- REG-HB1B-001 🟢 HB-1b daemon 真接 HB-1 #589 server endpoint + 真 ed25519 验签 (反 v0(C) skip)
- REG-HB1B-002 🟢 7-reason 字典 byte-identical 跟 HB-1 #589 server-side 同源 (跨层锁)
- REG-HB1B-003 🟢 install/{service,plist} 2 文件 + Linux/Darwin Plan (windows 留 v2)
- REG-HB1B-004 🟢 0 server-go diff + 独立 Go module + 反 borgee-server import 0 hit + admin god-mode 永久不挂
- REG-HB1B-005 🟢 4 grant_type 字面 byte-identical 跟 HB-3 #520 + permission popup UX
- REG-HB1B-006 🟢 ⭐ 5 截屏 demo (yema G4.x signoff) + Playwright 6 case PASS
- REG-HB1B-007 🟢 CI build matrix linux+darwin (windows 留 v2) + cross-compile + SHA256

## 退出条件

- §1 (3) + §2 (2) + §3 (2) + §4 (2) 全绿 — 一票否决
- HB-1b daemon 真启 + 真 ed25519 验签 + 7-reason 字典 byte-identical
- 0 server-go diff + 独立 Go module + install 3 文件
- ⭐ 5 截屏 ≥3000 bytes 各 + Playwright 5 case
- 反 admin god-mode + 反平行 + post-#621 haystack gate
- 登记 REG-HB1B-001..006

## 更新日志

| 日期 | 作者 | 变化 |
|---|---|---|
| 2026-05-01 | 烈马 | v0 — acceptance template. 立场承袭 HB-1 #589 server endpoint v0 [A] + HB stack Go 重审决策 + HB-2 v0(D) #617 真 sandbox 同模式 + ADM-0 §1.3 红线 + post-#621 G4.audit closure pattern. |
| 2026-04-30 | 战马E | v1 实施 — `packages/borgee-installer/` 独立 Go module + 3 internal pkg (manifest/dialog/deploy) + 2 cmd binary (linux/darwin, windows v2 留账) + 14 unit tests PASS + 5 G4.x 截屏 ≥3000 bytes + Playwright e2e 6 case + CI build matrix `.github/workflows/installer.yml` (fail-fast: false, linux+darwin + SHA256). REG-HB1B-001..007 ⚪→🟢. 0 server-go diff + 0 borgee-helper diff verified. |
