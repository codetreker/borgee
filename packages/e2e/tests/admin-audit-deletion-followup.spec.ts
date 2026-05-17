// tests/admin-audit-deletion-followup.spec.ts — admin audit-log page + red banner rendering + G4.2 demo screenshots.
//
// Test scope:
//   - admin SPA `/admin/audit-log` renders the audit-list DOM.
//   - admin elevated-access red banner renders during the session and shows the 24h limit copy.
//   - Screenshot archive docs/qa/screenshots/g4.2-adm2-audit-list.png.
//   - Screenshot archive docs/qa/screenshots/g4.2-adm2-red-banner.png.
//
// Related docs:
//   - Blueprint: docs/blueprint/current/admin-model.md §1.3 (admin elevated-access path is separate)
//   - Acceptance: docs/_archive/qa/acceptance-templates/adm-2-followup.md §1+§2
//
// Implementation constraints:
//   - Browser-driven UI path: page.goto, page.click, and DOM assertions.
//   - Obtain admin cookie through `/admin-api/auth/login`, inject it into BrowserContext, then visit admin SPA.
//   - Red banner copy remains byte-identical: "当前以业主身份操作 — 该会话受 24h 时限".
//   - Do not reuse user-SPA Chinese verbs; admin/user copy intentionally differs.
//   - Do not use fs.*, page.evaluate(fetch), API-only checks, or empty placeholder tests.

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

    // Vite dev does not auto-serve admin.html for /admin/* paths; push history
    // so BrowserRouter mounts at /admin/audit-log. This matches
    // adm-3-audit-events.spec.ts case-1 admin SPA loading: production uses the
    // server-go SPA fallback, while dev uses admin.html.
    await page.addInitScript(() => {
      window.history.replaceState({}, '', '/admin/audit-log');
    });
    await page.goto(`${clientURL()}/admin.html`);
    await page.waitForLoadState('domcontentloaded');

    // DOM anchor check: admin SPA AdminAuditLogPage renders.
    await expect(page.locator('[data-page="admin-audit-log"]')).toBeVisible();
    await expect(page.locator('[data-adm2-audit-list="true"]')).toBeVisible();

    // Chinese title remains byte-identical; do not regress to English "Audit Log" h2.
    await expect(page.locator('h2', { hasText: '审计日志' })).toBeVisible();

    // §2.1 G4.2 screenshot #1: audit-list first viewport.
    if (process.env.E2E_EVIDENCE_SCREENSHOTS === '1') await page.screenshot({
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

    // Red banner DOM anchor + byte-identical literal (blueprint §1.4 boundary 1).
    const banner = page.locator('[data-adm2-red-banner="active"]');
    await expect(banner).toBeVisible();
    await expect(banner).toContainText('当前以业主身份操作 — 该会话受 24h 时限');

    // §2.2 G4.2 screenshot #2: persistent red banner.
    await banner.scrollIntoViewIfNeeded();
    if (process.env.E2E_EVIDENCE_SCREENSHOTS === '1') await page.screenshot({
      path: path.join(SCREENSHOT_DIR, 'g4.2-adm2-red-banner.png'),
      fullPage: false,
    });
  });
});
