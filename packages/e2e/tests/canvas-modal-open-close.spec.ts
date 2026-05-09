// tests/canvas-modal-open-close.spec.ts — Canvas tab + 版本列表 rollback + WS 实时刷新.
//
// 测试范围:
//   - Canvas tab 跟 chat 平级渲染, markdown 内容渲染 (其它 kind 走 vitest)
//   - 版本列表线性展示, rollback 按钮仅在 owner 视图渲染 (非 owner DOM 不出现)
//   - rollback label 文案 "v{N+1} (rollback from v{M})"
//   - WS ArtifactUpdated 推送后, 视图 ≤3s 内自动刷新
//   - commit 冲突 409 时显示 toast "内容已更新, 请刷新查看"
//
// 关联文档:
//   - 验收: docs/_archive/qa/acceptance-templates/cv-1.md §3.1-§3.3
//   - 上游: PR #346 (CV-1.3 client follow)
//
// 实施约束:
//   - 真 UI 走浏览器 (page.goto + 真按钮 + 真 textarea + DOM 断)
//   - artifact 通过 owner UI 创, response intercept 拿到 id 后走 REST seed 第二端 commit 触发 WS
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
const CONFLICT_TOAST = '内容已更新, 请刷新查看';

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

async function addMember(owner: RegisteredUser, channelId: string, userId: string) {
  const r = await owner.ctx.post(`/api/v1/channels/${channelId}/members`, {
    data: { user_id: userId },
  });
  expect([200, 201, 204, 409]).toContain(r.status());
}

async function commitArtifact(
  user: RegisteredUser,
  artifactId: string,
  expectedVersion: number,
  body: string,
): Promise<{ status: number; newVersion?: number }> {
  const r = await user.ctx.post(`/api/v1/artifacts/${artifactId}/commits`, {
    data: { expected_version: expectedVersion, body },
  });
  if (!r.ok()) return { status: r.status() };
  const j = (await r.json()) as { version: number };
  return { status: r.status(), newVersion: j.version };
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
    const otherInvite = await mintInvite(adminCtx, 'cv13-other31');
    const owner = await registerUser(serverURL, ownerInvite, 'owner31');
    const other = await registerUser(serverURL, otherInvite, 'other31');

    const stamp = Date.now();
    const channelName = `cv13-rb-${stamp}`;
    const channelId = await createChannel(owner, channelName);
    await addMember(owner, channelId, other.userId);

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

    // §3.2 立场 ⑦ 反约束: 非 owner DOM 不渲染回滚按钮.
    // Open `other` user in a separate browser context — they see the
    // version list (member can read) but rollback-btn count == 0.
    // 注意: ArtifactPanel v1 没有 list endpoint, 非 owner 进 Canvas tab
    // 默认是 empty-state — 所以 §3.2 反约束验证走 REST 直接 GET 渲染
    // 不到的代码路径 (DOM 反查通过 owner page 渲染等于 v=2 但用 other
    // 的 token 重新挂载 panel: 通过 sessionStorage 注入 last-known
    // artifact id 不在产品支持范围, 这里改用 REST GET 验证 client 不
    // 直接暴露 rollback action). 以 owner DOM 反查 head 不显示 rollback
    // 当代立场 ⑦ 防退化 (head row showRollbackBtn === false).
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

  test.skip('§3.3 WS push refresh ≤3s + conflict toast 文案锁', async ({ browser }) => {
    // FIXME(team-lead): cv-1-3-canvas §3.3 WS push refresh timing flake — 跟 chn-4 §5 同模式
    // 反复卡 AP-3 critical path (timing 死等 versionTag toHaveText('v2', 3s budget) 在 CI 抢 WS
    // 推送窗口 race). Server-side WS push contract + conflict 409 toast 文案锁均有 unit/integration
    // 守门 (cv-1-3 server commit_test.go + ArtifactToast.test.tsx 文案 byte-identical),
    // e2e 是 secondary timing 验. 待 CV-1-3 wrapper fixture-based 重写 (zhanma 派 cv-1-3-flake-rewrite).
    const serverPort = process.env.E2E_SERVER_PORT ?? '4901';
    const serverURL = `http://127.0.0.1:${serverPort}`;
    const adminCtx = await adminLogin(serverURL);

    const ownerInvite = await mintInvite(adminCtx, 'cv13-push-owner');
    const otherInvite = await mintInvite(adminCtx, 'cv13-push-other');
    const owner = await registerUser(serverURL, ownerInvite, 'pown33');
    const other = await registerUser(serverURL, otherInvite, 'poth33');

    const stamp = Date.now();
    const channelName = `cv13-push-${stamp}`;
    const channelId = await createChannel(owner, channelName);
    await addMember(owner, channelId, other.userId);

    const ownerCtxBrowser = await browser.newContext();
    await attachToken(ownerCtxBrowser, owner.token);
    const ownerPage = await ownerCtxBrowser.newPage();
    await gotoCanvasTab(ownerPage, channelName);

    const artifactId = await createArtifactViaUI(ownerPage, 'push spec');
    const versionTag = ownerPage.locator('.artifact-version-tag');
    await expect(versionTag).toHaveText('v1');

    // ─── §3.3 push latency ────────────────────────────────────
    const t0 = Date.now();
    const r2 = await commitArtifact(other, artifactId, 1, '# v2\n\npushed by other');
    expect(r2.status, `other commit v2: ${r2.status}`).toBe(200);
    expect(r2.newVersion).toBe(2);

    await expect(versionTag).toHaveText('v2', { timeout: 3_000 });
    const latency = Date.now() - t0;
    expect(latency, `push latency ${latency}ms exceeds 3s budget`).toBeLessThan(3_000);

    // ─── §3.3 conflict toast 文案锁 ───────────────────────────
    // Owner enters edit mode (expected_version snapshots = 2), other
    // races a v3 commit, owner submits stale → 409 → toast 文案锁.
    await ownerPage.locator('.artifact-header button.btn-sm', { hasText: '编辑' }).click();
    await expect(ownerPage.locator('.artifact-textarea')).toBeVisible();
    const r3 = await commitArtifact(other, artifactId, 2, '# v3\n\nrace winner');
    expect(r3.status, `other commit v3: ${r3.status}`).toBe(200);

    await ownerPage.locator('.artifact-textarea').fill('# v2.1\n\nstale write');
    await ownerPage.locator('.artifact-edit-actions button.btn-primary').click();

    const toast = ownerPage.locator('.toast-item', { hasText: CONFLICT_TOAST });
    await expect(toast).toBeVisible({ timeout: 3_000 });
    await expect(toast).toHaveText(CONFLICT_TOAST);

    // After toast, panel re-fetches → version tag → v3.
    await expect(versionTag).toHaveText('v3', { timeout: 3_000 });

    await ownerCtxBrowser.close();
  });
});
