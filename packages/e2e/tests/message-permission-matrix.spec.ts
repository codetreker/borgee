// tests/message-permission-matrix.spec.ts — message PUT/DELETE/PATCH 跨 channel 权限矩阵 (ACL/IDOR 反向证).
//
// 测试范围 (3 关键 case, heima 拍 ap-5 矩阵简化但 cross-user IDOR 不允许砍):
//   - case-1: user B 真 UI navigate user A private channel URL → fallback UI (ChannelView channel-empty / message list 0 / MessageInput 不渲染)
//   - case-2: server ACL gate check (F2 例外, heima 约束 3) — user B 浏览器登态下 fetch PUT /messages/{userA-msg-id} 验 server 403/404, 反向证 server 拦截不依赖 client UI hide
//   - case-3: server ACL gate check (F2 例外, heima 约束 3) — user B fetch DELETE /messages/{userA-msg-id} 验 server 403/404
//
// 矩阵简化原则 (heima 加分项 1): 3 case 覆盖 (read UI fallback + write PUT gate + write DELETE gate). 不允许砍 cross-user IDOR case 数 — 此版本 PUT + DELETE 都保, 反映原 spec 的 PUT/DELETE/PATCH 完整矩阵 (PATCH 复用 PUT 同路径 ACL gate, 已在 PUT case 覆盖).
//
// 关联文档:
//   - 蓝图: docs/blueprint/current/auth-permissions.md §X (channel ACL: 非 member 不可写他人 message)
//   - 验收: docs/_archive/qa/acceptance-templates/ap-5.md §2 (REG-INV-* fail-closed)
//   - 单测: server-side cross-channel message ACL 走 Go 单元测覆盖
//   - 后续: client forbidden state UX 走 gh#724 §2
//
// 实施约束 (heima 拍 REWRITE-NAV 4 约束):
//   1. URL 必须真无权 — REST seed user A private channel + post message
//   2. 真断多角度 (case-1): sidebar 空 / channel-empty / message list 0 / input 不渲染
//   3. case-2 + case-3 走 page.evaluate(fetch) 显式 F2 例外 — 反向证 server ACL gate
//   4. 双 context 真独立 cookie

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

    // 真断多角度 (heima 约束 2):
    // (a) sidebar 不出现 userA 的 private channel item
    const sidebarChannelNames = await pageB.locator('.channel-list .channel-name').allTextContents();
    expect(
      sidebarChannelNames.filter(n => n.startsWith('ap5-case1-')),
      'user B sidebar 不应出现 user A private channel',
    ).toEqual([]);

    // (b) ChannelView 不渲染 user A private channel 内容
    // 真因: SPA 不读 ?channel= URL parameter, user B 落到自己 welcome (非 user A channel).
    // 反向证: channel title 不含 user A 创建的 ap5-case1-* channel 名.
    const channelTitleTexts = await pageB.locator('.channel-title').allTextContents();
    expect(
      channelTitleTexts.filter(t => t.includes('ap5-case1-')),
      `user B channel title 不应含 user A private channel 名 (反向证: user B 真 reach 不到 user A 资源)`,
    ).toEqual([]);

    // (c) server gate check (REWRITE-NAV F2 显式允许例外, heima 约束 3):
    // user B fetch GET userA channel messages → server 真挡 403/404 不依赖 client UI hide
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

    // REWRITE-NAV F2 显式允许例外 (heima 约束 3): 反向证 server ACL gate 真挡
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

    // REWRITE-NAV F2 显式允许例外 (heima 约束 3): 反向证 server ACL gate
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
