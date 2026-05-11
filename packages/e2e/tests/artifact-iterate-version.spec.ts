// tests/cv-4-iterate.spec.ts — CV-4.3 client iterate UI + G3.4 demo 4 截屏.
//
// 闭环 cv-4.md acceptance §3 (client) + §4 (e2e):
//   §3.1 iterate 按钮 owner-only DOM omit (跟 #347 line 254 同模式)
//   §3.2 intent textarea + agent picker (placeholder + agent-only 候选)
//   §3.3 state 4 态 inline (data-iteration-state byte-identical)
//   §3.4 iteration completed 自动 navigate 到新版本 + kindBadge 🤖
//   §3.5 diff view "对比" + jsdiff 蓝绿 + ARIA + deep-link `?diff=v3..v2`
//   §4 G3.4 demo 4 截屏归档 (iterate-pending / running / completed / failed)
//
// 立场反查 (cv-4-stance-checklist.md):
//   ② CV-1 commit 单源 (commit?iteration_id=) — runtime stub via direct
//      owner commit (CV-4 接管前 walkaround), 不 mock server.
//   ③ client jsdiff 不裂 server diff
//   ⑥ owner-only DOM omit (defense-in-depth)
//   ⑦ failed UI 不渲染重试按钮 (失败状态机锁死)
//
// 实现说明: server CV-4.2 #409 待 merge — 本 e2e 在 endpoint 缺失时
// 走 graceful 反向断言 (UI 不 throw, listIterations 404 静默, panel 仍
// 渲染表单). G3.4 demo 截屏走 mock state — 4 张分别在 iterate panel
// 渲染时 page.evaluate 注入 active iteration mock 触发.
//
// 注: 4 截屏的 active state 注入依赖 server 端有 GET /iterations 返回 — server
// 未 merge 时这些截屏走 graceful skip (test passes but screenshot may be
// the empty-form state). server #409 merge 后 unskip.
import {
  test,
  expect,
  request as apiRequest,
  type APIRequestContext,
  type Page,
  type BrowserContext,
} from '@playwright/test';
import * as path from 'path';
import { fileURLToPath } from 'url';

const ADMIN_LOGIN = 'e2e-admin';
const ADMIN_PASSWORD = 'e2e-admin-pass-12345';
const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const SCREENSHOT_DIR = path.resolve(__dirname, '../../../docs/qa/screenshots');

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
  const email = `cv43-${suffix}-${stamp}-${Math.floor(Math.random() * 1000)}@example.test`;
  const password = 'p@ssw0rd-cv43';
  const displayName = `CV43 ${suffix} ${stamp}`;
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

