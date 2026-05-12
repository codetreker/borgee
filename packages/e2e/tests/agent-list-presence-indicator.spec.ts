// tests/agent-list-presence-indicator.spec.ts — agent-list online status indicator.
//
// Test scope:
//   - Newly created agent without runtime renders `data-presence="offline"` + text "已离线".
//   - In AgentManager, every [data-presence] row role must be agent; user rows have no presence slot.
//   - Cross-org owner sees the same offline DOM shape for agent rows, without org-specific divergence.
//
// Follow-up:
//   - Add online / error states after server-side presence.changed push lands (al-3.md §2.5).
//   - Add the full cross-org path after PR #318 invitation acceptance is complete (al-3.md §3.4).
//
// Related docs:
//   - Acceptance: docs/_archive/qa/acceptance-templates/al-3.md §3.1 / §3.2 / §3.4
//
// Implementation constraints:
//   - Browser-driven UI path: page.goto + DOM assertions.
//   - Seed data through REST: admin login + invite + register + POST /api/v1/agents.
//   - Main test path uses the SPA AgentManager view and checks DOM.
//   - Do not use fs.*, page.evaluate(fetch), API-only checks, or empty placeholder tests.
import { test, expect, request as apiRequest } from '@playwright/test';

const ADMIN_LOGIN = 'e2e-admin';
const ADMIN_PASSWORD = 'e2e-admin-pass-12345';

test.describe('AL-3.3 client SPA presence dot (al-3.md §3.1 / §3.2)', () => {
  test('§3.1 default offline + §3.2 only-agent reverse: 新建 agent 渲染 data-presence="offline" + "已离线", 人行无 [data-presence]', async ({
    browser,
    baseURL,
  }) => {
    const serverPort = process.env.E2E_SERVER_PORT ?? '4901';
    const serverURL = `http://127.0.0.1:${serverPort}`;
    const adminCtx = await apiRequest.newContext({ baseURL: serverURL });

    const loginRes = await adminCtx.post('/admin-api/auth/login', {
      data: { login: ADMIN_LOGIN, password: ADMIN_PASSWORD },
    });
    expect(loginRes.ok(), `admin login: ${loginRes.status()}`).toBe(true);

    const inviteRes = await adminCtx.post('/admin-api/v1/invites', {
      data: { note: 'al-3.3-presence-owner' },
    });
    expect(inviteRes.ok(), `mint invite: ${inviteRes.status()}`).toBe(true);
    const inviteCode = ((await inviteRes.json()) as { invite: { code: string } })
      .invite.code;

    // Register owner and create agent. The agent has no runtime, so REST returns state='offline'.
    const stamp = Date.now();
    const ownerCtx = await apiRequest.newContext({ baseURL: serverURL });
    const ownerReg = await ownerCtx.post('/api/v1/auth/register', {
      data: {
        invite_code: inviteCode,
        email: `al33-owner-${stamp}@example.test`,
        password: 'p@ssw0rd-al33',
        display_name: `AL33 Owner ${stamp}`,
      },
    });
    expect(ownerReg.ok(), `owner register: ${ownerReg.status()}`).toBe(true);

    const agentRes = await ownerCtx.post('/api/v1/agents', {
      data: { display_name: `AL33 Agent ${stamp}` },
    });
    expect(agentRes.ok(), `agent create: ${agentRes.status()}`).toBe(true);

    // Forward owner cookie to the SPA context.
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

    // Enter AgentManager through the persistent sidebar nav [data-testid="sidebar-nav-agents"].
    // Do not use the #welcome quick-action button: that path flips only
    // CreateAgentModal and does not enter AgentList, so [data-presence] never renders.
    // Race guard: wait for GET /api/v1/agents, which AgentManager must call before
    // rendering the list, then check the dot. Avoid networkidle because Vite HMR WS
    // can keep it open indefinitely.
    const agentsNavBtn = page.locator('[data-testid="sidebar-nav-agents"]');
    await expect(agentsNavBtn).toBeVisible({ timeout: 10_000 });
    const agentsListResp = page.waitForResponse(
      (resp) => /\/api\/v1\/agents(\?|$)/.test(resp.url()) && resp.request().method() === 'GET',
      { timeout: 10_000 },
    );
    await agentsNavBtn.click();
    await agentsListResp;

    // §3.1 — AL-3.3 PresenceDot DOM literal lock. Check [data-presence] directly;
    // do not rely on the AL-1a [data-testid="agent-state-badge"] entry. AL-1a +
    // AL-3 nesting is expected, but this e2e must use AL-3's own entry so internal
    // AL-1a coupling cannot hide drift.
    const dot = page.locator('[data-presence]').first();
    await expect(dot).toBeVisible({ timeout: 5000 });
    await expect(dot).toHaveAttribute('data-presence', 'offline');
    // Class literal lock: nested .presence-dot child carries .presence-offline
    // (PresenceDot.tsx:38). The outer [data-presence] wrapper class is
    // .presence-inline; the color class belongs to the child span.
    const dotInner = dot.locator('.presence-dot');
    await expect(dotInner).toHaveClass(/presence-offline/);
    // Text literal "已离线" is locked by describeAgentState() and #305 content lock.
    // In compact mode, PresenceDot copy is in sr-only / title; use the nearest
    // badge container to check visible copy.
    const badge = page.locator('[data-testid="agent-state-badge"]').first();
    await expect(badge).toContainText('已离线');
    // §5.1 negative constraint: badge does not show busy / idle / 忙 / 空闲.
    const badgeText = (await badge.textContent()) ?? '';
    expect(badgeText).not.toMatch(/busy|idle|忙|空闲/i);

    // §3.2 only-agent check: full page [data-role="user"][data-presence] count==0.
    // Sidebar DM rows / ChannelMembersModal rows carry data-role; only agent role renders PresenceDot.
    const peopleWithPresence = page.locator('[data-role="user"] [data-presence]');
    await expect(peopleWithPresence).toHaveCount(0);
    const adminsWithPresence = page.locator('[data-role="admin"] [data-presence]');
    await expect(adminsWithPresence).toHaveCount(0);

    // TODO(AL-3.x): after server `presence.changed` push frame lands (§2.5 TBD),
    // add online → data-presence="online" + "在线" and the two error-copy cases
    // covering the 6 reason codes. This phase only locks the default offline state.

    // TODO(AL-3.x cross-org §3.4): cross-org owner views should keep the same
    // agent-row DOM shape. Add this case after #318 invitation acceptance and
    // push frame support are both ready.
  });
});
