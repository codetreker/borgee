// tests/artifact-comment-search.spec.ts — CV-12 ArtifactCommentSearchBox 生产挂载真验证 (#972).
//
// 范围 (真浏览器, 真 input/click/DOM 断):
//   - ArtifactCommentSearchBox 在生产 ArtifactPanel → ArtifactComments 内被挂载 (MOUNTED).
//     orphan 修复证据: data-cv12-search-input / cv12-search-submit 在真 DOM 可见.
//   - 输入命中查询 + 点搜索 → 真后端 GET /channels/{artifactChannelId}/messages/search
//     返回结果, result list 渲染 data-cv12-search-result-id (真 DOM, 非 mock).
//   - 输入不命中查询 → 空状态文案 "未找到匹配评论" byte-identical 渲染.
//
// 关键 wiring (#972): ArtifactCommentSearchBox 需要 artifactChannelId =
// 虚拟 `artifact:<artifactId>` namespace channel 的 UUID. 该 id 由 server 在
// artifact_comments.go handleListComments 时 stamp 进每条 comment 的 channel_id.
// ArtifactComments 从已加载的 comment row 解析它再传给 search box —— 故必须先发
// 一条 comment, channel 才存在, search box 才挂载. 本测试走真 composer UI 发评论.
//
// 实施约束 (项目铁律):
//   - 真 UI 走浏览器 (page.goto + 真 fill/click + DOM 断). 不允许 page.evaluate(fetch)
//     / cURL / route-mock 当挂载证据.
//   - server-backed setup 仅用于 auth / channel 前置 (跟 production-surface-reverse-proof
//     + comment-anchor-scroll 同模式), 产品路径 (建 artifact / 发评论 / 搜索) 全走真 UI.

import {
  test,
  expect,
  request as apiRequest,
  type APIRequestContext,
  type BrowserContext,
  type Page,
} from '@playwright/test';

const ADMIN_LOGIN = 'e2e-admin';
const ADMIN_PASSWORD = 'e2e-admin-pass-12345';

const NO_RESULT_TEXT = '未找到匹配评论';

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
      email: `cv12-${suffix}-${stamp}@example.test`,
      password: 'p@ssw0rd-cv12',
      display_name: `CV12 ${suffix} ${stamp}`,
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
  return body.channel.id;
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
      !res.url().includes('/versions') &&
      !res.url().includes('/anchors'),
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

// Post a comment through the real ArtifactComments composer UI (NOT the REST
// API directly). Posting the first comment is what materializes the virtual
// `artifact:<id>` channel and lets ArtifactComments resolve artifactChannelId
// → mount the search box.
async function postCommentViaUI(page: Page, body: string): Promise<void> {
  const postResponse = page.waitForResponse(
    (res) =>
      res.request().method() === 'POST' &&
      /\/api\/v1\/artifacts\/[^/]+\/comments$/.test(new URL(res.url()).pathname),
  );
  await page.locator('[data-testid="cv5-composer-input"]').fill(body);
  await page.locator('[data-testid="cv5-composer-submit"]').click();
  expect((await postResponse).status()).toBe(201);
  // Comment row renders → list non-empty → search box mounts.
  await expect(page.locator('[data-cv5-comment-id]').filter({ hasText: body })).toBeVisible();
}

test.describe('CV-12 artifact comment search box production mount (#972)', () => {
  test('search box is MOUNTED in ArtifactComments and searches the real backend', async ({
    browser,
  }) => {
    const adminCtx = await adminLogin();
    const invite = await mintInvite(adminCtx, 'cv12-owner');
    const owner = await registerUser(invite, 'owner');
    const channelName = `cv12-${Date.now().toString(36)}`;
    await createChannel(owner, channelName);

    const ctx = await browser.newContext();
    await attachToken(ctx, owner.token);
    const page = await ctx.newPage();

    await gotoCanvas(page, channelName);
    await createArtifactViaUI(page, 'CV-12 comment search proof');
    await expect(page.locator('[data-testid="cv5-artifact-comments"]')).toBeVisible();

    // Before any comment exists, the virtual channel is absent, so the search
    // box is intentionally not mounted (no channel id to search).
    await expect(page.locator('[data-testid="cv12-search-mount"]')).toHaveCount(0);

    // Post a comment via the real composer UI. The body carries a unique token
    // we will later search for (substring LIKE match on the server).
    const uniqueToken = `kumquat-${Date.now().toString(36)}`;
    const commentBody = `Please review the ${uniqueToken} section before sign-off.`;
    await postCommentViaUI(page, commentBody);

    // ── MOUNTED proof ── search box is now visible in the real DOM.
    const searchMount = page.locator('[data-testid="cv12-search-mount"]');
    await expect(searchMount).toBeVisible();
    const searchInput = page.locator('[data-cv12-search-input]');
    await expect(searchInput).toBeVisible();
    await expect(searchInput).toHaveAttribute('placeholder', '搜索评论...');

    // ── matching query ── type a real substring, click search, assert the
    // backend search hit renders in the real DOM (result-id anchor + body text).
    const searchResponse = page.waitForResponse(
      (res) =>
        res.request().method() === 'GET' &&
        /\/api\/v1\/channels\/[^/]+\/messages\/search/.test(new URL(res.url()).pathname),
    );
    await searchInput.fill(uniqueToken);
    await page.locator('[data-testid="cv12-search-submit"]').click();
    expect((await searchResponse).status()).toBe(200);

    const resultRows = page.locator('[data-cv12-search-result-id]');
    await expect(resultRows).toHaveCount(1);
    await expect(resultRows.first()).toContainText(uniqueToken);
    // No empty state while results are present.
    await expect(page.locator('[data-testid="cv12-no-result"]')).toHaveCount(0);

    // ── non-matching query ── absent string → byte-identical empty state.
    const noHitResponse = page.waitForResponse(
      (res) =>
        res.request().method() === 'GET' &&
        /\/api\/v1\/channels\/[^/]+\/messages\/search/.test(new URL(res.url()).pathname),
    );
    await searchInput.fill('zzz-no-such-comment-zzz');
    await page.locator('[data-testid="cv12-search-submit"]').click();
    expect((await noHitResponse).status()).toBe(200);

    const noResult = page.locator('[data-testid="cv12-no-result"]');
    await expect(noResult).toBeVisible();
    await expect(noResult).toHaveText(NO_RESULT_TEXT);
    await expect(page.locator('[data-cv12-search-result-id]')).toHaveCount(0);

    await ctx.close();
    await adminCtx.dispose();
    await owner.ctx.dispose();
  });
});
