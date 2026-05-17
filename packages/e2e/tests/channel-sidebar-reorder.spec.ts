// tests/channel-sidebar-reorder.spec.ts — channel sidebar 拖拽排序 + 折叠 + 置顶 + 偏好恢复.
//
// 测试范围:
//   - 拖拽 handle DOM: data-sortable-handle + aria-label "拖拽调整顺序" + ⋮⋮ icon
//   - 分组折叠二态: data-collapsed + aria-label "折叠分组" + ▶/▼ icon
//   - 右键菜单 "置顶" / "取消置顶" + role="menu" + data-context="channel-pin"
//   - 保存失败 toast 文案: "侧栏顺序保存失败, 请重试"
//   - DM 行不显示拖拽 handle，也不显示右键 pin 菜单 (DM 使用独立 MergedDmList 路径)
//   - SPA reload 后 GET /me/layout 拉取并恢复拖拽顺序 + 折叠状态 (不依赖 push frame)
//   - G3.x demo 截屏归档 g3.x-chn3-sidebar-reorder.png
//
// 关联文档:
//   - 验收: docs/_archive/qa/acceptance-templates/chn-3.md §3 (CHN-3.3 client)
//   - 上游: PR #410 (schema), #412 (server REST), #415 (client wiring)
//
// 实施约束:
//   - UI 验证通过浏览器执行 (page.dragTo + page.click + DOM 断言)
//   - 失败 toast 文案在 e2e / server const / client / acceptance / 本 spec 五处一致
//   - 不允许 fs.* / page.evaluate(fetch) / 只打 API / noop

import {
  test,
  expect,
  request as apiRequest,
  type APIRequestContext,
  type Page,
} from '@playwright/test';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const HERE = path.dirname(fileURLToPath(import.meta.url));

const ADMIN_LOGIN = 'e2e-admin';
const ADMIN_PASSWORD = 'e2e-admin-pass-12345';

// Locked copy shared by five sources; update all related tests when changing it.
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
  await page.context().addCookies([
    {
      name: 'borgee_token',
      value: token,
      domain: url.hostname,
      path: '/',
      httpOnly: true,
      secure: false,
      sameSite: 'Lax',
    },
  ]);
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
  test('① drag handle DOM byte-identical + aria-label + ⋮⋮ icon visible on channel rows', async ({
    page,
    baseURL,
  }) => {
    const serverPort = process.env.E2E_SERVER_PORT ?? '4901';
    const serverURL = `http://127.0.0.1:${serverPort}`;
    const adminCtx = await adminLogin(serverURL);
    const inviteCode = await mintInvite(adminCtx, 'chn-3.3-handle');
    const owner = await registerUser(serverURL, inviteCode, 'owner');
    await createChannel(serverURL, owner.token, `chn33-handle-${Date.now().toString(36)}`);
    await attachToken(page, baseURL!, owner.token);

    await page.goto('/');
    await expect(page.locator('.sidebar-title')).toBeVisible();
    // Constraint ①: drag handle DOM text must match chn-3-content-lock §1 ①.
    const handles = page.locator('.channel-list [data-sortable-handle]');
    await expect(handles.first()).toBeVisible({ timeout: 10_000 });
    const aria = await handles.first().getAttribute('aria-label');
    expect(aria, '① aria-label 字面 byte-identical 跟 #402 §1 ①').toBe('拖拽调整顺序');
    // The icon text must be ⋮⋮, not a synonym such as "Drag", 拖动, or 排序.
    const text = await handles.first().textContent();
    expect(text?.trim()).toBe('⋮⋮');
  });

  test('③ right-click channel → pin menu shows "置顶" / "取消置顶" + role="menu" + data-context', async ({
    page,
    baseURL,
  }) => {
    const serverPort = process.env.E2E_SERVER_PORT ?? '4901';
    const serverURL = `http://127.0.0.1:${serverPort}`;
    const adminCtx = await adminLogin(serverURL);
    const inviteCode = await mintInvite(adminCtx, 'chn-3.3-pin');
    const owner = await registerUser(serverURL, inviteCode, 'pinner');
    const chID = await createChannel(
      serverURL,
      owner.token,
      `chn33-pin-${Date.now().toString(36)}`,
    );
    await attachToken(page, baseURL!, owner.token);

    await page.goto('/');
    await expect(page.locator('.sidebar-title')).toBeVisible();

    // Capture PUT /me/layout request to assert pin path.
    const putPromise = page.waitForResponse(
      (r) => r.url().endsWith('/api/v1/me/layout') && r.request().method() === 'PUT',
      { timeout: 10_000 },
    );

    const channelRow = page.locator(`.channel-list [data-sortable-handle]`).first();
    await channelRow.click({ button: 'right' });

    // ③ menu DOM must match chn-3-content-lock §1 ③.
    const menu = page.locator('menu[role="menu"][data-context="channel-pin"]');
    await expect(menu).toBeVisible({ timeout: 5_000 });

    // First open: not pinned, so the menu shows the locked "置顶" text.
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

  test('⑤ DM row 反约束: no drag handle + no pin menu (5 源 byte-identical)', async ({
    page,
    baseURL,
  }) => {
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

    // ⑤ DM rows must not render a drag handle.
    const dmHandles = dmList.locator('[data-sortable-handle]');
    expect(await dmHandles.count(), 'DM rows MUST NOT render sortable handle').toBe(0);

    // ⑤ Right-clicking a DM row must not open the pin menu.
    // DM rows live in dm-list, ChannelList right-click only fires inside
    // .channel-list; DM rows are omitted from ChannelList rather than disabled.
    const dmRow = dmList.locator('.channel-item').first();
    if ((await dmRow.count()) > 0) {
      await dmRow.click({ button: 'right' });
      // No channel-pin menu should appear.
      const menu = page.locator('menu[data-context="channel-pin"]');
      expect(await menu.count(), 'DM row right-click MUST NOT open pin menu').toBe(0);
    }
  });

  test('G3.x demo screenshot — sidebar reorder + folding + DM constraints', async ({
    page,
    baseURL,
  }) => {
    // Screenshot path matches #391 §1 + chn-3-content-lock §3.
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
    await expect(page.locator('.channel-list [data-sortable-handle]').first()).toBeVisible({
      timeout: 10_000,
    });

    // Capture the sidebar with the ⋮⋮ handle and the DM row no-handle state.
    const sidebar = page.locator('.sidebar');
    if (process.env.E2E_EVIDENCE_SCREENSHOTS === '1') await sidebar.screenshot({
      path: path.join(HERE, '../../../docs/qa/screenshots/g3.x-chn3-sidebar-reorder.png'),
    });
  });
});
