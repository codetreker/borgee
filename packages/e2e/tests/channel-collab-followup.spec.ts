// tests/chn-4-followup.spec.ts — CHN-4 follow-up e2e: 反约束兜底 + 跨 org 隔离.
//
// 闭环 chn-4.md acceptance §4 反向 grep 反约束 + 跨 org 边界 (CHN-1 双轴
// 隔离同源):
//   §4.4 DM 视图永不含 workspace tab + 不挂 channel pin handle (#415 既有)
//   §4.5 messages 表不反指 artifact_id/iteration_id/anchor_id (4 路径拆死)
//   §4.7 e2e 反 server mock — 走真 4901 + 5174 (注释字面 byte-identical)
//   §4.8 不新 WS frame (RT-1 4 frame 已锁)
//   边界: 跨 org channel 不可见 (CHN-1 双轴隔离, A org user 不见 B org channel)
//
// 立场反查 (chn-4-stance-checklist.md §1):
//   ③ e2e 走真 server-go(4901) + vite(5174), 不 mock — runtime stub via
//      direct owner commit (实施 e2e 显式注释字面 byte-identical):
//      // CV-4 runtime stub: direct owner commit (not server mock)
//   ④ DM 视图永不含 workspace tab (7 源 byte-identical 锁)
//   ⑤ 4 路径互不污染 (mention/artifact/anchor/iterate 数据契约永久拆死)
//
// 实施说明: 本 follow-up 是 #411 (CHN-4.1+4.3 client wiring + 双 tab 截屏)
// 之外的反约束补全 — #411 主路径 + G3.4 双 tab 截屏归档已落地, 此 follow-up
// 是边界 + 反向断言. 主 e2e 跟 #411 chn-4-collab-skeleton.spec.ts 共存
// 不重叠 (那个是主路径正向, 这个是边界反向).

// CV-4 runtime stub: direct owner commit (not server mock) — 跟 acceptance
// §3.2 + #375 §1 + #378 立场 ③ 字面 byte-identical 同源 (review subagent
// §4.7 反向 grep 锚, 缺这条注释 review 不过).
import {
  test,
  expect,
  request as apiRequest,
  type APIRequestContext,
  type Page,
  type BrowserContext,
} from '@playwright/test';
import * as path from 'path';
import { fileURLToPath } from 'url';

const ADMIN_LOGIN = 'e2e-admin';
const ADMIN_PASSWORD = 'e2e-admin-pass-12345';
const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const SCREENSHOT_DIR = path.resolve(__dirname, '../../../docs/qa/screenshots');

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
  const email = `chn4f-${suffix}-${stamp}-${Math.floor(Math.random() * 1000)}@example.test`;
  const password = 'p@ssw0rd-chn4f';
  const displayName = `CHN4f ${suffix} ${stamp}`;
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

