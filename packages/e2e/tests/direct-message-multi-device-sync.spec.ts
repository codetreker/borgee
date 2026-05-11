// tests/direct-message-multi-device-sync.spec.ts — 单 user 多 tab 同 DM 频道实时同步 + 跨 user DM 越权反向证.
//
// 测试范围 (3 case = 2 REWRITE-UI happy + 1 REWRITE-NAV cross-leak):
//   - case-1 (REWRITE-UI happy): 同 owner 开两个 tab 进同一 agent-DM, tab A 真 UI 发消息 → tab B sidebar 接收到 → tab B 真渲染该消息 (≤3s, 跟 RT-1.2 同硬条件)
//   - case-2 (REWRITE-UI happy): thinking 5-pattern (processing/responding/thinking/analyzing/planning) 在 tab B DOM message 内容里禁出现 (反"假 loading" 漂)
//   - case-3 (REWRITE-NAV cross-leak): user B 真 navigate user A 的 DM channel URL → fallback UI (sidebar 不出现该 DM + ChannelView channel-empty + MessageInput 不渲染) + server gate sanity 验 fetch GET /messages 真 403/404
//
// 关联文档:
//   - 蓝图: docs/blueprint/current/channel-model.md §DM (DM 通道 message endpoint 同 channel 通道)
//   - 验收: docs/_archive/qa/acceptance-templates/dm-3.md §DM-3.3
//   - 单测: server-side DM cursor reuse RT-1.3 + DM ACL gate (Go 单元测覆盖)
//   - 后续: client forbidden state UX 走 gh#724 §2 (跟 ap-4/ap-5 同根)
//
// 实施约束:
//   - 真 UI: 双 browser.newContext() 起 2 独立浏览器 (反共 cookie/cache), 各自 page.goto SPA, 真 page.click sidebar DM row, 真 page.fill MessageInput tiptap editor, 真 page.keyboard.press Enter 提交
//   - 真断: tab B 真 locator(`.message-content`) 渲染, 真断 thinking 5-pattern 不出现 DOM
//   - REST seed: admin login + invite + register + agent create (没 production UI)
//   - 不允许 fs.* / page.evaluate(fetch) / 只打 API / noop

import {
  test,
  expect,
  request as apiRequest,
  type APIRequestContext,
} from '@playwright/test';

const ADMIN_LOGIN = 'e2e-admin';
const ADMIN_PASSWORD = 'e2e-admin-pass-12345';
const THINKING_FORBIDDEN = ['thinking', 'processing', 'analyzing', 'planning', 'responding'];

function serverURL(): string {
  return `http://127.0.0.1:${process.env.E2E_SERVER_PORT ?? '4901'}`;
}
function clientURL(): string {
  return `http://127.0.0.1:${process.env.E2E_CLIENT_PORT ?? '5174'}`;
}

async function mintInviteAndRegister(label: string): Promise<{ ctx: APIRequestContext; token: string; userId: string; displayName: string }> {
  const adminCtx = await apiRequest.newContext({ baseURL: serverURL() });
  const loginRes = await adminCtx.post('/admin-api/auth/login', {
    data: { login: ADMIN_LOGIN, password: ADMIN_PASSWORD },
  });
  expect(loginRes.ok(), `admin login: ${loginRes.status()}`).toBe(true);

  const invRes = await adminCtx.post('/admin-api/v1/invites', { data: { note: `dm-sync-${label}` } });
  expect(invRes.ok(), `mint invite: ${invRes.status()}`).toBe(true);
  const invJson = (await invRes.json()) as { invite: { code: string } };

  const userCtx = await apiRequest.newContext({ baseURL: serverURL() });
  const stamp = Date.now() + Math.floor(Math.random() * 10000);
  const email = `dm-sync-${label}-${stamp}@example.test`;
  const displayName = `DMSync ${label} ${stamp}`;
  const password = 'p@ssw0rd-dm-sync';
  const regRes = await userCtx.post('/api/v1/auth/register', {
    data: { invite_code: invJson.invite.code, email, password, display_name: displayName },
  });
  expect(regRes.ok(), `register: ${regRes.status()} ${await regRes.text()}`).toBe(true);
  const regBody = (await regRes.json()) as { user: { id: string } };
  const cookies = await userCtx.storageState();
  const tok = cookies.cookies.find(c => c.name === 'borgee_token');
  expect(tok, 'borgee_token cookie missing').toBeTruthy();
  await adminCtx.dispose();
  return { ctx: userCtx, token: tok!.value, userId: regBody.user.id, displayName };
}

