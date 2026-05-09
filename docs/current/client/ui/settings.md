# settings — 用户设置页 (ADM-1 起)

代码位置：`/workspace/borgee/packages/client/src/components/Settings/`

## 1. 入口

- Sidebar 底部 ⚙️ 按钮（`data-action="open-settings"`）→ `App.tsx::onSettingsOpen` → `App.tsx::requestMainView('settings')`。
- 跟 agents / invitations / workspaces / remote-nodes 同模式：5 个 sidepane 共用一个 `mainView: MainView` 字符串状态（见 `lib/mainView.ts`），同时只有一个能 active，反堆栈。`requestMainView()` 在切换前跑 `runUnsavedGuards()`，触发 `useUnsavedChangesGuard` 注册的未保存改动提示。
- `closeAllViews()` = `setMainView('channel')`。
- 视图渲染：`<SettingsPage onBack={() => setMainView('channel')} />`。

> **路径分叉**（ADM-0 红线）：用户 SPA `components/Settings/SettingsPage.tsx` 跟 admin SPA `admin/pages/SettingsPage.tsx` 同名共存不混用 — admin SPA 走 `react-router` + `/admin-api/*` cookie；用户 SPA 走 App-level state + `/api/v1/*` cookie。两者 cookie 拆 + endpoint 拆，不会串扰（REG-ADM0-001/002 共享底线）。

## 2. SettingsPage 骨架

`SettingsPage.tsx`：

- header：`<` 返回按钮（`data-page="settings"` + `[data-action="open-settings"]` 反查出处）+ "设置" 标题。
- 导航：`<nav className="settings-tabs">` — v1 仅一个 tab "隐私"（`data-tab="privacy"` + `aria-current="page"`，默认 active）。
- 内容：`<main className="settings-page-content">` 渲染 `<PrivacyPromise/>`。

> **v1 反向约束**：`activeTab: SettingsTab = 'privacy'` 是常量，不挂 `useState` — 后续 tab（账号 / 通知）加入时反开锁定 disabled。注释字面 byte-identical。

## 3. PrivacyPromise 组件 (ADM-1 核心)

`PrivacyPromise.tsx` — 用户隐私承诺页，**默认展开不可折叠**（野马 R3，反 `<details>` 包裹）。

### 3.1 三承诺锁定

`PRIVACY_PROMISES`：3 元组字面 byte-identical 跟 `docs/blueprint/current/admin-model.md §4.1 R3` 同源（脱节 test CI 拦）：

1. **Admin 是平台运维, 不是协作者** — 永不出现在 channel / DM / 团队列表里。
2. **Admin 看不到消息 / 文件 / artifact 内容** — 除非你主动授权 impersonate (24h 时窗, 顶部红色横幅常驻, 可随时撤销)。
3. **Admin 能看的是元数据** (用户名 / channel 名 / 条数 / 登录时间), **看不到正文**。

渲染走 `<ol className="privacy-promise-list">` + `<li className="privacy-promise-item" dangerouslySetInnerHTML={{ __html: renderMarkdown(promise) }} />`（marked + DOMPurify, 跟 system message bubble 同 stack — `**bold**` 加粗显眼）。

### 3.2 八行 ✅/❌ 表格 (三色锁定)

`PRIVACY_TABLE_ROWS`：8 行 `{ category, mark, kind }`，三色锁定 byte-identical：

| kind | className | mark | 颜色 token |
|---|---|---|---|
| `allow` | `privacy-row-allow` | ✅ | gray default（不加色）|
| `deny` | `privacy-row-deny` | ❌ | `#d33` 加粗 |
| `impersonate` | `privacy-row-impersonate` | ✅ (临时) | `#d97706` amber |

8 行（顺序锁定，acceptance §1 "顺序不变"）：

1. 用户名 / 邮箱 → ✅ allow
2. channel 名 / 列表 → ✅ allow
3. 消息条数 / 登录时间 → ✅ allow
4. 消息正文 (channel / DM) → ❌ deny
5. artifact / 文件内容 → ❌ deny
6. 你和 owner-agent 内置 DM → ❌ deny
7. API key 原值 → ❌ deny
8. 授权 impersonate 后 24h 实时入站 → ✅ (临时) impersonate

DOM：`<tr className={'privacy-row-' + row.kind} data-row-kind={row.kind}>` — `data-row-kind` 属性是 e2e 反查出处，跟 `chn-3-3-sidebar-reorder.spec.ts` `data-collapsed` / `data-sortable-handle` 同模式。

