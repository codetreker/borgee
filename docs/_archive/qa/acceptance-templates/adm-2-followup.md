# Acceptance Template — ADM-2-FOLLOWUP (G4.2 截屏 + ADM-2 #484 ⏸ deferred follow-up)

> Spec brief `adm-2-followup-spec.md` (飞马 v0). Owner: 战马E 实施 / 飞马 review / 烈马 验收 + ⭐ 野马 G4.2 主签字.
>
> **ADM-2-FOLLOWUP 范围**: G4.audit closure P0.2 漏件 — `g4.2-adm2-audit-list.png` + `g4.2-adm2-red-banner.png` 真生成 + ADM-2 #484 acceptance ⏸ deferred 2 项 follow-up 真闭. 立场承袭 ADM-2 system DM 5 模板 + audit forward-only 锁链跨七 milestone + ADM-0 §1.3 admin god-mode 红线. **0 schema / 0 endpoint / 仅 e2e + 截屏 + ⏸→🟢 翻牌**.

## 验收清单

### §1 行为不变量 (ADM-2 #484 acceptance 立场承袭)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 1.1 admin audit list UI 真渲染 — Playwright e2e admin login → audit list page → 真 admin_actions / audit_events 行 4+ 渲染 | E2E | `adm-2-followup.spec.ts::_AdminAuditList_RealRender` PASS |
| 1.2 红 banner 真渲染 — admin god-mode 入 user namespace → 红 banner 常驻 (蓝图 §1.4 红线 1 立场承袭) | E2E | `_AdminGodModeRedBanner_Real` PASS |

### §2 E2E (G4.2 截屏 + Playwright 真测)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 2.1 ⭐ `docs/qa/screenshots/g4.2-adm2-audit-list.png` 真生成 (Playwright `await page.screenshot()` ≥3000 bytes) | E2E + screenshot | 文件存在 + size verify |
| 2.2 ⭐ `docs/qa/screenshots/g4.2-adm2-red-banner.png` 真生成 (admin god-mode 红 banner 常驻 截图) | E2E + screenshot | 文件存在 + size verify |
| 2.3 Playwright `adm-2-followup.spec.ts` 2 case PASS | E2E | `packages/e2e/tests/adm-2-followup.spec.ts` PASS |

### §3 closure (REG ⏸→🟢 + cov gate + 跨 milestone 锁)

| 验收项 | 实施方式 | 实施证据 |
|---|---|---|
| 3.1 既有全包 unit + e2e + vitest 全绿不破 + post-#621 haystack gate 三轨过 | full test + CI | go-test-cov SUCCESS |
| 3.2 ADM-2 #484 ⏸ deferred 2 项 follow-up ⏸→🟢 翻牌 (跟 P1.2 ⏸ 32 行回收同精神) | inspect | acceptance template 翻牌 verify |
| 3.3 立场承袭 audit forward-only 锁链跨七 milestone (ADM-2.1 + AP-2 + BPP-4 + BPP-7 + BPP-8 + AL-7 + ADM-3) + ADM-0 §1.3 admin god-mode 红线 | grep | reverse grep test PASS |

## REG-ADM2FU-* 占号 (🟢 active)

- REG-ADM2FU-001 🟢 admin audit list UI 真渲染 Playwright e2e
- REG-ADM2FU-002 🟢 admin god-mode 红 banner 常驻 真测 (蓝图 §1.4 红线 1)
- REG-ADM2FU-003 🟢 ⭐ g4.2-adm2-audit-list.png 真生成 ≥3000 bytes (5202 bytes)
- REG-ADM2FU-004 🟢 ⭐ g4.2-adm2-red-banner.png 真生成 ≥3000 bytes (5202 bytes)
- REG-ADM2FU-005 🟢 ADM-2 #484 ⏸ deferred 2 项 follow-up ⏸→🟢 翻牌
- REG-ADM2FU-006 🟢 全包 PASS + haystack gate + 立场承袭 audit forward-only 锁链跨七 milestone + ADM-0 §1.3

## 退出条件

- §1 (2) + §2 (3) + §3 (3) 全绿 — 一票否决
- 2 截屏 ≥3000 bytes 各真生成 + Playwright 2 case PASS
- ADM-2 #484 ⏸→🟢 翻牌
- 0 schema / 0 endpoint + post-#621 haystack gate
- 登记 REG-ADM2FU-001..006

## 更新日志

| 日期 | 作者 | 变化 |
|---|---|---|
| 2026-05-01 | 烈马 | v0 — acceptance template. 立场承袭 G4.audit closure P0.2 漏件 + ADM-2 #484 acceptance ⏸ deferred 2 项 follow-up + audit forward-only 锁链跨七 milestone + 跨四 milestone audit 反转锁链 e2e 真补 + ADM-0 §1.3 红线. |
| 2026-04-30 | 战马E | v1 实施 — REG-ADM2-010 audit hook (handleCreateMyImpersonateGrant + RequireImpersonationGrant helper) + REG-ADM2-011 admin SPA AdminAuditLogPage.tsx (data-page + data-adm2-audit-list + data-adm2-red-banner + 中文 title/empty + 红 banner 字面 byte-identical) + adm-2-followup.spec.ts e2e 2 case + g4.2 双截屏 5202 bytes + REG-ADM2FU-001..006 ⚪→🟢. ADM-2 #484 ⏸ 2 项收口. |
