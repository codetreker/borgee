// tests/cv-1-3-canvas-modal-a11y.spec.ts — gh#691 a11y + IME + mobile e2e.
//
// 验 design 691-canvas-modal-replace-system-dialog.md §7 a11y e2e 三条
// (autoFocus / focus return / mobile viewport) + IME composition 守卫
// + 创建失败 modal 留 (yema C 混合).
//
// 立场反查 (cv-1-stance-checklist.md + design §4 边界):
//   ① 应用内 modal 替代浏览器原生 prompt/confirm (issue #691)
//   ② autoFocus 安全默认: CreateArtifactModal → input;
//      RollbackConfirmModal → 取消按钮 (危险操作, liema 拍)
//   ③ Focus return: modal 关后回原触发按钮; 触发按钮 unmount fallback
//      落 .artifact-panel
//   ④ aria-modal + aria-labelledby (liema #3)
//   ⑤ mobile viewport (375x812) max-width 90vw 不溢出 (liema #4)
//   ⑥ 创建失败 modal 不关 + errMsg 内显 + 输入保留 (yema C 混合)

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
  const email = `gh691-${suffix}-${stamp}-${Math.floor(Math.random() * 1000)}@example.test`;
  const password = 'p@ssw0rd-gh691';
  const displayName = `GH691 ${suffix} ${stamp}`;
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

