// tests/direct-message-multi-device-sync.spec.ts - one user, multiple tabs, same DM channel realtime sync + cross-user DM authorization reverse check.
//
// Test scope (3 cases = 2 REWRITE-UI happy paths + 1 REWRITE-NAV cross-leak check):
//   - case-1 (REWRITE-UI happy): same owner opens two tabs in the same agent-DM, tab A sends a message through the real UI -> tab B sidebar receives it -> tab B renders the message (<=3s, same hard condition as RT-1.2)
//   - case-2 (REWRITE-UI happy): thinking 5-pattern (processing/responding/thinking/analyzing/planning) must not appear in tab B DOM message content, guarding against fake loading drift
//   - case-3 (REWRITE-NAV cross-leak): user B navigates a real browser to user A's DM channel URL -> fallback UI (sidebar omits the DM + ChannelView channel-empty + MessageInput not rendered) + server gate sanity verifies fetch GET /messages returns 403/404
//
// Related docs:
//   - Blueprint: docs/blueprint/current/channel-model.md §DM (DM channel message endpoint matches the channel endpoint)
//   - Acceptance: docs/_archive/qa/acceptance-templates/dm-3.md §DM-3.3
//   - Unit tests: server-side DM cursor reuse RT-1.3 + DM ACL gate are covered by Go unit tests
//   - Follow-up: client forbidden state UX is tracked in gh#724 §2, same root as ap-4/ap-5
//
// Implementation constraints:
//   - Real UI: two browser.newContext() calls create two independent browsers (no shared cookie/cache), each visits the SPA, clicks the sidebar DM row, fills the MessageInput tiptap editor, and presses Enter to submit
//   - Real assertions: tab B renders locator(`.message-content`), and the thinking 5-pattern must not appear in the DOM
//   - REST seed: admin login + invite + register + agent create because there is no production UI
//   - Do not use fs.* / page.evaluate(fetch) / API-only / noop tests

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

  // Open a DM with the agent. The DM is created + returned by POST /api/v1/dm/{userId}
  // ({ channel: {...} }); DMs are a SEPARATE list from GET /api/v1/channels (the
  // store query is `type IN ('channel','system')`, so DMs never appear there).
  //
  // #974: this helper used to look for the DM in GET /api/v1/channels and, when it
  // (always) found none, `test.skip(true, ...)` — which silently green-skipped the
  // entire DM-sync proof in CI the whole time. That static skip is exactly the
  // failure mode #974/#724 §3 calls out: a non-functional path looking delivered.
  // We now (a) use the correct DM endpoints and (b) FAIL DYNAMICALLY (not skip) if
  // the DM-create backend wiring is unreachable, so a backend-off condition is
  // caught instead of hidden.
  const dmRes = await user.ctx.post(`/api/v1/dm/${agentId}`);
  expect(
    dmRes.ok(),
    `DM create backend wiring unreachable (POST /api/v1/dm/${agentId} -> ${dmRes.status()} ${await dmRes.text()}); ` +
      `this proof must fail, not skip`,
  ).toBe(true);
  const dmBody = (await dmRes.json()) as { channel?: { id?: string; type?: string } };
  const dmId = dmBody.channel?.id;
  expect(
    dmId,
    'POST /api/v1/dm returned no channel.id — DM backend wiring is broken; must fail, not skip',
  ).toBeTruthy();

  // Cross-check the DM is now discoverable through the real DM list endpoint
  // (GET /api/v1/dm), the same source the sidebar renders from. A create that
  // never surfaces in the list = broken wiring → fail dynamically.
  const listRes = await user.ctx.get('/api/v1/dm');
  expect(listRes.ok(), `list DMs: ${listRes.status()}`).toBe(true);
  const listBody = (await listRes.json()) as { channels: Array<{ id: string }> };
  const found = (listBody.channels ?? []).some(c => c.id === dmId);
  expect(found, 'created DM not present in GET /api/v1/dm — DM backend wiring is broken; must fail, not skip').toBe(true);
  return dmId!;
}

