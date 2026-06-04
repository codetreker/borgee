// tests/button-nesting.spec.ts — bf-wo fix-skill-findings / task button-nesting.
//
// AC-5: load a page that renders the channel list in a real browser, capture
// dev-mode console.error for the full session, and assert zero entries match
// /validateDOMNesting.*button.*button/i.
//
// Strategy mirrors channel-sidebar-reorder.spec.ts: admin invite → register a
// user → seed at least one channel via REST → attach the user's session cookie
// to the Playwright context → page.goto('/') → wait for the channel list to
// render → assert no validateDOMNesting button-in-button warning appeared in
// the browser console.
//
// Why a freshly seeded channel: SortableChannelItem only renders for channel
// rows (DM rows take a different path). To exercise the pre-fix bug we need
// at least one channel in the sidebar; otherwise the channel-list section is
// empty and the offending component never mounts.
import {
  test,
  expect,
  request as apiRequest,
  type APIRequestContext,
  type Page,
} from '@playwright/test';

const ADMIN_LOGIN = 'e2e-admin';
const ADMIN_PASSWORD = 'e2e-admin-pass-12345';

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
  const email = `btn-nest-${suffix}-${stamp}-${Math.floor(Math.random() * 1000)}@example.test`;
  const password = 'p@ssw0rd-btn-nest';
  const displayName = `BTN-NEST ${suffix} ${stamp}`;
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

test.describe('bf task button-nesting — channel-list renders without React validateDOMNesting button-in-button warning', () => {
  test('channel-list page emits zero validateDOMNesting button-in-button console.error', async ({
    page,
    baseURL,
  }) => {
    // Capture every console.error for the whole session BEFORE navigating.
    const consoleErrors: string[] = [];
    page.on('console', (msg) => {
      if (msg.type() === 'error') {
        consoleErrors.push(msg.text());
      }
    });
    // pageerror covers uncaught exceptions that some React paths surface as.
    page.on('pageerror', (err) => {
      consoleErrors.push(`pageerror: ${err.message}`);
    });

    const serverPort = process.env.E2E_SERVER_PORT ?? '4901';
    const serverURL = `http://127.0.0.1:${serverPort}`;
    const adminCtx = await adminLogin(serverURL);
    const inviteCode = await mintInvite(adminCtx, 'btn-nest');
    const owner = await registerUser(serverURL, inviteCode, 'owner');
    await createChannel(
      serverURL,
      owner.token,
      `btn-nest-${Date.now().toString(36)}`,
    );
    await attachToken(page, baseURL!, owner.token);

    await page.goto('/');
    // Wait for the sidebar + at least one rendered channel row so we know
    // SortableChannelItem actually mounted; without this the test could pass
    // vacuously on an empty page.
    await expect(page.locator('.sidebar-title')).toBeVisible({ timeout: 10_000 });
    await expect(
      page.locator('.channel-list [data-sortable-handle]').first(),
    ).toBeVisible({ timeout: 10_000 });

    // Allow React's dev-mode warnings to flush (validateDOMNesting fires
    // during render, before paint — a microtask is plenty in practice, but
    // an explicit 500ms idle window catches any deferred warning).
    await page.waitForTimeout(500);

    const offending = consoleErrors.filter((line) =>
      /validateDOMNesting.*button.*button/i.test(line),
    );
    expect(
      offending,
      `validateDOMNesting button-in-button warnings detected: ${JSON.stringify(offending)}`,
    ).toEqual([]);
  });
});
