// tests/production-surface-reverse-proof.spec.ts — M3 Task4 reverse proof.
//
// Scope:
//   - ArtifactComments is reachable from the production ArtifactPanel surface.
//   - ArtifactComments and ArtifactPanel render non-leaky forbidden states.
//   - Settings reaches PermissionsView and renders empty / forbidden / error states.
//
// Constraints:
//   - Browser-driven product paths for every surface.
//   - Server-backed setup for auth, channels, artifacts, and archived-channel denial.
//   - No broad quality-platform rewrite, mobile expansion, or Task7 Helper/Remote Nodes scope.

import {
  test,
  expect,
  request as apiRequest,
  type APIRequestContext,
  type Browser,
  type BrowserContext,
  type Page,
} from '@playwright/test';

const ADMIN_LOGIN = 'e2e-admin';
const ADMIN_PASSWORD = 'e2e-admin-pass-12345';

interface RegisteredUser {
  ctx: APIRequestContext;
  token: string;
  userId: string;
}

function serverURL(): string {
  return `http://127.0.0.1:${process.env.E2E_SERVER_PORT ?? '4901'}`;
}

function clientURL(): string {
  return `http://127.0.0.1:${process.env.E2E_CLIENT_PORT ?? '5174'}`;
}

async function adminLogin(): Promise<APIRequestContext> {
  const ctx = await apiRequest.newContext({ baseURL: serverURL() });
  const res = await ctx.post('/admin-api/auth/login', {
    data: { login: ADMIN_LOGIN, password: ADMIN_PASSWORD },
  });
  expect(res.ok(), `admin login: ${res.status()} ${await res.text()}`).toBe(true);
  return ctx;
}

async function mintInvite(adminCtx: APIRequestContext, note: string): Promise<string> {
  const res = await adminCtx.post('/admin-api/v1/invites', { data: { note } });
  expect(res.ok(), `mint invite: ${res.status()} ${await res.text()}`).toBe(true);
  const body = (await res.json()) as { invite: { code: string } };
  return body.invite.code;
}

async function registerUser(inviteCode: string, suffix: string): Promise<RegisteredUser> {
  const ctx = await apiRequest.newContext({ baseURL: serverURL() });
  const stamp = `${Date.now()}-${Math.floor(Math.random() * 10000)}`;
  const res = await ctx.post('/api/v1/auth/register', {
    data: {
      invite_code: inviteCode,
      email: `m3t4-${suffix}-${stamp}@example.test`,
      password: 'p@ssw0rd-m3t4',
      display_name: `M3T4 ${suffix} ${stamp}`,
    },
  });
  expect(res.ok(), `register ${suffix}: ${res.status()} ${await res.text()}`).toBe(true);
  const body = (await res.json()) as { user: { id: string } };
  const state = await ctx.storageState();
  const token = state.cookies.find((cookie) => cookie.name === 'borgee_token')?.value;
  expect(token, `borgee_token cookie missing for ${suffix}`).toBeTruthy();
  return { ctx, token: token!, userId: body.user.id };
}

async function createChannel(user: RegisteredUser, name: string): Promise<string> {
  const res = await user.ctx.post('/api/v1/channels', {
    data: { name, visibility: 'private' },
  });
  expect(res.ok(), `channel create: ${res.status()} ${await res.text()}`).toBe(true);
  const body = (await res.json()) as { channel: { id: string; name: string } };
  expect(body.channel.name).toBe(name);
  return body.channel.id;
}

async function archiveChannel(user: RegisteredUser, channelId: string): Promise<void> {
  const res = await user.ctx.put(`/api/v1/channels/${channelId}`, {
    data: { archived: true },
  });
  expect(res.ok(), `archive channel: ${res.status()} ${await res.text()}`).toBe(true);
}

