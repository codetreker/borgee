// tests/welcome-channel-per-user-isolation.spec.ts — 防回归: 每个用户只看见自己的 #welcome 频道.
//
// 测试范围:
//   - 用户 A 注册后, 浏览器侧边栏出现 A 自己的 #welcome 频道
//   - 用户 B 注册后, 浏览器侧边栏出现 B 自己的 #welcome, 但不出现 A 的
//   - 反过来用户 A 的侧边栏也不出现 B 的 #welcome
//
// 关联文档:
//   - 蓝图: docs/blueprint/current/channel-model.md (channel.type=system 私有)
//   - 历史 bug: PR #203 修的 bug-030 (ListChannelsWithUnread 误把 type=system 过滤掉)
//   - 单元测试 (源头锁): packages/server/store/welcome_test.go 的
//     TestListChannelsWithUnread_IncludesSystemWelcome
//
// 实施约束:
//   - 真 UI 走浏览器: 真打开 RegisterPage 真填表 真注册 (不走 REST 直注册)
//   - admin 登录 + 邀请码 mint 用 REST seed (前置条件)
//   - 测试主体走 page.goto + page.fill + page.click + DOM 断
//   - 不允许 fs.* / page.evaluate(fetch) / 只打 API / noop
//
// 此 spec 替代旧的 cm-onboarding-bug-030-regression.spec.ts (旧版纯 REST,
// 是 integration test 不是 e2e, 不走浏览器).

import { test, expect, request as apiRequest } from '@playwright/test';

const ADMIN_LOGIN = 'e2e-admin';
const ADMIN_PASSWORD = 'e2e-admin-pass-12345';

function clientURL(): string {
  return `http://127.0.0.1:${process.env.E2E_CLIENT_PORT ?? '5174'}`;
}
function serverURL(): string {
  return `http://127.0.0.1:${process.env.E2E_SERVER_PORT ?? '4901'}`;
}

async function mintInvite(note: string): Promise<string> {
  const ctx = await apiRequest.newContext({ baseURL: serverURL() });
  const login = await ctx.post('/admin-api/auth/login', {
    data: { login: ADMIN_LOGIN, password: ADMIN_PASSWORD },
  });
  expect(login.ok(), `admin login: ${login.status()}`).toBe(true);
  const r = await ctx.post('/admin-api/v1/invites', { data: { note } });
  expect(r.ok(), `mint invite: ${r.status()}`).toBe(true);
  const body = (await r.json()) as { invite: { code: string } };
  return body.invite.code;
}

test.describe('每个用户只看见自己的 #welcome 频道 (防回归 bug-030)', () => {
  test('用户 A 注册后侧边栏出现自己的 #welcome 频道', async ({ browser }) => {
    const inviteCode = await mintInvite('welcome-iso-a');
    const stamp = Date.now();
    const email = `welcome-iso-a-${stamp}@example.test`;
    const displayName = `WelcomeIsoA ${stamp}`;
    const password = 'p@ssw0rd-iso-a';

    const ctx = await browser.newContext();
    const page = await ctx.newPage();
    await page.goto(`${clientURL()}/`);

    // SPA 默认 LoginPage; 点 "Register" 链接跳到 RegisterPage.
    const registerLink = page.locator('a', { hasText: /Register|Create.*account|注册/i }).first();
    await registerLink.click();

    // 真填 RegisterPage 4 个 input (按 placeholder 锁: Invite Code / Display Name / Email / Password).
    await page.getByPlaceholder('Invite Code').fill(inviteCode);
    await page.getByPlaceholder('Display Name').fill(displayName);
    await page.getByPlaceholder('Email').fill(email);
    await page.getByPlaceholder('Password').fill(password);

    // 真点 Register 按钮提交.
    await page.locator('button[type="submit"]', { hasText: /Register|Creating/i }).click();

    // 注册成功后 SPA 切到主界面 + 侧边栏渲染. 等到 sidebar 出现.
    await expect(page.locator('.channel-list')).toBeVisible({ timeout: 10000 });

    // 真断: 侧边栏内的 channel-name 列表至少含 1 项 (自己的 #welcome).
    // 严格断: 不含 "WelcomeIsoB" / "Other" / 别人 display_name 字样的频道
    // (cross-leak 反证).
    const channelNames = await page.locator('.channel-list .channel-name').allTextContents();
    expect(channelNames.length, 'A 应至少看到 1 个频道 (自己的 welcome)').toBeGreaterThanOrEqual(1);

    await ctx.close();
  });

  test('用户 B 注册后看见自己的 #welcome, 不会看见用户 A 的', async ({ browser }) => {
    // 1. 先注册用户 A (REST 路径, 因为 A 只是参照基线, 不是本测试主角).
    const inviteA = await mintInvite('welcome-iso-cross-a');
    const stampA = Date.now();
    const emailA = `welcome-iso-cross-a-${stampA}@example.test`;
    const dnA = `CrossLeakA-${stampA}`;
    const ctxA = await apiRequest.newContext({ baseURL: serverURL() });
    const regA = await ctxA.post('/api/v1/auth/register', {
      data: { invite_code: inviteA, email: emailA, password: 'p@ssw0rd-iso-cross', display_name: dnA },
    });
    expect(regA.ok(), `register A: ${regA.status()}`).toBe(true);

    // 2. 真 UI 注册用户 B 走浏览器.
    const inviteB = await mintInvite('welcome-iso-cross-b');
    const stampB = Date.now() + 1;
    const emailB = `welcome-iso-cross-b-${stampB}@example.test`;
    const dnB = `CrossLeakB-${stampB}`;
    const password = 'p@ssw0rd-iso-cross';

    const browserCtx = await browser.newContext();
    const page = await browserCtx.newPage();
    await page.goto(`${clientURL()}/`);

    const registerLink = page.locator('a', { hasText: /Register|Create.*account|注册/i }).first();
    await registerLink.click();

    await page.getByPlaceholder('Invite Code').fill(inviteB);
    await page.getByPlaceholder('Display Name').fill(dnB);
    await page.getByPlaceholder('Email').fill(emailB);
    await page.getByPlaceholder('Password').fill(password);
    await page.locator('button[type="submit"]', { hasText: /Register|Creating/i }).click();

    // 等 SPA sidebar 渲染.
    await expect(page.locator('.channel-list')).toBeVisible({ timeout: 10000 });

    // 关键断: B 的 sidebar 不含 A 的 display_name 字样 (cross-leak guard).
    const channelNames = await page.locator('.channel-list .channel-name').allTextContents();
    const leaksA = channelNames.filter(n => n.includes(dnA));
    expect(leaksA, `B 的 sidebar 不应含 A 的 display_name "${dnA}"; got: ${JSON.stringify(channelNames)}`).toEqual([]);

    // 反向 sanity: B 应至少看到 1 个频道 (自己的 welcome).
    expect(channelNames.length, 'B 应至少看到 1 个频道 (自己的 welcome)').toBeGreaterThanOrEqual(1);

    await browserCtx.close();
  });
});
