// tests/chn-2-3-dm-flow.spec.ts — CHN-2.3 e2e DM 主流 + 反约束兜底.
//
// 闭环 chn-2.md acceptance §4.d / §4.e + chn-2-content-lock.md §1 ④⑤
// byte-identical:
//   §4.d 双窗口 DM 创建 + 加人 attempt → 403 双断 (立场 ② 兜底)
//   §4.e DM 视图反查 Canvas tab count==0 + topic input count==0 + 添加成员
//        button count==0 (立场 ②③④ 三反约束兜底)
//   §1 ④ DM 视图 DOM **不渲染** workspace tab + topic banner + 添加成员 btn
//   §1 ⑤ DM "升级"提示反约束 — 升级路径不存在 (mention 第 3 人候选空)
//
// 立场反查 (chn-2.md §0):
//   ② DM 永远 2 人 — POST /channels/:id/members → 403
//   ③ DM 没 workspace — DOM 不渲染 .channel-view-tabs (CHN-2.2 #406 + ChannelView.tsx:159)
//   ④ DM 没 topic — DOM 不渲染 .channel-topic
//   ⑤ UI 跟 channel 视觉显著不同 — data-channel-type="dm" 反查锚 (#354 §1 ① + #406)
//   ⑥ raw UUID 不漏文本节点 — 跟 DM-2 §3.1 + ADM-0 #211 同源
//
// 上游接力: CHN-2.1 #407 server reject + CHN-2.2 #406 client 视觉拆 +
// DM-2.3 #388 mention 渲染. 本 spec 闭环 G3.x demo 截屏路径.

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

// CHN-2 文案锁 (#354 byte-identical, content-lock §1 ①⑤).
const DM_GROUP_LABEL = '私信';
const DM_THIRD_PARTY_PLACEHOLDER = '私信仅限两人, 想加人请新建频道';

interface RegisteredUser {
  email: string;
  token: string;
  userId: string;
  displayName: string;
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
  const email = `chn23-${suffix}-${stamp}-${Math.floor(Math.random() * 1000)}@example.test`;
  const password = 'p@ssw0rd-chn23';
  const displayName = `CHN23 ${suffix} ${stamp}`;
  const res = await ctx.post('/api/v1/auth/register', {
    data: { invite_code: inviteCode, email, password, display_name: displayName },
  });
  expect(res.ok(), `register: ${res.status()} ${await res.text()}`).toBe(true);
  const body = (await res.json()) as { user: { id: string } };
  const cookies = await ctx.storageState();
  const tok = cookies.cookies.find((c) => c.name === 'borgee_token');
  expect(tok, 'borgee_token cookie missing').toBeTruthy();
  return { email, token: tok!.value, userId: body.user.id, displayName, ctx };
}

function clientURL(): string {
  return `http://127.0.0.1:${process.env.E2E_CLIENT_PORT ?? '5174'}`;
}

