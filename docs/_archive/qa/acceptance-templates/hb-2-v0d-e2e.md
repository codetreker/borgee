# Acceptance Template — HB-2-V0D-E2E (HB-2 v0(D) #617 follow-up Playwright e2e + 5 截屏 demo)

> Spec brief `hb-2-v0d-e2e-spec.md` (飞马 v0). Owner: 战马E 实施 / 飞马 review / 烈马 验收 + ⭐ 野马 G4.x 主签字.
>
> **HB-2-V0D-E2E 范围**: HB-2 v0(D) #617 已 land 真 sandbox + 真 IO + SQLite consumer, 但仅 unit cov 没 Playwright daemon 真启 + 5 截屏 (我 acceptance §5.3 锁要 yema G4.x signoff 漏件). 立场承袭 G4.audit closure 烈马交叉核验 P0.1 + 跨四 milestone audit 反转锁链 (RT-3 + REFACTOR-2 + DL-3 + AP-2 v1) e2e 真补立场承袭. **0 production code 改 (仅 e2e + 截屏)**.

## 验收清单

### §1 行为不变量 (HB-2 v0(D) #617 立场承袭)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 1.1 daemon binary 真启 + landlock LSM 真 syscall (Linux runner) | E2E | `cs-test-helper-e2e.sh` 真启 + Playwright `hb-2-v0d.spec.ts::DaemonStartup` PASS |
| 1.2 plugin manifest 真 ed25519 验签 (好 sig + 反 bad sig) | E2E | `_ManifestVerify_GoodSig` + `_ManifestVerify_BadSig` PASS |
| 1.3 SQLite consumer 真接 DL-2 host_grants table + 撤销 <100ms (HB-4 release-gate 第 5 行) | E2E | `_RevocationImmediate` E2E PASS, elapsed <100ms 真验 |

### §2 E2E (5 截屏 demo + Playwright 真测)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 2.1 ⭐ 5 截屏 demo (yema G4.x signoff): daemon 真启 / handshake / landlock / manifest 验签 / SQLite consumer 各 1 PNG | E2E + screenshot | `docs/qa/screenshots/g4.x-hb-2-v0d-{daemon-startup,handshake,landlock,manifest-verify,sqlite-consumer}.png` × 5 ≥3000 bytes 各 |
| 2.2 Playwright e2e `hb-2-v0d.spec.ts` 5 case PASS (一个 case 一个 screenshot) | E2E | `packages/e2e/tests/hb-2-v0d.spec.ts` 5 case PASS (Playwright `--timeout=30000`) |

### §3 closure (REG + cov gate + 跨 milestone 锁)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 3.1 既有全包 unit + e2e + vitest 全绿不破 + post-#621 haystack gate 三轨过 | full test + CI | go-test-cov SUCCESS |
| 3.2 0 production code 改 (仅 e2e + 截屏 + acceptance 翻牌) | git diff | `git diff main -- packages/borgee-helper/` 0 行 + `git diff main -- packages/server-go/` 0 行 |
| 3.3 反 admin god-mode bypass + 反 NATS/Redis dep + 立场承袭 ADM-0 §1.3 红线 | grep | reverse grep test PASS |

## REG-HB2DE2E-* 真翻 🟢

- REG-HB2DE2E-001 🟢 daemon source 真启 + Playwright /health upstream anchor + screenshot
- REG-HB2DE2E-002 🟢 IPC handshake (RoundTrip + RejectsMalformed) + Playwright manifest anonymous → 401 + screenshot
- REG-HB2DE2E-003 🟢 sandbox per-platform (PlatformMatchesGOOS + RealCallSucceeds, skip-with-reason 真带 message) + Playwright + screenshot
- REG-HB2DE2E-004 🟢 ⭐ ed25519 manifest 真验签 (24h validity + base64 + openclaw byte-identical + admin god-mode 反向断 404) + screenshot
- REG-HB2DE2E-005 🟢 ⭐ SQLite consumer 撤销 <100ms (HB-4 §1.5 release-gate 第 5 行 真测) + forward-only revoke + admin god-mode 反向断 + screenshot
- REG-HB2DE2E-006 🟢 0 production .go 改 + 5 screenshot 入 docs/evidence/g4-exit/ + Playwright 6 case PASS

## 退出条件

- §1 (3) + §2 (2) + §3 (3) 全绿 — 一票否决
- daemon 真启 + landlock + ed25519 + SQLite consumer + <100ms 撤销 真测
- ⭐ 5 截屏 ≥3000 bytes 各 + yema G4.x signoff
- 0 production code 改 + post-#621 haystack gate 三轨过
- 登记 REG-HB2DE2E-001..006

## 更新日志

| 日期 | 作者 | 变化 |
|---|---|---|
| 2026-05-01 | 烈马 | v0 — acceptance template 草稿. 立场承袭 HB-2 v0(D) #617 + G4.audit closure P0.1 漏件 + 跨四 milestone audit 反转锁链 (RT-3 + REFACTOR-2 + DL-3 + AP-2 v1) e2e 真补 + ADM-0 §1.3 红线. |
| 2026-05-01 | 战马D | v1 实施 — 真补 post-#622 liema CONDITIONAL LGTM 三抓: ⭐ ed25519 manifest 真验签 (case-4 24h validity + base64 + openclaw byte-identical + 反 admin god-mode 404) + ⭐ SQLite consumer 撤销 <100ms 真测 (case-5 HB-4 §1.5 release-gate 第 5 行 真测 + forward-only revoke + 反 admin god-mode 404) + 5 截屏归档 docs/evidence/g4-exit/ (跟 G4.1 #459 同模式 yema G4.x signoff 锚) + Playwright 6 case PASS 4.8s (5 真 case + client URL anchor) + Go integration 3 file build tag integration 隔离. REG-HB2DE2E-001..006 ⚪→🟢 全翻. 0 production .go 改 (post-#617 + post-#491 + post-#520 byte-identical). |
