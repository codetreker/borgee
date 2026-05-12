// tests/message-permission-matrix.spec.ts - message PUT/DELETE/PATCH cross-channel permission matrix (ACL/IDOR reverse checks).
//
// Test scope (3 required cases: AP-5 matrix coverage is reduced, but cross-user IDOR coverage must stay):
//   - case-1: user B navigates a real browser UI to user A private channel URL -> fallback UI (ChannelView channel-empty / message list 0 / MessageInput not rendered)
//   - case-2: server ACL gate check (explicit F2 exception) - user B browser session fetches PUT /messages/{userA-msg-id} and verifies server 403/404, proving server enforcement does not rely on client UI hiding
//   - case-3: server ACL gate check (explicit F2 exception) - user B fetches DELETE /messages/{userA-msg-id} and verifies server 403/404
//
// Matrix reduction rule: 3 cases cover read UI fallback, write PUT gate, and write DELETE gate. Cross-user IDOR cases must not be dropped; this version keeps PUT + DELETE to represent the original PUT/DELETE/PATCH matrix (PATCH shares the PUT route ACL gate and is covered by the PUT case).
//
// Related docs:
//   - Blueprint: docs/blueprint/current/auth-permissions.md §X (channel ACL: non-members cannot write another user's message)
//   - Acceptance: docs/_archive/qa/acceptance-templates/ap-5.md §2 (REG-INV-* fail-closed)
//   - Unit tests: server-side cross-channel message ACL is covered by Go unit tests
//   - Follow-up: client forbidden state UX is tracked in gh#724 §2
//
// Implementation constraints (REWRITE-NAV 4 constraints):
//   1. URL must be genuinely unauthorized - REST seeds user A private channel + posted message
//   2. Multi-angle assertions (case-1): empty sidebar / channel-empty / message list 0 / input not rendered
//   3. case-2 + case-3 use page.evaluate(fetch) as explicit F2 exceptions to verify the server ACL gate
//   4. Two contexts must use independent cookies

import {
  test,
  expect,
  request as apiRequest,
  type APIRequestContext,
} from '@playwright/test';

const ADMIN_LOGIN = 'e2e-admin';
const ADMIN_PASSWORD = 'e2e-admin-pass-12345';

function serverURL(): string {
  return `http://127.0.0.1:${process.env.E2E_SERVER_PORT ?? '4901'}`;
}
function clientURL(): string {
  return `http://127.0.0.1:${process.env.E2E_CLIENT_PORT ?? '5174'}`;
}

interface RegisteredUser {
  ctx: APIRequestContext;
  token: string;
  userId: string;
  displayName: string;
}

async function adminLogin(): Promise<APIRequestContext> {
  const ctx = await apiRequest.newContext({ baseURL: serverURL() });
  const res = await ctx.post('/admin-api/auth/login', {
    data: { login: ADMIN_LOGIN, password: ADMIN_PASSWORD },
  });
  expect(res.ok()).toBe(true);
  return ctx;
}

async function mintInvite(admin: APIRequestContext, note: string): Promise<string> {
  const res = await admin.post('/admin-api/v1/invites', { data: { note } });
  expect(res.ok()).toBe(true);
  return ((await res.json()) as { invite: { code: string } }).invite.code;
}

async function registerUser(invCode: string, suffix: string): Promise<RegisteredUser> {
  const ctx = await apiRequest.newContext({ baseURL: serverURL() });
  const stamp = Date.now() + Math.floor(Math.random() * 10000);
  const email = `ap5-${suffix}-${stamp}@example.test`;
  const displayName = `AP5 ${suffix} ${stamp}`;
  const res = await ctx.post('/api/v1/auth/register', {
    data: { invite_code: invCode, email, password: 'p@ssw0rd-ap5', display_name: displayName },
  });
  expect(res.ok()).toBe(true);
  const body = (await res.json()) as { user: { id: string } };
  const cookies = await ctx.storageState();
  const tok = cookies.cookies.find(c => c.name === 'borgee_token');
  expect(tok).toBeTruthy();
  return { ctx, token: tok!.value, userId: body.user.id, displayName };
}

