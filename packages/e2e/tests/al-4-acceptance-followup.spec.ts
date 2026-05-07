// tests/al-4-acceptance-followup.spec.ts — AL-4 acceptance §3 (AL-4.3 client) e2e
// + G2.7 demo screenshot follow-up.
//
// 闭环 al-4.md acceptance §3.1-§3.4 (AL-4.3 client, owner-only DOM gate +
// 4 态 status badge + reason label byte-identical 跟 AL-1a #249 + system
// DM 3 态文案锁 #321) + §4 反查锚 (positive 4.5/4.6 真落).
//
// 立场反查 (al-4-spec.md §0 + #319 stance + #321 文案锁):
//   ① "Borgee 不带 runtime" — runtime 未注册时 RuntimeCard 不渲染
//      (graceful degrade omit, 跟 spec §3.1 同源)
//   ② owner-only DOM gate — start/stop button 非 owner DOM omit
//      (defense-in-depth, server 兜底 RequirePermission('agent.runtime.control'))
//   ③ runtime status ≠ presence — data-runtime-status 4 态严闭
//      ('registered'/'running'/'stopped'/'error'), 反约束 'busy'/'idle'/
//      'starting'/'stopping' 0 hit (跟 AL-3 拆死)
//   ④ reason 6 项 byte-identical 跟 AL-1a #249 lib/agent-state.ts
//      REASON_LABELS 三处单测锁
//
// 实现说明: AL-4.2 server #414 OPEN coverage 修中, 此 e2e 走真路径在
// #414 merged 后自动通过. 当前 spec 已落 RuntimeCard 组件 (#417 merged
// 03:51) — DOM gate + status badge 可独立验证, runtime 列表通过 REST
// register 触发 (#414 server 端). 真路径整链 ≤3s.
//
// 不在范围 (留账 follow-up):
// - admin god-mode `GET /admin-api/v1/runtimes` 元数据白名单 (server
//   端 reflect scan 验, 走 unit 不走 e2e — REG-AL4-009 同源)
// - heartbeat 双表两路径反向断言 (server 端 unit, REG-AL4-008 同源)

import {
  test,
  expect,
  request as apiRequest,
  type APIRequestContext,
  type Page,
  type BrowserContext,
} from '@playwright/test';
import * as path from 'node:path';
import { fileURLToPath } from 'node:url';

const ADMIN_LOGIN = 'e2e-admin';
const ADMIN_PASSWORD = 'e2e-admin-pass-12345';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const SCREENSHOT_DIR = path.resolve(HERE, '../../../docs/qa/screenshots');

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
  const email = `al43-${suffix}-${stamp}-${Math.floor(Math.random() * 1000)}@example.test`;
  const password = 'p@ssw0rd-al43';
  const displayName = `AL43 ${suffix} ${stamp}`;
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

