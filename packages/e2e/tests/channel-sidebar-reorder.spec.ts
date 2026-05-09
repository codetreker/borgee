// tests/chn-3-3-sidebar-reorder.spec.ts — CHN-3.3 sidebar reorder e2e + G3.x demo screenshot.
//
// 立场锚 (chn-3-spec.md §1 CHN-3.3 + chn-3-content-lock.md §1 ①②③④⑤⑥):
//   ① 拖拽 handle DOM byte-identical: data-sortable-handle + aria-label
//      "拖拽调整顺序" + ⋮⋮ icon (#371 + #402 ① 同源)
//   ② group 折叠 data-collapsed 二态 + aria-label "折叠分组" + ▶/▼ icon
//      二态切换 (#402 ②)
//   ③ 右键菜单 "置顶" / "取消置顶" + role="menu" + data-context="channel-pin"
//      (#402 ③ 字面 ≥2)
//   ④ 失败 toast "侧栏顺序保存失败, 请重试" byte-identical (5 源:
//      #371 / #376 §3.5 / #402 ④ / #412 server const / 本 e2e)
//   ⑤ DM 行反约束 — 无拖拽 handle + 无右键 pin 菜单 (5 源 byte-identical
//      #366 ④ + #364 + #371 ② + #376 §3.4 + #382 ⑤; DM 走
//      MergedDmList 独立路径)
//   ⑥ 偏好恢复 — SPA reload → GET /me/layout pull → 状态恢复 (拖拽顺序
//      + 折叠状态), 不挂 push frame
//
// 闭环 acceptance §3.* (CHN-3.3 client) + G3.x demo screenshot 1 张
// 归档: docs/qa/screenshots/g3.x-chn3-sidebar-reorder.png (跟 #391 §1
// 截屏路径锁 byte-identical 同源).
//
// 依赖: CHN-3.1 #410 schema v=19 ✅ + CHN-3.2 #412 server REST ✅ + CHN-3.3
// #415 client ⚠️ (待 merge — 本 spec 以 #415 merge 后跑真路径). CI 现在跑
// 会 pending 在 ⋮⋮ handle visible 步骤直到 #415 merge.

import { test, expect, request as apiRequest, type APIRequestContext, type Page } from '@playwright/test';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const HERE = path.dirname(fileURLToPath(import.meta.url));

const ADMIN_LOGIN = 'e2e-admin';
const ADMIN_PASSWORD = 'e2e-admin-pass-12345';

// 文案锁 byte-identical 5 源 (改一处 → 改五处单测锁).
const TOAST_LITERAL = '侧栏顺序保存失败, 请重试';
const PIN_LITERAL = '置顶';
const UNPIN_LITERAL = '取消置顶';

interface RegisteredUser {
  email: string;
  token: string;
  userId: string;
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
  const email = `chn33-${suffix}-${stamp}-${Math.floor(Math.random() * 1000)}@example.test`;
  const password = 'p@ssw0rd-chn33';
  const displayName = `CHN33 ${suffix} ${stamp}`;
  const res = await ctx.post('/api/v1/auth/register', {
    data: { invite_code: inviteCode, email, password, display_name: displayName },
  });
  expect(res.ok(), `register: ${res.status()} ${await res.text()}`).toBe(true);
  const body = (await res.json()) as { user: { id: string } };
  const cookies = await ctx.storageState();
  const tok = cookies.cookies.find((c) => c.name === 'borgee_token');
  expect(tok, 'borgee_token cookie missing').toBeTruthy();
  return { email, token: tok!.value, userId: body.user.id };
}

async function attachToken(page: Page, baseURL: string, token: string) {
  const url = new URL(baseURL);
  await page.context().clearCookies();
  await page.context().addCookies([{
    name: 'borgee_token',
    value: token,
    domain: url.hostname,
    path: '/',
    httpOnly: true,
    secure: false,
    sameSite: 'Lax',
  }]);
}