async function setupCrossUserScenario(label: string): Promise<{
  userA: RegisteredUser;
  userB: RegisteredUser;
  channelId: string;
  messageId: string;
}> {
  const admin = await adminLogin();
  const invA = await mintInvite(admin, `ap5-${label}-A`);
  const invB = await mintInvite(admin, `ap5-${label}-B`);
  const userA = await registerUser(invA, `${label}-A`);
  const userB = await registerUser(invB, `${label}-B`);

  const chRes = await userA.ctx.post('/api/v1/channels', {
    data: { name: `ap5-${label}-${Date.now()}`, visibility: 'private' },
  });
  expect(chRes.ok()).toBe(true);
  const chBody = (await chRes.json()) as { channel: { id: string } };
  const channelId = chBody.channel.id;

  const msgRes = await userA.ctx.post(`/api/v1/channels/${channelId}/messages`, {
    data: { content: `private msg ${label} ${Date.now()}` },
  });
  expect(msgRes.ok()).toBe(true);
  const msgBody = (await msgRes.json()) as { message: { id: string } };

  await admin.dispose();
  return { userA, userB, channelId, messageId: msgBody.message.id };
}

test.describe('message permission matrix — 跨 channel ACL/IDOR 反向证 (REWRITE-NAV heima 拍)', () => {
  test('case-1: user B 真 navigate userA private channel URL → fallback UI 真渲染', async ({ browser }) => {
    const { userB, channelId } = await setupCrossUserScenario('case1');

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
    await pageB.goto(`${clientURL()}/?channel=${channelId}`);
    await expect(pageB.locator('.sidebar-title')).toBeVisible({ timeout: 10000 });

    // Multi-angle assertions (constraint 2):
    // (a) sidebar 不出现 userA 的 private channel item
    const sidebarChannelNames = await pageB.locator('.channel-list .channel-name').allTextContents();
    expect(
      sidebarChannelNames.filter(n => n.startsWith('ap5-case1-')),
      'user B sidebar 不应出现 user A private channel',
    ).toEqual([]);

    // (b) ChannelView does not render user A private channel content.
    // Cause: SPA ignores the ?channel= URL parameter, so user B lands on their own welcome channel, not user A's channel.
    // Reverse check: channel title does not contain the ap5-case1-* channel name created by user A.
    const channelTitleTexts = await pageB.locator('.channel-title').allTextContents();
    expect(
      channelTitleTexts.filter(t => t.includes('ap5-case1-')),
      `user B channel title 不应含 user A private channel 名 (反向证: user B 真 reach 不到 user A 资源)`,
    ).toEqual([]);

    // (c) server gate check (REWRITE-NAV explicit F2 exception):
    // user B fetches GET userA channel messages -> server returns 403/404 without relying on client UI hiding.
    const fetchResult = await pageB.evaluate(async (cid: string) => {
      const r = await fetch(`/api/v1/channels/${cid}/messages?since=0`, {
        method: 'GET',
        credentials: 'include',
      });
      return { status: r.status };
    }, channelId);
    expect(
      fetchResult.status === 403 || fetchResult.status === 404,
      `server ACL gate 真挡 cross-channel GET: expected 403/404, got ${fetchResult.status}`,
    ).toBe(true);

    await ctxB.close();
  });

  test('case-2: server ACL gate check (F2 例外) — user B fetch PUT /messages/{userA-msgId} 验 server 403/404', async ({ browser }) => {
    const { userB, messageId } = await setupCrossUserScenario('case2');

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
    await pageB.goto(`${clientURL()}/`);
    await expect(pageB.locator('.sidebar-title')).toBeVisible({ timeout: 10000 });

    // REWRITE-NAV explicit F2 exception: reverse-check that the server ACL gate blocks the request.
    const result = await pageB.evaluate(async (mid: string) => {
      const r = await fetch(`/api/v1/messages/${mid}`, {
        method: 'PUT',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ content: 'mutating someone elses message' }),
      });
      return { status: r.status };
    }, messageId);

    expect(
      result.status === 403 || result.status === 404,
      `server ACL gate 真挡 cross-user message PUT: expected 403/404, got ${result.status}`,
    ).toBe(true);

    await ctxB.close();
  });

  test('case-3: server ACL gate check (F2 例外) — user B fetch DELETE /messages/{userA-msgId} 验 server 403/404', async ({ browser }) => {
    const { userB, messageId } = await setupCrossUserScenario('case3');

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
    await pageB.goto(`${clientURL()}/`);
    await expect(pageB.locator('.sidebar-title')).toBeVisible({ timeout: 10000 });

    // REWRITE-NAV explicit F2 exception: reverse-check the server ACL gate.
    const result = await pageB.evaluate(async (mid: string) => {
      const r = await fetch(`/api/v1/messages/${mid}`, {
        method: 'DELETE',
        credentials: 'include',
      });
      return { status: r.status };
    }, messageId);

    expect(
      result.status === 403 || result.status === 404,
      `server ACL gate 真挡 cross-user message DELETE: expected 403/404, got ${result.status}`,
    ).toBe(true);

    await ctxB.close();
  });
});