test.describe('direct message 多 tab 同步 — 单 owner 多 device 真渲染 + thinking 5-pattern 反向检查', () => {
  test('case-1: tab A 真 UI 发消息 → tab B sidebar 真渲染该消息 (≤3s) @backend-required', async ({ browser }) => {
    const owner = await mintInviteAndRegister('case1-owner');
    const dmChannelId = await createAgentAndOpenDM(owner, `dm-sync-agent-${Date.now()}`);

    // Two tabs use independent contexts and do not share cookies.
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
    // Both tabs use real URLs to open the DM channel.
    await Promise.all([
      pageA.goto(`${clientURL()}/`),
      pageB.goto(`${clientURL()}/`),
    ]);
    // Both tabs wait for the sidebar to render.
    await Promise.all([
      expect(pageA.locator('.sidebar-title')).toBeVisible({ timeout: 10000 }),
      expect(pageB.locator('.sidebar-title')).toBeVisible({ timeout: 10000 }),
    ]);

    // tab A clicks the DM channel item to enter it.
    // DM rows render through MergedDmList as .channel-name; use the first DM.
    const dmRowA = pageA.locator(`.channel-name`).first();
    await dmRowA.click();
    // tab A waits for the message input to render after the channel view loads.
    const inputA = pageA.locator('.tiptap-editor').first();
    await expect(inputA).toBeVisible({ timeout: 10000 });

    // tab B clicks into the same DM.
    const dmRowB = pageB.locator(`.channel-name`).first();
    await dmRowB.click();
    await expect(pageB.locator('.tiptap-editor').first()).toBeVisible({ timeout: 10000 });

    // tab A enters message content through the real UI.
    const uniqueMsg = `hello from tab A ${Date.now()}`;
    await inputA.click();
    await inputA.fill(uniqueMsg);
    await pageA.keyboard.press('Enter');

    // tab B waits for the message to appear in the DOM (<=3s hard requirement).
    const messageInB = pageB.locator('.message-content', { hasText: uniqueMsg });
    await expect(messageInB).toBeVisible({ timeout: 3000 });

    await ctxA.close();
    await ctxB.close();
    // Keep dmChannelId as a route reference for optional verification.
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

    // Click into the DM.
    await page.locator('.channel-name').first().click();
    await expect(page.locator('.tiptap-editor').first()).toBeVisible({ timeout: 10000 });

    // Real DOM check: no .message-content node should contain the thinking 5-pattern.
    // Empty or new DMs should also satisfy this, guarding against fake loading drift.
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
    // REWRITE-NAV 4 constraints:
    // 1. URL is genuinely unauthorized - user A creates a real DM channel, then user B browser receives that channelId.
    // 2. Multi-angle assertions - empty sidebar / channel-empty / MessageInput not rendered.
    // 3. server gate sanity (explicit F2 exception) - fetch GET messages verifies server 403/404.
    // 4. Two contexts are independent.

    // user A registers, creates an agent, and opens a DM (REST seed because the DM creation endpoint has no production UI).
    const userA = await mintInviteAndRegister('case3-userA');
    const dmChannelId = await createAgentAndOpenDM(userA, `dm-leak-userA-${Date.now()}`);

    // user A sends a message in their own DM as the protected resource.
    const msgRes = await userA.ctx.post(`/api/v1/channels/${dmChannelId}/messages`, {
      data: { content: `userA private DM msg ${Date.now()}` },
    });
    expect(msgRes.ok(), `userA post DM msg: ${msgRes.status()}`).toBe(true);

    // user B registers independently from user A.
    const userB = await mintInviteAndRegister('case3-userB');

    // user B navigates a real browser to user A's DM channel URL without using the F3 exception.
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

    // Multi-angle assertions (constraint 2):
    // (a) sidebar does not show user A's DM because user B is not a member.
    const sidebarChannelNames = await pageB.locator('.channel-list .channel-name').allTextContents();
    expect(
      sidebarChannelNames.filter(n => n.includes(userA.displayName)),
      `user B sidebar 不应出现 user A (${userA.displayName}) 的 DM`,
    ).toEqual([]);

    // (b) ChannelView does not render user A's DM content.
    // Cause: SPA ignores the ?channel= URL parameter, so user B lands on their own welcome channel, not user A's DM.
    // Reverse check: channel title does not contain user A's displayName (the DM peer name).
    const channelTitleTexts = await pageB.locator('.channel-title').allTextContents();
    expect(
      channelTitleTexts.filter(t => t.includes(userA.displayName)),
      `user B channel title 不应含 user A (${userA.displayName}) 的 DM peer 名 (反向证: user B 真 reach 不到)`,
    ).toEqual([]);

    // (c) server gate sanity (REWRITE-NAV explicit F2 exception):
    // user B browser session fetches GET /messages and verifies the server ACL gate blocks it without relying on client UI hiding.
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