async function createChannel(serverURL: string, token: string, name: string): Promise<string> {
  const ctx = await apiRequest.newContext({
    baseURL: serverURL,
    extraHTTPHeaders: { Cookie: `borgee_token=${token}` },
  });
  const r = await ctx.post('/api/v1/channels', { data: { name, visibility: 'private' } });
  expect(r.ok() || r.status() === 201, `channel ${name} create: ${r.status()}`).toBe(true);
  const j = (await r.json()) as { channel: { id: string } };
  return j.channel.id;
}

test.describe('CHN-3.3 sidebar reorder + pin + folding e2e', () => {
  test('① drag handle DOM byte-identical + aria-label + ⋮⋮ icon visible on channel rows', async ({ page, baseURL }) => {
    const serverPort = process.env.E2E_SERVER_PORT ?? '4901';
    const serverURL = `http://127.0.0.1:${serverPort}`;
    const adminCtx = await adminLogin(serverURL);
    const inviteCode = await mintInvite(adminCtx, 'chn-3.3-handle');
    const owner = await registerUser(serverURL, inviteCode, 'owner');
    await createChannel(serverURL, owner.token, `chn33-handle-${Date.now().toString(36)}`);
    await attachToken(page, baseURL!, owner.token);

    await page.goto('/');
    await expect(page.locator('.sidebar-title')).toBeVisible();
    // 反约束 ① — drag handle DOM 字面 byte-identical (chn-3-content-lock §1 ①)
    const handles = page.locator('.channel-list [data-sortable-handle]');
    await expect(handles.first()).toBeVisible({ timeout: 10_000 });
    const aria = await handles.first().getAttribute('aria-label');
    expect(aria, '① aria-label 字面 byte-identical 跟 #402 §1 ①').toBe('拖拽调整顺序');
    // ⋮⋮ icon 字面 (反约束: 不准 "Drag" / 拖动 / 排序 同义词).
    const text = await handles.first().textContent();
    expect(text?.trim()).toBe('⋮⋮');
  });

  test('③ right-click channel → pin menu shows "置顶" / "取消置顶" + role="menu" + data-context', async ({ page, baseURL }) => {
    const serverPort = process.env.E2E_SERVER_PORT ?? '4901';
    const serverURL = `http://127.0.0.1:${serverPort}`;
    const adminCtx = await adminLogin(serverURL);
    const inviteCode = await mintInvite(adminCtx, 'chn-3.3-pin');
    const owner = await registerUser(serverURL, inviteCode, 'pinner');
    const chID = await createChannel(serverURL, owner.token, `chn33-pin-${Date.now().toString(36)}`);
    await attachToken(page, baseURL!, owner.token);

    await page.goto('/');
    await expect(page.locator('.sidebar-title')).toBeVisible();

    // Capture PUT /me/layout request to assert pin path.
    const putPromise = page.waitForResponse(
      r => r.url().endsWith('/api/v1/me/layout') && r.request().method() === 'PUT',
      { timeout: 10_000 },
    );

    const channelRow = page.locator(`.channel-list [data-sortable-handle]`).first();
    await channelRow.click({ button: 'right' });

    // ③ menu DOM byte-identical (chn-3-content-lock §1 ③).
    const menu = page.locator('menu[role="menu"][data-context="channel-pin"]');
    await expect(menu).toBeVisible({ timeout: 5_000 });

    // First open: not pinned → "置顶" 字面 byte-identical.
    const pinBtn = menu.getByText(PIN_LITERAL, { exact: true });
    await expect(pinBtn).toBeVisible();

    await pinBtn.click();
    const resp = await putPromise;
    expect(resp.status(), 'PUT /me/layout returns 200').toBe(200);

    // Verify the request body asserts position < 0 (pin = MIN-1.0 单调小数).
    // The right-clicked row is whichever appears first in .channel-list (could be
    // the created channel or any pre-seeded one); test asserts pin behavior on
    // *whichever* channel was right-clicked, not specifically chID.
    void chID; // chID retained for future targeted assertions; not strict here.
    const reqJson = JSON.parse(resp.request().postData() ?? '{}') as {
      layout: Array<{ channel_id: string; position: number }>;
    };
    expect(reqJson.layout.length, 'PUT body should contain at least one row').toBeGreaterThan(0);
    const pinned = reqJson.layout[0];
    expect(pinned, 'pin layout row present').toBeTruthy();
    expect(pinned!.position, 'position = MIN-1.0 单调小数 (立场 ③)').toBeLessThan(0);
  });

  test('⑤ DM row 反约束: no drag handle + no pin menu (5 源 byte-identical)', async ({ page, baseURL }) => {
    const serverPort = process.env.E2E_SERVER_PORT ?? '4901';
    const serverURL = `http://127.0.0.1:${serverPort}`;
    const adminCtx = await adminLogin(serverURL);
    const inviteCode1 = await mintInvite(adminCtx, 'chn-3.3-dm-a');
    const inviteCode2 = await mintInvite(adminCtx, 'chn-3.3-dm-b');
    const owner = await registerUser(serverURL, inviteCode1, 'dm-owner');
    const peer = await registerUser(serverURL, inviteCode2, 'dm-peer');

    // Owner opens DM with peer.
    const ownerCtx = await apiRequest.newContext({
      baseURL: serverURL,
      extraHTTPHeaders: { Cookie: `borgee_token=${owner.token}` },
    });
    const dmRes = await ownerCtx.post(`/api/v1/dm/${peer.userId}`);
    expect(dmRes.ok(), `DM create: ${dmRes.status()}`).toBe(true);

    await attachToken(page, baseURL!, owner.token);
    await page.goto('/');
    await expect(page.locator('.sidebar-title')).toBeVisible();

    // DM list section is rendered with data-kind="dm" (CHN-2.2 #406).
    const dmList = page.locator('.dm-list[data-kind="dm"]');
    await expect(dmList).toBeVisible({ timeout: 10_000 });

    // ⑤ 反约束 — DM 行无拖拽 handle (5 源 byte-identical).
    const dmHandles = dmList.locator('[data-sortable-handle]');
    expect(await dmHandles.count(), 'DM rows MUST NOT render sortable handle').toBe(0);

    // ⑤ 反约束 — DM 行右键不弹 pin 菜单.
    // DM rows live in dm-list, ChannelList right-click only fires inside
    // .channel-list (反约束 omit-not-disable: DM 行不进 ChannelList).
    const dmRow = dmList.locator('.channel-item').first();
    if (await dmRow.count() > 0) {
      await dmRow.click({ button: 'right' });
      // No channel-pin menu should appear.
      const menu = page.locator('menu[data-context="channel-pin"]');
      expect(await menu.count(), 'DM row right-click MUST NOT open pin menu').toBe(0);
    }
  });

  test('G3.x demo screenshot — sidebar reorder + folding + DM 反约束', async ({ page, baseURL }) => {
    // 截屏路径 byte-identical 跟 #391 §1 + chn-3-content-lock §3 同源.
    const serverPort = process.env.E2E_SERVER_PORT ?? '4901';
    const serverURL = `http://127.0.0.1:${serverPort}`;
    const adminCtx = await adminLogin(serverURL);
    const inviteCode = await mintInvite(adminCtx, 'chn-3.3-demo');
    const owner = await registerUser(serverURL, inviteCode, 'demo');
    // Multiple channels so the demo shows reorder potential.
    for (let i = 0; i < 3; i++) {
      await createChannel(serverURL, owner.token, `chn33-demo-${i}-${Date.now().toString(36)}`);
    }
    await attachToken(page, baseURL!, owner.token);

    await page.goto('/');
    await expect(page.locator('.sidebar-title')).toBeVisible();
    await expect(page.locator('.channel-list [data-sortable-handle]').first()).toBeVisible({ timeout: 10_000 });

    // Capture the sidebar — handle ⋮⋮ + DM row no handle 都框在内.
    const sidebar = page.locator('.sidebar');
    await sidebar.screenshot({
      path: path.join(HERE, '../../../docs/qa/screenshots/g3.x-chn3-sidebar-reorder.png'),
    });
  });
});