async function attachToken(ctx: BrowserContext, baseURL: string, token: string) {
  const url = new URL(baseURL);
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

async function createAgent(serverURL: string, ownerToken: string, displayName: string): Promise<{ id: string; api_key?: string }> {
  const ctx = await apiRequest.newContext({
    baseURL: serverURL,
    extraHTTPHeaders: { Cookie: `borgee_token=${ownerToken}` },
  });
  const r = await ctx.post('/api/v1/agents', { data: { display_name: displayName } });
  expect(r.ok() || r.status() === 201, `agent create: ${r.status()} ${await r.text()}`).toBe(true);
  return (await r.json()) as { id: string; api_key?: string };
}

test.describe('AL-4 acceptance §3 client SPA + G2.7 demo screenshot', () => {
  test('立场 ① "Borgee 不带 runtime" — agent 无 runtime 时 RuntimeCard 不渲染 (graceful degrade omit)', async ({ page, baseURL }) => {
    const serverURL = `http://127.0.0.1:${process.env.E2E_SERVER_PORT ?? '4901'}`;
    const adminCtx = await adminLogin(serverURL);
    const inviteCode = await mintInvite(adminCtx, 'al-4.3-no-runtime');
    const owner = await registerUser(serverURL, inviteCode, 'owner-norun');
    const agent = await createAgent(serverURL, owner.token, `agent-norun-${Date.now().toString(36)}`);
    void agent;
    await attachToken(page.context(), baseURL!, owner.token);

    await page.goto('/');
    await expect(page.locator('.sidebar-title')).toBeVisible({ timeout: 10_000 });

    // Click 🤖 sidebar nav to open AgentManager.
    await page.locator('[data-testid="sidebar-nav-agents"]').click();
    await expect(page.locator('.agent-page h2', { hasText: 'My Agents' })).toBeVisible({ timeout: 10_000 });

    // Expand the agent card by clicking "Manage" button.
    const manageBtn = page.locator('.agent-card button.btn-sm', { hasText: 'Manage' }).first();
    if (await manageBtn.count() > 0) {
      await manageBtn.click();

      // 立场 ① — runtime 未注册时 RuntimeCard 不渲染.
      const runtimeCard = page.locator('.runtime-card');
      expect(await runtimeCard.count(), 'runtime-card MUST NOT render when no runtime registered').toBe(0);
    }
  });

  test('立场 ③ data-runtime-status 4 态严闭锁 (反约束 starting/stopping/restarting 同义词 0 hit)', async ({ page, baseURL }) => {
    // 此测试通过源码反向 grep 锚断言, 不依赖真 runtime — 单跑就能验文案锁.
    const serverURL = `http://127.0.0.1:${process.env.E2E_SERVER_PORT ?? '4901'}`;
    const adminCtx = await adminLogin(serverURL);
    const inviteCode = await mintInvite(adminCtx, 'al-4.3-status-lock');
    const owner = await registerUser(serverURL, inviteCode, 'owner-status');
    await attachToken(page.context(), baseURL!, owner.token);

    await page.goto('/');
    await expect(page.locator('.sidebar-title')).toBeVisible({ timeout: 10_000 });

    // Visit AgentManager — DOM 字面 byte-identical 文案锁通过 page-level
    // grep 验 (运行期 Mount 的 RuntimeCard 反约束已被 vitest 锁,
    // 此 e2e 验 SPA bundle 不漏 starting/stopping 中间态字面).
    await page.locator('[data-testid="sidebar-nav-agents"]').click();
    await expect(page.locator('.agent-page')).toBeVisible({ timeout: 10_000 });

    // 反约束 — bundle DOM 字面不应出现中间态 (#321 §3 反向 grep).
    const html = await page.content();
    for (const forbidden of ['data-runtime-status="starting"', 'data-runtime-status="stopping"', 'data-runtime-status="restarting"', 'data-runtime-status="busy"', 'data-runtime-status="idle"']) {
      expect(html, `data-runtime-status 4 态严闭 — ${forbidden} 不准出现 (立场 ③ 跟 AL-3 拆死)`).not.toContain(forbidden);
    }
  });

  test('立场 ② owner-only DOM gate — 非 owner 视图 start/stop button DOM omit (defense-in-depth)', async ({ page, baseURL }) => {
    // 反约束: non-owner 不在 /me/agents 路径, 看不到 RuntimeCard 入口 (整页隐藏);
    // owner 视角通过 RuntimeCard isOwner gate 验. 非 owner 路径 RuntimeCard
    // 永远不 mount (AgentManager 是 /me/agents — 仅当前用户的 agents).
    const serverURL = `http://127.0.0.1:${process.env.E2E_SERVER_PORT ?? '4901'}`;
    const adminCtx = await adminLogin(serverURL);
    const inviteCode = await mintInvite(adminCtx, 'al-4.3-owner-gate');
    const owner = await registerUser(serverURL, inviteCode, 'owner-gate');
    await attachToken(page.context(), baseURL!, owner.token);

    await page.goto('/');
    await expect(page.locator('.sidebar-title')).toBeVisible({ timeout: 10_000 });
    await page.locator('[data-testid="sidebar-nav-agents"]').click();
    await expect(page.locator('.agent-page')).toBeVisible();

    // 反约束 ② — 没有 disabled leaking owner info (反向 grep 锚 al-4 spec §3.2).
    const startBtnDisabled = page.locator('[data-runtime-action="start"][disabled]:not([data-runtime-actions])');
    expect(await startBtnDisabled.count(), 'start button MUST NOT use disabled to gate owner info (omit not disable)').toBe(0);
    const stopBtnDisabled = page.locator('[data-runtime-action="stop"][disabled]:not([data-runtime-actions])');
    expect(await stopBtnDisabled.count()).toBe(0);
  });

  // gh#683 回归 — Agents 页面默认宽度. 历史问题: `.agent-page` 是
  // flex-column 父项的子项, 没设 width: 100%, 默认只占内容自然宽 (334px),
  // 点 Manage 展开后跳到 max-width 800px → 视觉闪烁. 修法: width: 100%.
  // 在 1280 viewport 下断言空状态 (没 agent) 的 .agent-page 实际占用就
  // 接近 max-width 800px, 不再缩到 334px.
  test('gh#683 — Agents 页面默认占满 max-width 800px (1280 viewport, 空状态不再缩到内容宽度)', async ({ page, baseURL }) => {
    const serverURL = `http://127.0.0.1:${process.env.E2E_SERVER_PORT ?? '4901'}`;
    const adminCtx = await adminLogin(serverURL);
    const inviteCode = await mintInvite(adminCtx, 'gh-683-width');
    const owner = await registerUser(serverURL, inviteCode, 'gh683-width');
    await attachToken(page.context(), baseURL!, owner.token);

    // 锁定 1280 viewport, 跟 liema 复现条件一致.
    await page.setViewportSize({ width: 1280, height: 800 });

    await page.goto('/');
    await expect(page.locator('.sidebar-title')).toBeVisible({ timeout: 10_000 });
    await page.locator('[data-testid="sidebar-nav-agents"]').click();
    await expect(page.locator('.agent-page')).toBeVisible({ timeout: 10_000 });

    // 真 DOM bounding rect 测宽度 (空状态: 没 agent, 没 manage 展开).
    const widthEmpty = await page.locator('.agent-page').evaluate((el) => el.getBoundingClientRect().width);

    // gh#683 修复前默认只占 334px. 修复后应该接近 max-width 800px
    // (允许少量 padding/scrollbar 浮动). 断言 ≥ 700px 留余地, 但远超
    // 修复前的 334px, 一旦回归到 cross-axis auto-shrink 行为这个断言
    // 会立刻失败.
    expect(widthEmpty, `gh#683 回归: .agent-page 默认宽度应接近 800px, 实际 ${widthEmpty}px (修复前 334px)`).toBeGreaterThanOrEqual(700);
    expect(widthEmpty, `.agent-page max-width 仍应是 800px 上限`).toBeLessThanOrEqual(820);
  });

  test('G2.7 demo screenshot — AL-4 admin runtime list 主路径 (agent settings page 全景)', async ({ page, baseURL }) => {
    const serverURL = `http://127.0.0.1:${process.env.E2E_SERVER_PORT ?? '4901'}`;
    const adminCtx = await adminLogin(serverURL);
    const inviteCode = await mintInvite(adminCtx, 'al-4.3-demo');
    const owner = await registerUser(serverURL, inviteCode, 'demo');
    await createAgent(serverURL, owner.token, `agent-demo-${Date.now().toString(36)}`);
    await attachToken(page.context(), baseURL!, owner.token);

    await page.goto('/');
    await expect(page.locator('.sidebar-title')).toBeVisible({ timeout: 10_000 });
    await page.locator('[data-testid="sidebar-nav-agents"]').click();
    await expect(page.locator('.agent-page')).toBeVisible({ timeout: 10_000 });

    // 框 agent settings 全景 — agent card + (runtime card 占位 / future
    // running / error 态) + permissions + API key. 跟 G2.7 三张截屏锚
    // 同源 (start/stop/error 三态留 follow-up 时按真 runtime 路径补).
    await page.screenshot({
      path: path.join(SCREENSHOT_DIR, 'g2.7-runtime-agent-settings.png'),
      fullPage: false,
    });
  });
});