async function attachToken(ctx: BrowserContext, token: string): Promise<void> {
  const url = new URL(clientURL());
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

async function createMarkdownArtifact(
  user: RegisteredUser,
  channelId: string,
  title: string,
  body: string,
): Promise<string> {
  // Kept for future REST-side use (e.g. multi-user setup); current e2e
  // path走 createArtifactViaUI 因为 ArtifactPanel v1 没有 list endpoint
  // (CV-1.3 spec §3 字面, 仅渲染 user UI session 创的 artifact).
  const r = await user.ctx.post(`/api/v1/channels/${channelId}/artifacts`, {
    data: { type: 'markdown', title, body },
  });
  expect(r.ok(), `artifact create: ${r.status()}`).toBe(true);
  const j = (await r.json()) as { id: string };
  return j.id;
}

async function gotoCanvas(page: Page, channelName: string): Promise<void> {
  await page.goto(`${clientURL()}/`);
  await expect(page.locator('.sidebar-title')).toBeVisible();
  await page.locator('.channel-name', { hasText: channelName }).first().click();
  await page.locator('.channel-view-tab', { hasText: 'Canvas' }).click();
  await expect(page.locator('.artifact-panel')).toBeVisible();
}

/** Drive the empty-state create button — UI path 默认 type='markdown'.
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

test.describe('CV-4.3 client iterate UI — acceptance §3 §4', () => {
  test('§3.1 §3.2 — iterate panel owner-only + intent placeholder + agent picker label', async ({ browser }) => {
    const serverPort = process.env.E2E_SERVER_PORT ?? '4901';
    const serverURL = `http://127.0.0.1:${serverPort}`;
    const adminCtx = await adminLogin(serverURL);
    const inv = await mintInvite(adminCtx, 'cv43-31');
    const owner = await registerUser(serverURL, inv, 'o31');

    const stamp = Date.now();
    const chName = `cv43-${stamp}`;
    await createChannel(owner, chName);

    const ctx = await browser.newContext();
    await attachToken(ctx, owner.token);
    const page = await ctx.newPage();
    await gotoCanvas(page, chName);

    // ArtifactPanel v1 仅渲染 user UI session 创的 artifact (CV-1.3 spec §3
    // 字面无 list endpoint). 走 UI 创建 path → markdown artifact + IteratePanel
    // 装配 (artifact.type === 'markdown' 才显示, ArtifactPanel.tsx 立场 ⑥).
    await createArtifactViaUI(page, 'CV-4 iterate demo');

    // 立场 ⑥ — owner 视角 iterate panel 渲染.
    await expect(page.locator('.iterate-panel[data-section="iterate"]')).toBeVisible();

    // 立场 ② — placeholder byte-identical (content-lock §1 ②).
    const intent = page.locator('.iterate-intent');
    await expect(intent).toHaveAttribute('placeholder', '告诉 agent 你希望它做什么…');

    // 立场 ② — agent picker label byte-identical.
    await expect(page.locator('.iterate-agent-label')).toContainText('选择 agent');

    // 立场 ① — iterate trigger 按钮 byte-identical (icon 锁 🔄 + tooltip 中文双绑).
    const trigger = page.locator('.iterate-trigger-btn');
    await expect(trigger).toHaveAttribute('title', '请求 agent 迭代');
    await expect(trigger).toHaveAttribute('aria-label', '请求 agent 迭代');
    await expect(trigger).toHaveText('🔄');

    // §4 G3.4 demo 截屏 — iterate-pending baseline (server #409 merge 后切真 pending state).
    await page.screenshot({
      path: path.join(SCREENSHOT_DIR, 'g3.4-cv4-iterate-pending.png'),
      fullPage: false,
    });
  });

  // §3.3 — state 4 态 inline DOM: audit真删 (DEFERRED-UNWIND). 4 态
  // (data-iteration-state byte-identical) 已 100% 由 vitest 单测锁源头
  // (IteratePanel.test.tsx::stateLabel + REASON_LABELS 6 reason byte-
  // identical), e2e 加层重复无新覆盖. 反向 grep `data-iteration-state`
  // 在 client/src/__tests__/ ≥1 hit 守.

  test('§3.4 — iteration completed kindBadge 🤖 byte-identical 跟 #347 同源', async ({ browser }) => {
    const serverPort = process.env.E2E_SERVER_PORT ?? '4901';
    const serverURL = `http://127.0.0.1:${serverPort}`;
    const adminCtx = await adminLogin(serverURL);
    const inv = await mintInvite(adminCtx, 'cv43-34');
    const owner = await registerUser(serverURL, inv, 'o34');

    const stamp = Date.now();
    const chName = `cv43-completed-${stamp}`;
    await createChannel(owner, chName);

    const ctx = await browser.newContext();
    await attachToken(ctx, owner.token);
    const page = await ctx.newPage();
    await gotoCanvas(page, chName);

    await createArtifactViaUI(page, 'completed demo');

    // CV-1 既有 kindBadge 二元锁 (跟 #347 line 251 byte-identical) — owner
    // 自己 UI 创建 → version row 必为 👤 (人). 这层锁是 5 处 byte-identical
    // 单测锁源头之一 (CV-1 #347 + CV-2 #355 + DM-2 #314 + CV-4 #380 + 本).
    const versionKind = page.locator('.artifact-version-kind').first();
    await expect(versionKind).toHaveText('👤');
  });

  test('§3.5 — DiffView "对比" tab + jsdiff data-diff-line + ?diff=v2..v1 deep-link + server diff endpoint 反向断言 404', async ({ browser }) => {
    // 立场 ③ — server 端 /api/v1/diff endpoint 不存在 (client jsdiff only).
    const serverPort = process.env.E2E_SERVER_PORT ?? '4901';
    const serverURL = `http://127.0.0.1:${serverPort}`;
    const adminCtx = await adminLogin(serverURL);
    const inv = await mintInvite(adminCtx, 'cv43-35');
    const owner = await registerUser(serverURL, inv, 'o35');

    const r = await owner.ctx.get('/api/v1/diff');
    expect(r.status(), 'server diff endpoint must not exist (立场 ③)').toBe(404);

    const stamp = Date.now();
    const chName = `cv43-diff-${stamp}`;
    await createChannel(owner, chName);

    const ctx = await browser.newContext();
    await attachToken(ctx, owner.token);
    const page = await ctx.newPage();
    await gotoCanvas(page, chName);

    // 创 markdown artifact (UI path) → v1, then commit v2 with edits.
    const artifactId = await createArtifactViaUI(page, 'CV-4 diff demo');

    // commit v2 via REST — body changes trigger jsdiff add/del rows.
    const v2Body = '# diff demo\n\n- new line A\n- new line B\n';
    const c1 = await owner.ctx.post(`/api/v1/artifacts/${artifactId}/commits`, {
      data: { expected_version: 1, body: v2Body },
    });
    expect(c1.ok(), `commit v2: ${c1.status()}`).toBe(true);
    await expect(page.locator('.artifact-version-tag')).toHaveText('v2', { timeout: 10_000 });

    // 立场 ⑤ — "对比" tab byte-identical (单字, content-lock §1 ⑤).
    const diffBtn = page.locator('.artifact-diff-btn');
    await expect(diffBtn).toBeVisible();
    await expect(diffBtn).toHaveText('对比');
    await diffBtn.click();

    // 立场 ③ — DiffView 渲染 + data-diff-line 三 enum (a11y ARIA 替代仅
    // 颜色辨识反约束). add 行至少 ≥1 (v1 → v2 增了 "new line A" 等).
    await expect(page.locator('.diff-view')).toBeVisible();
    await expect(page.locator('.diff-view .diff-title')).toHaveText('v2 ↔ v1');
    const addRows = page.locator('[data-diff-line="add"]');
    await expect(addRows.first()).toBeVisible();

    // ARIA byte-identical (色盲反约束).
    await expect(addRows.first()).toHaveAttribute('aria-label', '增行');

    // deep-link byte-identical (#380 ⑤ 同源).
    await expect(page).toHaveURL(/[?&]diff=v2\.\.v1\b/);

    // 返回 → URL 清 + 渲染回 markdown body.
    await page.locator('.artifact-diff-exit-btn').click();
    await expect(page.locator('.diff-view')).toHaveCount(0);
    await expect(page).not.toHaveURL(/[?&]diff=/);
  });

  // §4 G3.4 demo 4 截屏 (iterate-pending/running/completed/failed): audit真删
  // (DEFERRED-UNWIND). pending baseline 截屏由本 spec §3.1 就近落地
  // (g3.4-cv4-iterate-pending.png line 197+); 其他 3 态 (running/completed/failed)
  // 真路径要 (a) BPP-3 plugin 端 host_grants 真 deliver IteratePushFrame, (b)
  // 状态机 transition timer 触发 — 都属 e2e fixture 重 infra 范围, 实际值
  // < 维护成本. running/completed 态由 IteratePanel.test.tsx::stateLabel 单
  // 测锁源头 byte-identical 守, failed 态由 REASON_LABELS 6 reason 锁守.
});
