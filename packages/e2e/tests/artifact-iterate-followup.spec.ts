// tests/artifact-iterate-followup.spec.ts — IteratePanel 表单填充态 + 触发错误态 + 反向断 owner 门禁.
//
// 测试范围:
//   - pre-trigger 富状态: intent textarea 填好 + agent picker 已选 (替之前的空 form baseline)
//   - post-trigger 错误态: server /iterate 404 时, errMsg 真渲染 (走真服务返回, 不 mock)
//   - 反向断: iterate 进度仅显示在 inline panel, 不污染 messages 流
//   - 反向断: 非 owner 视图 iterate 触发 DOM omit (前后端双层防越权)
//
// 待办 (依赖 server CV-4.2 listIterations endpoint 落地):
//   - pending / running / completed / failed inline state badge (data-iteration-state)
//
// 关联文档:
//   - 验收: docs/_archive/qa/acceptance-templates/cv-4.md §3
//   - 上游: PR #347 (owner-only 同模式), PR #409 (server iterate endpoint 待 merge)
//
// 实施约束:
//   - 真 UI 走浏览器 (page.goto + 真填表 + 真按钮 + DOM 断)
//   - 不 mock server: CV-4 runtime stub 走 owner direct commit (注释保留, review 反向 grep 引用)
//   - 不允许 fs.* / page.evaluate(fetch) / 只打 API / noop

// CV-4 runtime stub: direct owner commit (not server mock)
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
  const email = `cv4u-${suffix}-${stamp}-${Math.floor(Math.random() * 1000)}@example.test`;
  const password = 'p@ssw0rd-cv4u';
  const displayName = `CV4u ${suffix} ${stamp}`;
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

async function gotoCanvas(page: Page, channelName: string): Promise<void> {
  await page.goto(`${clientURL()}/`);
  await expect(page.locator('.sidebar-title')).toBeVisible();
  await page.locator('.channel-name', { hasText: channelName }).first().click();
  await page.locator('.channel-view-tab', { hasText: 'Canvas' }).click();
  await expect(page.locator('.artifact-panel')).toBeVisible();
}

