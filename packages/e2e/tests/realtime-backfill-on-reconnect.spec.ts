// tests/rt-1-2-backfill-on-reconnect.spec.ts — RT-1.2 (#290 follow) e2e.
//
// Behavioral anchors (RT-1 spec §1.2):
//   ① WS 断 → 重连后 client 调 GET /api/v1/events?since=<last_seen_cursor>
//      把断线期间漏掉的 event 拉回来 (≤3s 完成)
//   ② Negative constraint: cold start (sessionStorage empty) must not default to fetching all history —
//      since=0 时 client 跳过 backfill, 不打 /api/v1/events
//   ③ server 永不返回 cursor <= since 的事件 (服务端契约, 客户端
//      `last_seen_cursor` dedup fail-closed)
//
// Implementation note: the full "disconnect 5s + 5 commits" scenario needs the
// server to inject ArtifactUpdated events. The CV-1 artifact table is not landed
// yet (Phase 3+), so this test uses the existing messages → events path.
//
// **Reconnect trigger** — `page.context().setOffline(true)` only blocks new
// sockets / new HTTP. It does not close an already-open WebSocket frame stream.
// In the browser view, WS stays OPEN, `onclose` never fires, `scheduleReconnect`
// is not called, and `wasReconnect` remains false, so backfill never triggers.
// That was the root cause of repeated #297 e2e flakes; it was not a timing flake
// because `backfillCalls.length` stayed 0.
//
// Fix: `addInitScript` wraps `window.WebSocket`, stores each new instance on
// `window.__lastWS`, and the test calls `.close()` through `evaluate` to trigger
// onclose → reconnect → backfill. That matches the user-visible network-drop
// chain: OS resets the connection and the browser receives a close frame.
// Playwright `setOffline` does not provide that behavior.

import { test, expect, request as apiRequest, type APIRequestContext, type Page } from '@playwright/test';

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

async function registerUser(serverURL: string, inviteCode: string, suffix: string): Promise<RegisteredUser> {
  const ctx = await apiRequest.newContext({ baseURL: serverURL });
  const stamp = Date.now();
  const email = `rt12-${suffix}-${stamp}@example.test`;
  const password = 'p@ssw0rd-rt12';
  const displayName = `RT12 ${suffix} ${stamp}`;
  const res = await ctx.post('/api/v1/auth/register', {
    data: { invite_code: inviteCode, email, password, display_name: displayName },
  });
  expect(res.ok(), `register: ${res.status()} ${await res.text()}`).toBe(true);
  const body = (await res.json()) as { user: { id: string } };
  const cookies = await ctx.storageState();
  const tok = cookies.cookies.find(c => c.name === 'borgee_token');
  expect(tok, 'borgee_token cookie missing').toBeTruthy();
  return { email, token: tok!.value, userId: body.user.id };
}

async function attachToken(page: Page, baseURL: string, token: string) {
  const url = new URL(baseURL);
  await page.context().clearCookies();
  await page.context().addCookies([{
    name: 'borgee_token',
    value: token,
    domain: url.hostname,
    path: '/',
    httpOnly: true,
    secure: false,
    sameSite: 'Lax',
  }]);
}

// installWsCapture wraps `window.WebSocket` so each constructed
// instance is stashed at `window.__lastWS`. Tests use this to force a
// real `onclose` (which `setOffline` does NOT do) so the client's
// reconnect→backfill path actually fires.
async function installWsCapture(page: Page) {
  await page.addInitScript(() => {
    const NativeWS = window.WebSocket;
    const Wrapped = function (this: WebSocket, url: string | URL, protocols?: string | string[]) {
      const ws = new NativeWS(url, protocols);
      (window as unknown as { __lastWS?: WebSocket }).__lastWS = ws;
      return ws;
    } as unknown as typeof WebSocket;
    Wrapped.prototype = NativeWS.prototype;
    (Wrapped as unknown as { CONNECTING: number }).CONNECTING = NativeWS.CONNECTING;
    (Wrapped as unknown as { OPEN: number }).OPEN = NativeWS.OPEN;
    (Wrapped as unknown as { CLOSING: number }).CLOSING = NativeWS.CLOSING;
    (Wrapped as unknown as { CLOSED: number }).CLOSED = NativeWS.CLOSED;
    (window as unknown as { WebSocket: typeof WebSocket }).WebSocket = Wrapped;
  });
}

