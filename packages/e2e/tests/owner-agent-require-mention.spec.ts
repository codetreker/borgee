// tests/owner-agent-require-mention.spec.ts — owner My Agents 页
// "仅 @mention 时响应" toggle 真浏览器 + 反 IDOR endpoint 直打.
//
// 测试范围:
//   1. owner 登录 → My Agents → toggle 真 click → UI 翻动
//      + GET /api/v1/agents/{id} 回 require_mention=new_value (DB 真值对账)
//   2. 反 IDOR: 第二个 user PATCH 第一个 user 的 agent endpoint 直打 →
//      4xx + DB 不变
//
// 不在范围:
//   - admin Edit User Modal (#157 删了, 这次不补; 跟用户拍的对齐)
//   - ChannelMentionControls (per-channel override, 不是全局默认)
//
// 真浏览器 (memory `e2e_no_curl_only_ui`): UI 步骤走 click; 反 IDOR
// 单独做后端 contract 检验时, 走 apiRequest.newContext 直打 endpoint
// 模拟攻击者绕过前端, 这是 anti-IDOR 真测的标准做法 (浏览器 UI 走过
// happy path 后, 后端 authz 单独验; 跟 e2e_full_smoke_regression
// memory 立场一致).

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
  const email = `req-mention-${suffix}-${stamp}-${Math.floor(Math.random() * 1000)}@example.test`;
  const password = 'p@ssw0rd-req-mention';
  const displayName = `ReqMention ${suffix} ${stamp}`;
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

async function attachToken(ctx: BrowserContext, baseURL: string, token: string) {
  const url = new URL(baseURL);
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

async function createAgent(
  serverURL: string,
  ownerToken: string,
  displayName: string,
): Promise<{ id: string; require_mention: boolean }> {
  const ctx = await apiRequest.newContext({
    baseURL: serverURL,
    extraHTTPHeaders: { Cookie: `borgee_token=${ownerToken}` },
  });
  const r = await ctx.post('/api/v1/agents', { data: { display_name: displayName } });
  expect(r.ok() || r.status() === 201, `agent create: ${r.status()}`).toBe(true);
  const body = (await r.json()) as { agent: { id: string; require_mention: boolean } };
  return { id: body.agent.id, require_mention: body.agent.require_mention };
}

async function getAgent(
  serverURL: string,
  token: string,
  agentID: string,
): Promise<{ ok: boolean; status: number; require_mention?: boolean }> {
  const ctx = await apiRequest.newContext({
    baseURL: serverURL,
    extraHTTPHeaders: { Cookie: `borgee_token=${token}` },
  });
  const r = await ctx.get(`/api/v1/agents/${agentID}`);
  if (!r.ok()) return { ok: false, status: r.status() };
  const body = (await r.json()) as { agent: { require_mention: boolean } };
  return { ok: true, status: r.status(), require_mention: body.agent.require_mention };
}

async function openAgentManage(page: Page) {
  await page.goto('/');
  await expect(
    page.locator('.hamburger-btn, .sidebar-title').first(),
  ).toBeVisible({ timeout: 10_000 });
  const hamburger = page.locator('.hamburger-btn');
  const isMobile = await hamburger.count() > 0 && await hamburger.isVisible();
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
  const manageBtn = page.locator('.agent-card button.btn-sm', { hasText: 'Manage' }).first();
  await manageBtn.click();
  await expect(page.locator('.agent-detail-card-mention')).toBeVisible({ timeout: 5_000 });
}

const SERVER_URL = `http://127.0.0.1:${process.env.E2E_SERVER_PORT ?? '4901'}`;

test.describe('Owner My Agents — require_mention toggle (PATCH /api/v1/agents/{id})', () => {
  test('owner click 翻动 + GET /api/v1/agents/{id} DB 真值对账', async ({ page, baseURL }) => {
    const adminCtx = await adminLogin(SERVER_URL);
    const inviteCode = await mintInvite(adminCtx, 'req-mention-happy');
    const owner = await registerUser(SERVER_URL, inviteCode, 'owner');
    const agent = await createAgent(SERVER_URL, owner.token, `req-mention-${Date.now().toString(36)}`);

    // 默认 require_mention=true (跟 schema 默认对齐).
    expect(agent.require_mention).toBe(true);

    await attachToken(page.context(), baseURL!, owner.token);
    await page.setViewportSize({ width: 1280, height: 800 });
    await openAgentManage(page);

    const toggle = page.locator('[data-testid="agent-require-mention-toggle"]');
    await expect(toggle).toBeVisible();
    await expect(toggle).toBeChecked();

    // 真 click → 翻成 false.
    await toggle.click();
    await expect(toggle).not.toBeChecked({ timeout: 5_000 });

    // DB 真值对账 (反"UI 翻了 DB 没翻").
    const got1 = await getAgent(SERVER_URL, owner.token, agent.id);
    expect(got1.ok).toBe(true);
    expect(got1.require_mention).toBe(false);

    // 翻回 true.
    await toggle.click();
    await expect(toggle).toBeChecked({ timeout: 5_000 });
    const got2 = await getAgent(SERVER_URL, owner.token, agent.id);
    expect(got2.require_mention).toBe(true);
  });

  test('反 IDOR: 第二个 user 直打 PATCH endpoint → 4xx + DB 不变', async () => {
    const adminCtx = await adminLogin(SERVER_URL);
    const inviteCode1 = await mintInvite(adminCtx, 'req-mention-idor-victim');
    const inviteCode2 = await mintInvite(adminCtx, 'req-mention-idor-attacker');
    const victim = await registerUser(SERVER_URL, inviteCode1, 'victim');
    const attacker = await registerUser(SERVER_URL, inviteCode2, 'attacker');

    const victimAgent = await createAgent(SERVER_URL, victim.token, `idor-${Date.now().toString(36)}`);
    const before = await getAgent(SERVER_URL, victim.token, victimAgent.id);
    expect(before.require_mention).toBe(true);

    // attacker 用自己的 cookie 直打 victim 的 agent — 这是 anti-IDOR 真测.
    const attackerCtx = await apiRequest.newContext({
      baseURL: SERVER_URL,
      extraHTTPHeaders: { Cookie: `borgee_token=${attacker.token}` },
    });
    const r = await attackerCtx.patch(`/api/v1/agents/${victimAgent.id}`, {
      data: { require_mention: false },
    });
    // server 把 cross-owner 折成 404 (反存在性泄露). 4xx 即可, 404 优先.
    expect(r.status(), `expected 4xx (404 preferred), got ${r.status()}`).toBeGreaterThanOrEqual(400);
    expect(r.status()).toBeLessThan(500);

    // DB 没变.
    const after = await getAgent(SERVER_URL, victim.token, victimAgent.id);
    expect(after.require_mention).toBe(true);
  });
});