async function attachToken(ctx: BrowserContext, token: string): Promise<void> {
  const url = new URL(clientURL());
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

async function gotoCanvas(page: Page, channelName: string): Promise<void> {
  await page.goto(clientURL());
  await expect(page.locator('.sidebar-title')).toBeVisible({ timeout: 10_000 });
  await page.locator('.channel-name', { hasText: channelName }).first().click();
  await page.locator('.channel-view-tab', { hasText: 'Canvas' }).click();
  await expect(page.locator('.artifact-panel')).toBeVisible();
}

async function createArtifactViaUI(page: Page, title: string): Promise<string> {
  const response = page.waitForResponse(
    (res) =>
      res.request().method() === 'POST' &&
      res.url().includes('/artifacts') &&
      !res.url().includes('/commits') &&
      !res.url().includes('/rollback') &&
      !res.url().includes('/versions'),
  );
  await page.locator('.artifact-empty button.btn-primary').click();
  const modal = page.locator('[data-testid="artifact-create-modal"]');
  await expect(modal).toBeVisible({ timeout: 3_000 });
  await modal.locator('input.input-field').fill(title);
  await modal.locator('button[type="submit"]').click();
  const res = await response;
  expect(res.status(), `artifact create: ${await res.text()}`).toBe(201);
  const body = (await res.json()) as { id: string };
  await expect(page.locator('.artifact-version-tag')).toHaveText('v1', { timeout: 5_000 });
  return body.id;
}

async function setupOwnerAndChannel(label: string): Promise<{
  adminCtx: APIRequestContext;
  owner: RegisteredUser;
  channelId: string;
  channelName: string;
}> {
  const adminCtx = await adminLogin();
  const invite = await mintInvite(adminCtx, label);
  const owner = await registerUser(invite, `${label}-owner`);
  const channelName = `${label}-${Date.now().toString(36)}`;
  const channelId = await createChannel(owner, channelName);
  return { adminCtx, owner, channelId, channelName };
}

test.describe('M3 Task4 production surface reverse proof', () => {
  test.describe.configure({ mode: 'serial' });

  test('ArtifactComments production mount uses the real ArtifactPanel UI and comment API', async ({ browser }) => {
    const { adminCtx, owner, channelName } = await setupOwnerAndChannel('m3t4-comments');
    const ctx = await browser.newContext();
    await attachToken(ctx, owner.token);
    const page = await ctx.newPage();

    await gotoCanvas(page, channelName);
    const listResponse = page.waitForResponse(
      (res) => res.request().method() === 'GET' && /\/api\/v1\/artifacts\/[^/]+\/comments$/.test(new URL(res.url()).pathname),
    );
    await createArtifactViaUI(page, 'M3T4 comment proof');
    expect((await listResponse).status()).toBe(200);

    await expect(page.locator('[data-testid="cv5-artifact-comments"]')).toBeVisible();
    await expect(page.locator('[data-testid="cv5-empty"]')).toHaveText('No comments yet.');

    const body = `M3T4 body ${Date.now()}`;
    const postResponse = page.waitForResponse(
      (res) => res.request().method() === 'POST' && /\/api\/v1\/artifacts\/[^/]+\/comments$/.test(new URL(res.url()).pathname),
    );
    await page.locator('[data-testid="cv5-composer-input"]').fill(body);
    await page.locator('[data-testid="cv5-composer-submit"]').click();
    expect((await postResponse).status()).toBe(201);

    await expect(page.locator('[data-cv5-comment-id]').filter({ hasText: body })).toBeVisible();
    await expect(page.locator('[data-cv5-forbidden]')).toHaveCount(0);
    await expect(page.locator('[data-cv5-unavailable]')).toHaveCount(0);

    await ctx.close();
    await adminCtx.dispose();
    await owner.ctx.dispose();
  });

  test('ArtifactComments renders a non-leaky forbidden state for a server ACL denial', async ({ browser }) => {
    const { adminCtx, owner, channelName } = await setupOwnerAndChannel('m3t4-cv5-denied');
    const otherInvite = await mintInvite(adminCtx, 'm3t4-cv5-denied-other');
    const other = await registerUser(otherInvite, 'm3t4-cv5-denied-other');

    const ctx = await browser.newContext();
    await attachToken(ctx, owner.token);
    const page = await ctx.newPage();
    let deniedListCount = 0;
    let deniedCode: string | undefined;
    await page.route(/\/api\/v1\/artifacts\/[^/]+\/comments$/, async (route) => {
      if (route.request().method() !== 'GET') {
        await route.continue();
        return;
      }

      const path = new URL(route.request().url()).pathname;
      const deniedRes = await other.ctx.get(path);
      const body = await deniedRes.text();
      if (deniedRes.status() === 403) {
        deniedListCount += 1;
        deniedCode = (JSON.parse(body) as { code?: string }).code;
      }
      await route.fulfill({
        status: deniedRes.status(),
        contentType: deniedRes.headers()['content-type'] ?? 'application/json',
        body,
      });
    });

    await gotoCanvas(page, channelName);
    await createArtifactViaUI(page, 'M3T4 comment denial proof');

    await expect(page.locator('[data-cv5-forbidden]')).toHaveText('You do not have access to these comments.');
    expect(deniedListCount, 'ArtifactComments should receive a server ACL denial').toBeGreaterThanOrEqual(1);
    expect(deniedCode, 'ArtifactComments denial should come from server ACL').toBe('comment.cross_channel_reject');
    await expect(page.locator('[data-cv5-forbidden]')).not.toContainText('comment.cross_channel_reject');
    await expect(page.locator('[data-cv5-forbidden]')).not.toContainText('not a member');
    await expect(page.locator('[data-testid="cv5-composer-input"]')).toHaveCount(0);

    await ctx.close();
    await adminCtx.dispose();
    await owner.ctx.dispose();
    await other.ctx.dispose();
  });

  test('ArtifactPanel renders a non-leaky forbidden state when an archived channel denies artifact create', async ({ browser }) => {
    const { adminCtx, owner, channelId, channelName } = await setupOwnerAndChannel('m3t4-artifact-denied');
    await archiveChannel(owner, channelId);

    const ctx = await browser.newContext();
    await attachToken(ctx, owner.token);
    const page = await ctx.newPage();
    await gotoCanvas(page, channelName);

    const secretTitle = `M3T4 archived secret ${Date.now()}`;
    const createResponse = page.waitForResponse(
      (res) =>
        res.request().method() === 'POST' &&
        res.url().includes('/artifacts') &&
        !res.url().includes('/commits') &&
        !res.url().includes('/rollback') &&
        !res.url().includes('/versions'),
    );
    await page.locator('.artifact-empty button.btn-primary').click();
    const modal = page.locator('[data-testid="artifact-create-modal"]');
    await expect(modal).toBeVisible({ timeout: 3_000 });
    await modal.locator('input.input-field').fill(secretTitle);
    await modal.locator('button[type="submit"]').click();

    expect((await createResponse).status()).toBe(403);
    await expect(page.locator('[data-artifact-forbidden]')).toHaveText('You do not have access to this artifact.');
    await expect(page.locator('.artifact-panel')).not.toContainText(secretTitle);
    await expect(page.locator('.artifact-panel')).not.toContainText('Channel is archived');

    await ctx.close();
    await adminCtx.dispose();
    await owner.ctx.dispose();
  });

  test('Settings reaches PermissionsView and renders empty, forbidden, and error states', async ({ browser }) => {
    const adminCtx = await adminLogin();
    const invite = await mintInvite(adminCtx, 'm3t4-settings');
    const owner = await registerUser(invite, 'm3t4-settings-owner');

    await expectSettingsState(browser, owner.token, {
      status: 200,
      body: {
        user_id: owner.userId,
        role: 'member',
        permissions: [],
        details: [],
        capabilities: [],
      },
      selector: '[data-ap2-empty]',
      text: '暂无授权',
    });

    await expectSettingsState(browser, owner.token, {
      status: 403,
      body: { error: 'private permission payload channel.manage_members should not render' },
      selector: '[data-ap2-forbidden]',
      text: '无权查看授权',
      forbiddenText: 'channel.manage_members',
    });

    await expectSettingsState(browser, owner.token, {
      status: 500,
      body: { error: 'database secret token should not render' },
      selector: '[data-ap2-error]',
      text: '加载失败',
      forbiddenText: 'database secret token',
    });

    await adminCtx.dispose();
    await owner.ctx.dispose();
  });
});

async function expectSettingsState(
  browser: Browser,
  token: string,
  opts: {
    status: number;
    body: unknown;
    selector: string;
    text: string;
    forbiddenText?: string;
  },
): Promise<void> {
  const ctx = await browser.newContext();
  await attachToken(ctx, token);
  const page = await ctx.newPage();
  let requestCount = 0;
  await page.route('**/api/v1/me/permissions', async (route) => {
    requestCount += 1;
    await route.fulfill({
      status: opts.status,
      contentType: 'application/json',
      body: JSON.stringify(opts.body),
    });
  });

  await page.goto(clientURL());
  await expect(page.locator('.sidebar-title')).toBeVisible({ timeout: 10_000 });
  await page.locator('[data-action="open-settings"]').click();
  await expect(page.locator('[data-page="settings"]')).toBeVisible();
  await expect(page.locator('[data-settings-permissions-surface]')).toBeVisible();
  await expect(page.locator(opts.selector)).toHaveText(opts.text);
  expect(requestCount, 'Settings path should call /api/v1/me/permissions').toBeGreaterThanOrEqual(1);
  if (opts.forbiddenText) {
    await expect(page.locator('[data-settings-permissions-surface]')).not.toContainText(opts.forbiddenText);
  }

  await ctx.close();
}
