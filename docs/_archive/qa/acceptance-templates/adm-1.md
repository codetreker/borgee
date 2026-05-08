# Acceptance Template — ADM-1 PrivacyPromise (用户隐私承诺页)

> 蓝图: `docs/blueprint/current/admin-model.md` §4.1 (3 条承诺锁) + §1.3 (Admin 看 / 看不到 边界)
> 反查表: `docs/qa/adm-1-privacy-promise-checklist.md` (野马 #211/#228 spec) · 实施 spec: `docs/qa/adm-1-implementation-spec.md`
> 前置: ADM-0.1/0.2/0.3 ✅ · Owner: 战马B (实施) / 烈马 (验收) / 野马 (文案 + 闸 4 demo 签字)

## 验收清单

### 文案 1:1 锁 (野马 §1 三条 + §2 表格, 一字不漏)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| §1 三条承诺字面 1:1 渲染 (admin-model §4.1 R3) | `PrivacyPromise.test.tsx::TestPromiseLiteral` snapshot | 战马B / 野马 | ✅ #455 — `packages/client/src/__tests__/PrivacyPromise.test.tsx` 9 cases PASS (PRIVACY_PROMISES 三元组字面 byte-identical 跟 admin-model.md §4.1 R3 同源, `dangerouslySetInnerHTML + renderMarkdown` 渲染 `**bold**`); e2e #459 `adm-1-privacy-promise.spec.ts::§1` 6 fragment toContainText PASS |
| §2 八行 ✅/❌ 表格字面 1:1 + 顺序不变 | vitest grep 8 行 | 战马B / 野马 | ✅ #455 — PRIVACY_TABLE_ROWS 8 行 (3 allow + 4 deny + 1 impersonate) + mark (✅ × 3 / ❌ × 4 / ✅ (临时) × 1) byte-identical; e2e #459 §2 TABLE_CATEGORIES 8 行 toHaveText 顺序锁 PASS |
| drift test: doc §4.1 ↔ `PrivacyPromise.tsx` 常量 (doc-as-truth, CI 拦) | `PrivacyPromise.drift.test.ts` (CM-onboarding `TestWelcomeConstantsMirrorMigrations` 同模式) | 战马B / 烈马 | ✅ #455 — `packages/client/src/__tests__/PrivacyPromise.drift.test.ts` (vite `?raw` import `admin-model.md`, heading anchor `### 4.1 用户侧隐私承诺页文案 (ADM-1 acceptance 硬标尺)` + 三条 numbered `${i+1}. ${promise}` toContain) PASS |

### DOM / 视觉锁

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| `/settings?tab=privacy` 默认展开不可折叠, 不得 `<details>` 包裹 (野马 R3) | vitest `expect(section).toBeVisible()` + grep `<details` count==0 | 战马B / 烈马 | ✅ #455 + #459 — `SettingsPage.test.tsx::PrivacyPromise section is always visible` `querySelectorAll('details')` count==0 PASS; e2e §2.3 反约束 `page.locator('details').count()` count==0 PASS |
| 三色锁: ❌ 行 `.privacy-row-deny` `#d33` 加粗 / ✅ 行 `.privacy-row-allow` 灰 / impersonate `.privacy-row-impersonate` `#d97706` amber | vitest CSS token 反查 | 战马B / 野马 | ✅ #455 + #459 — className `privacy-row-${row.kind}` + `data-row-kind` 双锚锁; e2e §3 三色锁 `rows.nth(0/3/7)` toHaveClass `/privacy-row-(allow\|deny\|impersonate)/` + toHaveAttribute `data-row-kind` PASS; CSS token 在 `index.css` `.privacy-row-deny { color:#d33; font-weight:600 }` / `.privacy-row-impersonate { color:#d97706 }` 落地 |
| 反向 grep "折叠" / "collapse" / "展开/收起" 在 `PrivacyPromise.tsx` count==0 | CI grep | 烈马 | ✅ #455 — `grep -rEn "折叠\|collapse\|展开收起" packages/client/src/components/Settings/PrivacyPromise.tsx \| grep -v "反 details-element\|反约束"` count==0 PASS (注释里的反约束说明不计) |

### 联签 (ADM-0 §1.4 admin 写动作)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| admin 写动作 (重置 API key 等) → 业主 system DM 含 `admin_name` 非 raw UUID | `internal/api/admin_audit_test.go::TestAdminWriteEmitsNamedDM` (ADM-0 反查 ③ 共享) | 战马B / 烈马 | ✅ #484 ADM-2 — `internal/api/admin.go` 4 admin handler audit hook wired (force_delete_channel / patch disabled / patch password / patch role); `store.EmitAdminActionAudit` composite 走 actor admin.Login 非 raw UUID; `TestADM22_ForceDeleteChannel_WritesAuditAndSystemDM` 反向断言 DM body 含 "admin {login}" 不含 raw actor_id PASS |
| DM body 字面 "你的 API key 被 admin {admin_name} 重置, 请重新生成" | grep snapshot | 战马B / 野马 | ✅ #484 ADM-2 — `store.RenderAdminActionDMBody` reset_password 模板字面 byte-identical 跟 `docs/qa/adm-2-content-lock.md §1` 同源 ("你的登录密码被 admin {admin_username} 于 {ts} 重置, 请重新生成。详情见设置页"隐私 → 影响记录"。"); `TestRenderAdminActionDMBody_ByteIdentical` PASS |

### 闸 4 demo (野马签字必备)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 截屏 1: 设置页 "隐私承诺" 全屏 (3 条 + 8 行表格) → `docs/qa/screenshots/g4.1-adm1-privacy-promise.png` | Playwright | 烈马 / 野马 | ✅ #459 — `adm-1-privacy-promise.spec.ts::§1+§2` 真 4901+5174 不 mock, `page.screenshot({ path: 'docs/qa/screenshots/g4.1-adm1-privacy-promise.png' })` 入 git (89.5 KB, 首屏含 3 承诺 + 表头) |
| 截屏 2: 八行表格全景 (滚到表格视野) → `docs/qa/screenshots/g4.1-adm1-privacy-table.png` | Playwright | 烈马 / 野马 | ✅ #459 — 同 spec 第二张 `scrollIntoViewIfNeeded()` + `page.screenshot()` 入 git (89.5 KB, 八行 ✅/❌/✅(临时) 三色锁全景); 路径锁 `g4.1-adm1-{privacy-promise,privacy-table}.png` 跟 G2.4 / G3.4 命名同模式 (`g4.1-` Phase 4 闸号前缀) |

## 退出条件

- 上表 11 项 ✅ 全闭 (post ADM-2 #484 真实施兑现 deferred 2 项); ADM-0 REG-ADM0-001/002 回归不破; drift CI 拦实测 ✅
- REG-ADM1-001..006 落 6 行 🟢 (drift / promise literal / table byte-identical / 三色锁 / details-element 反约束 / admin-user 路径分叉)
- 野马 `docs/qa/signoffs/adm-1-yema-signoff.md` ✅ SIGNED (三签机制全签: 战马B 实施 ✅ #455 + 烈马 acceptance 双截屏 ✅ #459 + 野马 G4.1 demo ✅)
- G2.4 demo #6 闸 → 5/6 → 6/6 (#257 §2 留账闭, ADM-1 落地后野马补一行)

## 更新日志

| 日期 | 作者 | 变化 |
|---|---|---|
| 2026-04-28 | 烈马 | v0 — Phase 4 ADM-1 11 验收项 (al-1b.md 同模板) |
| 2026-04-29 | 战马D | flip 9/11 ⚪→✅ (#455 PrivacyPromise + SettingsPage 实施 + #459 e2e + G4.1 双截屏); 联签 2 项 (admin 写动作 system DM) ✅ ADM-2 #484 真实施兑现 |
