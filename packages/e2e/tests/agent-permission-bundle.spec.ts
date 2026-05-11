// tests/agent-permission-bundle.spec.ts — Agent permission bundle UI 测试 (capability 透明展开).
//
// 状态: SKIP+followup (gh#716 + gh#724 §1, 2026-05-11 真验 mount 缺失发现).
//
// 跳过原因: BundleSelector / PermissionsView 组件在 client SPA 当前没有 production
// mount (反向 grep `<BundleSelector\b` / `<PermissionsView\b` 在 production code
// 0 hit, 仅 `__tests__/` 内单测渲染). 走真 UI 路径不可达. 现 spec 全走 REST 直调
// + page.screenshot 锚 (反模式 F3 纯 REST), 不是真 UI input/click 路径.
// v2 BundleSelector / PermissionsView mount 落地后 unskip + 改真 UI (用户点击
// bundle → capability checkbox 展开 → confirm → server 收 N PUT 调用 DOM 断).
//
// 4 case (v2 unskip 时验):
//   - capability response shape — /api/v1/me/permissions 含 capabilities 数组
//   - 反 bundle endpoint 漂 — POST /api/v1/bundles 不存在 (复用 AP-1 PUT)
//   - UI 真渲染 — capability 透明展开 (反 RBAC 8 词 0 hit body)
//   - admin god-mode UI 独立路径 — admin login 不含 bundle UI 字面 (ADM-0 §1.3 红线)
//
// 关联文档:
//   - 蓝图: docs/blueprint/current/auth-permissions.md §4 (capability bundle UI)
//   - 验收: docs/_archive/qa/acceptance-templates/ap-2.md §3.2
//   - 单测: vitest ap-2-capabilities.test.ts + ap-2-reverse-grep.test.ts
//   - 后续: gh#724 §1 (BundleSelector / PermissionsView mount 待做)
//
// 实施约束 (unskip 后):
//   - 真 UI 路径走浏览器 (page.goto Settings 页 → page.click bundle 行 → 真展开
//     capability checkboxes → 真点 confirm → 真断 server PUT 收到调用)
//   - 不允许 fs.* / page.evaluate(fetch) / 只打 API / noop
//   - admin god-mode 独立路径 case 走 REWRITE-NAV (page.goto admin URL + 真断 bundle UI 字面 0 hit)

import { test, expect } from '@playwright/test';

void expect; // 避免 unused; v2 unskip 时实施.

test.describe.skip('agent permission bundle UI — capability 透明 (gh#716 + gh#724 §1, mount 待做)', () => {
  test('§3.2 capability response shape — /api/v1/me/permissions 含 capabilities 数组', async () => {
    // v2 unskip 时实施 — 见头部注释 4 case.
  });
});