test.describe('RT-1.2 client backfill on reconnect', () => {
  test('立场 ②: cold start does NOT auto-pull history (反约束)', async ({ page, baseURL }) => {
    const serverPort = process.env.E2E_SERVER_PORT ?? '4901';
    const serverURL = `http://127.0.0.1:${serverPort}`;
    const adminCtx = await adminLogin(serverURL);
    const inviteCode = await mintInvite(adminCtx, 'rt-1.2-cold');
    const user = await registerUser(serverURL, inviteCode, 'cold');
    await attachToken(page, baseURL!, user.token);

    // Track all GET /api/v1/events calls. Cold start (sessionStorage
    // empty) MUST NOT issue a backfill.
    const backfillCalls: string[] = [];
    page.on('request', req => {
      if (req.method() === 'GET' && req.url().includes('/api/v1/events')) {
        backfillCalls.push(req.url());
      }
    });

    await page.goto('/');
    await expect(page.locator('.sidebar-title')).toBeVisible();
    // Idle window long enough for the WS open to settle but short
    // enough to keep CI fast.
    await page.waitForTimeout(1500);

    expect(backfillCalls, 'cold start MUST NOT call /api/v1/events').toHaveLength(0);
  });

  test('立场 ①: WS close → reconnect → backfill within 3s', async ({ page, baseURL }) => {
    const serverPort = process.env.E2E_SERVER_PORT ?? '4901';
    const serverURL = `http://127.0.0.1:${serverPort}`;
    const adminCtx = await adminLogin(serverURL);
    const inviteCode = await mintInvite(adminCtx, 'rt-1.2-offline');
    const user = await registerUser(serverURL, inviteCode, 'offline');
    await attachToken(page, baseURL!, user.token);

    // Capture every WS instance the client creates so we can force a
    // real onclose. Must run BEFORE any goto (addInitScript only fires
    // on new docs).
    await installWsCapture(page);

    // Seed a non-zero last_seen_cursor before the test by injecting
    // sessionStorage on first navigation. Cursor 1 is below any
    // realistic server max so the backfill has plenty to fetch and
    // the contract `cursor > since` is exercised.
    await page.addInitScript(() => {
      window.sessionStorage.setItem('borgee.rt1.last_seen_cursor', '1');
    });

    const backfillCalls: { url: string; status: number; receivedAt: number }[] = [];
    page.on('response', async resp => {
      if (resp.request().method() === 'GET' && resp.url().includes('/api/v1/events?since=')) {
        backfillCalls.push({ url: resp.url(), status: resp.status(), receivedAt: Date.now() });
      }
    });

    await page.goto('/');
    await expect(page.locator('.sidebar-title')).toBeVisible();

    // Wait for the WS to actually open (readyState === 1). Without this
    // the close() below races against handshake and the reconnect path
    // doesn't fire.
    await expect.poll(async () => {
      return await page.evaluate(() => {
        const ws = (window as unknown as { __lastWS?: WebSocket }).__lastWS;
        return ws ? ws.readyState : -1;
      });
    }, { timeout: 5_000 }).toBe(1 /* OPEN */);

    // Force a real onclose by calling close() on the captured WS
    // instance. setOffline does NOT close already-open WebSocket frame
    // streams (root cause of #297 flake — backfillCalls.length stayed
    // 0 because onclose never fired). close() drives the same code
    // path as a real network drop.
    const reconnectAt = Date.now();
    await page.evaluate(() => {
      const ws = (window as unknown as { __lastWS?: WebSocket }).__lastWS;
      ws?.close();
    });

    // Wait up to 3s for the backfill GET to fire after the WS reopens.
    // Reconnect delay is RECONNECT_DELAYS[0] = 1000ms (useWebSocket.ts
    // line 17), so backfill should land ~1.0–1.5s after close().
    await expect.poll(() => backfillCalls.length, {
      message: 'expected GET /api/v1/events backfill on reconnect within 3s',
      timeout: 3_000,
    }).toBeGreaterThan(0);

    const first = backfillCalls[0]!;
    expect(first.status, 'backfill must return 2xx').toBeLessThan(400);
    const latency = first.receivedAt - reconnectAt;
    expect(latency, `backfill latency ${latency}ms exceeds 3s budget`).toBeLessThan(3_000);

    // Confirm the URL carries the seeded since=1 (NOT since=0 — would
    // imply the反约束 was violated by a cold-start fallback).
    expect(first.url).toContain('since=1');

    // 立场 ③ (server contract reverse-check via response body):
    // every event in the response must have cursor > 1.
    const respBody = await (async () => {
      const resp = await page.request.get(`/api/v1/events?since=1`, {
        headers: { Cookie: `borgee_token=${user.token}` },
      });
      expect(resp.ok()).toBe(true);
      return resp.json() as Promise<{ cursor: number; events: { cursor: number }[] }>;
    })();
    for (const ev of respBody.events) {
      expect(ev.cursor, `server returned cursor ${ev.cursor} <= since=1 (反约束 broken)`).toBeGreaterThan(1);
    }
  });
});
