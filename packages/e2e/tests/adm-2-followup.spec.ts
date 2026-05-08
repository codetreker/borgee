// tests/adm-2-followup.spec.ts — ADM-2-FOLLOWUP acceptance §1+§2 e2e + G4.2
// 双截屏 (`g4.2-adm2-audit-list.png` + `g4.2-adm2-red-banner.png`).
//
// 闭环 docs/_archive/qa/acceptance-templates/adm-2-followup.md:
//   §1.1 admin audit list UI 真渲染 (AdminAuditList_RealRender)
//   §1.2 红 banner 真渲染 (AdminGodModeRedBanner_Real)
//   §2.1 g4.2-adm2-audit-list.png ≥3000 bytes
//   §2.2 g4.2-adm2-red-banner.png ≥3000 bytes
//   §2.3 2 case PASS
//
// 立场 (adm-2-followup-stance §1):
//   - admin SPA `/admin/audit-log` 走 admin cookie (拆 user cookie, ADM-0 §1.3)
//   - DOM 锚 `[data-page="admin-audit-log"]` + `[data-adm2-audit-list="true"]`
//     + `[data-adm2-red-banner="active"]` (反向 grep 锚)
//   - 红 banner 字面 byte-identical "当前以业主身份操作 — 该会话受 24h 时限"
//   - 反约束: 不引用 user SPA 中文动词 (跨端字面拆死, content-lock §5)
//
// 实现说明: 真 server-go(4901) + vite(5174) admin SPA 路径分叉. admin cookie
// 通过 `/admin-api/auth/login` 拿到, 注入 BrowserContext 后访问 `/admin/audit-log`.
// 页面渲染后双截屏存 docs/qa/screenshots (跟 ADM-1 G4.1 同模式).

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
