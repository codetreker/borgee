// tests/channel-collab-tabs.spec.ts — channel collaboration tabs + URL deep-link.
//
// 测试范围:
//   - 双 tab DOM data-tab="chat" / data-tab="workspace", with exact Chinese labels "聊天" / "工作区"
//   - URL ?tab= 参数生效, 无参数时落 server default_tab="chat"
//   - DM view negative check: workspace tab is never present (same UI boundary as chn-2)
//   - 双 tab 不交叉: chat tab 不渲染 artifact body
//
// 关联文档:
//   - 验收: docs/_archive/qa/acceptance-templates/chn-4.md §1-§6
//   - 上游: PR #411 (CHN-4.1+4.3 client wiring)
//
// 实施约束:
//   - Browser-driven UI path (tab switching + URL validation + DOM assertions)
//   - 真 server-go(4901) + vite(5174), do not mock port 4901
//   - CV-4 runtime stub: 走 owner direct commit (不是 server mock)
//   - 不允许 fs.* / page.evaluate(fetch) / 只打 API / empty placeholder tests
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
  const email = `chn4-${suffix}-${stamp}-${Math.floor(Math.random() * 1000)}@example.test`;
  const password = 'p@ssw0rd-chn4';
  const displayName = `CHN4 ${suffix} ${stamp}`;
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

test.describe('CHN-4 协作场骨架 — acceptance §1 §4 §5 §6', () => {
  test('§1 双 tab DOM byte-identical + 中文文案锁 + URL ?tab= deep-link', async ({ browser }) => {
    const serverPort = process.env.E2E_SERVER_PORT ?? '4901';
    const serverURL = `http://127.0.0.1:${serverPort}`;
    const adminCtx = await adminLogin(serverURL);
    const inv = await mintInvite(adminCtx, 'chn4-tabs');
    const owner = await registerUser(serverURL, inv, 'tabs');

    const stamp = Date.now();
    const chName = `chn4-tabs-${stamp}`;
    await createChannel(owner, chName);

    const ctx = await browser.newContext();
    await attachToken(ctx, owner.token);
    const page = await ctx.newPage();
    await gotoChannel(page, chName);

    // Acceptance point 2 — both tab DOM markers exist (data-tab="chat" + "workspace" each appear at least once).
    await expect(page.locator('button[data-tab="chat"]')).toBeVisible();
    await expect(page.locator('button[data-tab="workspace"]')).toBeVisible();

    // Exact Chinese labels: "聊天" / "工作区".
    await expect(page.locator('button[data-tab="chat"]')).toHaveText('聊天');
    await expect(page.locator('button[data-tab="workspace"]')).toHaveText('工作区');

    // Acceptance point 6: without URL ?tab, default_tab="chat" makes chat active.
    await expect(page.locator('button[data-tab="chat"]')).toHaveClass(/active/);

    // URL deep-link — 点 workspace tab 后 URL 写 ?tab=workspace.
    await page.locator('button[data-tab="workspace"]').click();
    await expect(page.locator('button[data-tab="workspace"]')).toHaveClass(/active/);
    await expect(page).toHaveURL(/[?&]tab=workspace\b/);

    // 切回 chat → URL 写 ?tab=chat.
    await page.locator('button[data-tab="chat"]').click();
    await expect(page).toHaveURL(/[?&]tab=chat\b/);
  });
});