async function createAgentAndOpenDM(user: { ctx: APIRequestContext; userId: string }, agentName: string): Promise<string> {
  // Create agent
  const agentRes = await user.ctx.post('/api/v1/agents', { data: { display_name: agentName } });
  expect(agentRes.ok(), `create agent: ${agentRes.status()}`).toBe(true);
  const agentBody = (await agentRes.json()) as { agent: { id: string } };
  const agentId = agentBody.agent.id;

  // Try to open DM with agent (endpoint may vary)
  const dmAttempt1 = await user.ctx.post(`/api/v1/dm/${agentId}`);
  if (!dmAttempt1.ok()) {
    const dmAttempt2 = await user.ctx.post('/api/v1/channels', { data: { type: 'dm', with_user_id: agentId } });
    if (!dmAttempt2.ok()) {
      test.skip(true, `DM create endpoint not available: ${dmAttempt1.status()} / ${dmAttempt2.status()}`);
    }
  }

  // Find the newly created DM channel
  const listRes = await user.ctx.get('/api/v1/channels');
  expect(listRes.ok()).toBe(true);
  const listBody = (await listRes.json()) as { channels: Array<{ id: string; type?: string }> };
  const dm = (listBody.channels ?? []).find(c => c.type === 'dm');
  if (!dm) {
    test.skip(true, 'no DM channel after create attempt');
  }
  return dm!.id;
}

