// tests/owner-plugin-connections.spec.ts — #1049 owner plugin connections UI.
//
// 真浏览器 (memory `e2e_no_curl_only_ui`): UI 步骤走 click; 无 `page.evaluate(fetch)`
// 没绕后端走 cookie 直调.
//
// Scope:
//   1. Owner logs in → My Agents → Manage → assert "Plugin connections"
//      section visible.
//   2. Empty state ("No plugin connections") rendered when no helper
//      enrollment exists for the owner.
//   3. With a helper enrollment seeded (owner-side mint API) the
//      section transitions out of empty state (still empty list since
//      no configure has happened yet — but the section continues to
//      render, no error).
//   4. Add form: invalid `connection_id` keeps submit disabled; valid
//      one enables submit.
//
// Out of scope (covered by Go API + unit tests):
//   - End-to-end configure → list → remove cycle requires a real helper
//     daemon online to complete the jobs. See
//     packages/server-go/internal/api/helper_jobs_plugin_connections_test.go
//     for the in-process Go contract test.

import {
  test,
  expect,
  request as apiRequest,
  type APIRequestContext,
  type Page,
  type BrowserContext,
} from '@playwright/test';

const ADMIN_LOGIN = 'e2e-admin';
const ADMIN_PASSWORD = 'e2e-admin-pass-12345';

interface RegisteredUser {
  email: string;
  token: string;
  userId: string;
  ctx: APIRequestContext;
}

async function adminLogin(serverURL: string): Promise<APIRequestContext> {
  const ctx = await apiRequest.newContext({ baseURL: serverURL });
  const res = await ctx.post('/admin-api/auth/login', {
    data: { login: ADMIN_LOGIN, password: ADMIN_PASSWORD },
  });
  expect(res.ok(), `admin login: ${res.status()}`).toBe(true);
  return ctx;
}

async function mintInvite(adminCtx: APIRequestContext, note: string): Promise<string> {
  const res = await adminCtx.post('/admin-api/v1/invites', { data: { note } });
  expect(res.ok(), `mint invite: ${res.status()}`).toBe(true);
  const body = (await res.json()) as { invite: { code: string } };
  return body.invite.code;
}

async function registerUser(
  serverURL: string,
  inviteCode: string,
  suffix: string,
): Promise<RegisteredUser> {
  const ctx = await apiRequest.newContext({ baseURL: serverURL });
  const stamp = Date.now();
  const email = `plugin-conn-${suffix}-${stamp}-${Math.floor(Math.random() * 1000)}@example.test`;
  const password = 'p@ssw0rd-plugin-conn';
  const displayName = `PluginConn ${suffix} ${stamp}`;
  const res = await ctx.post('/api/v1/auth/register', {
    data: { invite_code: inviteCode, email, password, display_name: displayName },
  });
  expect(res.ok(), `register: ${res.status()}`).toBe(true);
  const body = (await res.json()) as { user: { id: string } };
  const cookies = await ctx.storageState();
  const tok = cookies.cookies.find((c) => c.name === 'borgee_token');
  expect(tok, 'borgee_token cookie missing').toBeTruthy();
  return { email, token: tok!.value, userId: body.user.id, ctx };
}

async function attachToken(ctx: BrowserContext, baseURL: string, token: string) {
  const url = new URL(baseURL);
  await ctx.clearCookies();
  await ctx.addCookies([
    {
      name: 'borgee_token',
      value: token,
      domain: url.hostname,
      path: '/',
      httpOnly: true,
      secure: false,
      sameSite: 'Lax',
    },
  ]);
}

async function createAgent(
  serverURL: string,
  ownerToken: string,
  displayName: string,
): Promise<{ id: string }> {
  const ctx = await apiRequest.newContext({
    baseURL: serverURL,
    extraHTTPHeaders: { Cookie: `borgee_token=${ownerToken}` },
  });
  const r = await ctx.post('/api/v1/agents', { data: { display_name: displayName } });
  expect(r.ok() || r.status() === 201, `agent create: ${r.status()}`).toBe(true);
  const body = (await r.json()) as { agent: { id: string } };
  return { id: body.agent.id };
}

