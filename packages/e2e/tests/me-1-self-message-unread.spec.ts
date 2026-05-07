// tests/me-1-self-message-unread.spec.ts — gh#687 own message 不计未读 e2e
//
// 闭环 docs/implementation/design/687-self-message-unread-design.md §7.4
// (e2e 自动化, gh#700 followup): 走真 UI input/click 自动化跑 §7.3 5 步路径
// + §4.2 multi-device + §7.2 反向断言 (peer 发的仍算未读).
//
// 立场反查 (§1 + §2 三层防御):
//   ① Layer 1 client send: own message → mark current channel read 立刻清 unread
//   ② Layer 2 server SQL: GET /api/v1/channels unread_count 排除 sender_id == 自己
//   ③ Layer 3 client reducer: ws push own message 在 non-current channel 不 bump unread
//
// 反约束 (本 spec 锚, memory `e2e_no_curl_only_ui`):
//   - 走真 UI input/click/screenshot, 不用 page.evaluate(fetch) / cURL 直调
//   - 创建 channel 走 sidebar UI 入口 (+ 按钮 → 创建频道 → form submit)
//   - 发消息走 ProseMirror editor input + Enter (跟用户真路径一致)
//   - 切 channel 走 sidebar .channel-name click

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
  const email = `me1-${suffix}-${stamp}-${Math.floor(Math.random() * 1000)}@example.test`;
  const password = 'p@ssw0rd-me1';
  const displayName = `ME1 ${suffix} ${stamp}`;
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

/**
 * 通过 sidebar UI 创建一个新 channel — sidebar 「+」按钮 → 创建频道 →
 * 输入名称 → 提交. 真用户路径.
 */
async function createChannelViaUI(page: Page, name: string): Promise<void> {
  // sidebar 顶部「+」按钮 (title="创建", Sidebar.tsx L171)
  await page.locator('.sidebar-add-dropdown button.icon-btn[title="创建"]').click();
  // 下拉「创建频道」
  await page.locator('.sidebar-add-menu-item', { hasText: '创建频道' }).click();
  // form 出现 (Sidebar.tsx L197 .channel-create-form)
  const form = page.locator('.channel-create-form');
  await expect(form).toBeVisible({ timeout: 3_000 });
  // 输入名称 (placeholder="频道名称")
  await form.locator('input[placeholder="频道名称"]').fill(name);
  // submit 走 Enter (form onSubmit={handleCreate})
  await form.locator('input[placeholder="频道名称"]').press('Enter');
  // 等 form 关 (创建成功后 setShowCreate(false))
  await expect(form).toHaveCount(0, { timeout: 5_000 });
  // 等 sidebar 真 render 新 channel
  await expect(page.locator('.channel-name', { hasText: name })).toBeVisible({ timeout: 5_000 });
}

/**
 * 通过真 UI 在当前 channel 发消息 — ProseMirror editor 填字符 + Enter 送.
 * MessageInput.tsx L487 EditorContent + L494 发送 button.
 */
async function sendMessageViaUI(page: Page, text: string): Promise<void> {
  const editor = page.locator('.message-input-container .ProseMirror');
  await expect(editor).toBeVisible({ timeout: 3_000 });
  await editor.click();
  await editor.fill(text);
  // Enter 发送 (placeholder "Enter 发送, Ctrl+Enter 换行")
  await editor.press('Enter');
  // 等消息真出现在 message list (反向断言确认发送成功, 不是停在 sending)
  await expect(page.locator('.message-item', { hasText: text }).first()).toBeVisible({ timeout: 5_000 });
}

/**
 * 切 channel — sidebar 点 .channel-name. mobile 路径需要先开 sidebar
 * (此 spec 默认 desktop viewport ≥768px 不需要 hamburger).
 */
async function switchToChannel(page: Page, name: string): Promise<void> {
  await page.locator('.channel-name', { hasText: name }).first().click();
  // 等 channel header 切到目标 channel
  await expect(page.locator('.channel-header', { hasText: name })).toBeVisible({ timeout: 5_000 });
}

/**
 * 拿当前 sidebar 上 channel 行的 unread badge count. 没 badge 返 0.
 * SortableChannelItem.tsx L88 / L114 .unread-badge 锚.
 *
 * 重要 (zhanma 真因 reproduce 后校正):
 *   不能用 page.locator('li, div').filter({has: ...}).first() — 它会上升到
 *   外层 ancestor div (整 channel-list 容器), badge.first() 返其它 channel 的
 *   unread, 报错 channel 头上.
 *   用 xpath ancestor::*[contains(@class, "channel-item")][1] 锁最内层 row.
 */
