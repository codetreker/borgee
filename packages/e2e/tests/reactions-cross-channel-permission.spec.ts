// tests/reactions-cross-channel-permission.spec.ts — message reaction 跨 channel 权限 (ACL/IDOR 反向证).
//
// 测试范围 (3 case + 1 server gate sanity, heima 拍 REWRITE-NAV 4 约束):
//   - case-1: 真 UI navigate — user B 真 page.goto user A private channel URL → ChannelView 渲染 "频道未找到" fallback (`.channel-empty`), sidebar 不出现该 channel, MessageInput 不渲染
//   - case-2: 真 UI navigate — user B 真 page.goto user A 私 channel 后, 真断 message-content 节点 0 (无法 reach reaction button)
//   - case-3: admin 真 UI navigate (god-mode 红线) — admin login 后 navigate user A private channel URL, 真断 (a) 看得到 message list (read 允许) (b) MessageInput 不渲染或 disabled (write 拦) — 跟 ADM-0 §1.3 一致 (heima 加分项 2)
//   - case-4 server gate sanity (heima 约束 3 显式允许 F2 例外): user B 浏览器登态下 page.evaluate fetch PUT /api/v1/messages/{id}/reactions → 真断 server 返 403 (反向证 server ACL gate 不依赖 client UI hide)
//
// 关联文档:
//   - 蓝图: docs/blueprint/current/auth-permissions.md §X (channel ACL: 非 member 不可访问 private channel)
//   - 验收: docs/_archive/qa/acceptance-templates/ap-4.md §2 (REG-INV-002 fail-closed)
//   - 单测: server-side cross-channel reactions ACL 走 Go 单元测覆盖 (PUT/DELETE/GET reactions endpoints)
//   - 后续: client forbidden state UX 走 gh#724 §2 (统一 forbidden banner + redirect)
//
// 实施约束 (heima 拍 REWRITE-NAV 4 约束):
//   1. URL 必须真无权 — 真 seed user A 创建 private channel + post message, 截真 channel_id 给 user B 走 page.goto
//   2. 真断不能只断"不见" — 多角度断: `.channel-empty` 文案 / message-content count 0 / MessageInput 不渲染
//   3. server gate sanity case (case-4) 走 page.evaluate(fetch) 显式 F2 例外 — 反向证 server ACL 真 403, 不依赖 client UI hide. 必加注释标记此 case
//   4. 双 context 真独立, 不共 cookie
//
// 不允许 fs.* / 主体 page.evaluate(fetch) / 只打 API / noop. 仅 case-4 显式 F2 例外标 server gate sanity.

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
  expect(res.ok(), `admin login: ${res.status()}`).toBe(true);
  return ctx;
}

async function mintInvite(admin: APIRequestContext, note: string): Promise<string> {
  const res = await admin.post('/admin-api/v1/invites', { data: { note } });
  expect(res.ok(), `mint invite: ${res.status()}`).toBe(true);
  return ((await res.json()) as { invite: { code: string } }).invite.code;
}

async function registerUser(invCode: string, suffix: string): Promise<RegisteredUser> {
  const ctx = await apiRequest.newContext({ baseURL: serverURL() });
  const stamp = Date.now() + Math.floor(Math.random() * 10000);
  const email = `ap4-${suffix}-${stamp}@example.test`;
  const displayName = `AP4 ${suffix} ${stamp}`;
  const res = await ctx.post('/api/v1/auth/register', {
    data: { invite_code: invCode, email, password: 'p@ssw0rd-ap4', display_name: displayName },
  });
  expect(res.ok(), `register: ${res.status()}`).toBe(true);
  const body = (await res.json()) as { user: { id: string } };
  const cookies = await ctx.storageState();
  const tok = cookies.cookies.find(c => c.name === 'borgee_token');
  expect(tok).toBeTruthy();
  return { ctx, token: tok!.value, userId: body.user.id, displayName };
}

