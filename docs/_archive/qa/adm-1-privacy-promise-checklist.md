# ADM-1 用户隐私承诺页反查表

> **状态**: v0 (野马, 2026-04-28)
> **目的**: ADM-1 用户设置页"隐私承诺"区块实施反查; 战马B PR 直接吃此表为 acceptance 锚点; 野马 / 飞马 PR review 拿此表反查文案 / 立场漂移; ADM-0 / ADM-1 闸 4 demo 野马签字必备 (admin-model §4.1 R3 锁)。
> **前置**: ADM-0.1 ✅ + ADM-0.2 ✅ + ADM-0.3 (god-mode + impersonation grant 落地后 ADM-1 可启)
> **关联**: `admin-model.md` §0 / §1.3 / §1.4 / §2 不变量 / §4.1 (3 条承诺文案锁), `adm-0-stance-checklist.md` (#205 同根), 14 立场 §1.1 (UI 永不暴露 raw ID)。

---

## 1. 隐私承诺页 — 3 条文案锁 (一字不漏 / 顺序不变)

> 来源: `admin-model.md §4.1` 野马 R3 锁定。ADM-1 实施 PR 必须**字面 1:1** 渲染下面 3 条, 任一字面漂移 → ❌ 不签。drift test 双声明锁同 CM-onboarding `WelcomeMessageBody` 模式。

```
1. **Admin 是平台运维, 不是协作者** — 永不出现在 channel / DM / 团队列表里。
2. **Admin 看不到消息 / 文件 / artifact 内容** — 除非你主动授权 impersonate (24h 时窗, 顶部红色横幅常驻, 可随时撤销)。
3. **Admin 能看的是元数据** (用户名 / channel 名 / 条数 / 登录时间), **看不到正文**。
```

**渲染位置**: `packages/client/src/settings/PrivacyPromise.tsx` (新组件), 嵌在 `SettingsPage` "隐私" tab 顶部, 必须**默认展开**不可折叠 (野马 R3: "用户在第一次进设置页就能读懂")。

---

## 2. Admin 看 ✅ vs 看不到 ❌ 表格

> 同源 admin-model §1.3 边界, 但**业主视角**重述, 用户读得懂。ADM-1 设置页该表格紧跟 §1 3 条承诺, 字面锁:

| 类别 | Admin 看 ✅ / 看不到 ❌ | 业主可见文案锁 |
|------|-----|------|
| 你的用户名 / 邮箱 | ✅ | "Admin 看得到 (运维必需)" |
| 你的 channel 名 / 列表 | ✅ | "Admin 看得到 (元数据)" |
| 你的消息条数 / 登录时间 | ✅ | "Admin 看得到 (统计)" |
| 你的消息正文 (channel / DM) | ❌ | "Admin **看不到**" |
| 你的 artifact / 文件内容 | ❌ | "Admin **看不到**" |
| 你和 owner-agent 的内置 DM | ❌ | "Admin **看不到** (含内置 DM)" |
| API key 原值 | ❌ | "Admin **看不到** (只能重置)" |
| 你授权 impersonate 后 24h 内的实时入站 | ✅ (临时) | "授权后 admin 才能看, 红色横幅会一直提醒你" |

> 颜色锁: ❌ 行**红色加粗** (`color: #d33; font-weight: 600`); ✅ 行普通灰; impersonate 行 amber 警示 (`#d97706`) — 跟 ADM-0 反查表 §1 ② 红色横幅 `#d33` 区分: 横幅是**正在发生**用红, 表格是**理论分类**用灰/红/黄三色。

---

## 3. 截屏触发条件 (G2.4 后置, ADM-1 闸 4 demo 必备)

| # | 内容 | Playwright 触发 | 文案锁 |
|---|------|----------------|--------|
| 1 | 设置页 "隐私承诺" 区块全屏 | 业主登录 → `/settings?tab=privacy` → DOM 含 §1 3 条字面 + §2 8 行表格 | 3 条字面 1:1 + 8 行表格全在 |
| 2 | admin 写动作 system DM 通知 | admin 重置业主 API key → 业主端 system DM 列表 → DOM 含 `"你的 API key 被 admin {admin_name} 重置, 请重新生成"` | 含 `admin_name` 非 raw UUID (§1.1 + ADM-0 反查 ③ 同根) |

> 截屏存放: `docs/evidence/adm-1/<n>-<slug>.png` + `blueprint-sha.txt` (execution-plan §闸 4 防漂移)

---

## 4. 反向断言锁 (ADM-1 PR 必含测试)

| 反向断言 | 测试位置 | 锁点 |
|---------|---------|------|
| `PrivacyPromise.tsx` 渲染 §1 3 条字面 1:1 | `packages/client/src/settings/PrivacyPromise.test.tsx::TestPromiseLiteral` | snapshot + grep 3 条字面命中 |
| 设置页"隐私"tab 默认展开不可折叠 | 同上 + `expect(promiseSection).toBeVisible()` | 不可有 `<details>` collapse 包裹 |
| §2 表格 ❌ 行红色 + ✅ 行灰 + impersonate amber | CSS class 反查 `.privacy-row-deny` / `.privacy-row-allow` / `.privacy-row-impersonate` | 颜色 token 锁 `#d33` / `#d97706` |
| admin 写动作 system DM 含 `admin_name` 非 UUID | `internal/api/admin_audit_test.go::TestAdminWriteEmitsNamedDM` (与 ADM-0 反查 ③ 共测试) | grep DM body 不含 raw UUID 字面 |
| drift test: doc §4.1 3 条文案 = `PrivacyPromise.tsx` 常量 | `packages/client/src/settings/PrivacyPromise.drift.test.ts` | doc-as-truth 反查, 漂移 CI 拦 (跟 CM-onboarding `TestWelcomeConstantsMirrorMigrations` 同模式) |

---

## 5. 不在 ADM-1 范围 (避免 PR 膨胀)

- ❌ Impersonation grant 创建 / 撤销 UI — Phase 4 ADM-2 (本 PR 仅文案+表格+提示, 不实施 grant 流程)
- ❌ 跨 org admin 多 admin 看不同业主 — multi-org admin v1+
- ❌ Audit log 自助查询 (业主端) — admin-model §1.4 v2, 不在 ADM-1
- ❌ 隐私承诺页国际化 — v0 中文锁, en 翻译 v1
- ❌ 用户撤销已发的 impersonate grant 的 UI 流程 — ADM-2 (本 PR 仅文案"可随时撤销"提示, 实际撤销路径 ADM-2)
- ❌ admin 操作触发的 email / push 通知 — system DM 即可, email 留账 v1

---

## 6. 验收挂钩

- ADM-1 PR: §1 3 条文案 drift test + §2 8 行表格颜色锁 + §3 2 张截屏触发就位 + §4 反向断言 5 项全绿
- ADM-1 闸 4 demo 野马签字: §3 2 张截屏 + §1 字面 1:1 + §2 ❌ 6 行视觉醒目 → 落 `docs/qa/signoffs/adm-1-yema-signoff.md` (跟 cm-4 / adm-0 同格式)
- ADM-0 闸 4 demo 联动: ADM-0 反查 ③ (受影响者 system message) 测试与 §4 第 4 项共享, ADM-0 / ADM-1 双签依赖此一个测试

---

## 7. 更新日志

| 日期 | 作者 | 变化 |
|------|------|------|
| 2026-04-28 | 野马 | v0, 3 条文案锁 + 8 行表格 + 2 张截屏 + 5 项反向断言 + 不在范围 6 条 |
