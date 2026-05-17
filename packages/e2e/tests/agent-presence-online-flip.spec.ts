// tests/agent-presence-online-flip.spec.ts — agent connects → PresenceDot flips to 在线.
//
// Bug being regression-locked (PR #989):
//   server emits `presence` frame for every WS user (humans + agents share
//   /ws); old client useWebSocket.ts only routed it into the human user
//   reducer, so the agent presence cache (read by usePresence) stayed empty
//   and PresenceDot fell back to 'offline' / "已离线" even while the agent
//   runtime was online. Fix mirrors `presence` frame into markPresence().
//
// Test scope:
//   - Owner registers, creates agent (REST seed).
//   - Open SPA → AgentManager → verify initial dot = offline (cache empty).
//   - Open a WS to /ws?token=<agent api_key> in this same test process. The
//     server-go hub.go Register / client.go:210 emits
//     `{type:'presence', user_id:<agent.id>, status:'online'}` on connect.
//   - Wait for the browser's own /ws to receive that broadcast frame → fix
//     mirrors into markPresence() → PresenceDot flips to data-presence='online' + 在线.
//   - Close the agent WS → server emits status='offline' broadcast → SPA
//     dot returns to offline + 已离线.
//
// Pre-fix expected behavior: dot stays offline forever — this test asserts
// the failure mode is gone.
//
// Implementation constraints (project rules):
//   - Browser-driven UI assertions (no page.evaluate(fetch) / cURL-only).
//   - REST seed via @playwright/test request context.
//   - Node 22 built-in WebSocket used to act as the agent runtime.
//   - All Playwright waits have explicit timeouts (no infinite loops).
import { test, expect, request as apiRequest } from '@playwright/test';

const ADMIN_LOGIN = 'e2e-admin';
const ADMIN_PASSWORD = 'e2e-admin-pass-12345';

test.describe('AL-3.x agent presence cache fill from `presence` frame', () => {
  test('agent runtime connects → SPA PresenceDot flips offline → online (在线) → offline', async ({
    browser,
    baseURL,
  }) => {
    test.setTimeout(60_000);
    const serverPort = process.env.E2E_SERVER_PORT ?? '4901';
    const serverURL = `http://127.0.0.1:${serverPort}`;
    const wsURL = `ws://127.0.0.1:${serverPort}/ws`;

    // ── REST seed ────────────────────────────────────────────────────────
    const adminCtx = await apiRequest.newContext({ baseURL: serverURL });
    const loginRes = await adminCtx.post('/admin-api/auth/login', {
      data: { login: ADMIN_LOGIN, password: ADMIN_PASSWORD },
    });
    expect(loginRes.ok(), `admin login: ${loginRes.status()}`).toBe(true);

    const inviteRes = await adminCtx.post('/admin-api/v1/invites', {
      data: { note: 'al-3.x-presence-flip' },
    });
    expect(inviteRes.ok(), `mint invite: ${inviteRes.status()}`).toBe(true);
    const inviteCode = ((await inviteRes.json()) as { invite: { code: string } })
      .invite.code;

    const stamp = Date.now();
    const ownerCtx = await apiRequest.newContext({ baseURL: serverURL });
    const ownerReg = await ownerCtx.post('/api/v1/auth/register', {
      data: {
        invite_code: inviteCode,
        email: `al3x-owner-${stamp}@example.test`,
        password: 'p@ssw0rd-al3x',
        display_name: `AL3x Owner ${stamp}`,
      },
    });
    expect(ownerReg.ok(), `owner register: ${ownerReg.status()}`).toBe(true);

    const agentCreate = await ownerCtx.post('/api/v1/agents', {
      data: { display_name: `AL3x Agent ${stamp}` },
    });
    expect(agentCreate.ok(), `agent create: ${agentCreate.status()}`).toBe(true);
    const agentBody = (await agentCreate.json()) as {
      agent: { id: string; api_key?: string };
    };
    const agentAPIKey = agentBody.agent.api_key;
    expect(agentAPIKey, 'agent create response must carry api_key').toBeTruthy();

    // ── Browser as owner ─────────────────────────────────────────────────
    const ownerStorage = await ownerCtx.storageState();
    const tokenCookie = ownerStorage.cookies.find(c => c.name === 'borgee_token');
    expect(tokenCookie, 'borgee_token cookie should exist').toBeTruthy();

    const page = await browser.newPage();
    const url = new URL(baseURL!);
    await page.context().addCookies([{
      name: 'borgee_token',
      value: tokenCookie!.value,
      domain: url.hostname,
      path: '/',
      httpOnly: true,
      secure: false,
      sameSite: 'Lax',
    }]);

    await page.goto('/');

    const agentsNavBtn = page.locator('[data-testid="sidebar-nav-agents"]');
    await expect(agentsNavBtn).toBeVisible({ timeout: 10_000 });
    const agentsListResp = page.waitForResponse(
      (resp) => /\/api\/v1\/agents(\?|$)/.test(resp.url()) && resp.request().method() === 'GET',
      { timeout: 10_000 },
    );
    await agentsNavBtn.click();
    await agentsListResp;

    // ── Initial state: agent has no runtime, dot = offline ───────────────
    // Owner only has one agent (just created), so the first [data-presence]
    // in AgentManager is this agent.
    const dot = page.locator('[data-presence]').first();
    await expect(dot).toBeVisible({ timeout: 5_000 });
    await expect(dot).toHaveAttribute('data-presence', 'offline');
    const badge = page.locator('[data-testid="agent-state-badge"]').first();
    await expect(badge).toContainText('已离线');

    // ── Connect WS as the agent (acts as the "runtime is up") ────────────
    // Server-go `client.go:210` broadcasts `{type:'presence', user_id:<agent.id>,
    // status:'online'}` on Register. Browser's own /ws is already open (the
    // SPA established it on load), so it will receive this broadcast frame.
    // Fix mirrors that into markPresence(agent.id, 'online'), and the SPA
    // re-renders PresenceDot.
    const agentWS = new WebSocket(`${wsURL}?token=${encodeURIComponent(agentAPIKey!)}`);
    const opened = new Promise<void>((resolve, reject) => {
      agentWS.addEventListener('open', () => resolve(), { once: true });
      agentWS.addEventListener('error', () => reject(new Error('agent WS errored before open')), { once: true });
      setTimeout(() => reject(new Error('agent WS open timeout')), 10_000);
    });
    await opened;

    // ── Dot flips to online ──────────────────────────────────────────────
    // 5s server-egress throttle + 5s client throttle worst case ≈ 10s.
    await expect(dot).toHaveAttribute('data-presence', 'online', { timeout: 15_000 });
    const dotInner = dot.locator('.presence-dot');
    await expect(dotInner).toHaveClass(/presence-online/);
    await expect(badge).toContainText('在线');

    // ── Close WS → dot returns to offline ────────────────────────────────
    agentWS.close();
    await expect(dot).toHaveAttribute('data-presence', 'offline', { timeout: 15_000 });
    await expect(badge).toContainText('已离线');
  });
});
