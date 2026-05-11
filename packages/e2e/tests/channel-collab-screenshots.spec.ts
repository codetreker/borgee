// tests/channel-collab-screenshots.spec.ts — G3.4 协作场 5 张截屏归档 (PM 签字依据).
//
// 测试范围:
//   - g3.4-chn4-collab-skeleton-overview.png — 主路径全景 (PM 签字主图, fullPage)
//   - g3.4-chn4-dual-tab-chat.png — "聊天" tab 激活态 fullPage
//   - g3.4-chn4-dual-tab-workspace.png — "工作区" tab 激活态 fullPage
//   - g3.4-chn4-followup-dm-no-handle.png — 已落地 (PR #423)
//   - g3.4-chn4-followup-cross-org-isolation.png — 已落地 (PR #423)
//
// 关联文档:
//   - 验收: docs/_archive/qa/acceptance-templates/chn-4.md §6 (G3.4 截屏依据)
//   - 文案: "聊天" / "工作区" 中文跟 client/server/锁定文档保持一致
//
// 实施约束:
//   - 真 UI 走浏览器 (page.goto + 真 tab 切换 + page.screenshot 入 git)
//   - 真 server-go(4901) + vite(5174), 不 mock
//   - 跟 G2.4 / G2.5 / G2.6 demo 截屏同模式
//   - 不允许 PS 后期修改截屏 / fs.* / page.evaluate(fetch) / 只打 API / noop
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
  const email = `g34-shot-${suffix}-${stamp}-${Math.floor(Math.random() * 1000)}@example.test`;
  const password = 'p@ssw0rd-g34-shot';
  const displayName = `G34Shot ${suffix} ${stamp}`;
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

async function gotoChannel(page: Page, channelName: string): Promise<void> {
  await page.goto(`${clientURL()}/`);
  await expect(page.locator('.sidebar-title')).toBeVisible();
  await page.locator('.channel-name', { hasText: channelName }).first().click();
  await expect(page.locator('.channel-view-tabs')).toBeVisible();
}

test.describe('CHN-4 G3.4 5 张截屏 follow-up — 野马 PM 双 tab + 边界态文案锁字面验', () => {
  test('§1 协作场骨架 overview — 主路径 demo 截屏 (野马 PM 签字主图)', async ({ browser }) => {
    const serverPort = process.env.E2E_SERVER_PORT ?? '4901';
    const serverURL = `http://127.0.0.1:${serverPort}`;
    const adminCtx = await adminLogin(serverURL);
    const inv = await mintInvite(adminCtx, 'g34-overview');
    const owner = await registerUser(serverURL, inv, 'overview');

    const stamp = Date.now();
    const chName = `chn4-overview-${stamp}`;
    await createChannel(owner, chName);

    const ctx = await browser.newContext();
    await attachToken(ctx, owner.token);
    const page = await ctx.newPage();
    await gotoChannel(page, chName);

    // 字面验 byte-identical (chn-4-content-lock ①):
    //   "聊天" / "工作区" 中文 byte-identical
    await expect(page.locator('button[data-tab="chat"]')).toHaveText('聊天');
    await expect(page.locator('button[data-tab="workspace"]')).toHaveText('工作区');

    // 立场 ⑥ default_tab="chat" — 进入无 URL ?tab 时 chat active.
    await expect(page.locator('button[data-tab="chat"]')).toHaveClass(/active/);

    // 主路径截屏 (fullPage 截 sidebar + 主区, 野马 PM demo 签字主图).
    await page.screenshot({
      path: path.join(SCREENSHOT_DIR, 'g3.4-chn4-collab-skeleton-overview.png'),
      fullPage: true,
    });
  });

  test('§2 dual-tab chat fullPage — "聊天" tab byte-identical', async ({ browser }) => {
    const serverPort = process.env.E2E_SERVER_PORT ?? '4901';
    const serverURL = `http://127.0.0.1:${serverPort}`;
    const adminCtx = await adminLogin(serverURL);
    const inv = await mintInvite(adminCtx, 'g34-chat');
    const owner = await registerUser(serverURL, inv, 'chat');

    const stamp = Date.now();
    const chName = `chn4-chat-${stamp}`;
    await createChannel(owner, chName);

    const ctx = await browser.newContext();
    await attachToken(ctx, owner.token);
    const page = await ctx.newPage();
    await gotoChannel(page, chName);

    // chat tab default active (字面验 byte-identical "聊天").
    await expect(page.locator('button[data-tab="chat"]')).toHaveText('聊天');
    await expect(page.locator('button[data-tab="chat"]')).toHaveClass(/active/);

    // URL deep-link 显式锁 ?tab=chat (cd 通过 click 行为已锁; 此处验默认无 ?tab).
    await page.screenshot({
      path: path.join(SCREENSHOT_DIR, 'g3.4-chn4-dual-tab-chat.png'),
      fullPage: true,
    });
  });

  test('§3 dual-tab workspace fullPage — "工作区" tab byte-identical', async ({ browser }) => {
    const serverPort = process.env.E2E_SERVER_PORT ?? '4901';
    const serverURL = `http://127.0.0.1:${serverPort}`;
    const adminCtx = await adminLogin(serverURL);
    const inv = await mintInvite(adminCtx, 'g34-workspace');
    const owner = await registerUser(serverURL, inv, 'workspace');

    const stamp = Date.now();
    const chName = `chn4-workspace-${stamp}`;
    await createChannel(owner, chName);

    const ctx = await browser.newContext();
    await attachToken(ctx, owner.token);
    const page = await ctx.newPage();
    await gotoChannel(page, chName);

    // 切到 workspace tab — URL ?tab=workspace deep-link.
    await page.locator('button[data-tab="workspace"]').click();
    await expect(page.locator('button[data-tab="workspace"]')).toHaveClass(/active/);
    await expect(page.locator('button[data-tab="workspace"]')).toHaveText('工作区');
    await expect(page).toHaveURL(/[?&]tab=workspace\b/);

    await page.screenshot({
      path: path.join(SCREENSHOT_DIR, 'g3.4-chn4-dual-tab-workspace.png'),
      fullPage: true,
    });
  });

});