async function createHelperEnrollment(
  serverURL: string,
  ownerToken: string,
): Promise<{ enrollmentId: string }> {
  const ctx = await apiRequest.newContext({
    baseURL: serverURL,
    extraHTTPHeaders: { Cookie: `borgee_token=${ownerToken}` },
  });
  const r = await ctx.post('/api/v1/helper/enrollments', {
    data: {
      host_label: 'e2e-plugin-conn-host',
      allowed_categories: ['openclaw_config'],
    },
  });
  expect(r.ok() || r.status() === 201, `helper enrollment create: ${r.status()}`).toBe(true);
  const body = (await r.json()) as {
    enrollment: { enrollment_id: string };
    enrollment_secret: string;
  };
  // Claim the enrollment so its status moves out of "pending". The
  // AgentPluginConnectionsSection wrapper filters pending enrollments
  // (helper not yet online), which would otherwise omit the section.
  // Claim acts as the helper-side install-time step in this test seam;
  // no real helper daemon needs to be running for the UI to render.
  const claimRes = await ctx.post(
    `/api/v1/helper/enrollments/${body.enrollment.enrollment_id}/claim`,
    {
      data: {
        enrollment_secret: body.enrollment_secret,
        helper_device_id: 'e2e-plugin-conn-device',
      },
    },
  );
  expect(
    claimRes.ok() || claimRes.status() === 201,
    `helper enrollment claim: ${claimRes.status()}`,
  ).toBe(true);
  return { enrollmentId: body.enrollment.enrollment_id };
}

async function openAgentManage(page: Page) {
  await page.goto('/');
  await expect(
    page.locator('.hamburger-btn, .sidebar-title').first(),
  ).toBeVisible({ timeout: 10_000 });
  const hamburger = page.locator('.hamburger-btn');
  const isMobile = (await hamburger.count()) > 0 && (await hamburger.isVisible());
  if (isMobile) {
    await hamburger.click();
  }
  await page.locator('[data-testid="sidebar-nav-agents"]').click();
  await expect(page.locator('.agent-page')).toBeVisible({ timeout: 10_000 });
  if (isMobile) {
    await page.evaluate(() => {
      const overlay = document.querySelector('.sidebar-overlay') as HTMLElement | null;
      if (overlay) overlay.click();
    });
    await expect(page.locator('.sidebar-overlay')).toHaveCount(0, { timeout: 5_000 });
  }
  const manageBtn = page
    .locator('.agent-card button.btn-sm', { hasText: 'Manage' })
    .first();
  await manageBtn.click();
}

const SERVER_URL = `http://127.0.0.1:${process.env.E2E_SERVER_PORT ?? '4901'}`;

test.describe('Owner plugin connections UI (#1049)', () => {
  test('Section renders empty state, form validates connection_id', async ({ page, baseURL }) => {
    const adminCtx = await adminLogin(SERVER_URL);
    const inviteCode = await mintInvite(adminCtx, 'plugin-conn-owner');
    const owner = await registerUser(SERVER_URL, inviteCode, 'owner');
    await createAgent(SERVER_URL, owner.token, `plugin-conn-${Date.now().toString(36)}`);
    // Seed a helper enrollment so the wrapper resolves an active
    // enrollment id and the section renders. Without an enrollment the
    // wrapper intentionally omits the section.
    await createHelperEnrollment(SERVER_URL, owner.token);

    await attachToken(page.context(), baseURL!, owner.token);
    await page.setViewportSize({ width: 1280, height: 800 });
    await openAgentManage(page);

    const section = page.locator('[data-testid="plugin-connections-section"]');
    await expect(section).toBeVisible({ timeout: 10_000 });

    // Empty state.
    await expect(page.locator('[data-testid="plugin-connections-empty"]')).toBeVisible();

    // Open the add form.
    await page.locator('[data-testid="plugin-connection-add-btn"]').click();
    const submit = page.locator('[data-testid="plugin-connection-form-submit"]');
    await expect(submit).toBeDisabled();

    // Type an invalid connection_id → submit remains disabled, error visible.
    await page
      .locator('[data-testid="plugin-connection-form-connection-id"]')
      .fill('not-a-prefix');
    await page.locator('[data-testid="plugin-connection-form-channel-id"]').fill('chan-x');
    await expect(submit).toBeDisabled();
    await expect(
      page.locator('[data-testid="plugin-connection-form-connection-id-error"]'),
    ).toBeVisible();

    // Fix the connection_id → submit enables.
    await page
      .locator('[data-testid="plugin-connection-form-connection-id"]')
      .fill('borgee-plugin:test-1');
    await expect(submit).toBeEnabled();
  });
});
