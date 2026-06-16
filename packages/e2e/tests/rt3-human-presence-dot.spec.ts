// tests/rt3-human-presence-dot.spec.ts — #971 RT3PresenceDot mount + live human presence.
//
// Bug being regression-locked (#971):
//   RT3PresenceDot rendered NOWHERE (a live orphan) and its store feeder
//   markRT3Presence had ZERO production callers, so even if it were mounted it
//   would always show offline. Two-part fix:
//     1. RENDER — ChannelMembersModal renders <RT3PresenceDot userID> on each
//        human (non-agent) member row (agent rows keep the AL-3 PresenceDot).
//     2. DATA  — useWebSocket's `case 'presence'` now also calls
//        markRT3Presence(userId, status) for online/offline, fed by the server
//        broadcast (server-go internal/ws/client.go emits
//        {type:'presence', user_id, status} for every WS user on connect /
//        disconnect).
//
// Test scope (real-browser, two human users):
//   - Admin REST-seeds two human users (A owner, B member) and a shared
//     private channel; A adds B.
//   - User A opens the SPA, clicks the channel, opens the members modal
//     (成员管理 header button) through REAL clicks.
//   - B is NOT connected → A sees B's RT-3 dot = offline (离线).
//   - User B opens the SPA in a second browser context → B's /ws connects →
//     server broadcasts presence online → A's browser receives it →
//     markRT3Presence → B's row dot flips to data-rt3-presence-dot='online'
//     (在线), asserted live in A's open modal.
//   - B's context closes → server broadcasts presence offline → A's dot
//     returns to offline (离线).
//
// Pre-fix expected behavior: the dot never renders at all (orphan), and even
// when rendered would stay offline because nothing feeds the RT-3 store. This
// test asserts both failure modes are gone.
//
// Implementation constraints (project rules):
//   - Real-browser UI: page.goto + click sidebar channel + click 成员管理 +
//     DOM visibility assertions on data-rt3-presence-dot. No page.evaluate(fetch)
//     / cURL-only / route-mock proof of mounting.
//   - REST seed via @playwright/test request context (no production UI to
//     register users / add members / create channels).
//   - All Playwright waits carry explicit timeouts.
import {
  test,
  expect,
  request as apiRequest,
  type APIRequestContext,
} from '@playwright/test';

const ADMIN_LOGIN = 'e2e-admin';
const ADMIN_PASSWORD = 'e2e-admin-pass-12345';

function serverURL(): string {
  return `http://127.0.0.1:${process.env.E2E_SERVER_PORT ?? '4901'}`;
}

async function adminLogin(): Promise<APIRequestContext> {
  const ctx = await apiRequest.newContext({ baseURL: serverURL() });
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
  invite: string,
  label: string,
): Promise<{ ctx: APIRequestContext; userId: string; token: string; displayName: string }> {
  const ctx = await apiRequest.newContext({ baseURL: serverURL() });
  const stamp = `${Date.now()}-${Math.floor(Math.random() * 100000)}`;
  const email = `rt3-${label}-${stamp}@example.test`;
  const displayName = `RT3 ${label} ${stamp}`;
  const res = await ctx.post('/api/v1/auth/register', {
    data: { invite_code: invite, email, password: 'p@ssw0rd-rt3', display_name: displayName },
  });
  expect(res.ok(), `register ${label}: ${res.status()} ${await res.text()}`).toBe(true);
  const body = (await res.json()) as { user: { id: string } };
  const cookies = await ctx.storageState();
  const tok = cookies.cookies.find((c) => c.name === 'borgee_token');
  expect(tok, `${label} borgee_token cookie missing`).toBeTruthy();
  return { ctx, userId: body.user.id, token: tok!.value, displayName };
}

async function createChannel(ownerCtx: APIRequestContext, name: string): Promise<string> {
  const res = await ownerCtx.post('/api/v1/channels', {
    data: { name, visibility: 'private' },
  });
  expect(res.ok(), `channel create: ${res.status()}`).toBe(true);
  const body = (await res.json()) as { channel: { id: string } };
  return body.channel.id;
}