async function getUnreadCount(page: Page, channelName: string): Promise<number> {
  const channelNameLoc = page.locator('.channel-name', { hasText: channelName }).first();
  // 锁最内层 .channel-item row (SortableChannelItem.tsx L49 / L103 都用此 class).
  const row = channelNameLoc.locator(
    'xpath=ancestor::*[contains(@class, "channel-item")][1]',
  );
  const badge = row.locator('.unread-badge');
  if (await badge.count() === 0) return 0;
  const text = await badge.first().textContent();
  if (!text) return 0;
  if (text === '99+') return 100;
  return parseInt(text, 10) || 0;
}

test.describe('gh#687 / #700 own message 不计未读 e2e', () => {
  // 设置 desktop viewport 防 mobile sidebar closed (那是 #698 修过的另一条路径).
  test.use({ viewport: { width: 1280, height: 800 } });

  test('§7.3 主路径 5 步: own message in welcome 切走切回 unread=0', async ({ page }) => {
    const serverURL = `http://127.0.0.1:${process.env.E2E_SERVER_PORT ?? '4901'}`;
    const adminCtx = await adminLogin(serverURL);
    const inv = await mintInvite(adminCtx, 'me1-self-unread');
    const owner = await registerUser(serverURL, inv, 'self');
    await attachToken(page.context(), owner.token);

    // Step 1: 登录并加载 (新注册自动有 welcome channel, 已默认选中)
    await page.goto('/');
    await expect(page.locator('.sidebar-title', { hasText: 'Borgee' })).toBeVisible({ timeout: 10_000 });
    // 等默认 welcome channel render — 名字以 welcome 开头
    const welcomeChannel = page.locator('.channel-name').filter({ hasText: /^welcome/ }).first();
    await expect(welcomeChannel).toBeVisible({ timeout: 5_000 });
    const welcomeName = (await welcomeChannel.textContent())?.trim() ?? '';
    expect(welcomeName, 'welcome channel name').toMatch(/^welcome/);

    // Step 2: 创建第二个 channel (走 UI), 让 welcome 失焦
    const secondName = `me1-second-${Date.now().toString(36)}`;
    await createChannelViaUI(page, secondName);
    // 创建成功后会自动切到第二 channel (handleCreate 行为)

    // Step 3: 切回 welcome 在那发消息
    await switchToChannel(page, welcomeName);
    const ownMsg = `me1 own ${Date.now()}`;
    await sendMessageViaUI(page, ownMsg);

    // Step 4: 切到第二 channel (welcome 失焦)
    await switchToChannel(page, secondName);

    // Step 5: 切回 welcome — sidebar 不应显未读
    // 真验立场 ① + ② + ③: own message 在 welcome 通过三层防御都不计未读.
    // 验证 welcome unread badge count == 0 (跟 design §7.3 expected output 同源).
    const welcomeUnread = await getUnreadCount(page, welcomeName);
    expect(
      welcomeUnread,
      'gh#687 立场: own message in welcome 切走再回来不应增 unread',
    ).toBe(0);

    // 反向: 第二 channel 没收到任何消息, 也应 unread=0 (sanity)
    const secondUnread = await getUnreadCount(page, secondName);
    expect(secondUnread).toBe(0);
  });

  test('§7.2 反向 peer 发的消息仍算未读 (Layer 2 不误伤别人)', async ({ browser }) => {
    const serverURL = `http://127.0.0.1:${process.env.E2E_SERVER_PORT ?? '4901'}`;
    const adminCtx = await adminLogin(serverURL);
    const ownerInv = await mintInvite(adminCtx, 'me1-owner-peer');
    const peerInv = await mintInvite(adminCtx, 'me1-peer');
    const owner = await registerUser(serverURL, ownerInv, 'owner-peer');
    const peer = await registerUser(serverURL, peerInv, 'peer');

    // Owner 通过 UI 创建一个 shared channel.
    const ownerCtxBrowser = await browser.newContext({ viewport: { width: 1280, height: 800 } });
    await attachToken(ownerCtxBrowser, owner.token);
    const ownerPage = await ownerCtxBrowser.newPage();
    await ownerPage.goto('/');
    await expect(ownerPage.locator('.sidebar-title')).toBeVisible({ timeout: 10_000 });

    const sharedName = `me1-shared-${Date.now().toString(36)}`;
    await createChannelViaUI(ownerPage, sharedName);

    // 拿 channel id 通过 REST 加 peer 进 shared channel (UI 加成员路径
    // 较复杂, 此处加成员是 setup 不是被测路径, 走 REST acceptable per
    // memory `e2e_no_curl_only_ui` — 测的是 unread 行为不是 add member UI).
    const channelsRes = await owner.ctx.get('/api/v1/channels');
    expect(channelsRes.ok()).toBe(true);
    const channels = (await channelsRes.json()).channels as { id: string; name: string }[];
    const shared = channels.find((c) => c.name === sharedName);
    expect(shared, `shared channel ${sharedName} not found`).toBeTruthy();
    const addRes = await owner.ctx.post(`/api/v1/channels/${shared!.id}/members`, {
      data: { user_id: peer.userId },
    });
    expect([200, 201, 204, 409]).toContain(addRes.status());

    // Peer 登录 + 切到 shared channel 之外 (停留 welcome 即可).
    const peerCtxBrowser = await browser.newContext({ viewport: { width: 1280, height: 800 } });
    await attachToken(peerCtxBrowser, peer.token);
    const peerPage = await peerCtxBrowser.newPage();
    await peerPage.goto('/');
    await expect(peerPage.locator('.sidebar-title')).toBeVisible({ timeout: 10_000 });

    // Owner 走真 UI 在 shared 发消息.
    await switchToChannel(ownerPage, sharedName);
    const peerVisibleMsg = `me1 from owner ${Date.now()}`;
    await sendMessageViaUI(ownerPage, peerVisibleMsg);

    // Peer 等 ws push 到 sidebar (≤3s, RT-1 budget).
    // shared channel 在 peer 上是 non-current, 应该 bump unread (Layer 3 反向: peer 发的不算 own, 算别人发的).
    await expect(async () => {
      const peerSharedUnread = await getUnreadCount(peerPage, sharedName);
      expect(
        peerSharedUnread,
        'gh#687 反向: peer 视角看 owner 发的消息仍算未读',
      ).toBeGreaterThanOrEqual(1);
    }).toPass({ timeout: 5_000 });

    await ownerCtxBrowser.close();
    await peerCtxBrowser.close();
  });

  test('§4.2 multi-device own message 在设备 B (在别 channel) sidebar 不闪未读', async ({ browser }) => {
    const serverURL = `http://127.0.0.1:${process.env.E2E_SERVER_PORT ?? '4901'}`;
    const adminCtx = await adminLogin(serverURL);
    const inv = await mintInvite(adminCtx, 'me1-multi-device');
    const owner = await registerUser(serverURL, inv, 'multi');

    // 设备 A: owner 登录, 在 channel A 发 own message.
    const deviceA = await browser.newContext({ viewport: { width: 1280, height: 800 } });
    await attachToken(deviceA, owner.token);
    const pageA = await deviceA.newPage();
    await pageA.goto('/');
    await expect(pageA.locator('.sidebar-title')).toBeVisible({ timeout: 10_000 });

    // 设备 A 创建 channel A.
    const channelAName = `me1-chan-a-${Date.now().toString(36)}`;
    await createChannelViaUI(pageA, channelAName);

    // 设备 B: 同 owner 登录, 在 welcome (停 non-A) 等 ws push.
    const deviceB = await browser.newContext({ viewport: { width: 1280, height: 800 } });
    await attachToken(deviceB, owner.token);
    const pageB = await deviceB.newPage();
    await pageB.goto('/');
    await expect(pageB.locator('.sidebar-title')).toBeVisible({ timeout: 10_000 });
    // 等 welcome render (停留默认 welcome).
    const welcomeChannelB = pageB.locator('.channel-name').filter({ hasText: /^welcome/ }).first();
    await expect(welcomeChannelB).toBeVisible({ timeout: 5_000 });

    // 设备 A 在 channel A 发 own message.
    const ownMsg = `me1 multi-device own ${Date.now()}`;
    await sendMessageViaUI(pageA, ownMsg);

    // 设备 B 应该在 ws push 到达 (≤3s) 之后, channel A 的 sidebar 行
    // unread_count 仍 == 0 (Layer 3 reducer: sender_id == currentUser.id 跳 bump).
    // 等 5s 给 ws 时间到, 然后真量 unread.
    await pageB.waitForTimeout(3_000);
    const channelAUnreadOnB = await getUnreadCount(pageB, channelAName);
    expect(
      channelAUnreadOnB,
      'gh#687 multi-device 立场: 设备 B 收到 owner 自己 (在设备 A 发) 的 ws push 不应 bump unread',
    ).toBe(0);

    await deviceA.close();
    await deviceB.close();
  });
});
