// tests/agent-presence-plugin-online.spec.ts — fix/agent-presence-online.
//
// Bug being regression-locked:
//   When agents connect via the /ws/plugin path (server-go HandlePlugin
//   — the path the real Borgee plugin runtime uses), the server was NOT
//   calling `hub.store.UpdateLastSeen` nor broadcasting the `presence`
//   frame the way /ws (HandleClient) does. Result: PresenceDot for that
//   agent stayed gray in every observer's UI even though the runtime was
//   live (REST `/api/v1/online` excluded the agent and no peer's
//   usePresence cache ever filled).
//
// Test scope (mirrors agent-presence-online-flip.spec.ts but for the
// /ws/plugin path — that test alone does NOT cover this bug because it
// dials /ws, which always worked):
//   - Owner registers + creates agent.
//   - SPA → Agents tab → initial dot = offline.
//   - Open a WebSocket to /ws/plugin with the agent api_key on the
//     Authorization: Bearer header in this test process (the `?apiKey=` query
//     form was dropped in #1031 — header-only auth).
//   - Wait for the SPA dot to flip to data-presence='online' (背景色 真测
//     resolves to var(--success) RGB) + 在线 badge.
//   - Close plugin WS → dot returns to offline.

import { test, expect, request as apiRequest } from '@playwright/test';

const ADMIN_LOGIN = 'e2e-admin';
const ADMIN_PASSWORD = 'e2e-admin-pass-12345';

test.describe('fix/agent-presence-online: agent runtime via /ws/plugin → PresenceDot', () => {
  test('plugin runtime connects → SPA PresenceDot flips offline → online → offline', async ({
    browser,
    baseURL,
  }) => {
    test.setTimeout(60_000);
    const serverPort = process.env.E2E_SERVER_PORT ?? '4901';
    const serverURL = `http://127.0.0.1:${serverPort}`;
    const pluginWSURL = `ws://127.0.0.1:${serverPort}/ws/plugin`;

    // ── REST seed ────────────────────────────────────────────────────────
    const adminCtx = await apiRequest.newContext({ baseURL: serverURL });
    const loginRes = await adminCtx.post('/admin-api/auth/login', {
      data: { login: ADMIN_LOGIN, password: ADMIN_PASSWORD },
    });
    expect(loginRes.ok(), `admin login: ${loginRes.status()}`).toBe(true);

    const inviteRes = await adminCtx.post('/admin-api/v1/invites', {
      data: { note: 'plugin-presence-fix' },
    });
    expect(inviteRes.ok(), `mint invite: ${inviteRes.status()}`).toBe(true);
    const inviteCode = ((await inviteRes.json()) as { invite: { code: string } })
      .invite.code;

    const stamp = Date.now();
    const ownerCtx = await apiRequest.newContext({ baseURL: serverURL });
    const ownerReg = await ownerCtx.post('/api/v1/auth/register', {
      data: {
        invite_code: inviteCode,
        email: `plugin-presence-owner-${stamp}@example.test`,
        password: 'p@ssw0rd-plugin',
        display_name: `Plugin Presence Owner ${stamp}`,
      },
    });
    expect(ownerReg.ok(), `owner register: ${ownerReg.status()}`).toBe(true);

    const agentCreate = await ownerCtx.post('/api/v1/agents', {
      data: { display_name: `Plugin Presence Agent ${stamp}` },
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
    const dot = page.locator('[data-presence]').first();
    await expect(dot).toBeVisible({ timeout: 5_000 });
    await expect(dot).toHaveAttribute('data-presence', 'offline');

    // ── Connect plugin WS (the /ws/plugin path the bug missed) ───────────
    // Server-go HandlePlugin (post-fix) calls UpdateLastSeen +
    // BroadcastToAll(presence online) on register — the SPA's own /ws
    // mirrors that into markPresence(agent.id, 'online') and the dot
    // flips.
    const pluginWS = new WebSocket(pluginWSURL, {
      headers: { Authorization: `Bearer ${agentAPIKey!}` },
    });
    await new Promise<void>((resolve, reject) => {
      pluginWS.addEventListener('open', () => resolve(), { once: true });
      pluginWS.addEventListener('error', () => reject(new Error('plugin WS errored before open')), { once: true });
      setTimeout(() => reject(new Error('plugin WS open timeout')), 10_000);
    });

    // ── Dot flips to online + computed style is the success token color ──
    await expect(dot).toHaveAttribute('data-presence', 'online', { timeout: 15_000 });
    const dotInner = dot.locator('.presence-dot');
    await expect(dotInner).toHaveClass(/presence-online/);

    // Real computed style assertion — the bug's user-visible symptom was a
    // gray dot, so we lock the actual background color resolved from the
    // CSS var. Compare resolved RGB of `var(--success)` and the dot's
    // computed background to defeat any "looks the same" coincidence.
    const colors = await page.evaluate(() => {
      const el = document.querySelector('[data-presence="online"] .presence-dot') as HTMLElement | null;
      if (!el) return null;
      const dotBg = window.getComputedStyle(el).backgroundColor;
      const probe = document.createElement('div');
      probe.style.backgroundColor = 'var(--success)';
      document.body.appendChild(probe);
      const successBg = window.getComputedStyle(probe).backgroundColor;
      probe.remove();
      return { dotBg, successBg };
    });
    expect(colors, 'computed style lookup must find the online dot').toBeTruthy();
    // The dot's backgroundColor must resolve to a real non-grey RGB and match
    // the var(--success) probe — gray-on-gray was the regressed state.
    expect(colors!.dotBg).not.toBe('');
    expect(colors!.dotBg).not.toBe('rgba(0, 0, 0, 0)');
    expect(colors!.dotBg).toBe(colors!.successBg);

    // ── Close plugin → dot returns to offline (offline broadcast path) ───
    pluginWS.close();
    await expect(dot).toHaveAttribute('data-presence', 'offline', { timeout: 15_000 });
  });
});