async function addMember(ownerCtx: APIRequestContext, channelId: string, userId: string): Promise<void> {
  const res = await ownerCtx.post(`/api/v1/channels/${channelId}/members`, {
    data: { user_id: userId },
  });
  if (!res.ok() && res.status() !== 409) {
    throw new Error(`add member ${userId}: ${res.status()} ${await res.text()}`);
  }
}

test.describe('RT-3 ⭐ (#971) human presence dot — mount + live online/offline', () => {
  test('A sees B\'s RT-3 dot flip offline → online → offline in the members modal', async ({ browser, baseURL }) => {
    test.setTimeout(60_000);

    // ── REST seed: two human users + shared private channel ──────────────
    const adminCtx = await adminLogin();
    const invA = await mintInvite(adminCtx, 'rt3-owner');
    const invB = await mintInvite(adminCtx, 'rt3-member');
    const userA = await registerUser(invA, 'owner');
    const userB = await registerUser(invB, 'member');

    const channelId = await createChannel(userA.ctx, `rt3-${Date.now().toString(36)}`);
    await addMember(userA.ctx, channelId, userB.userId);

    const url = new URL(baseURL!);
    const cookieFor = (token: string) => ({
      name: 'borgee_token',
      value: token,
      domain: url.hostname,
      path: '/',
      httpOnly: true,
      secure: false,
      sameSite: 'Lax' as const,
    });

    // ── User A: open SPA, open the channel, open the members modal ───────
    const ctxA = await browser.newContext();
    await ctxA.addCookies([cookieFor(userA.token)]);
    const pageA = await ctxA.newPage();
    await pageA.goto('/');
    await expect(pageA.locator('.sidebar-title')).toBeVisible({ timeout: 10_000 });

    // Click the shared channel in the sidebar (real UI navigation).
    const channelRow = pageA.locator('.channel-name').filter({ hasText: 'rt3-' }).first();
    await channelRow.click();

    // Open the members modal via the 成员管理 header button (real click).
    const membersBtn = pageA.locator('button[title="成员管理"]').first();
    await expect(membersBtn).toBeVisible({ timeout: 10_000 });
    await membersBtn.click();

    // The modal renders user B's human member row with the RT-3 dot. The dot
    // is keyed by data-rt3-cursor-user so we target B's row exactly.
    const bDot = pageA.locator(
      `.member-row[data-role="user"] [data-rt3-presence-dot][data-rt3-cursor-user="${userB.userId}"]`,
    );
    await expect(bDot, 'B\'s human row must mount the RT-3 presence dot').toBeVisible({ timeout: 10_000 });

    // ── Initial: B is not connected → dot offline (proves the data wiring
    //    isn't faking online, and that the orphan store is actually read) ──
    await expect(bDot).toHaveAttribute('data-rt3-presence-dot', 'offline');
    await expect(bDot).toHaveAttribute('title', '离线');

    // ── User B connects (opens the SPA) → B's /ws Register → server
    //    broadcasts presence online → A's browser markRT3Presence → flip ──
    const ctxB = await browser.newContext();
    await ctxB.addCookies([cookieFor(userB.token)]);
    const pageB = await ctxB.newPage();
    await pageB.goto('/');
    await expect(pageB.locator('.sidebar-title')).toBeVisible({ timeout: 10_000 });

    // Dot flips to online live inside A's still-open modal.
    await expect(bDot, 'B online broadcast must flip A\'s RT-3 dot to online').toHaveAttribute(
      'data-rt3-presence-dot',
      'online',
      { timeout: 15_000 },
    );
    await expect(bDot).toHaveAttribute('title', '在线');

    // ── User B disconnects → server broadcasts presence offline → flip back ─
    await ctxB.close();
    await expect(bDot, 'B offline broadcast must return A\'s RT-3 dot to offline').toHaveAttribute(
      'data-rt3-presence-dot',
      'offline',
      { timeout: 15_000 },
    );
    await expect(bDot).toHaveAttribute('title', '离线');

    await ctxA.close();
  });
});
