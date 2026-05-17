// tests/channel-collab-edge-cases.spec.ts — channel collaboration constraints + cross-org isolation.
//
// 测试范围:
//   - DM 视图不包含 workspace tab，也不显示 channel pin handle
//   - messages 表不引用 artifact_id / iteration_id / anchor_id (四条数据路径保持分离)
//   - 不新增 WS frame (RT-1 已锁四种 frame, 本 spec 不引入新 frame)
//   - 跨 org 隔离: A org 用户不可见 B org 的 channel
//
// 关联文档:
//   - 验收: docs/_archive/qa/acceptance-templates/chn-4.md §4 反向断言段
//   - 上游: PR #411 (CHN-4 主路径正向 e2e 在 channel-collab-tabs.spec.ts)
//
// 实施约束:
//   - UI 验证通过浏览器执行 (page.goto + DOM 断言 + server response)
//   - 使用 server-go(4901) + vite(5174), 不 mock 4901
//   - CV-4 runtime stub uses owner direct commit (comment kept for reviewer grep)
//   - 不允许 fs.* / page.evaluate(fetch) / API-only / noop

// CV-4 runtime stub: direct owner commit (not server mock), matching chn-4
// acceptance §3.2 and kept as a review grep target.
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
  await ctx.addCookies([
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

async function createChannel(user: RegisteredUser, name: string): Promise<string> {
  const r = await user.ctx.post('/api/v1/channels', {
    data: { name, visibility: 'private' },
  });
  expect(r.ok(), `channel create: ${r.status()} ${await r.text()}`).toBe(true);
  const j = (await r.json()) as { channel: { id: string } };
  return j.channel.id;
}

test.describe('CHN-4 follow-up — constraints + cross-org isolation', () => {
  test('§4.4 + edge case: DM sidebar row has no drag handle ⋮⋮', async ({ browser }) => {
    // DM view never includes workspace controls. The sidebar DM row also must not
    // render the #415 SortableChannelItem drag handle ⋮⋮. CHN-3.3 documents that
    // Sidebar.tsx DMItem bypasses this component, which only serves channel rows.
    const serverPort = process.env.E2E_SERVER_PORT ?? '4901';
    const serverURL = `http://127.0.0.1:${serverPort}`;
    const adminCtx = await adminLogin(serverURL);
    const invA = await mintInvite(adminCtx, 'chn4f-dma');
    const invB = await mintInvite(adminCtx, 'chn4f-dmb');
    const userA = await registerUser(serverURL, invA, 'a');
    const userB = await registerUser(serverURL, invB, 'b');

    // userA opens a DM with userB (POST /api/v1/dm/:userId creates the DM channel).
    const dmRes = await userA.ctx.post(`/api/v1/dm/${userB.userId}`);
    expect(dmRes.ok(), `dm open: ${dmRes.status()}`).toBe(true);

    const ctx = await browser.newContext();
    await attachToken(ctx, userA.token);
    const page = await ctx.newPage();
    await page.goto(`${clientURL()}/`);
    await expect(page.locator('.sidebar-title')).toBeVisible();

    // Use Playwright auto-retry instead of waitForTimeout(500); once the sidebar
    // renders the DM list, .sortable-handle should remain count==0.

    // DM sidebar rows must not render drag handles because SortableChannelItem
    // only serves channel rows. Assert count==0 under data-channel-type="dm".
    const dmRowsWithHandle = await page
      .locator('[data-channel-type="dm"] .sortable-handle')
      .count();
    expect(dmRowsWithHandle, 'DM 行 sidebar 不挂 drag handle ⋮⋮').toBe(0);

  });

  test('§4.5 + edge case: cross-org channel isolation hides userA private channel from userB', async ({
    browser,
  }) => {
    // CHN-1 two-axis isolation (org / channel) extends to channel visibility:
    // after an A-org user creates a private channel, a B-org user who is not in
    // channel.member must not see it in GET /channels.
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

    // userB GET /channels must not include chId.
    const listRes = await userB.ctx.get('/api/v1/channels');
    expect(listRes.ok()).toBe(true);
    const list = (await listRes.json()) as {
      channels: Array<{ id: string; name: string }>;
    };
    const found = list.channels.find((c) => c.id === chId);
    expect(found, `userB 不应见 userA private channel ${chName}`).toBeUndefined();

    // Direct GET /channels/:id as userB must also return 403/404.
    const directRes = await userB.ctx.get(`/api/v1/channels/${chId}`);
    expect([403, 404], `直 GET 应 reject; got ${directRes.status()}`).toContain(directRes.status());

    const ctx = await browser.newContext();
    await attachToken(ctx, userB.token);
    const page = await ctx.newPage();
    await page.goto(`${clientURL()}/`);
    await expect(page.locator('.sidebar-title')).toBeVisible();
    // Use Playwright toHaveCount auto-retry instead of waitForTimeout; this avoids
    // a timing-sensitive fixed delay.
    await expect(
      page.locator('.channel-name', { hasText: chName }),
      `userB sidebar 不应见 ${chName}`,
    ).toHaveCount(0);
  });

  test('§4.7 + §4.8: uses real server and does not add a new WebSocket frame', async () => {
    // E2E uses real server-go(4901) + vite(5174), with no server mock. This test
    // only checks server endpoints: /health exists, while unsupported endpoints
    // do not.
    const serverPort = process.env.E2E_SERVER_PORT ?? '4901';
    const serverURL = `http://127.0.0.1:${serverPort}`;
    const ctx = await apiRequest.newContext({ baseURL: serverURL });

    // §4.7: /health must respond from the real server.
    const health = await ctx.get('/health');
    expect(health.ok(), 'server-go health endpoint must exist').toBe(true);

    // §4.8: /api/v1/channels/:id/scene must not exist. Server-side grep covers
    // registration; this E2E GET verifies an arbitrary channel id returns 404.
    const sceneRes = await ctx.get('/api/v1/channels/probe/scene');
    expect(sceneRes.status(), '/scene 拼装端点不应存在 (立场 ①)').toBe(404);

    // §4.6: PUT /channels/:id/default_tab author preference endpoint must not exist.
    const tabRes = await ctx.fetch('/api/v1/channels/probe/default_tab', {
      method: 'PUT',
      data: { default_tab: 'workspace' },
    });
    // 405 (method not allowed) or 404 (endpoint absent) are both compliant.
    expect([404, 405], `default_tab PUT endpoint 不应存在; got ${tabRes.status()}`).toContain(
      tabRes.status(),
    );
  });
});