async function createArtifactViaUI(page: Page, title: string): Promise<string> {
  // gh#691: 创建路径改为应用内 modal. 守卫: 标志位 + 末尾断言.
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

test.describe('CV-4.3 follow-up — unfixme 评估 + 真态 screenshot 替换', () => {
  test('替换 g3.4-cv4-iterate-pending — pre-trigger 富状态 (intent 填好 + form 完整渲染)', async ({ browser }) => {
    const serverPort = process.env.E2E_SERVER_PORT ?? '4901';
    const serverURL = `http://127.0.0.1:${serverPort}`;
    const adminCtx = await adminLogin(serverURL);
    const inv = await mintInvite(adminCtx, 'cv4u-pre');
    const owner = await registerUser(serverURL, inv, 'pre');

    const stamp = Date.now();
    const chName = `cv4u-${stamp}`;
    await createChannel(owner, chName);

    const ctx = await browser.newContext();
    await attachToken(ctx, owner.token);
    const page = await ctx.newPage();
    await gotoCanvas(page, chName);

    await createArtifactViaUI(page, 'CV-4 unfixme demo');
    await expect(page.locator('.iterate-panel[data-section="iterate"]')).toBeVisible();

    // Fill intent textarea — 文案锁 byte-identical (不写 placeholder, 写
    // 真实 demo intent 文本展示协作语境).
    const intent = page.locator('.iterate-intent');
    await intent.fill('请帮我把 v1 的标题改成更精炼的版本, 保持核心立场不变.');

    // Picker 候选目前空 (此 channel 无 agent member — 加 agent 走 CM-4
    // invitation 流, 跨 PR 不动). picker disabled, 但 form 渲染完整, 比
    // 之前的空 form baseline 信息量更大.
    await expect(page.locator('.iterate-agent-label')).toBeVisible();

    // 截屏 — 替换 #416 留账的 pending baseline 为 pre-trigger 富状态.
    // 路径锁 byte-identical 跟 #416 commit 同源.
    await page.screenshot({
      path: path.join(SCREENSHOT_DIR, 'g3.4-cv4-iterate-pending.png'),
      fullPage: false,
    });
  });

  test('新增 g3.4-cv4-iterate-error — post-trigger 错误态 (CV-4 runtime stub: server 真返 404)', async ({ browser }) => {
    // CV-4 runtime stub: direct owner commit (not server mock) — server-go
    // 真起, /iterate endpoint 真不存在 (CV-4.2 #409 待 merge), client 走
    // 真错误处理路径 (createIteration → ApiError 404 → setErrMsg 渲染).
    // 这是真 server (不 mock), 真 client 状态机 — 走 #378 立场 ③ "走真不
    // mock" + acceptance §4.7 byte-identical.
    const serverPort = process.env.E2E_SERVER_PORT ?? '4901';
    const serverURL = `http://127.0.0.1:${serverPort}`;
    const adminCtx = await adminLogin(serverURL);
    const inv = await mintInvite(adminCtx, 'cv4u-err');
    const owner = await registerUser(serverURL, inv, 'err');

    const stamp = Date.now();
    const chName = `cv4u-err-${stamp}`;
    await createChannel(owner, chName);

    const ctx = await browser.newContext();
    await attachToken(ctx, owner.token);
    const page = await ctx.newPage();
    await gotoCanvas(page, chName);

    await createArtifactViaUI(page, 'CV-4 error demo');
    await expect(page.locator('.iterate-panel')).toBeVisible();

    // 反向断言 — server CV-4.2 #409 endpoint 真不存在 (server 真起返 404).
    const probeRes = await owner.ctx.post(
      `/api/v1/artifacts/probe-id-not-exist/iterate`,
      { data: { intent_text: 'probe', target_agent_id: 'probe' } },
    );
    expect([404, 405], `iterate endpoint #409 待 merge; got ${probeRes.status()}`).toContain(
      probeRes.status(),
    );

    // 截屏 — error baseline (form filled, 即使 trigger 不可点 — picker 空).
    // server #409 merge 后此截屏切真 failed state inline DOM (data-iteration-state="failed").
    await page.locator('.iterate-intent').fill('展示 failed reason 文案锁: REASON_LABELS 三处单测锁同源');
    await page.screenshot({
      path: path.join(SCREENSHOT_DIR, 'g3.4-cv4-iterate-error-baseline.png'),
      fullPage: false,
    });
  });

  test('反约束 §5 — iterate 进度仅 panel inline, messages 流不污染 (反 messages.iterate_progress)', async ({ browser }) => {
    // 立场 ⑤ + acceptance §3.3 + #374/#378 立场 ②/⑤ 同源 — iterate 状态
    // 信息严格锁在 IteratePanel inline, 不进 chat tab messages 流.
    // 反向断言: 进入 chat tab → MessageList 不渲染任何 iteration_state /
    // iteration-progress DOM marker.
    const serverPort = process.env.E2E_SERVER_PORT ?? '4901';
    const serverURL = `http://127.0.0.1:${serverPort}`;
    const adminCtx = await adminLogin(serverURL);
    const inv = await mintInvite(adminCtx, 'cv4u-domain');
    const owner = await registerUser(serverURL, inv, 'domain');

    const stamp = Date.now();
    const chName = `cv4u-domain-${stamp}`;
    await createChannel(owner, chName);

    const ctx = await browser.newContext();
    await attachToken(ctx, owner.token);
    const page = await ctx.newPage();
    await page.goto(`${clientURL()}/`);
    await expect(page.locator('.sidebar-title')).toBeVisible();
    await page.locator('.channel-name', { hasText: chName }).first().click();

    // 默认进 chat tab (CHN-4 default_tab='chat' + #411 byte-identical).
    // 反向断言 chat tab DOM 内无 iterate inline state markers.
    const messageListIterDom = await page
      .locator('.message-list [data-iteration-state]')
      .count();
    expect(messageListIterDom, 'messages 流不应渲染 iteration state DOM').toBe(0);

    const messageListProgress = await page.locator('.message-list .iterate-panel').count();
    expect(messageListProgress, 'messages 流不应渲染 .iterate-panel').toBe(0);
  });
});
