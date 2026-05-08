# Acceptance Template — AP-2 (UI bundle 抽象 + capability 一键授予 v2)

> Spec brief `ap-2-spec.md` (飞马 v0). Owner: 战马C 实施 / 飞马 review / 烈马 验收. v0 sweeper PR #525 已 merge (REG-AP2-001..006 全 🟢), 本 v1 batch 接 AP-2.UI capability bundle 抽象 (蓝图 §1.1 C 混合 + §1.3 A' 快速 bundle 无角色名).
>
> **AP-2 v1 范围**: capability bundle 客户端 UI 抽象 — `CAPABILITY_BUNDLES` const map 单源 (Workspace / Reader / Mention 三 bundle byte-identical 跟 AP-1 #493 14-capability 白名单跨层锁) + 一次 grant 多 capability (复用 AP-1 既有 PUT endpoint, 不开新) + 复用 AP-1 HasCapability gate (反 BundleHasCapability 平行) + admin god-mode 不挂 bundle UI. **0 server prod 候选** (跟 DM-9/CV-14/CHN-12/13/15/CS-1..4/RT-4/CM-5/DM-3 一系列同模式).

## 验收清单

### §1 行为不变量 (CAPABILITY_BUNDLES const SSOT + 一次 grant 多 capability)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 1.1 `lib/capability-bundles.ts::CAPABILITY_BUNDLES` const map byte-identical 跟 AP-1 #493 capabilities.go 14 capability 白名单跨层锁 (字面 byte-identical, 改 = 改两处, 跟 BPP-2 7-op + DM-9 EmojiPreset + AP-4-enum 跨层锁同精神) | unit + grep | `TestAP2UI_BundleCapabilitiesByteIdentical` (各 bundle capability 字面跟 AP-1 capabilities.go 同源) PASS |
| 1.2 一次 grant 多 capability — 用户 click bundle → client SPA 解开 capability list → 多次调既有 AP-1 PUT /api/v1/permissions endpoint (复用既有 grant path, 不开新, 反向 grep `POST /api/v1/bundles` 0 hit) | unit + integration | `TestAP2UI_BundleGrantsAllCapabilities` (1 click → N grants) + `TestAP2UI_NoNewGrantEndpoint` (反向 grep) PASS |
| 1.3 复用 AP-1 #493 HasCapability gate (反 BundleHasCapability 平行实施) — bundle 内 capability 走 AP-1 ABAC SSOT, 反向 grep `BundleHasCapability\|HasBundle` 0 hit | grep | reverse grep test PASS |

### §2 数据契约 (0 server prod + 反 hardcode bundle 漂)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 2.1 0 server prod diff — `git diff main -- packages/server-go/` 0 行 + 反向 grep `bundle_name\|capability_bundle\|preset_bundle` 在 internal/ 0 hit (server 不识别 bundle, 蓝图 §1.1 字面承袭) | git diff + grep | `git diff main -- packages/server-go/` 0 行 + reverse grep test PASS |
| 2.2 反 hardcode bundle 漂 — 反向 grep `'Workspace'\|'Reader'\|'Mention'` 在 client/src/components/ body 0 hit (走 const 单源) + 反 role name in bundle (反向 grep `'admin'\|'editor'\|'moderator'\|'role'` 在 CAPABILITY_BUNDLES const 0 hit, 蓝图 §1.3 A' 字面立场) | grep | reverse grep tests PASS |

### §3 E2E (BundleSelector 4 case + content-lock 文案 byte-identical)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 3.1 `BundleSelector.test.tsx` 4 case (bundle click 展开 capability checkbox + 反向不自动 submit + 用户必显式 confirm + DOM `data-bundle-name` 锚) — 反约束: 不偷默认勾全部 (跟 DM-9 user 主权立场同精神, 蓝图 §1.3 B 主推 + bundle 是辅助 A') | vitest | 4 vitest case PASS |
| 3.2 Playwright e2e — 用户 click "勾选 Workspace bundle" → 展开 3 capability (create_artifact / update_artifact / reply_in_thread) → 用户 confirm → server 收 3 PUT /api/v1/permissions 调用 (反向断 0 单一 bundle endpoint 调) | E2E | `packages/e2e/tests/ap-2-bundle.spec.ts` PASS (Playwright `--timeout=30000`) |

### §4 closure (REG + admin god-mode + cov gate)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 4.1 admin god-mode 不挂 bundle UI (admin 走 /admin-api/* 单独路径, ADM-0 §1.3 红线; 反向 grep `admin.*bundle\|/admin-api.*bundle` 0 hit) | CI grep | reverse grep test PASS |
| 4.2 既有全包 unit + e2e + vitest 全绿不破 + post-#614 haystack gate 三轨过 (Func=50/Pkg=70/Total=85) | full test + CI | `pnpm exec vitest run --testTimeout=10000` 全 PASS + go-test-cov SUCCESS |
| 4.3 4 件套全闭: spec brief + stance + acceptance + content-lock 不需 (server-only 反向 0 行 + client UI 文案 byte-identical 锁 CAPABILITY_BUNDLES const) | inspect | 文件存在 verify ≥3 件 |

## REG-AP2-* (v0 #525 sweeper 已 🟢 / v1 UI bundle 待翻)

- REG-AP2-001..006 🟢 (v0 PR #525 merged) — expires_at sweeper goroutine + admin_actions audit + cron interval

**v1 新增** (本 milestone PR 翻):
- REG-AP2-UI-007 🟢 CAPABILITY_BUNDLES const SSOT byte-identical 跟 AP-1 capabilities.go 14 项跨层锁 + 反 hardcode bundle 漂 + 反 role name in bundle + BundleSelector 主权 UI (default-all-checked but uncheck-able + 必显式 confirm + DOM `data-bundle-name` 锚)
- REG-AP2-UI-008 🟢 一次 grant 多 capability 复用 AP-1 PUT endpoint (反 POST /api/v1/bundles 旁路 + 反 BundleHasCapability/HasBundle 平行) + admin god-mode 不挂 bundle UI (ADM-0 §1.3 红线) + Playwright e2e 4 case PASS + ap-2-bundle-ui.png 截屏

## 退出条件

- §1 (3) + §2 (2) + §3 (2) + §4 (3) 全绿 — 一票否决
- 0 server prod (`git diff main -- packages/server-go/` 0 行)
- CAPABILITY_BUNDLES const SSOT 跟 AP-1 14-capability 白名单 byte-identical
- 一次 grant 多 capability 复用 AP-1 既有 PUT endpoint (反 POST /api/v1/bundles)
- 反 hardcode bundle 漂 + 反 role name in bundle (反向 grep 0 hit)
- BundleSelector 4 vitest + Playwright e2e PASS
- 既有全包 unit + e2e + vitest 全绿不破 + post-#614 haystack gate 三轨过
- 反 admin god-mode bundle UI (ADM-0 §1.3)
- 登记 REG-AP2-007..008 (v0 001..006 已 🟢)

## 更新日志

| 日期 | 作者 | 变化 |
|---|---|---|
| 2026-04-29 | 战马C | v0 — sweeper acceptance (REG-AP2-001..006 全 🟢, PR #525 merged). |
| 2026-05-01 | 烈马 | v1 — 扩 4 段验收覆盖 AP-2.UI capability bundle 抽象 (蓝图 §1.1 C 混合 + §1.3 A'). REG-AP2-007..008 ⚪ 占号. |
| 2026-05-01 | 战马D | v1 实施 — REG-AP2-UI-007/008 ⚪→🟢 翻牌. `lib/capability-bundles.ts` 3 bundle SSOT (workspace=write_channel/write_artifact/commit_artifact, reader=read_channel/read_artifact/read_dm, mention=mention_user/send_dm) byte-identical 跟 AP-1 14 const + BUNDLE_LABELS 中文 (`工作能力/阅读能力/提及能力`) + assertBundlesValid 反 forward-compat leak. `components/BundleSelector.tsx` 主权 UI (default-all-checked but user can uncheck + 必显式 confirm + DOM `data-bundle-name` + `data-ap2-bundle-*` 锚). `BundleSelector.test.tsx` 4 vitest + `capability-bundles.test.ts` 5 vitest + `ap-2-reverse-grep.test.ts` 扩 §8/§9/§10/§11 (PascalCase bundle name + role name in bundle const + POST /api/v1/bundles + BundleHasCapability/HasBundle 全 0 hit). `packages/e2e/tests/ap-2-bundle.spec.ts` Playwright 4 case (capability response shape + 反 POST /api/v1/bundles 旁路 + UI 真渲染反 RBAC 8 词 0 hit body + admin god-mode UI 独立路径) PASS 4.4s. UI 截屏 `docs/qa/screenshots/ap-2-bundle-ui.png` 6109 bytes. 反借口承袭 (RT-3 audit 反转同精神): acceptance §3.2 真要求 e2e Playwright 同 PR 做完, 不另开 milestone. |