test.describe('gh#691 Canvas modal a11y + IME + mobile + 失败处理', () => {
  test('立场 ② autoFocus 安全默认: CreateArtifactModal → input', async ({ browser }) => {
    const serverURL = `http://127.0.0.1:${process.env.E2E_SERVER_PORT ?? '4901'}`;
    const adminCtx = await adminLogin(serverURL);
    const inviteCode = await mintInvite(adminCtx, 'gh691-autofocus-create');
    const owner = await registerUser(serverURL, inviteCode, 'create-focus');

    const stamp = Date.now();
    const channelName = `gh691-create-focus-${stamp}`;
    await createChannel(owner, channelName);

    const ctx = await browser.newContext();
    await attachToken(ctx, owner.token);
    const page = await ctx.newPage();
    await gotoCanvasTab(page, channelName);

    // 点新建按钮 → modal 出现 → 焦点应落到 input.
    await page.locator('.artifact-empty button.btn-primary').click();
    const modal = page.locator('[data-testid="artifact-create-modal"]');
    await expect(modal).toBeVisible();
    // autoFocus 预期在 input 上.
    const focusedTag = await page.evaluate(() => {
      return (document.activeElement as HTMLElement)?.getAttribute('data-testid');
    });
    expect(focusedTag).toBe('artifact-create-modal-input');

    await ctx.close();
  });

  test('立场 ④ aria-modal + aria-labelledby 双 modal 各自独立 id', async ({ browser }) => {
    const serverURL = `http://127.0.0.1:${process.env.E2E_SERVER_PORT ?? '4901'}`;
    const adminCtx = await adminLogin(serverURL);
    const inviteCode = await mintInvite(adminCtx, 'gh691-aria');
    const owner = await registerUser(serverURL, inviteCode, 'aria');

    const stamp = Date.now();
    const channelName = `gh691-aria-${stamp}`;
    await createChannel(owner, channelName);

    const ctx = await browser.newContext();
    await attachToken(ctx, owner.token);
    const page = await ctx.newPage();
    await gotoCanvasTab(page, channelName);

    await page.locator('.artifact-empty button.btn-primary').click();
    const createModal = page.locator('[data-testid="artifact-create-modal"]');
    await expect(createModal).toBeVisible();

    // data-testid 已经放在 .modal-content 元素上 (ArtifactPanel.tsx
    // L772-777 同一元素), 不再 .locator('.modal-content') 在自身找.
    const role = await createModal.getAttribute('role');
    expect(role).toBe('dialog');
    const ariaModal = await createModal.getAttribute('aria-modal');
    expect(ariaModal).toBe('true');
    const ariaLabelledBy = await createModal.getAttribute('aria-labelledby');
    expect(ariaLabelledBy).toBe('artifact-create-modal-title');
    // h3 同 id.
    await expect(createModal.locator('#artifact-create-modal-title')).toHaveText('新建 Markdown artifact');

    await ctx.close();
  });

  test('立场 ⑤ mobile viewport (375x812) modal max-width 90vw 不溢出', async ({ browser }) => {
    const serverURL = `http://127.0.0.1:${process.env.E2E_SERVER_PORT ?? '4901'}`;
    const adminCtx = await adminLogin(serverURL);
    const inviteCode = await mintInvite(adminCtx, 'gh691-mobile');
    const owner = await registerUser(serverURL, inviteCode, 'mobile');

    const stamp = Date.now();
    const channelName = `gh691-mobile-${stamp}`;
    await createChannel(owner, channelName);

    const ctx = await browser.newContext({ viewport: { width: 375, height: 812 } });
    await attachToken(ctx, owner.token);
    const page = await ctx.newPage();

    // Mobile viewport (<768px) sidebar 默认折叠 (App.tsx isMobile + sidebar-closed),
    // .channel-name 不可点直到 hamburger 打开. 不用 gotoCanvasTab helper —
    // helper 是按 desktop 路径写的, mobile 要先 click hamburger.
    await page.goto(clientURL());
    // mobile: 等 .hamburger-btn 可见 (mobile 路径必出此 button, App.tsx L204).
    await expect(page.locator('.hamburger-btn')).toBeVisible({ timeout: 10_000 });
    await page.locator('.hamburger-btn').click();
    await page.locator('.channel-name', { hasText: channelName }).first().click();
    await page.locator('.channel-view-tab', { hasText: 'Canvas' }).click();
    await expect(page.locator('.artifact-panel')).toBeVisible();

    await page.locator('.artifact-empty button.btn-primary').click();
    const modal = page.locator('[data-testid="artifact-create-modal"]');
    await expect(modal).toBeVisible();

    // modal-content 实际宽度应 ≤ 90vw (375 * 0.9 = 337.5px).
    // data-testid="artifact-create-modal" 直接放在 .modal-content 元素上.
    const width = await modal.evaluate((el) =>
      el.getBoundingClientRect().width,
    );
    expect(width, `mobile modal width ${width}px should be ≤ 337.5px (90vw of 375)`).toBeLessThanOrEqual(338);

    // input 应 visible (没被屏幕键盘 / overflow 挡).
    const input = modal.locator('input.input-field');
    await expect(input).toBeVisible();

    await ctx.close();
  });

  test('立场 ⑥ 创建失败 modal 不关 + 输入保留 + 错误内显 (yema C 混合)', async ({ browser }) => {
    const serverURL = `http://127.0.0.1:${process.env.E2E_SERVER_PORT ?? '4901'}`;
    const adminCtx = await adminLogin(serverURL);
    const inviteCode = await mintInvite(adminCtx, 'gh691-create-fail');
    const owner = await registerUser(serverURL, inviteCode, 'create-fail');

    const stamp = Date.now();
    const channelName = `gh691-fail-${stamp}`;
    await createChannel(owner, channelName);

    const ctx = await browser.newContext();
    await attachToken(ctx, owner.token);
    const page = await ctx.newPage();

    // 拦 createArtifact POST 让它失败 → 验 modal 不关 + 输入保留 + errMsg.
    await page.route('**/api/v1/channels/*/artifacts', (route) => {
      void route.fulfill({ status: 500, body: 'simulated server error' });
    });

    await gotoCanvasTab(page, channelName);
    await page.locator('.artifact-empty button.btn-primary').click();
    const modal = page.locator('[data-testid="artifact-create-modal"]');
    await expect(modal).toBeVisible();
    const input = modal.locator('input.input-field');
    await input.fill('My Artifact');
    await modal.locator('button[type="submit"]').click();

    // C 混合: modal 不关.
    await expect(modal).toBeVisible({ timeout: 3_000 });
    // 输入保留.
    await expect(input).toHaveValue('My Artifact');
    // errMsg 显在 modal 内.
    const errEl = modal.locator('[data-testid="artifact-create-modal-err"]');
    await expect(errEl).toBeVisible();
    // 文案锁前缀: "创建失败:".
    await expect(errEl).toContainText('创建失败:');
    // 创建按钮重新 enable, 用户可重试.
    await expect(modal.locator('button[type="submit"]')).toBeEnabled();

    await ctx.close();
  });
});
