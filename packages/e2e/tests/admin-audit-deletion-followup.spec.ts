// tests/admin-audit-deletion-followup.spec.ts — admin 审计日志页 + 红横幅渲染 + G4.2 demo 截屏.
//
// 测试范围:
//   - admin SPA `/admin/audit-log` 真渲染审计列表 DOM
//   - admin god-mode 红色横幅在会话内渲染并显示 24h 时限文案
//   - 截屏存档 docs/qa/screenshots/g4.2-adm2-audit-list.png
//   - 截屏存档 docs/qa/screenshots/g4.2-adm2-red-banner.png
//
// 关联文档:
//   - 蓝图: docs/blueprint/current/admin-model.md §1.3 (admin god-mode 路径独立)
//   - 验收: docs/_archive/qa/acceptance-templates/adm-2-followup.md §1+§2
//
// 实施约束:
//   - 真 UI 走浏览器 (page.goto + page.click + DOM 断)
//   - admin cookie 走 `/admin-api/auth/login` 拿, 注入 BrowserContext 后访问 admin SPA
//   - 红横幅文案字面相等: "当前以业主身份操作 — 该会话受 24h 时限"
//   - 不引用 user SPA 中文动词 (admin/user 文案分叉)
//   - 不允许 fs.* / page.evaluate(fetch) / 只打 API / noop

import {
  test,
  expect,
  request as apiRequest,
  type APIRequestContext,
  type BrowserContext,
} from '@playwright/test';
import * as path from 'node:path';
import { fileURLToPath } from 'node:url';

const ADMIN_LOGIN = 'e2e-admin';
const ADMIN_PASSWORD = 'e2e-admin-pass-12345';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const SCREENSHOT_DIR = path.resolve(HERE, '../../../docs/qa/screenshots');

function clientURL(): string {
  return `http://127.0.0.1:${process.env.E2E_CLIENT_PORT ?? '5174'}`;
}

async function adminLoginCookie(serverURL: string): Promise<string> {
  const ctx = await apiRequest.newContext({ baseURL: serverURL });
  const res = await ctx.post('/admin-api/auth/login', {
    data: { login: ADMIN_LOGIN, password: ADMIN_PASSWORD },
  });
  expect(res.ok(), `admin login: ${res.status()}`).toBe(true);
  const state = await ctx.storageState();
  const adminCookie = state.cookies.find((c) => c.name === 'borgee_admin_session');
  expect(adminCookie, 'admin cookie missing after login').toBeTruthy();
  return adminCookie!.value;
}

async function attachAdminCookie(ctx: BrowserContext, token: string): Promise<void> {
  const url = new URL(clientURL());
  await ctx.addCookies([
    {
      name: 'borgee_admin_session',
      value: token,
      domain: url.hostname,
      path: '/',
      httpOnly: true,
      secure: false,
      sameSite: 'Lax',
    },
  ]);
}

test.describe('ADM-2-FOLLOWUP — REG-ADM2-011 admin SPA audit-log 页 + G4.2 双截屏', () => {
  test('§1.1+§2.1 — AdminAuditList real render + g4.2-adm2-audit-list.png 截屏', async ({
    browser,
  }) => {
    const serverPort = process.env.E2E_SERVER_PORT ?? '4901';
    const serverURL = `http://127.0.0.1:${serverPort}`;
    const adminToken = await adminLoginCookie(serverURL);

    const ctx = await browser.newContext();
    await attachAdminCookie(ctx, adminToken);
    const page = await ctx.newPage();

    // Vite dev does not auto-serve admin.html for /admin/* paths; push
    // history so BrowserRouter mounts at /admin/audit-log target. (跟
    // adm-3-audit-events.spec.ts case-1 admin SPA 加载模式同源 — Prod
    // 走 server-go SPA fallback, dev 走 admin.html.)
    await page.addInitScript(() => {
      window.history.replaceState({}, '', '/admin/audit-log');
    });
    await page.goto(`${clientURL()}/admin.html`);
    await page.waitForLoadState('domcontentloaded');

    // DOM 锚反查 — admin SPA AdminAuditLogPage 渲染.
    await expect(page.locator('[data-page="admin-audit-log"]')).toBeVisible();
    await expect(page.locator('[data-adm2-audit-list="true"]')).toBeVisible();

    // 中文 title byte-identical (反 English "Audit Log" h2).
    await expect(page.locator('h2', { hasText: '审计日志' })).toBeVisible();

    // §2.1 G4.2 截屏 #1 — audit list 首屏.
    await page.screenshot({
      path: path.join(SCREENSHOT_DIR, 'g4.2-adm2-audit-list.png'),
      fullPage: false,
    });
  });

  test('§1.2+§2.2 — AdminGodMode red banner active + g4.2-adm2-red-banner.png 截屏', async ({
    browser,
  }) => {
    const serverPort = process.env.E2E_SERVER_PORT ?? '4901';
    const serverURL = `http://127.0.0.1:${serverPort}`;
    const adminToken = await adminLoginCookie(serverURL);

    const ctx = await browser.newContext();
    await attachAdminCookie(ctx, adminToken);
    const page = await ctx.newPage();

    await page.addInitScript(() => {
      window.history.replaceState({}, '', '/admin/audit-log');
    });
    await page.goto(`${clientURL()}/admin.html`);
    await page.waitForLoadState('domcontentloaded');

    // 红 banner DOM 锚 + 字面 byte-identical (蓝图 §1.4 红线 1).
    const banner = page.locator('[data-adm2-red-banner="active"]');
    await expect(banner).toBeVisible();
    await expect(banner).toContainText('当前以业主身份操作 — 该会话受 24h 时限');

    // §2.2 G4.2 截屏 #2 — 红 banner 常驻.
    await banner.scrollIntoViewIfNeeded();
    await page.screenshot({
      path: path.join(SCREENSHOT_DIR, 'g4.2-adm2-red-banner.png'),
      fullPage: false,
    });
  });
});