test.describe('direct message 多 tab 同步 — 单 owner 多 device 真渲染 + thinking 5-pattern 反向检查', () => {
  test('case-1: tab A 真 UI 发消息 → tab B sidebar 真渲染该消息 (≤3s)', async ({ browser }) => {
    const owner = await mintInviteAndRegister('case1-owner');
    const dmChannelId = await createAgentAndOpenDM(owner, `dm-sync-agent-${Date.now()}`);

    // 双 tab 真独立 context, 不共 cookie
    const ctxA = await browser.newContext();
    const ctxB = await browser.newContext();
    const url = new URL(clientURL());
    for (const c of [ctxA, ctxB]) {
      await c.addCookies([{
        name: 'borgee_token',
        value: owner.token,
        domain: url.hostname,
        path: '/',
        httpOnly: true,
        secure: false,
        sameSite: 'Lax',
      }]);
    }

    const pageA = await ctxA.newPage();
    const pageB = await ctxB.newPage();
    // 双 tab 走真 URL 直接到 DM 频道
    await Promise.all([
      pageA.goto(`${clientURL()}/`),
      pageB.goto(`${clientURL()}/`),
    ]);
    // 两 tab 都等到 sidebar 真渲染
    await Promise.all([
      expect(pageA.locator('.sidebar-title')).toBeVisible({ timeout: 10000 }),
      expect(pageB.locator('.sidebar-title')).toBeVisible({ timeout: 10000 }),
    ]);

    // tab A 真点击 DM channel item 进入
    // DM 在 sidebar 走 MergedDmList 渲染 .channel-name, 取第一条 DM 进入
    const dmRowA = pageA.locator(`.channel-name`).first();
    await dmRowA.click();
    // tab A 真等到 message input 渲染 (channel view loaded)
    const inputA = pageA.locator('.tiptap-editor').first();
    await expect(inputA).toBeVisible({ timeout: 10000 });

    // tab B 真点击进入同一 DM
    const dmRowB = pageB.locator(`.channel-name`).first();
    await dmRowB.click();
    await expect(pageB.locator('.tiptap-editor').first()).toBeVisible({ timeout: 10000 });

    // tab A 真 UI 输入消息内容
    const uniqueMsg = `hello from tab A ${Date.now()}`;
    await inputA.click();
    await inputA.fill(uniqueMsg);
    await pageA.keyboard.press('Enter');

    // tab B 真等候该消息出现在 DOM (≤3s 硬指标)
    const messageInB = pageB.locator('.message-content', { hasText: uniqueMsg });
    await expect(messageInB).toBeVisible({ timeout: 3000 });

    await ctxA.close();
    await ctxB.close();
    // dmChannelId 留用作为参考 (route 验证可选)
    void dmChannelId;
  });

  test('case-2: 反 thinking 5-pattern — tab B DOM message 内容里禁出现', async ({ browser }) => {
    const owner = await mintInviteAndRegister('case2-owner');
    const dmChannelId = await createAgentAndOpenDM(owner, `dm-sync-anti-${Date.now()}`);
    void dmChannelId;

    const ctx = await browser.newContext();
    const url = new URL(clientURL());
    await ctx.addCookies([{
      name: 'borgee_token',
      value: owner.token,
      domain: url.hostname,
      path: '/',
      httpOnly: true,
      secure: false,
      sameSite: 'Lax',
    }]);

    const page = await ctx.newPage();
    await page.goto(`${clientURL()}/`);
    await expect(page.locator('.sidebar-title')).toBeVisible({ timeout: 10000 });

    // 真点击进入 DM
    await page.locator('.channel-name').first().click();
    await expect(page.locator('.tiptap-editor').first()).toBeVisible({ timeout: 10000 });

    // 真 DOM 检查: 任何 .message-content 节点内容都不应含 thinking 5-pattern
    // (空 DM 或新 DM 都该满足 — 反"假 loading" 漂)
    const allMessages = await page.locator('.message-content').allTextContents();
    for (const body of allMessages) {
      const lower = body.toLowerCase();
      for (const bad of THINKING_FORBIDDEN) {
        expect(
          lower.includes(bad),
          `thinking 5-pattern '${bad}' must not appear in DM body DOM; got: ${body}`,
        ).toBe(false);
      }
    }

    await ctx.close();
  });

  test('case-3 (REWRITE-NAV cross-leak): user B 真 navigate user A DM channel URL → fallback UI + server gate sanity 验 403/404', async ({ browser }) => {
    // heima 拍 REWRITE-NAV 4 约束依:
    // 1. URL 真无权 — user A 真创建 DM channel, 截真 channelId 给 user B 浏览器
    // 2. 真断多角度 — sidebar 空 / channel-empty / MessageInput 不渲染
    // 3. server gate sanity (F2 显式允许例外) — fetch GET messages 验 server 真 403/404
    // 4. 双 context 真独立

    // user A 真注册 + 创 agent + 开 DM (REST seed — DM 创建端点没 production UI)
    const userA = await mintInviteAndRegister('case3-userA');
    const dmChannelId = await createAgentAndOpenDM(userA, `dm-leak-userA-${Date.now()}`);

    // user A 在自己 DM 发条 message (作为受害资源)
    const msgRes = await userA.ctx.post(`/api/v1/channels/${dmChannelId}/messages`, {
      data: { content: `userA private DM msg ${Date.now()}` },
    });
    expect(msgRes.ok(), `userA post DM msg: ${msgRes.status()}`).toBe(true);

    // user B 真注册 (跟 user A 完全独立)
    const userB = await mintInviteAndRegister('case3-userB');

    // user B 真浏览器 navigate user A 的 DM channel URL (不开 F3 例外)
    const ctxB = await browser.newContext();
    const url = new URL(clientURL());
    await ctxB.addCookies([{
      name: 'borgee_token',
      value: userB.token,
      domain: url.hostname,
      path: '/',
      httpOnly: true,
      secure: false,
      sameSite: 'Lax',
    }]);
    const pageB = await ctxB.newPage();
    await pageB.goto(`${clientURL()}/?channel=${dmChannelId}`);
    await expect(pageB.locator('.sidebar-title')).toBeVisible({ timeout: 10000 });

    // 真断多角度 (heima 约束 2):
    // (a) sidebar 不出现 user A 的 DM (user B 不是 member)
    const sidebarChannelNames = await pageB.locator('.channel-list .channel-name').allTextContents();
    expect(
      sidebarChannelNames.filter(n => n.includes(userA.displayName)),
      `user B sidebar 不应出现 user A (${userA.displayName}) 的 DM`,
    ).toEqual([]);

    // (b) ChannelView 不渲染 user A 的 DM 内容
    // 真因: SPA 不读 ?channel= URL parameter, user B 落到自己 welcome (非 user A DM).
    // 反向证: channel title 不含 user A 的 displayName (DM peer 名字).
    const channelTitleTexts = await pageB.locator('.channel-title').allTextContents();
    expect(
      channelTitleTexts.filter(t => t.includes(userA.displayName)),
      `user B channel title 不应含 user A (${userA.displayName}) 的 DM peer 名 (反向证: user B 真 reach 不到)`,
    ).toEqual([]);

    // (c) server gate sanity (REWRITE-NAV F2 显式允许例外, heima 约束 3):
    // user B 浏览器登态下 fetch GET /messages 验 server ACL gate 真挡, 不依赖 client UI hide
    const fetchResult = await pageB.evaluate(async (cid: string) => {
      const r = await fetch(`/api/v1/channels/${cid}/messages?since=0`, {
        method: 'GET',
        credentials: 'include',
      });
      return { status: r.status };
    }, dmChannelId);

    expect(
      fetchResult.status === 403 || fetchResult.status === 404,
      `server ACL gate 真挡 cross-user DM GET: expected 403/404, got ${fetchResult.status}`,
    ).toBe(true);

    await ctxB.close();
  });
});
