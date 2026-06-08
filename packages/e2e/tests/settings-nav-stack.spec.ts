// tests/settings-nav-stack.spec.ts — PageHeader ← / × 真出栈行为, 真浏览器.
//
// 测试范围 (PR #1012 nav-history 框架 + PageHeader 三个页面迁完):
//   A — channel → settings → remote-nodes → ← 回 settings (真出栈, 不跳 channel)
//   B — channel → settings → remote-nodes → × 直接清栈回 channel
//
// (t2 删 helper-rail UI 后, Settings Runtime 只剩 Remote Nodes 单入口;
//  原 Scenario C 走 helper-status 二级页的出栈路径随入口删除, A/B 已用
//  remote-nodes 完整覆盖 ← / × 真出栈行为.)
//
// 实施约束 (memory `e2e_no_curl_only_ui` + `e2e_full_smoke_regression`):
//   - 真 page.click / page.locator().click() 触发按钮, 禁 page.evaluate(fetch)
//   - aria-label="返回" / aria-label="关闭" 锚 PageHeader 的 ← × 按钮
//   - 每 scenario 关键步骤 page.screenshot 存证
//   - seed 借 chat-first-time-onboarding 同款 admin invite + register, cookie 注入
//
// 反向断言:
//   - Scenario A 第一次 ← 后不能落到 .channel-view (跳过 settings 就 fail)
//   - Scenario B × 之后不能停在 [data-page="settings"] (没清栈就 fail)
import { test, expect, request as apiRequest } from '@playwright/test';
import path from 'node:path';

const ADMIN_LOGIN = 'e2e-admin';
const ADMIN_PASSWORD = 'e2e-admin-pass-12345';

const SCREENSHOT_DIR = path.join(
  process.env.PLAYWRIGHT_HTML_REPORT ?? 'playwright-report',
  'settings-nav-stack',
);

async function seedUserAndCookie(page: import('@playwright/test').Page, baseURL: string) {
  const serverPort = process.env.E2E_SERVER_PORT ?? '4901';
  const serverURL = `http://127.0.0.1:${serverPort}`;
  const ctx = await apiRequest.newContext({ baseURL: serverURL });

  const loginRes = await ctx.post('/admin-api/auth/login', {
    data: { login: ADMIN_LOGIN, password: ADMIN_PASSWORD },
  });
  expect(loginRes.ok(), `admin login failed: ${loginRes.status()}`).toBe(true);

  const inviteRes = await ctx.post('/admin-api/v1/invites', {
    data: { note: 'settings-nav-stack-e2e' },
  });
  expect(inviteRes.ok(), `mint invite failed: ${inviteRes.status()}`).toBe(true);
  const inviteJson = (await inviteRes.json()) as { invite: { code: string } };
  const inviteCode = inviteJson.invite.code;

  const stamp = Date.now();
  const email = `nav-stack-${stamp}-${Math.random().toString(36).slice(2, 8)}@example.test`;
  const password = 'p@ssw0rd-nav-stack';
  const displayName = `NavStacker ${stamp}`;
  const regCtx = await apiRequest.newContext({ baseURL: serverURL });
  const regRes = await regCtx.post('/api/v1/auth/register', {
    data: {
      invite_code: inviteCode,
      email,
      password,
      display_name: displayName,
    },
  });
  expect(
    regRes.ok(),
    `register failed: ${regRes.status()} ${await regRes.text()}`,
  ).toBe(true);

  const cookies = await regCtx.storageState();
  const tokenCookie = cookies.cookies.find(c => c.name === 'borgee_token');
  expect(tokenCookie, 'borgee_token cookie should be set by register').toBeTruthy();
  if (tokenCookie) {
    const url = new URL(baseURL);
    await page.context().addCookies([{
      name: 'borgee_token',
      value: tokenCookie.value,
      domain: url.hostname,
      path: '/',
      httpOnly: true,
      secure: false,
      sameSite: 'Lax',
    }]);
  }

  await ctx.dispose();
  await regCtx.dispose();
}

/** Open Settings via the sidebar gear icon. */
async function openSettings(page: import('@playwright/test').Page) {
  const gear = page.locator('[data-action="open-settings"]');
  await expect(gear, 'settings gear icon (sidebar)').toBeVisible();
  await gear.click();
  await expect(page.locator('[data-page="settings"]')).toBeVisible();
}

async function waitChannelLanded(page: import('@playwright/test').Page) {
  // App auto-selects welcome (system) channel after init. .channel-view
  // is the stable container in ChannelView.tsx.
  await expect(page.locator('.channel-view')).toBeVisible({ timeout: 15_000 });
}

test.describe('PR #1012 nav-history — PageHeader ← / × 真出栈', () => {
  test.beforeEach(async ({ page, baseURL }) => {
    await seedUserAndCookie(page, baseURL!);
    await page.goto('/');
    await waitChannelLanded(page);
  });

  test('A — channel → settings → remote-nodes → ← 回 settings (真出栈不跳过)', async ({ page }) => {
    await openSettings(page);
    await page.screenshot({ path: path.join(SCREENSHOT_DIR, 'A1-settings.png') });

    // Click "Remote Nodes" runtime entry → nav.push('remote-nodes').
    await page.locator('[data-runtime-entry="remote-nodes"]').click();
    await expect(page.locator('.node-manager')).toBeVisible();
    // settings page is no longer in DOM (App swaps mainView).
    await expect(page.locator('[data-page="settings"]')).toHaveCount(0);
    await page.screenshot({ path: path.join(SCREENSHOT_DIR, 'A2-remote-nodes.png') });

    // ← on Remote Nodes header → must land on settings, NOT channel.
    await page.locator('[aria-label="返回"]').first().click();
    await expect(page.locator('[data-page="settings"]')).toBeVisible();
    // 反向断: 出栈不能跳过 settings 直接到 channel.
    await expect(page.locator('.node-manager')).toHaveCount(0);
    await expect(page.locator('.channel-view')).toHaveCount(0);
    await page.screenshot({ path: path.join(SCREENSHOT_DIR, 'A3-back-to-settings.png') });

    // Another ← on settings → channel.
    await page.locator('[aria-label="返回"]').first().click();
    await expect(page.locator('.channel-view')).toBeVisible();
    await expect(page.locator('[data-page="settings"]')).toHaveCount(0);
    await page.screenshot({ path: path.join(SCREENSHOT_DIR, 'A4-back-to-channel.png') });
  });

  test('B — channel → settings → remote-nodes → × 一键清栈回 channel', async ({ page }) => {
    await openSettings(page);
    await page.locator('[data-runtime-entry="remote-nodes"]').click();
    await expect(page.locator('.node-manager')).toBeVisible();
    await page.screenshot({ path: path.join(SCREENSHOT_DIR, 'B1-remote-nodes.png') });

    // × on Remote Nodes header → clear stack → channel.
    await page.locator('[aria-label="关闭"]').first().click();
    await expect(page.locator('.channel-view')).toBeVisible();
    // 反向断: × 必须跳过中间 settings, 不留 settings 在 DOM.
    await expect(page.locator('[data-page="settings"]')).toHaveCount(0);
    await expect(page.locator('.node-manager')).toHaveCount(0);
    await page.screenshot({ path: path.join(SCREENSHOT_DIR, 'B2-closed-to-channel.png') });
  });
});
