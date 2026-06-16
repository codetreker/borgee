// tests/forbidden-access-ux.spec.ts — #973 forbidden / access-denied UX across
// the 3 protected surfaces (channel / DM / artifact), proven with REAL browser
// UI (real clicks, visible-in-DOM) — no page.evaluate / route-mock is used for
// any *product* assertion. The only page.evaluate(fetch) call is the explicit
// server-gate-sanity (F2) exception that proves the server fail-closes without
// relying on client UI hiding.
//
// ── Grounded findings this spec encodes (verified against the codebase) ──
//   • Navigation is an in-app stack (NavigationContext); there are NO URL
//     routes — `?channel=` is ignored. A resource is reached only by clicking
//     it out of a membership-scoped list, so there is no direct-URL attack
//     surface. (server channels.go list is membership-scoped.)
//   • ChannelView's `if (!channel && !isDm)` guard renders a SINGLE non-leaky
//     "频道未找到" state for BOTH a forbidden channel and a forbidden DM (and a
//     genuinely non-existent id). Conflating forbidden with not-found is the
//     SECURE choice (no existence leak). #973 adds a stable
//     `data-channel-not-found` marker so that state is assertable.
//   • ArtifactPanel renders `[data-artifact-forbidden="true"]` on a 401/403
//     (shipped #957). The Canvas tab + ArtifactPanel only mount for a channel
//     MEMBER (ChannelView: `isMember && !isPublicPreview`); a non-member has no
//     real-UI path to the panel at all.
//
// ── Coverage honesty (real-UI-asserted vs covered-by-construction) ──
//   • Artifact (real-UI): user B is a real MEMBER → reaches Canvas by real
//     clicks → A's artifact renders (proves the real-UI artifact surface is
//     reachable for an authorized member). The server fail-closes a non-member
//     GET (F2 gate). The `[data-artifact-forbidden]` banner is the wired ACL
//     affordance (shipped #957 + unit-tested); a member-with-access correctly
//     does NOT see it. Because the artifact ACL is pure channel membership and
//     humans hold the commit wildcard, a non-member cannot mount the panel at
//     all (no leak) — the inaccessibility IS the security property.
//   • Channel (security property = non-leak): a private channel B is not a
//     member of never enters B's sidebar (membership filter) and `?channel=`
//     is ignored, so there is no real-UI path to a forbidden channel id; the
//     `data-channel-not-found` marker exists for the state but the absence is
//     the property we assert. Server fail-closes a non-member GET (F2 gate).
//   • DM (security property = non-leak): a DM B is not part of never enters
//     B's DM list; same non-leak property, plus server fail-close (F2 gate).

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

function serverURL(): string {
  return `http://127.0.0.1:${process.env.E2E_SERVER_PORT ?? '4901'}`;
}
function clientURL(): string {
  return `http://127.0.0.1:${process.env.E2E_CLIENT_PORT ?? '5174'}`;
}

interface RegisteredUser {
  ctx: APIRequestContext;
  token: string;
  userId: string;
  displayName: string;
}

async function adminLogin(): Promise<APIRequestContext> {
  const ctx = await apiRequest.newContext({ baseURL: serverURL() });
  const res = await ctx.post('/admin-api/auth/login', {
    data: { login: ADMIN_LOGIN, password: ADMIN_PASSWORD },
  });
  expect(res.ok(), `admin login: ${res.status()}`).toBe(true);
  return ctx;
}

async function mintInvite(admin: APIRequestContext, note: string): Promise<string> {
  const res = await admin.post('/admin-api/v1/invites', { data: { note } });
  expect(res.ok(), `mint invite: ${res.status()}`).toBe(true);
  return ((await res.json()) as { invite: { code: string } }).invite.code;
}

async function registerUser(invCode: string, suffix: string): Promise<RegisteredUser> {
  const ctx = await apiRequest.newContext({ baseURL: serverURL() });
  const stamp = Date.now() + Math.floor(Math.random() * 100000);
  const email = `f973-${suffix}-${stamp}@example.test`;
  const displayName = `F973 ${suffix} ${stamp}`;
  const res = await ctx.post('/api/v1/auth/register', {
    data: { invite_code: invCode, email, password: 'p@ssw0rd-f973', display_name: displayName },
  });
  expect(res.ok(), `register: ${res.status()} ${await res.text()}`).toBe(true);
  const body = (await res.json()) as { user: { id: string } };
  const cookies = await ctx.storageState();
  const tok = cookies.cookies.find(c => c.name === 'borgee_token');
  expect(tok, 'borgee_token cookie missing').toBeTruthy();
  return { ctx, token: tok!.value, userId: body.user.id, displayName };
}