### 3.3 脱节 test (doc-as-truth)

`__tests__/PrivacyPromise.drift.test.ts` 走 vite `?raw` import 读 `docs/blueprint/current/admin-model.md`，断言：

- heading anchor `### 4.1 用户侧隐私承诺页文案 (ADM-1 acceptance 硬标尺)` toContain
- 三条 numbered `${i+1}. ${promise}` toContain（每条字面跟 PRIVACY_PROMISES 1:1）

任何一边脱节 CI 红，跟 CM-onboarding `TestWelcomeConstantsMirrorMigrations` 同模式（`docs/current/server-go.md` 已述同精神）。

## 4. 反向约束 (野马 R3 + ADM-0)

源码层强制：

- `<details>` 元素：源码 0 hit（`SettingsPage.test.tsx::querySelectorAll('details')` count==0 + e2e `page.locator('details').count()` count==0）。
- 折叠 / collapse / 展开收起 同义词：`PrivacyPromise.tsx` 字面 0 hit（注释里的反向约束说明不计 — 字面用全角逗号或换写避碰 grep）。
- admin/user 路径分叉：用户 cookie 调 `/admin-api/auth/me` → 401/403（e2e `adm-1-privacy-promise.spec.ts::§4` 反向断言, 跟 REG-ADM0-001/002 共享底线）。

## 5. 测试

- vitest（242/242 全过 +14 新）：
  - `PrivacyPromise.test.tsx` 9 cases — 三承诺字面 + 八行 byte-identical + 三色锁定 + 反向约束
  - `PrivacyPromise.drift.test.ts` — doc-as-truth (vite `?raw` import)
  - `SettingsPage.test.tsx` 4 cases — privacy tab 默认 active + back button + 反 `<details>`
- e2e（`packages/e2e/tests/adm-1-privacy-promise.spec.ts`，3 passed in 5.8s chromium）：
  - §1+§2: 三承诺 6 fragment + 八行 byte-identical + 三色锁定 + G4.1 双截屏（`docs/qa/screenshots/g4.1-adm1-{privacy-promise,privacy-table}.png`）
  - §2 反向约束: details count==0
  - §4 反向断言: admin/user 路径分叉

## 6. 出处