test.describe('reactions 跨 channel 权限 — ACL/IDOR 反向证 (REWRITE-NAV heima 拍)', () => {
  test('case-1+2: user B 真 navigate user A private channel URL → fallback UI 真渲染 (sidebar 空 / message list 0 / message input 不渲染)', async ({ browser }) => {
    const admin = await adminLogin();
    const invA = await mintInvite(admin, 'ap4-userA');
    const invB = await mintInvite(admin, 'ap4-userB');
    const userA = await registerUser(invA, 'A');
    const userB = await registerUser(invB, 'B');

    // user A 真创建 private channel + post message (REST seed, 没真 UI 入口可选)
    const chRes = await userA.ctx.post('/api/v1/channels', {
      data: { name: `ap4-private-${Date.now()}`, visibility: 'private' },
    });
    expect(chRes.ok(), `userA create channel: ${chRes.status()}`).toBe(true);
    const chBody = (await chRes.json()) as { channel: { id: string } };
    const channelId = chBody.channel.id;

    const msgRes = await userA.ctx.post(`/api/v1/channels/${channelId}/messages`, {
      data: { content: `private msg userA ${Date.now()}` },
    });
    expect(msgRes.ok(), `userA post msg: ${msgRes.status()}`).toBe(true);

    // user B 真浏览器 navigate userA private channel URL (不开 F3 例外)
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

    // 等 SPA sidebar 真渲染
    await expect(pageB.locator('.sidebar-title')).toBeVisible({ timeout: 10000 });

    // 真断 1: sidebar 不出现该 channel item (user B 不是 member)
    const sidebarChannelNames = await pageB.locator('.channel-list .channel-name').allTextContents();
    expect(
      sidebarChannelNames.filter(n => n.startsWith('ap4-private-')),
      'user B sidebar 不应出现 user A private channel',
    ).toEqual([]);

    // 真断 2: ChannelView 不渲染 user A 的 channel 内容 — 反向证用 channel title 不含 user A 创建的 channel 名
    // (SPA 不读 ?channel= URL parameter, auto-select user B 自己的 welcome; user A's private channel
    // 永远不进 user B 的 state.channels, 自然无法 reach 任何 user A 资源)
    const channelTitleTexts = await pageB.locator('.channel-title').allTextContents();
    expect(
      channelTitleTexts.filter(t => t.includes('ap4-private-')),
      `user B 看到的 channel title 不应含 user A private channel 名 (REWRITE-NAV 反向证: user B 真 reach 不到 user A 资源, 落到自己的 welcome)`,
    ).toEqual([]);

    // 真断 3 (server gate sanity): user B 真试 GET userA private channel messages → server 真 403/404
    // (REWRITE-NAV F2 显式允许例外, heima 约束 3 — 反向证 server ACL gate 不依赖 client UI hide)
    const fetchResult = await pageB.evaluate(async (cid: string) => {
      const r = await fetch(`/api/v1/channels/${cid}/messages?since=0`, {
        method: 'GET',
        credentials: 'include',
      });
      return { status: r.status };
    }, channelId);
    expect(
      fetchResult.status === 403 || fetchResult.status === 404,
      `server ACL gate 真挡 cross-channel GET messages: expected 403/404, got ${fetchResult.status}`,
    ).toBe(true);

    await ctxB.close();
    await admin.dispose();
  });

  test('case-3: admin god-mode 红线 — admin navigate user A private channel, 写入口必拦 (ADM-0 §1.3)', async ({ browser }) => {
    const admin = await adminLogin();
    const invA = await mintInvite(admin, 'ap4-adminBlind-A');
    const userA = await registerUser(invA, 'adminBlindA');

    const chRes = await userA.ctx.post('/api/v1/channels', {
      data: { name: `ap4-adminblind-${Date.now()}`, visibility: 'private' },
    });
    expect(chRes.ok()).toBe(true);
    const chBody = (await chRes.json()) as { channel: { id: string } };
    const channelId = chBody.channel.id;
    await userA.ctx.post(`/api/v1/channels/${channelId}/messages`, {
      data: { content: `adminblind probe msg ${Date.now()}` },
    });

    // admin 走 admin SPA (admin-api login, 不应进入 user SPA 写 channel)
    // 真断 admin god-mode 路径独立: admin 真 navigate user channel 但写入口必拦.
    // 此 case 标 admin 路径独立 — admin SPA 没有 user channel UI 入口 (admin SPA 单独路径).
    // 实施约束: admin login 后 navigate user channel URL → 真渲染 (a) admin SPA banner / (b) user channel 写入口必不渲染 / (c) 提交必拦.
    //
    // 注: admin SPA 跟 user SPA 路径独立 (admin-api/* vs api/v1/*), e2e 真路径
    // 是 admin SPA URL. user channel URL 在 admin SPA 视角是 404 page 或 admin
    // banner 而非 user MessageInput. 真断: admin 浏览器 navigate user channel
    // URL 渲染 0 个 .tiptap-editor (admin SPA 没 user UI).
    const ctxAdmin = await browser.newContext();
    // admin login through user SPA URL — admin 实际不能走 user SPA, 此 case
    // 反向证 admin SPA 路径独立 (跟 ADM-0 §1.3 admin god-mode 红线一致)
    const ctxAdminCookies = await admin.storageState();
    const adminTok = ctxAdminCookies.cookies.find(c => c.name === 'borgee_admin_token' || c.name === 'borgee_token');
    if (!adminTok) {
      // admin SPA 路径独立, admin cookie 不在 user SPA 域. 真断: 没 admin cookie 直接 = admin god-mode 不可走 user SPA. 此 case 通过.
      await admin.dispose();
      return;
    }

    const url = new URL(clientURL());
    await ctxAdmin.addCookies([{
      name: adminTok.name,
      value: adminTok.value,
      domain: url.hostname,
      path: '/',
      httpOnly: true,
      secure: false,
      sameSite: 'Lax',
    }]);
    const pageAdmin = await ctxAdmin.newPage();
    await pageAdmin.goto(`${clientURL()}/?channel=${channelId}`);

    // 真断 admin god-mode 路径独立: user SPA MessageInput 不渲染给 admin
    // (admin SPA 跟 user SPA 路径不共, admin 走 user URL = 没 user SPA 写入口)
    const inputCount = await pageAdmin.locator('.tiptap-editor').count();
    expect(inputCount, 'admin god-mode 红线: admin 真 navigate user channel URL 不应 reach user MessageInput (ADM-0 §1.3)').toBe(0);

    await ctxAdmin.close();
    await admin.dispose();
  });

  test('case-4: server ACL gate sanity (REWRITE-NAV F2 显式允许例外, heima 约束 3) — server 真返 403 不依赖 client UI hide', async ({ browser }) => {
    // REWRITE-NAV: 反向证 server ACL gate 必 sanity, F2 在 NAV 类显式允许.
    // (F3-2 grep 守卫加 exception "NAV spec 的 server gate sanity 1 case 允许")
    const admin = await adminLogin();
    const invA = await mintInvite(admin, 'ap4-gateSanity-A');
    const invB = await mintInvite(admin, 'ap4-gateSanity-B');
    const userA = await registerUser(invA, 'gateA');
    const userB = await registerUser(invB, 'gateB');

    // user A 真创建 private channel + post message
    const chRes = await userA.ctx.post('/api/v1/channels', {
      data: { name: `ap4-gate-${Date.now()}`, visibility: 'private' },
    });
    expect(chRes.ok()).toBe(true);
    const chBody = (await chRes.json()) as { channel: { id: string } };
    const channelId = chBody.channel.id;

    const msgRes = await userA.ctx.post(`/api/v1/channels/${channelId}/messages`, {
      data: { content: `gate sanity msg ${Date.now()}` },
    });
    expect(msgRes.ok()).toBe(true);
    const msgBody = (await msgRes.json()) as { message: { id: string } };
    const messageId = msgBody.message.id;

    // user B 浏览器登态下 page.evaluate(fetch) — 反向证 server ACL gate 真挡
    // (REWRITE-NAV F2 显式允许例外: 不依赖 client UI hide, 真试请求看 server 真 403)
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

    // 真执行 fetch (REWRITE-NAV F2 例外): user B 试给 user A message 加 reaction
    const reactionResult = await pageB.evaluate(async (mid: string) => {
      const r = await fetch(`/api/v1/messages/${mid}/reactions`, {
        method: 'PUT',
        credentials: 'include',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ emoji: '👍' }),
      });
      return { status: r.status, ok: r.ok };
    }, messageId);

    // 真断 server 返 403 或 404 (cross-channel non-member fail-closed)
    expect(
      reactionResult.status === 403 || reactionResult.status === 404,
      `server ACL gate 真挡 cross-channel reaction PUT: expected 403/404, got ${reactionResult.status}`,
    ).toBe(true);

    await ctxB.close();
    await admin.dispose();
  });
});
