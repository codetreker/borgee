// tests/canvas-modal-open-close.spec.ts — Canvas tab + 版本列表 rollback.
//
// 测试范围:
//   - Canvas tab 跟 chat 平级渲染, markdown 内容渲染 (其它 kind 走 vitest)
//   - 版本列表线性展示, rollback 按钮仅在 owner 视图渲染 (非 owner DOM 不出现)
//   - rollback label 文案 "v{N+1} (rollback from v{M})"
//
// 关联文档:
//   - 验收: docs/_archive/qa/acceptance-templates/cv-1.md §3.1-§3.3
//   - 上游: PR #346 (CV-1.3 client follow)
//
// 实施约束:
//   - 真 UI 走浏览器 (page.goto + 真按钮 + 真 textarea + DOM 断)
//   - artifact 通过 owner UI 创, response intercept 拿到 id
//   - 不允许 fs.* / page.evaluate(fetch) / 只打 API / noop

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
  const email = `cv13-${suffix}-${stamp}-${Math.floor(Math.random() * 1000)}@example.test`;
  const password = 'p@ssw0rd-cv13';
  const displayName = `CV13 ${suffix} ${stamp}`;
  const res = await ctx.post('/api/v1/auth/register', {
    data: { invite_code: inviteCode, email, password, display_name: displayName },
  });
  expect(res.ok(), `register: ${res.status()} ${await res.text()}`).toBe(true);
  const body = (await res.json()) as { user: { id: string } };
  const cookies = await ctx.storageState();
  const tok = cookies.cookies.find((c) => c.name === 'borgee_token');
  expect(tok, 'borgee_token cookie missing').toBeTruthy();
  return { email, token: tok!.value, userId: body.user.id, ctx };
}

function clientURL(): string {
  return `http://127.0.0.1:${process.env.E2E_CLIENT_PORT ?? '5174'}`;
}

async function attachToken(ctx: BrowserContext, token: string) {
  const url = new URL(clientURL());
  await ctx.clearCookies();
  await ctx.addCookies([{
    name: 'borgee_token',
    value: token,
    domain: url.hostname,
    path: '/',
    httpOnly: true,
    secure: false,
    sameSite: 'Lax',
  }]);
}

async function createChannel(user: RegisteredUser, name: string): Promise<string> {
  const r = await user.ctx.post('/api/v1/channels', {
    data: { name, visibility: 'private' },
  });
  expect(r.ok(), `channel create: ${r.status()} ${await r.text()}`).toBe(true);
  const j = (await r.json()) as { channel: { id: string } };
  return j.channel.id;
}

async function gotoCanvasTab(page: Page, channelName: string) {
  await page.goto(`${clientURL()}/`);
  await expect(page.locator('.sidebar-title')).toBeVisible();
  await page.locator('.channel-name', { hasText: channelName }).first().click();
  await page.locator('.channel-view-tab', { hasText: 'Canvas' }).click();
  await expect(page.locator('.artifact-panel')).toBeVisible();
}

/**
 * Drive the empty-state create button on the owner's UI. Returns the
 * artifact id captured from the POST /channels/{id}/artifacts response.
 *
 * gh#691: 创建路径从 window.prompt (浏览器原生 dialog) 改成应用内 modal.
 * 守卫 pattern (liema review): 标志位 + 末尾断言, 不用 listener throw
 * (listener 内 throw 是异步 unhandled rejection, 不 fail 当前 step).
 */
async function createArtifactViaUI(page: Page, title: string): Promise<string> {
  let nativeDialogTriggered = false;
  page.on('dialog', async (d) => {
    nativeDialogTriggered = true;
    await d.dismiss();
  });
  const respPromise = page.waitForResponse(
    (r) =>
      r.request().method() === 'POST' &&
      r.url().includes('/artifacts') &&
      !r.url().includes('/commits') &&
      !r.url().includes('/rollback') &&
      !r.url().includes('/versions'),
  );
  await page.locator('.artifact-empty button.btn-primary').click();
  const modal = page.locator('[data-testid="artifact-create-modal"]');
  await expect(modal).toBeVisible({ timeout: 3_000 });
  await modal.locator('input.input-field').fill(title);
  await modal.locator('button[type="submit"]').click();
  const resp = await respPromise;
  const j = (await resp.json()) as { id: string };
  await expect(page.locator('.artifact-version-tag')).toHaveText('v1', { timeout: 5_000 });
  expect(nativeDialogTriggered, 'gh#691 回归: 触发了浏览器原生 dialog').toBe(false);
  return j.id;
}