async function attachToken(ctx: BrowserContext, token: string) {
  const url = new URL(clientURL());
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

// openDmAs creates / fetches the DM channel between caller and peer.
// Mirrors AppContext.actions.openDm via REST so e2e can drive setup
// without UI flakiness (UI side is exercised via assertions later).
async function openDmAs(user: RegisteredUser, peerUserID: string): Promise<{ id: string; name: string }> {
  const r = await user.ctx.post(`/api/v1/dm/${peerUserID}`);
  expect(r.ok(), `open dm: ${r.status()} ${await r.text()}`).toBe(true);
  const j = (await r.json()) as { channel: { id: string; name: string } };
  return j.channel;
}

test.describe('CHN-2.3 DM e2e — 立场 ②③④⑤⑥ 反约束兜底 (#357 §1.2 / #354)', () => {
  test('§4.d DM creation + add-member 403 (立场 ② DM 永远 2 人)', async () => {
    const serverPort = process.env.E2E_SERVER_PORT ?? '4901';
    const serverURL = `http://127.0.0.1:${serverPort}`;
    const adminCtx = await adminLogin(serverURL);

    const aliceInvite = await mintInvite(adminCtx, 'chn23-alice');
    const alice = await registerUser(serverURL, aliceInvite, 'alice');
    const bobInvite = await mintInvite(adminCtx, 'chn23-bob');
    const bob = await registerUser(serverURL, bobInvite, 'bob');
    const eveInvite = await mintInvite(adminCtx, 'chn23-eve');
    const eve = await registerUser(serverURL, eveInvite, 'eve');

    // Alice opens DM with Bob — server returns idempotent dm channel.
    const dm = await openDmAs(alice, bob.userId);
    expect(dm.id).toBeTruthy();

    // Reverse-fetch from Bob's side returns the same dm — idempotent
    // (chn-2.md §2.1 + CHN-1 既有 createDmChannel 字面).
    const dmFromBob = await openDmAs(bob, alice.userId);
    expect(dmFromBob.id, 'DM idempotent across both directions').toBe(dm.id);

    // §4.d 立场 ② — POST /channels/:dmId/members → 403 (server enforce).
    // CHN-2.1 #407 已落 server reject path; e2e 是 belt 兜底.
    const addAttempt = await alice.ctx.post(`/api/v1/channels/${dm.id}/members`, {
      data: { user_id: eve.userId },
    });
    expect(addAttempt.ok(), 'DM add-member should reject').toBe(false);
    expect(addAttempt.status(), 'expected 403/400 reject for DM add-member').toBeGreaterThanOrEqual(400);
  });

  test('§4.e DM view DOM — Canvas tab + topic + 添加成员 全 count==0 (立场 ②③④)', async ({ browser }) => {
    const serverPort = process.env.E2E_SERVER_PORT ?? '4901';
    const serverURL = `http://127.0.0.1:${serverPort}`;
    const adminCtx = await adminLogin(serverURL);

    const aliceInvite = await mintInvite(adminCtx, 'chn23-dom-alice');
    const alice = await registerUser(serverURL, aliceInvite, 'dom-alice');
    const bobInvite = await mintInvite(adminCtx, 'chn23-dom-bob');
    const bob = await registerUser(serverURL, bobInvite, 'dom-bob');

    // Alice opens DM via REST so the channel is in dmChannels list when
    // she lands on the SPA.
    await openDmAs(alice, bob.userId);

    const aliceCtx = await browser.newContext();
    await attachToken(aliceCtx, alice.token);
    const alicePage = await aliceCtx.newPage();
    await alicePage.goto(`${clientURL()}/`);
    await expect(alicePage.locator('.sidebar-title')).toBeVisible();

    // Sidebar shows 私信 group header — content-lock §1 ① byte-identical
    // (跟 #354 + Sidebar.tsx:411 字面同源).
    await expect(alicePage.locator(`[data-kind="dm"].online-header`, { hasText: DM_GROUP_LABEL }))
      .toBeVisible({ timeout: 5_000 });

    // Click into the DM with bob.
    const dmRow = alicePage.locator(`[data-kind="dm"]`).filter({ hasText: bob.displayName }).first();
    await dmRow.click();
    await expect(alicePage.locator('.channel-view[data-channel-type="dm"]')).toBeVisible();

    // §4.e — 三反约束 DOM count==0:
    //   立场 ③ 无 Canvas / Workspace / Remote tab — entire .channel-view-tabs
    //     bar is gated by `!isDm` in ChannelView.tsx:159. Assert absence
    //     of the bar itself + each tab button.
    await expect(alicePage.locator('.channel-view[data-channel-type="dm"] .channel-view-tabs'))
      .toHaveCount(0);
    await expect(alicePage.locator('.channel-view[data-channel-type="dm"] .channel-view-tab', { hasText: 'Canvas' }))
      .toHaveCount(0);
    await expect(alicePage.locator('.channel-view[data-channel-type="dm"] .channel-view-tab', { hasText: 'Workspace' }))
      .toHaveCount(0);
    //   立场 ④ 无 topic banner — .channel-topic only renders for non-DM
    //     channels with topic field set (gated by !isDm + headerTopic).
    await expect(alicePage.locator('.channel-view[data-channel-type="dm"] .channel-topic'))
      .toHaveCount(0);
    //   立场 ② 无 添加成员 / 成员管理 / 离开频道 button — header buttons
    //     are gated by `!isDm && channel` in ChannelView.tsx:138.
    await expect(alicePage.locator('.channel-view[data-channel-type="dm"] button', { hasText: '成员管理' }))
      .toHaveCount(0);
    await expect(alicePage.locator('.channel-view[data-channel-type="dm"] button', { hasText: '添加成员' }))
      .toHaveCount(0);
    await expect(alicePage.locator('.channel-view[data-channel-type="dm"] button', { hasText: '离开频道' }))
      .toHaveCount(0);

    await aliceCtx.close();
  });

  test('§1 ⑤ mention 第 3 人 placeholder byte-identical "私信仅限两人, 想加人请新建频道"', async ({
    browser,
  }) => {
    const serverPort = process.env.E2E_SERVER_PORT ?? '4901';
    const serverURL = `http://127.0.0.1:${serverPort}`;
    const adminCtx = await adminLogin(serverURL);

    const aliceInvite = await mintInvite(adminCtx, 'chn23-mention-alice');
    const alice = await registerUser(serverURL, aliceInvite, 'mention-alice');
    const bobInvite = await mintInvite(adminCtx, 'chn23-mention-bob');
    const bob = await registerUser(serverURL, bobInvite, 'mention-bob');

    await openDmAs(alice, bob.userId);

    const aliceCtx = await browser.newContext();
    await attachToken(aliceCtx, alice.token);
    const alicePage = await aliceCtx.newPage();
    await alicePage.goto(`${clientURL()}/`);
    await expect(alicePage.locator('.sidebar-title')).toBeVisible();

    // Open DM with Bob.
    const dmRow = alicePage.locator(`[data-kind="dm"]`).filter({ hasText: bob.displayName }).first();
    await dmRow.click();
    await expect(alicePage.locator('.channel-view[data-channel-type="dm"]')).toBeVisible();

    // Type @ + a query that matches neither party (by typing a string
    // unlikely to match Alice or Bob's display name). The mention picker
    // should surface the locked DM placeholder.
    const editor = alicePage.locator('.tiptap-editor').first();
    await editor.click();
    await editor.type('@zzz', { delay: 30 });

    // Wait for the mention popup to render. Both branches (DM placeholder
    // div + items list) live under .mention-picker; the DM-only path is
    // pinned by data-mention-empty="dm-third-party".
    const dmEmpty = alicePage.locator('[data-mention-empty="dm-third-party"]');
    await expect(dmEmpty).toBeVisible({ timeout: 5_000 });
    await expect(dmEmpty).toContainText(DM_THIRD_PARTY_PLACEHOLDER);

    // 反约束 — 不准 "升级为频道" / "Convert to channel" / "Upgrade DM"
    // 同义词 (蓝图 §1.2: 想加人就**新建** channel; 是新建不是 DM 转换).
    for (const forbidden of ['升级为频道', 'Convert to channel', 'Upgrade DM', '转为频道']) {
      await expect(dmEmpty).not.toContainText(forbidden);
    }

    await aliceCtx.close();
  });

  test('§4.b 反约束 — DM 升级路径不存在 (POST /channels/:id/upgrade-to-channel 不存在)', async () => {
    const serverPort = process.env.E2E_SERVER_PORT ?? '4901';
    const serverURL = `http://127.0.0.1:${serverPort}`;
    const adminCtx = await adminLogin(serverURL);

    const aliceInvite = await mintInvite(adminCtx, 'chn23-upgrade-alice');
    const alice = await registerUser(serverURL, aliceInvite, 'upgrade-alice');
    const bobInvite = await mintInvite(adminCtx, 'chn23-upgrade-bob');
    const bob = await registerUser(serverURL, bobInvite, 'upgrade-bob');

    const dm = await openDmAs(alice, bob.userId);

    // §4.b chn-2.md acceptance — `POST /channels/:id/upgrade-to-channel`
    // endpoint MUST NOT exist (蓝图 §1.2 字面 "想加人就新建" 不是 DM 转换).
    // Probe gives 404 (route not registered) or 405 (route exists for
    // different method). Both signal "no upgrade path"; success would
    // signal stance leak.
    const probe = await alice.ctx.post(`/api/v1/channels/${dm.id}/upgrade-to-channel`);
    expect(probe.ok(), 'DM upgrade endpoint should not exist').toBe(false);
    expect([404, 405]).toContain(probe.status());
  });

  test('§3 立场 ⑥ DM message 文本节点不漏 raw UUID (跟 DM-2 §3.1 + ADM-0 #211 同源)', async ({
    browser,
  }) => {
    const serverPort = process.env.E2E_SERVER_PORT ?? '4901';
    const serverURL = `http://127.0.0.1:${serverPort}`;
    const adminCtx = await adminLogin(serverURL);

    const aliceInvite = await mintInvite(adminCtx, 'chn23-uuid-alice');
    const alice = await registerUser(serverURL, aliceInvite, 'uuid-alice');
    const bobInvite = await mintInvite(adminCtx, 'chn23-uuid-bob');
    const bob = await registerUser(serverURL, bobInvite, 'uuid-bob');

    const dm = await openDmAs(alice, bob.userId);

    // Alice mentions Bob in the DM via REST (text body uses the server's
    // `@<uuid>` token grammar — same as DM-2.2 #372 parser).
    const sendRes = await alice.ctx.post(`/api/v1/channels/${dm.id}/messages`, {
      data: { content: `hi <@${bob.userId}> ping`, content_type: 'text' },
    });
    expect(sendRes.ok(), `send: ${sendRes.status()} ${await sendRes.text()}`).toBe(true);

    // Open Bob's view — message renders. Assert the DOM text node has
    // no raw UUID (data-mention-id attr may carry it, that's fine).
    const bobCtx = await browser.newContext();
    await attachToken(bobCtx, bob.token);
    const bobPage = await bobCtx.newPage();
    await bobPage.goto(`${clientURL()}/`);
    await expect(bobPage.locator('.sidebar-title')).toBeVisible();
    const dmRow = bobPage.locator(`[data-kind="dm"]`).filter({ hasText: alice.displayName }).first();
    await dmRow.click();
    await expect(bobPage.locator('.channel-view[data-channel-type="dm"]')).toBeVisible();

    // Wait for Alice's message to appear.
    await expect(bobPage.locator('.message-item').filter({ hasText: 'ping' })).toBeVisible({
      timeout: 8_000,
    });

    // The mention span carries data-mention-id with the raw UUID; the
    // text node renders display name. Sniff the .channel-view content
    // for raw UUID text — should be 0.
    const textContent = await bobPage
      .locator('.channel-view[data-channel-type="dm"] .message-list')
      .evaluate((el) => el.textContent ?? '');
    // ID leak sniff: match either UUID-36 hex or ULID-26 Crockford base32
    // (post-ULID-MIGRATION rows). The mention DOM should show display name,
    // never the raw opaque id.
    const uuidPattern =
      /([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}|[0-9A-HJKMNP-TV-Z]{26})/;
    expect(textContent).not.toMatch(uuidPattern);

    // Belt: data-mention-id attr carries the bob UUID (反约束: attr ok,
    // 文本节点不准).
    const mentionEl = bobPage.locator(`[data-mention-id="${bob.userId}"]`).first();
    await expect(mentionEl).toBeVisible();

    await bobCtx.close();
  });
});