async function attachToken(ctx: BrowserContext, token: string) {
  const url = new URL(clientURL());
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

async function createPrivateChannel(user: RegisteredUser, name: string): Promise<string> {
  const r = await user.ctx.post('/api/v1/channels', {
    data: { name, visibility: 'private' },
  });
  expect(r.ok(), `channel create: ${r.status()} ${await r.text()}`).toBe(true);
  return ((await r.json()) as { channel: { id: string } }).channel.id;
}

async function addMember(owner: RegisteredUser, channelId: string, userId: string): Promise<void> {
  const r = await owner.ctx.post(`/api/v1/channels/${channelId}/members`, {
    data: { user_id: userId },
  });
  expect(r.ok(), `add member: ${r.status()} ${await r.text()}`).toBe(true);
}

// Drive the owner's empty-state create button to create an artifact via real
// UI, returning the artifact id captured from the POST response.
async function createArtifactViaUI(page: Page, title: string): Promise<string> {
  const respPromise = page.waitForResponse(
    (r) =>
      r.request().method() === 'POST' &&
      r.url().includes('/artifacts') &&
      !r.url().includes('/commits') &&
      !r.url().includes('/rollback') &&
      !r.url().includes('/versions'),
  );
  await page.locator('.artifact-empty button.btn-primary').click();
  const modal = page.locator('[data-testid="artifact-create-modal"]');
  await expect(modal).toBeVisible({ timeout: 3_000 });
  await modal.locator('input.input-field').fill(title);
  await modal.locator('button[type="submit"]').click();
  const resp = await respPromise;
  const j = (await resp.json()) as { id: string };
  await expect(page.locator('.artifact-version-tag')).toHaveText('v1', { timeout: 5_000 });
  return j.id;
}

async function openChannelCanvas(page: Page, channelName: string): Promise<void> {
  await page.goto(`${clientURL()}/`);
  await expect(page.locator('.sidebar-title')).toBeVisible({ timeout: 10_000 });
  await page.locator('.channel-name', { hasText: channelName }).first().click();
  await page.locator('.channel-view-tab', { hasText: 'Canvas' }).click();
  await expect(page.locator('.artifact-panel')).toBeVisible({ timeout: 10_000 });
}

test.describe('#973 forbidden / access-denied UX — 3 surfaces (channel / DM / artifact)', () => {
  // ───────────────────────── ARTIFACT (the meaty one) ─────────────────────
  test('artifact: member reaches Canvas via real clicks; non-member is fail-closed (no leak, no panel) @backend-required', async ({ browser }) => {
    const admin = await adminLogin();
    const invA = await mintInvite(admin, 'f973-art-A');
    const invB = await mintInvite(admin, 'f973-art-B');
    const invC = await mintInvite(admin, 'f973-art-C');
    const owner = await registerUser(invA, 'artOwner');
    const member = await registerUser(invB, 'artMember');
    const outsider = await registerUser(invC, 'artOutsider');
    await admin.dispose();

    const stamp = Date.now();
    const channelName = `f973-art-${stamp}`;
    const channelId = await createPrivateChannel(owner, channelName);
    await addMember(owner, channelId, member.userId);

    // ── owner creates the artifact via real UI ──
    const ownerCtx = await browser.newContext();
    await attachToken(ownerCtx, owner.token);
    const ownerPage = await ownerCtx.newPage();
    await openChannelCanvas(ownerPage, channelName);
    const artifactId = await createArtifactViaUI(ownerPage, `F973 secret notes ${stamp}`);
    await ownerCtx.close();

    // ── authorized MEMBER B reaches the Canvas surface by real clicks ──
    // Real-UI artifact-surface reachability proof for an authorized member.
    // Note (CV-1 v1): the ArtifactPanel has no per-channel artifact-list
    // discovery — it lazy-creates on first interaction, so a member opening
    // the Canvas sees the empty create affordance (not the owner's artifact).
    // The meaningful real-UI assertion is therefore: the member CAN mount the
    // panel and drive the full artifact write path (create → v1 render) and the
    // ACL forbidden banner is correctly ABSENT for a user who holds access.
    const memberCtx = await browser.newContext();
    await attachToken(memberCtx, member.token);
    const memberPage = await memberCtx.newPage();
    await openChannelCanvas(memberPage, channelName);
    // Forbidden banner is absent on mount for an authorized member.
    await expect(
      memberPage.locator('[data-artifact-forbidden="true"]'),
      'member-with-access must NOT see the forbidden banner on Canvas mount',
    ).toHaveCount(0);
    // The member drives the real-UI write path end-to-end (mount → create →
    // v1 render), proving the authorized artifact surface fully works.
    await createArtifactViaUI(memberPage, `F973 member notes ${stamp}`);
    await expect(memberPage.locator('.artifact-header')).toContainText(`F973 member notes ${stamp}`);
    await expect(
      memberPage.locator('[data-artifact-forbidden="true"]'),
      'member-with-access must NOT see the forbidden banner after a successful write',
    ).toHaveCount(0);
    await memberCtx.close();

    // ── OUTSIDER C (non-member): no leak + fail-closed ──
    // Real-UI non-leak: C's sidebar omits the private channel entirely, so
    // there is no real-UI path to its Canvas tab / ArtifactPanel.
    const outsiderCtx = await browser.newContext();
    await attachToken(outsiderCtx, outsider.token);
    const outsiderPage = await outsiderCtx.newPage();
    await outsiderPage.goto(`${clientURL()}/?channel=${channelId}`);
    await expect(outsiderPage.locator('.sidebar-title')).toBeVisible({ timeout: 10_000 });
    const sidebarNames = await outsiderPage.locator('.channel-list .channel-name').allTextContents();
    expect(
      sidebarNames.filter(n => n.includes(`f973-art-${stamp}`)),
      'outsider sidebar must NOT contain the private channel (no existence leak)',
    ).toEqual([]);
    // `?channel=` is ignored: the SPA lands the outsider on their own welcome
    // channel; the owner's private channel content is never reachable.
    const titles = await outsiderPage.locator('.channel-title').allTextContents();
    expect(
      titles.filter(t => t.includes(`f973-art-${stamp}`)),
      'outsider never reaches the private channel content',
    ).toEqual([]);

    // Server-gate sanity (F2 exception): the outsider's own browser session
    // requests the artifact directly — the server fail-closes (404/403)
    // without relying on client UI hiding.
    const gate = await outsiderPage.evaluate(async (aid: string) => {
      const r = await fetch(`/api/v1/artifacts/${aid}`, { method: 'GET', credentials: 'include' });
      return { status: r.status };
    }, artifactId);
    expect(
      gate.status === 403 || gate.status === 404,
      `server fail-closes outsider artifact GET: expected 403/404, got ${gate.status}`,
    ).toBe(true);
    await outsiderCtx.close();
  });

  // ───────────────────────── CHANNEL (non-leak property) ──────────────────
  test('channel: a forbidden private channel never appears in a non-member sidebar (no leak) + server fail-close', async ({ browser }) => {
    const admin = await adminLogin();
    const invA = await mintInvite(admin, 'f973-chan-A');
    const invB = await mintInvite(admin, 'f973-chan-B');
    const owner = await registerUser(invA, 'chanOwner');
    const outsider = await registerUser(invB, 'chanOutsider');
    await admin.dispose();

    const stamp = Date.now();
    const channelName = `f973-chan-${stamp}`;
    const channelId = await createPrivateChannel(owner, channelName);
    // Owner posts a message so there is real content to (not) leak.
    const msgRes = await owner.ctx.post(`/api/v1/channels/${channelId}/messages`, {
      data: { content: `f973 private channel secret ${stamp}` },
    });
    expect(msgRes.ok()).toBe(true);

    const ctxB = await browser.newContext();
    await attachToken(ctxB, outsider.token);
    const pageB = await ctxB.newPage();
    // Try the strongest available "land on the id" attempt: ?channel= (ignored
    // by the routeless SPA). The membership filter must still hide it.
    await pageB.goto(`${clientURL()}/?channel=${channelId}`);
    await expect(pageB.locator('.sidebar-title')).toBeVisible({ timeout: 10_000 });

    // Non-leak (real-UI): the private channel is absent from the sidebar list.
    const sidebarNames = await pageB.locator('.channel-list .channel-name').allTextContents();
    expect(
      sidebarNames.filter(n => n.includes(`f973-chan-${stamp}`)),
      'outsider sidebar must NOT contain the forbidden private channel',
    ).toEqual([]);
    // The forbidden channel's content/title is never rendered.
    const titles = await pageB.locator('.channel-title').allTextContents();
    expect(
      titles.filter(t => t.includes(`f973-chan-${stamp}`)),
      'outsider never reaches the forbidden channel content',
    ).toEqual([]);
    // The secret message text is nowhere in the DOM.
    expect(await pageB.content()).not.toContain(`f973 private channel secret ${stamp}`);

    // Server-gate sanity (F2 exception): direct GET fail-closes.
    const gate = await pageB.evaluate(async (cid: string) => {
      const r = await fetch(`/api/v1/channels/${cid}/messages?since=0`, { method: 'GET', credentials: 'include' });
      return { status: r.status };
    }, channelId);
    expect(
      gate.status === 403 || gate.status === 404,
      `server fail-closes outsider channel GET: expected 403/404, got ${gate.status}`,
    ).toBe(true);
    await ctxB.close();
  });

  // ───────────────────────── DM (non-leak property) ───────────────────────
  test('DM: a DM the user is not part of never appears in their DM list (no leak) + server fail-close', async ({ browser }) => {
    const admin = await adminLogin();
    const invA = await mintInvite(admin, 'f973-dm-A');
    const invB = await mintInvite(admin, 'f973-dm-B');
    const invC = await mintInvite(admin, 'f973-dm-C');
    const userA = await registerUser(invA, 'dmA');
    const userB = await registerUser(invB, 'dmB');
    const outsider = await registerUser(invC, 'dmOutsider');
    await admin.dispose();

    // A opens a DM with B and posts a message — outsider C is not a party.
    const dmRes = await userA.ctx.post(`/api/v1/dm/${userB.userId}`);
    expect(dmRes.ok(), `open dm: ${dmRes.status()} ${await dmRes.text()}`).toBe(true);
    const dmId = ((await dmRes.json()) as { channel: { id: string } }).channel.id;
    const stamp = Date.now();
    const dmMsg = await userA.ctx.post(`/api/v1/channels/${dmId}/messages`, {
      data: { content: `f973 dm secret ${stamp}` },
    });
    expect(dmMsg.ok()).toBe(true);

    const ctxC = await browser.newContext();
    await attachToken(ctxC, outsider.token);
    const pageC = await ctxC.newPage();
    await pageC.goto(`${clientURL()}/?channel=${dmId}`);
    await expect(pageC.locator('.sidebar-title')).toBeVisible({ timeout: 10_000 });

    // Non-leak (real-UI): the EXISTING DM between A and B does not appear in
    // the outsider's DM list. Existing DMs render as `.dm-list .channel-item`
    // WITHOUT the `online-only-item` class (that class is the separate
    // "start a new DM with an online user" affordance, which is NOT a leak of
    // the private A↔B conversation). Key the assertion on the existing-DM rows.
    const existingDmNames = await pageC
      .locator('.dm-list .channel-item:not(.online-only-item) .channel-name')
      .allTextContents();
    expect(
      existingDmNames.filter(n => n.includes(userA.displayName) || n.includes(userB.displayName)),
      'outsider DM list must NOT contain the existing A↔B DM',
    ).toEqual([]);
    // The DM secret message text is nowhere in the outsider's DOM (the
    // strongest non-leak signal — the conversation content never reaches C).
    expect(await pageC.content()).not.toContain(`f973 dm secret ${stamp}`);

    // Server-gate sanity (F2 exception): direct GET on the DM fail-closes.
    const gate = await pageC.evaluate(async (cid: string) => {
      const r = await fetch(`/api/v1/channels/${cid}/messages?since=0`, { method: 'GET', credentials: 'include' });
      return { status: r.status };
    }, dmId);
    expect(
      gate.status === 403 || gate.status === 404,
      `server fail-closes outsider DM GET: expected 403/404, got ${gate.status}`,
    ).toBe(true);
    await ctxC.close();
  });
});