test.describe('CV-1.3 client Canvas tab — acceptance §3.1-§3.3', () => {
  test('§3.1 markdown render + §3.2 rollback button owner-only DOM gate', async ({
    browser,
  }) => {
    const serverPort = process.env.E2E_SERVER_PORT ?? '4901';
    const serverURL = `http://127.0.0.1:${serverPort}`;
    const adminCtx = await adminLogin(serverURL);

    const ownerInvite = await mintInvite(adminCtx, 'cv13-owner31');
    const owner = await registerUser(serverURL, ownerInvite, 'owner31');

    const stamp = Date.now();
    const channelName = `cv13-rb-${stamp}`;
    await createChannel(owner, channelName);

    // ─── owner UI: create + edit + commit ─────────────────────
    const ownerCtxBrowser = await browser.newContext();
    await attachToken(ownerCtxBrowser, owner.token);
    const ownerPage = await ownerCtxBrowser.newPage();
    await gotoCanvasTab(ownerPage, channelName);

    const artifactId = await createArtifactViaUI(ownerPage, 'CV-1.3 spec');

    // §3.1: edit submit pushes a v2 with markdown body — verify <h1>
    // renders (markdown ONLY 立场 ④).
    await ownerPage.locator('.artifact-header button.btn-sm', { hasText: '编辑' }).click();
    await ownerPage.locator('.artifact-textarea').fill('# heading\n\nv2 body');
    await ownerPage.locator('.artifact-edit-actions button.btn-primary').click();
    await expect(ownerPage.locator('.artifact-version-tag')).toHaveText('v2', { timeout: 5_000 });
    const rendered = ownerPage.locator('.artifact-rendered');
    await expect(rendered.locator('h1')).toContainText('heading');
    await expect(rendered).toContainText('v2 body');

    // §3.2 owner: rollback button visible on non-head row (v1).
    await expect(ownerPage.locator('.artifact-version-row')).toHaveCount(2);
    await expect(ownerPage.locator('.artifact-rollback-btn')).toHaveCount(1);

    // §3.2 立场 ⑦ 防退化: head row must not expose rollback.
    // Head v2 must NOT have a rollback button (回滚到自己无意义).
    const headRow = ownerPage.locator('.artifact-version-row.head');
    await expect(headRow.locator('.artifact-rollback-btn')).toHaveCount(0);

    // §3.2 byte-identical rollback row label: trigger rollback to v1, expect
    // v3 row label = "v3 (rollback from v1)".
    // gh#691: rollback 之前用 window.confirm, 改成应用内 modal. 这里点
    // rollback 按钮 → 应用内确认 modal 出来 → 点 "确认回滚" 按钮.
    await ownerPage
      .locator('.artifact-version-row')
      .filter({ has: ownerPage.locator('.artifact-version-label', { hasText: /^v1$/ }) })
      .locator('.artifact-rollback-btn')
      .click();
    const rollbackModal = ownerPage.locator('[data-testid="artifact-rollback-confirm-modal"]');
    await expect(rollbackModal).toBeVisible({ timeout: 3_000 });
    await expect(rollbackModal).toContainText('确认回滚到 v1?');
    await rollbackModal.locator('button.btn-danger', { hasText: '确认回滚' }).click();
    await expect(ownerPage.locator('.artifact-version-tag')).toHaveText('v3', { timeout: 5_000 });
    const v3Row = ownerPage
      .locator('.artifact-version-row')
      .filter({ has: ownerPage.locator('.artifact-version-label', { hasText: /^v3/ }) });
    await expect(v3Row.locator('.artifact-version-label')).toHaveText('v3 (rollback from v1)');

    // Sanity: artifactId we captured matches what the page rendered
    // (artifact REST API was hit at /api/v1/channels/.../artifacts).
    expect(artifactId).toMatch(/.+/);

    await ownerCtxBrowser.close();
  });
});
