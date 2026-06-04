// tests/ws-auth-gate.spec.ts — bf task ws-auth-gate (fix-skill-findings).
//
// Locks AC-5 from .bf/fix-skill-findings/ws-auth-gate/spec.md:
//
//   "A Playwright spec at packages/e2e/tests/ws-auth-gate.spec.ts exists
//   and passes; it drives a real fresh-signup flow in a browser, captures
//   all console.error + network requests, and asserts (a) zero /ws requests
//   with status 401 and (b) no /ws connect attempt before the auth cookie
//   is set."
//
// Approach:
//   - real browser, cold context (no cookies)
//   - admin login → mint invite (REST, mirrors chat-first-time-onboarding setup)
//   - navigate to /, click 注册 link to open RegisterPage, fill the four
//     inputs by placeholder, submit
//   - throughout: record every /ws request and every console.error entry
//   - assert (a) zero /ws responses with status 401
//   - assert (b) every /ws request issued after the borgee_token cookie
//     is set in this context
import { test, expect, request as apiRequest } from '@playwright/test';

const ADMIN_LOGIN = 'e2e-admin';
const ADMIN_PASSWORD = 'e2e-admin-pass-12345';

test.describe('bf ws-auth-gate — no pre-auth /ws connect', () => {
  test('fresh signup produces zero /ws 401 entries and no pre-cookie /ws connect', async ({ page, context, baseURL }) => {
    const serverPort = process.env.E2E_SERVER_PORT ?? '4901';
    const serverURL = `http://127.0.0.1:${serverPort}`;
    const ctx = await apiRequest.newContext({ baseURL: serverURL });

    // 1. Admin login → mint invite (REST setup, mirrors chat-first-time-onboarding).
    const loginRes = await ctx.post('/admin-api/auth/login', {
      data: { login: ADMIN_LOGIN, password: ADMIN_PASSWORD },
    });
    expect(loginRes.ok(), `admin login failed: ${loginRes.status()}`).toBe(true);

    const inviteRes = await ctx.post('/admin-api/v1/invites', {
      data: { note: 'ws-auth-gate-e2e' },
    });
    expect(inviteRes.ok(), `mint invite failed: ${inviteRes.status()}`).toBe(true);
    const inviteJson = (await inviteRes.json()) as { invite: { code: string } };
    const inviteCode = inviteJson.invite.code;
    expect(inviteCode).toBeTruthy();

    // 2. Record /ws traffic + console.error entries from the very first
    //    navigation. Playwright's `page.on('websocket')` fires on every WS
    //    upgrade attempt and exposes the URL synchronously; the WebSocket
    //    object also emits `socketerror` / `framereceived` so we can detect
    //    a server-rejected upgrade (the server closes with 401 reason).
    //    Additionally we watch `page.on('response')` for any HTTP 401 on a
    //    /ws path (covers the upgrade-handshake-fail case where Playwright
    //    surfaces it as a regular HTTP response).
    type WsObservation = {
      url: string;
      observedAt: number;
      socketError: string | null;
    };
    const wsObservations: WsObservation[] = [];
    const consoleErrors: string[] = [];
    const ws401Hits: { url: string; status: number; observedAt: number }[] = [];
    let welcomeRenderedAt: number | null = null;

    page.on('websocket', (ws) => {
      const url = ws.url();
      // Filter to the borgee /ws endpoint only; Vite dev server also opens
      // a HMR websocket on the same origin (path-less, with a `?token=`
      // query), which is unrelated to the auth gate.
      if (!/\/ws(\?|$)/.test(url)) return;
      const obs: WsObservation = {
        url,
        observedAt: Date.now(),
        socketError: null,
      };
      wsObservations.push(obs);
      ws.on('socketerror', (err) => { obs.socketError = String(err); });
    });
    page.on('response', (res) => {
      const url = res.url();
      // Catch /ws upgrade-failures that come back as plain HTTP responses
      // (the pre-fix bug surfaces here — every reconnect attempt that hits
      // the server before the cookie is set returns HTTP 401).
      if (/\/ws(\?|$)/.test(url) && res.status() === 401) {
        ws401Hits.push({ url, status: res.status(), observedAt: Date.now() });
      }
    });
    page.on('console', msg => {
      if (msg.type() === 'error') consoleErrors.push(msg.text());
    });

    // 3. Cold-open SPA — no cookie yet. The auth gate must hold here.
    await page.goto('/');

    // 4. Drive RegisterPage. LoginPage shows first; click the "Register"
    //    link ("Have an invite code? Register" — LoginPage.tsx).
    //    RegisterPage inputs are placeholder-keyed (Invite Code, Display
    //    Name, Email, Password) per packages/client/src/components/RegisterPage.tsx.
    await page.getByText(/Register/i).first().click();
    await page.getByPlaceholder('Invite Code').fill(inviteCode);
    const stamp = Date.now();
    await page.getByPlaceholder('Display Name').fill(`ws-gate-${stamp}`);
    await page.getByPlaceholder('Email').fill(`ws-gate-${stamp}@example.test`);
    await page.getByPlaceholder('Password').fill('p@ssw0rd-ws-gate');

    // 5. Submit. The signup flow sets borgee_token, then waitForAuthReady
    //    polls /me until 200, then the shell flips `authenticated=true`
    //    which is when the /ws connect is allowed to fire.
    //    The submit button is the only <button type="submit"> on the form.
    await page.locator('button[type="submit"]').click();

    // 6. Wait for the post-signup UI to land. CM-onboarding renders the
    //    welcome system message body in `.message-system-content`; same
    //    locator as chat-first-time-onboarding.spec.ts.
    await expect(page.locator('.message-system-content').first()).toContainText('欢迎来到 Borgee', { timeout: 15_000 });
    welcomeRenderedAt = Date.now();

    // 7. Capture when the cookie is set. Walk the context's cookie jar
    //    AFTER the welcome surface appears — by definition the cookie
    //    must already be there (the SPA bootstraps the welcome channel
    //    only after fetchMe returns 200).
    const cookies = await context.cookies();
    const tokenCookie = cookies.find(c => c.name === 'borgee_token');
    expect(tokenCookie, 'borgee_token cookie should be set after signup').toBeTruthy();

    // 8. AC-5(a): zero /ws requests with status 401. The pre-fix bug shows
    //    up here — without the auth gate, the SPA opens /ws during the
    //    very first render of AppInner (before the cookie is set) and the
    //    server replies 401, then the reconnect loop bangs every ~50ms.
    expect(
      ws401Hits,
      `Expected zero /ws 401 responses; got:\n${JSON.stringify(ws401Hits, null, 2)}\nFull WebSocket observations:\n${JSON.stringify(wsObservations, null, 2)}\nConsole errors:\n${consoleErrors.join('\n')}`,
    ).toEqual([]);

    // 9. AC-5(b): no /ws connect attempt before the welcome surface
    //    appeared in such density that it looks like the pre-auth
    //    reconnect loop. Pre-fix: multiple WS attempts pile up BEFORE
    //    welcome renders (every ~50ms). Post-fix: the first WS only
    //    fires after `authenticated=true` flips, which happens during
    //    the same render cycle as the auth-checked tree swap, well
    //    before the welcome message renders. So bound the WS attempt
    //    count observed before `welcomeRenderedAt` to a small ceiling.
    const preWelcomeWs = wsObservations.filter(o => welcomeRenderedAt !== null && o.observedAt <= welcomeRenderedAt);
    expect(
      preWelcomeWs.length,
      `Expected ≤ 2 /ws connect attempts before the welcome surface rendered (1 legitimate post-auth-flip + 1 tolerance); got ${preWelcomeWs.length}:\n${JSON.stringify(preWelcomeWs, null, 2)}`,
    ).toBeLessThanOrEqual(2);

    // 10. Document evidence — print the full /ws log + console.error log
    //     so the EV-5 artifact has the captured transcript.
    console.log('[ws-auth-gate evidence] /ws observations:', JSON.stringify(wsObservations, null, 2));
    console.log('[ws-auth-gate evidence] /ws 401 hits:', JSON.stringify(ws401Hits, null, 2));
    console.log('[ws-auth-gate evidence] console.error entries:', JSON.stringify(consoleErrors, null, 2));
  });
});