- 蓝图：`docs/blueprint/current/admin-model.md` §4.1 (3 条承诺) + §1.3 (Admin 看 / 看不到 边界)
- spec：`docs/qa/adm-1-implementation-spec.md` (#228) + checklist `docs/qa/adm-1-privacy-promise-checklist.md` (#211)
- acceptance：`docs/qa/acceptance-templates/adm-1.md` (11 验收项，9/11 ✅ + 2/11 ⏸️ ADM-2 后续)
- registry：`docs/qa/regression-registry.md` REG-ADM1-001..006 (6 🟢)
- PR 串：#455 实施 + #459 e2e + 双截屏 + #464 closure

## 7. ADM-2 扩展 (PR #484, Phase 4 第二个 admin-model milestone)

`Settings/` 目录新增 3 个组件 + App 顶部 1 个 banner, 跟 ADM-1 PrivacyPromise 同 tab 三段:

### 7.1 ImpersonateGrantSection.tsx (业主授权 24h)

DOM 出处 `[data-section="impersonate-grant"]` + `[data-action="grant-impersonate"]` / `[data-action="revoke-impersonate"]`. 字面 byte-identical 跟 `docs/qa/adm-2-content-lock.md §3` 同源:
- 标题: "临时授权 admin 影响"
- 描述: "授权后 24h 内, admin 可对你的账号执行 password 重置 / suspend / role 调整等写动作; 24h 后自动失效。"
- 状态行: "当前状态: 未授权" / "当前状态: 已授权剩 23h59m (于 {ts} 起算)"
- 按钮: "授权 (24h, 顶部会显示红色横幅常驻)" / "立即撤销"
- 错误转换: 409 grant_already_active → "已有未过期授权, 请先撤销当前授权或等待自动过期。"

走 `lib/api.ts::getMyImpersonateGrant / createMyImpersonateGrant / revokeMyImpersonateGrant`. 设计 ⑦ + content-lock §3.

### 7.2 AdminActionsList.tsx (影响记录子段)

DOM 出处 `[data-section="admin-actions-history"]` + 每行 `[data-action-row data-action={action}]`. 字面 byte-identical 跟 content-lock §4 同源:
- 标题: "admin 对你的影响记录 (最近 50 条)"
- 空态: "从未被 admin 影响过 — 你的隐私边界完整。"
- 5 action 中文动词字面: 删除了你的 channel / 暂停了你的账号 / 调整了你的账号角色 / 重置了你的登录密码 / 开启了对你账号的 24h impersonate
- 时间格式: `YYYY-MM-DD HH:MM` (跟 server `time.Format("2006-01-02 15:04")` 同源)

走 `lib/api.ts::getMyAdminActions`. 设计 ④ user 只见自己 + 反向约束: 不渲染 raw `actor_id` UUID (server-side sanitizer omits, client 保底也不读).

### 7.3 BannerImpersonate.tsx (顶部红横幅, App-level)

mount 在 `App.tsx` 顶部 (跟 ADM-1 §4.1 R3 第 2 条 "顶部红色横幅常驻可随时撤销" 兑现出处). DOM `[data-banner="impersonate-active"]`. 仅在 `getMyImpersonateGrant()` 返 active grant (revoked_at=null + expires_at>now) 才渲染。

字面 byte-identical 跟 content-lock §2: `support {admin_username} 正在协助你, 剩 {h}h{m}m。 [立即撤销]`

刷新策略 (反向约束: 不挂 ws frame, 设计 ⑥ 跟 CHN-4 同精神):
- 30s 轮询拉 `getMyImpersonateGrant()` (服务端 grant 变化时下刷)
- 1s setInterval 重算 client 端倒计时 (active grant 期间)

`{admin_username}` 走 server `sanitizeImpersonateGrant` 派生 (admin SPA 真使用 grant 时 stamp 字段); fallback "support" 字面跟蓝图 §1.4 row 2 一致。

### 7.4 SettingsPage.tsx 更新

privacy tab 渲染从 `<PrivacyPromise/>` 单段扩为 3 段:
1. `<PrivacyPromise/>` (ADM-1 隐私承诺锁定)
2. `<ImpersonateGrantSection/>` (ADM-2 业主授权)
3. `<AdminActionsList/>` (ADM-2 影响记录)

注释字面: `// ADM-2.2 业主授权 24h impersonate (acceptance §4.2.a; 设计 ⑦ + content-lock §3)` + `// ADM-2.2 影响记录 (acceptance §4.1.c; 设计 ④ 只见自己 + content-lock §4 字面)`

### 7.5 lib/api.ts 扩展 (4 helpers)

`getMyAdminActions` / `getMyImpersonateGrant` / `createMyImpersonateGrant` / `revokeMyImpersonateGrant` — 跟 `getMyLayout / putMyLayout` (CHN-3.2) 同模式. 走 `request<T>()` helper + `BASE` (空字符串, vite proxy 同源). 反向约束: 不引入新 ws subscription (跟设计 ⑥ 同精神).

### 7.6 测试 (3 文件 18 cases PASS)

- `__tests__/BannerImpersonate.test.tsx` 6 cases: no-grant 不渲染 / revoked 不渲染 / active 字面 byte-identical / admin_username unset fallback / 反向 raw UUID 不渲染 (ADM2-NEG-001) / 撤销点击调 revokeGrant
- `__tests__/AdminActionsList.test.tsx` 3 cases: 空态字面 byte-identical / 5 action 中文动词字面 / 反向 actor_id raw 不渲染
- `__tests__/SettingsPage.test.tsx` (4 cases, 改 mock api 防 jsdom fetch unhandled rejection) — privacy tab 默认 active + back button + 反 details + tab 字面

### 7.7 出处

- 蓝图: `docs/blueprint/current/admin-model.md` §1.4 (谁能看到什么 + 三红线) + §3 (impersonation_grants 数据模型片段) + §4.1 R3 (ADM-1 文案兑现出处)
- spec: `docs/implementation/modules/adm-2-spec.md` §2-3
- content lock: `docs/qa/adm-2-content-lock.md` §1+§2+§3+§4
- 原则: `docs/qa/adm-2-stance-checklist.md` (7 条原则 + 10 反向约束)
- acceptance: `docs/qa/acceptance-templates/adm-2.md` §4.1.c+§4.2.a (9/11 ✅ + 2/11 ⏸️ 后续)
- registry: REG-ADM2-008 (BannerImpersonate) + REG-ADM2-009 (AdminActionsList + ImpersonateGrantSection)
- PR: #484 (一个 milestone 一个 PR)
