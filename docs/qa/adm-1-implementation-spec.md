# ADM-1 实施 Spec v1 — 用户隐私承诺页 (战马B 直接吃)

> **状态**: v1 (野马, 2026-04-28) — ADM-0.3 已 merged 08:03Z, ADM-1 实施前置就位
> **配套**: `docs/qa/adm-1-privacy-promise-checklist.md` (Phase 4 立场反查 v0); 本 spec 是**代码视角**, 反查表是**立场视角**, 战马B PR 同时吃两份。
> **派活**: 战马B (Phase 4 启动, **不阻塞 Phase 2**); 实施时间 ≤ 1 天 (前端纯组件, 无 server 改动)。
> **关联**: `admin-model.md §4.1` 文案锁 + `adm-1-privacy-promise-checklist.md §1/§2/§4` + `adm-0-stance-checklist.md §1 ③` (admin 写动作 system DM 共测试).

---

## 1. 实施 task — 1 个组件 + 1 个 tab + 2 个测试

| Task | 文件路径 | 说明 |
|------|---------|------|
| ① 新组件 `PrivacyPromise.tsx` | `packages/client/src/components/Settings/PrivacyPromise.tsx` | 渲染 §2 3 条承诺字面 + §3 8 行 ✅/❌ 表格; 默认展开不可折叠 |
| ② 新页面 `SettingsPage.tsx` (用户端) | `packages/client/src/components/Settings/SettingsPage.tsx` | 用户设置页骨架, "隐私" tab 顶部嵌 `<PrivacyPromise/>`; v1 只这 1 个 tab, 后续 tab (账号 / 通知) 留 placeholder |
| ③ App.tsx 路由 | `packages/client/src/App.tsx:44` 区域 | 加 `showSettings` state + 顶栏入口按钮 (与 `showAgents` / `showInvitations` 同模式); 默认 tab=`privacy` |
| ④ 单测 `PrivacyPromise.test.tsx` | 同目录 | snapshot 锁 §2 3 条字面 + §3 表格 8 行 + CSS class (`privacy-row-deny` / `-allow` / `-impersonate`) |
| ⑤ drift test `PrivacyPromise.drift.test.ts` | 同目录 | doc-as-truth 反查: 读 `admin-model.md §4.1` 3 条 → 等于组件常量 (跟 CM-onboarding `TestWelcomeConstantsMirrorMigrations` 同模式) |

> 注: admin 端 `packages/client/src/admin/pages/SettingsPage.tsx` 已存在, 是 admin SPA 的, **不要复用**, 不要混淆 (admin/user 路径分叉是 ADM-0 红线)。

---

## 2. 文案锁 (从 #211 §1 1:1 抄, 不重复人工录入 — drift test 反查)

```ts
// PrivacyPromise.tsx 内常量 (drift test 锁)
export const PRIVACY_PROMISES = [
  '**Admin 是平台运维, 不是协作者** — 永不出现在 channel / DM / 团队列表里。',
  '**Admin 看不到消息 / 文件 / artifact 内容** — 除非你主动授权 impersonate (24h 时窗, 顶部红色横幅常驻, 可随时撤销)。',
  '**Admin 能看的是元数据** (用户名 / channel 名 / 条数 / 登录时间), **看不到正文**。',
] as const;
```

> 一字漂移 → drift test 红 → CI 拦, 跟 `WelcomeMessageBody` 双声明锁同模式。markdown `**bold**` 由 `react-markdown` 渲染 (与 system message bubble 同 stack), 不要换 `<strong>` 手写。

---

## 3. ✅/❌ 表格 8 行 + 三色锁 (从 #211 §2 抄)

| 类别 | 标记 | CSS class | 颜色 token |
|------|------|-----------|-----------|
| 用户名 / 邮箱 | ✅ | `privacy-row-allow` | gray (default) |
| channel 名 / 列表 | ✅ | `privacy-row-allow` | gray |
| 消息条数 / 登录时间 | ✅ | `privacy-row-allow` | gray |
| 消息正文 (channel / DM) | ❌ | `privacy-row-deny` | **`#d33` + bold** |
| artifact / 文件内容 | ❌ | `privacy-row-deny` | **`#d33` + bold** |
| 你和 owner-agent 内置 DM | ❌ | `privacy-row-deny` | **`#d33` + bold** |
| API key 原值 | ❌ | `privacy-row-deny` | **`#d33` + bold** |
| 授权 impersonate 后 24h 实时入站 | ✅ (临时) | `privacy-row-impersonate` | **`#d97706` (amber)** |

> 颜色三色锁: gray / `#d33` (红, "正在发生" 同 ADM-0 横幅) / `#d97706` (amber, "临时态"); 不要新创第 4 色。

---

## 4. 反向断言 5 项 (从 #211 §4 抄, 测试位置具体到代码)

| 反向断言 | 测试位置 | 锁点 |
|---------|---------|------|
| `<PrivacyPromise/>` 渲染 §2 3 条字面 1:1 | `PrivacyPromise.test.tsx::renders 3 promises literally` | snapshot + grep 3 条字面命中 |
| 设置页"隐私" tab 默认展开不可折叠 | `SettingsPage.test.tsx::privacy section is always visible` | DOM 不可有 `<details>` 包裹; `expect(promiseSection).toBeVisible()` |
| §3 表格 ❌ 红 / ✅ 灰 / impersonate amber | `PrivacyPromise.test.tsx::row class names match policy` | CSS class 反查 + computed style `#d33` / `#d97706` |
| admin 写动作 system DM 含 `admin_name` 非 raw UUID | `internal/api/admin_audit_test.go::TestAdminWriteEmitsNamedDM` (与 ADM-0 反查 ③ **共测试**, 双签依赖一测) | grep DM body 不含 raw UUID 字面 |
| drift test: doc §4.1 = `PRIVACY_PROMISES` 常量 | `PrivacyPromise.drift.test.ts::doc 字面 = 组件常量` | 读 `docs/blueprint/current/admin-model.md` §4.1 → 等于 `PRIVACY_PROMISES` 三元组 |

---

## 5. 不在 ADM-1 范围 (从 #211 §5 抄)

- ❌ Impersonation grant 创建 / 撤销 UI — Phase 4 ADM-2
- ❌ 跨 org admin 多 admin 看不同业主 — multi-org admin v1+
- ❌ Audit log 自助查询 (业主端) — admin-model §1.4 v2
- ❌ 国际化 — v0 中文锁, en v1
- ❌ 用户撤销已发的 impersonate grant 流程 — ADM-2
- ❌ admin 操作触发的 email / push 通知 — system DM 即可, email v1

---

## 6. 验收挂钩

- ADM-1 PR 合并条件: §1 5 个 task 全绿 + §4 反向断言 5 项绿 + #211 §3 截屏触发 (ADM-1 闸 4 demo 准备)
- ADM-1 闸 4 demo 野马签字: 落 `docs/qa/signoffs/adm-1-yema-signoff.md` (与 cm-4 / adm-0 同格式), 2 张截屏 + §2 字面 1:1 + §3 ❌ 4 行视觉醒目
- 联动: ADM-0 反查 ③ (受影响者 system message) 测试与本 spec §4 第 4 项**共享一个测试**, 不重复落

---

## 7. 更新日志

| 日期 | 作者 | 变化 |
|------|------|------|
| 2026-04-28 | 野马 | v1 实施 spec, 1 组件 + 1 页面 + 2 测试 + 5 反向断言, 战马B 接 Phase 4 直接吃 |