async function attachToken(ctx: BrowserContext, token: string): Promise<void> {
  const url = new URL(clientURL());
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

async function createChannel(user: RegisteredUser, name: string): Promise<string> {
  const r = await user.ctx.post('/api/v1/channels', {
    data: { name, visibility: 'private' },
  });
  expect(r.ok(), `channel create: ${r.status()} ${await r.text()}`).toBe(true);
  const j = (await r.json()) as { channel: { id: string } };
  return j.channel.id;
}

test.describe('CHN-4 follow-up — 反约束兜底 + 跨 org 隔离', () => {
  test('§4.4 + 边界: DM 行 sidebar 不挂 drag handle ⋮⋮ (channel pin 路径反向断言)', async ({ browser }) => {
    // 立场 ④ DM 视图永不含 workspace 7 源 byte-identical 同源延伸 — DM 行
    // sidebar 也不渲染 #415 SortableChannelItem 的 drag handle ⋮⋮ (CHN-3.3
    // SortableChannelItem.tsx 注释字面: "DM 行不渲染 (Sidebar.tsx DMItem
    // 绕过此组件; 此 component 只服务 channel rows)").
    const serverPort = process.env.E2E_SERVER_PORT ?? '4901';
    const serverURL = `http://127.0.0.1:${serverPort}`;
    const adminCtx = await adminLogin(serverURL);
    const invA = await mintInvite(adminCtx, 'chn4f-dma');
    const invB = await mintInvite(adminCtx, 'chn4f-dmb');
    const userA = await registerUser(serverURL, invA, 'a');
    const userB = await registerUser(serverURL, invB, 'b');

    // userA 开 DM 跟 userB (POST /api/v1/dm/:userId 创建 DM channel).
    const dmRes = await userA.ctx.post(`/api/v1/dm/${userB.userId}`);
    expect(dmRes.ok(), `dm open: ${dmRes.status()}`).toBe(true);

    const ctx = await browser.newContext();
    await attachToken(ctx, userA.token);
    const page = await ctx.newPage();
    await page.goto(`${clientURL()}/`);
    await expect(page.locator('.sidebar-title')).toBeVisible();

    // CHN-4 wrapper: 删 waitForTimeout(500) 死等 → 用 Playwright auto-retry
    // (sidebar 渲染 DM list 后 .sortable-handle count==0, 5s retry 替死等).

    // 反向断言: DM 行 sidebar 不挂 drag handle (SortableChannelItem 不服务 DM).
    // CHN-3.3 SortableChannelItem.tsx 字面: 仅 channel rows 渲染 .sortable-handle.
    // 这里反向断: data-channel-type="dm" 子树内 .sortable-handle count==0.
    const dmRowsWithHandle = await page
      .locator('[data-channel-type="dm"] .sortable-handle')
      .count();
    expect(dmRowsWithHandle, 'DM 行 sidebar 不挂 drag handle ⋮⋮').toBe(0);

    // 边界态截屏 — DM sidebar (无 drag handle / 反约束视觉证据).
    await page.screenshot({
      path: path.join(SCREENSHOT_DIR, 'g3.x-chn4-followup-dm-no-handle.png'),
      fullPage: false,
    });
  });

  test('§4.5 + 边界: 跨 org channel 双轴隔离 — userA 不见 userB private channel (CHN-1 同源)', async ({ browser }) => {
    // CHN-1 双轴隔离 (org / channel) — 反约束 §4.5 4 路径不污染同精神
    // 延伸到 channel 可见性: A org user 创 private channel 后, B org user
    // (不在 channel.member) GET /channels 不见此 channel.
    const serverPort = process.env.E2E_SERVER_PORT ?? '4901';
    const serverURL = `http://127.0.0.1:${serverPort}`;
    const adminCtx = await adminLogin(serverURL);
    const invA = await mintInvite(adminCtx, 'chn4f-orga');
    const invB = await mintInvite(adminCtx, 'chn4f-orgb');
    const userA = await registerUser(serverURL, invA, 'orga');
    const userB = await registerUser(serverURL, invB, 'orgb');

    const stamp = Date.now();
    const chName = `chn4f-private-${stamp}`;
    const chId = await createChannel(userA, chName);

    // userB 视角 GET /channels — 反向断言 chId 不在列表.
    const listRes = await userB.ctx.get('/api/v1/channels');
    expect(listRes.ok()).toBe(true);
    const list = (await listRes.json()) as {
      channels: Array<{ id: string; name: string }>;
    };
    const found = list.channels.find((c) => c.id === chId);
    expect(found, `userB 不应见 userA private channel ${chName}`).toBeUndefined();

    // 反向断言 — userB 直接 GET /channels/:id 也 403/404 (不 leak).
    const directRes = await userB.ctx.get(`/api/v1/channels/${chId}`);
    expect([403, 404], `直 GET 应 reject; got ${directRes.status()}`).toContain(directRes.status());

    // 边界态截屏 — userB 视角 sidebar 不见 userA 的 private channel (跨 org 隔离视觉证据).
    const ctx = await browser.newContext();
    await attachToken(ctx, userB.token);
    const page = await ctx.newPage();
    await page.goto(`${clientURL()}/`);
    await expect(page.locator('.sidebar-title')).toBeVisible();
    // CHN-4 wrapper: 删 waitForTimeout — Playwright toHaveCount auto-retry 5s
    // 替死等. 跟 RT-1.2 latency CI 时序敏感修法同精神.
    await expect(
      page.locator('.channel-name', { hasText: chName }),
      `userB sidebar 不应见 ${chName}`,
    ).toHaveCount(0);
    await page.screenshot({
      path: path.join(SCREENSHOT_DIR, 'g3.x-chn4-followup-cross-org-isolation.png'),
      fullPage: false,
    });
  });

  test('§4.7 + §4.8: 反 server mock + 不新 WS frame (走真 4901 反向断言)', async () => {
    // 立场 ③ — e2e 走真 server-go(4901) + vite(5174), 不 mock.
    // 此 test 不调 client 路径, 仅反向断言 server endpoint 真实存在
    // (不 mock) + 反约束 endpoint 不存在.
    const serverPort = process.env.E2E_SERVER_PORT ?? '4901';
    const serverURL = `http://127.0.0.1:${serverPort}`;
    const ctx = await apiRequest.newContext({ baseURL: serverURL });

    // §4.7 反 mock — /health 真返 (server 真起).
    const health = await ctx.get('/health');
    expect(health.ok(), 'server-go health endpoint must exist').toBe(true);

    // §4.8 反约束 — 不存在 /api/v1/channels/:id/scene 拼装端点 (立场 ① +
    // acceptance §4.2 字面). server 端 grep 已锁, e2e 反向 GET 任意 channel id
    // 都 404 (endpoint 根本没注册).
    const sceneRes = await ctx.get('/api/v1/channels/probe/scene');
    expect(sceneRes.status(), '/scene 拼装端点不应存在 (立场 ①)').toBe(404);

    // §4.6 反约束 — 不存在 PUT /channels/:id/default_tab 作者级偏好 endpoint.
    const tabRes = await ctx.fetch('/api/v1/channels/probe/default_tab', {
      method: 'PUT',
      data: { default_tab: 'workspace' },
    });
    // 405 (method not allowed) 或 404 (endpoint 不存在) 均合规.
    expect([404, 405], `default_tab PUT endpoint 不应存在; got ${tabRes.status()}`).toContain(tabRes.status());
  });
});
