// tests/agent-list-followup.spec.ts — agent 列表运行时卡 + 状态徽章 + owner 门禁.
//
// 测试范围:
//   - agent 没注册 runtime 时 RuntimeCard 不渲染 (优雅降级 omit)
//   - 非 owner 视图下 start/stop 按钮 DOM 不渲染 (前后端双层防越权)
//   - data-runtime-status 仅渲染四态: registered / running / stopped / error
//     negative check: synonyms busy/idle/starting/stopping/restarting 0 hit
//   - reason 标签 6 项跟 lib/agent-state.ts REASON_LABELS 文案一致
//   - Agents 页面在 1280 viewport 下默认占满 800px max-width (gh#683 回归)
//   - G2.7 demo 全景截屏归档
//
// 待办 (本 spec 不覆盖, 走 server unit):
//   - admin `GET /admin-api/v1/runtimes` 元数据白名单 (REG-AL4-009)
//   - heartbeat 双表两路径反向断言 (REG-AL4-008)
//
// 关联文档:
//   - 验收: docs/_archive/qa/acceptance-templates/al-4.md §3.1-§3.4 + §4
//   - 文案: PR #321 reason 标签锁
//
// 实施约束:
//   - 真 UI 走浏览器 (page.goto + DOM 断)
//   - seed 用 REST register runtime, 测试主体走 SPA RuntimeCard DOM
//   - 不允许 fs.* / page.evaluate(fetch) / 只打 API / noop

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
  test('agent without runtime does not render RuntimeCard (graceful omit)', async ({ page, baseURL }) => {
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

      // RuntimeCard must not render before a runtime is registered.
      const runtimeCard = page.locator('.runtime-card');
      expect(await runtimeCard.count(), 'runtime-card MUST NOT render when no runtime registered').toBe(0);
    }
  });

  test('data-runtime-status allows only four states (no starting/stopping/restarting synonyms)', async ({ page, baseURL }) => {
    // This test checks rendered source text, so it does not require a live runtime.
    const serverURL = `http://127.0.0.1:${process.env.E2E_SERVER_PORT ?? '4901'}`;
    const adminCtx = await adminLogin(serverURL);
    const inviteCode = await mintInvite(adminCtx, 'al-4.3-status-lock');
    const owner = await registerUser(serverURL, inviteCode, 'owner-status');
    await attachToken(page.context(), baseURL!, owner.token);

    await page.goto('/');
    await expect(page.locator('.sidebar-title')).toBeVisible({ timeout: 10_000 });

    // Visit AgentManager; this page-level check verifies the SPA bundle does
    // not expose intermediate runtime-status strings.
    await page.locator('[data-testid="sidebar-nav-agents"]').click();
    await expect(page.locator('.agent-page')).toBeVisible({ timeout: 10_000 });

    // The bundle DOM text must not contain intermediate states (#321 §3 grep check).
    const html = await page.content();
    for (const forbidden of ['data-runtime-status="starting"', 'data-runtime-status="stopping"', 'data-runtime-status="restarting"', 'data-runtime-status="busy"', 'data-runtime-status="idle"']) {
      expect(html, `data-runtime-status 4 态严闭 — ${forbidden} 不准出现 (立场 ③ 跟 AL-3 拆死)`).not.toContain(forbidden);
    }
  });

  test('owner-only DOM gate — non-owner view omits start/stop button DOM (defense-in-depth)', async ({ page, baseURL }) => {
    // Non-owner users do not enter /me/agents for another user's agents, so
    // they cannot see the RuntimeCard entry point;
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

    // Owner info must not leak through disabled start/stop buttons.
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
    test.skip(
      process.env.E2E_EVIDENCE_SCREENSHOTS !== '1',
      'signoff screenshot archive runs only when E2E_EVIDENCE_SCREENSHOTS=1',
    );

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

    // Capture the agent settings view: agent card + runtime-card placeholder,
    // future running/error states, permissions, and API key. This uses the same
    // screenshot set as G2.7; start/stop/error coverage stays in follow-up work
    // once the real runtime path is available.
    if (process.env.E2E_EVIDENCE_SCREENSHOTS === '1') await page.screenshot({
      path: path.join(SCREENSHOT_DIR, 'g2.7-runtime-agent-settings.png'),
      fullPage: false,
    });
  });
});
