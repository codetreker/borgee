// tests/openclaw-install-trigger.spec.ts — Issue #1050 e2e.
//
// Owner clicks "Install OpenClaw" on the HelperStatusPanel and the request
// reaches the existing helper-jobs enqueue endpoint with the correct
// envelope. The UI then flips through the helper_job state machine and
// settles on "OpenClaw installed".
//
// Why this spec uses page.route() to shape the helper-enrollments
// response instead of a live dev-stack helper-vm:
//   - The visibility gate for the Install button requires the enrollment
//     status to be `connected` + `fresh`, i.e. an actually-heartbeating
//     helper daemon. The Playwright workspace does not boot the helper-VM
//     container by default, and the issue scope is the UI trigger, not
//     the daemon's `install-butler` rootd exec (PR-4 covers that).
//   - The server enqueue contract (auth, idempotency, payload shape,
//     manifest binding) is pinned in
//     packages/server-go/internal/api/helper_jobs_install_openclaw_ui_test.go
//     (Go tests for issue #1050, acceptance OUT-4 / OUT-5 / OUT-6).
//   - Vitest in HelperStatusPanel-install-openclaw.test.tsx pins the
//     component-level button visibility, modal interaction, error path,
//     and WS-driven state transitions.
//   - The route stub here keeps the rest of the stack identical to a
//     production run: real React build, real navigation, real fetch
//     against the dev server. The only intercept is the helper-enrollments
//     GET (so we can claim a fresh enrollment without running a daemon)
//     and the helper-jobs POST/result (so we can observe the request and
//     flip the aggregate to "installed").
//
// The full live-trigger path that the issue's OUT-7 verification table
// describes (button click → daemon `install-butler` → binary present at
// /usr/local/lib/borgee/openclaw) is captured in
// docs/runbooks/local-e2e-helper-container.md as a manual step until the
// dev-stack helper-vm container can be wired into the Playwright runner.

import { test, expect, request as apiRequest } from '@playwright/test';
import path from 'node:path';

const ADMIN_LOGIN = 'e2e-admin';
const ADMIN_PASSWORD = 'e2e-admin-pass-12345';

const SCREENSHOT_DIR = path.join(
  process.env.PLAYWRIGHT_HTML_REPORT ?? 'playwright-report',
  'openclaw-install-trigger',
);

async function seedUserAndCookie(page: import('@playwright/test').Page, baseURL: string) {
  const serverPort = process.env.E2E_SERVER_PORT ?? '4901';
  const serverURL = `http://127.0.0.1:${serverPort}`;
  const ctx = await apiRequest.newContext({ baseURL: serverURL });

  const loginRes = await ctx.post('/admin-api/auth/login', {
    data: { login: ADMIN_LOGIN, password: ADMIN_PASSWORD },
  });
  expect(loginRes.ok(), `admin login: ${loginRes.status()}`).toBe(true);

  const inviteRes = await ctx.post('/admin-api/v1/invites', {
    data: { note: 'openclaw-install-trigger-e2e' },
  });
  expect(inviteRes.ok(), `mint invite: ${inviteRes.status()}`).toBe(true);
  const inviteJson = (await inviteRes.json()) as { invite: { code: string } };
  const inviteCode = inviteJson.invite.code;

  const stamp = Date.now();
  const email = `openclaw-trigger-${stamp}-${Math.random().toString(36).slice(2, 8)}@example.test`;
  const regCtx = await apiRequest.newContext({ baseURL: serverURL });
  const regRes = await regCtx.post('/api/v1/auth/register', {
    data: {
      invite_code: inviteCode,
      email,
      password: 'p@ssw0rd-openclaw-trigger',
      display_name: `OpenclawTrigger ${stamp}`,
    },
  });
  expect(regRes.ok(), `register: ${regRes.status()} ${await regRes.text()}`).toBe(true);

  const cookies = await regCtx.storageState();
  const tokenCookie = cookies.cookies.find((c) => c.name === 'borgee_token');
  expect(tokenCookie, 'borgee_token cookie set by register').toBeTruthy();
  if (tokenCookie) {
    const url = new URL(baseURL);
    await page.context().addCookies([
      {
        name: 'borgee_token',
        value: tokenCookie.value,
        domain: url.hostname,
        path: '/',
        httpOnly: true,
        secure: false,
        sameSite: 'Lax',
      },
    ]);
  }

  await ctx.dispose();
  await regCtx.dispose();
}

async function waitChannelLanded(page: import('@playwright/test').Page) {
  await expect(page.locator('.channel-view')).toBeVisible({ timeout: 15_000 });
}

type EnrollmentRow = Record<string, unknown>;

function aliveEnrollment(overrides: EnrollmentRow = {}): EnrollmentRow {
  return {
    enrollment_id: 'enr-e2e-alive-1',
    host_label: 'E2E Alive Host',
    helper_device_id: 'device-e2e',
    allowed_categories: ['openclaw_lifecycle', 'openclaw_config'],
    status: 'connected',
    fresh: true,
    last_seen_at: Date.now() - 1_000,
    created_at: Date.now() - 60_000,
    ...overrides,
  };
}

test.describe('Issue #1050 — Owner-driven OpenClaw install via UI', () => {
  test.beforeEach(async ({ page, baseURL }) => {
    await seedUserAndCookie(page, baseURL!);
    await page.goto('/');
    await waitChannelLanded(page);
  });

  test('click Install OpenClaw → modal → Confirm → POST → progress badge → installed', async ({
    page,
  }) => {
    // Mutable view of the helper enrollment so subsequent GETs can return
    // the "installing" then "installed" aggregate states.
    let configureView: Record<string, unknown> | null = null;

    await page.route('**/api/v1/helper/enrollments', async (route) => {
      const row = aliveEnrollment(
        configureView ? { configure_openclaw: configureView } : {},
      );
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ enrollments: [row] }),
      });
    });

    // Capture the POST issued by the Confirm click so we can assert the
    // exact envelope the issue's acceptance OUT-2 / OUT-4 requires.
    const enqueueRequests: Array<{ url: string; body: any }> = [];
    await page.route(
      '**/api/v1/helper/enrollments/*/jobs',
      async (route, request) => {
        const url = request.url();
        let body: any = null;
        try {
          body = request.postDataJSON();
        } catch {
          body = request.postData();
        }
        enqueueRequests.push({ url, body });
        // Simulate the server's response shape: queued, with category +
        // job_type so the client's badge updates. Flip the cached
        // configure_openclaw aggregate so the next refresh shows the
        // queued step.
        configureView = {
          state: 'queued',
          label: 'Configure OpenClaw queued',
          audit_refs: [],
          log_refs: [],
          steps: [
            {
              job_type: 'openclaw.install_from_manifest',
              status: 'queued',
              audit_refs: [],
              log_refs: [],
            },
          ],
        };
        await route.fulfill({
          status: 201,
          contentType: 'application/json',
          body: JSON.stringify({
            job: {
              job_id: 'job-e2e-install-1',
              enrollment_id: 'enr-e2e-alive-1',
              job_type: 'openclaw.install_from_manifest',
              schema_version: 1,
              status: 'queued',
              category: 'openclaw_lifecycle',
              created_at: Date.now(),
              expires_at: Date.now() + 60_000,
            },
          }),
        });
      },
    );

    await page.locator('[data-action="open-settings"]').click();
    await expect(page.locator('[data-page="settings"]')).toBeVisible();
    await page.locator('[data-runtime-entry="helper-status"]').click();
    await expect(page.locator('[data-page="helper-status"]')).toBeVisible();
    await page.screenshot({ path: path.join(SCREENSHOT_DIR, '01-helper-panel.png') });

    // Install OpenClaw button is visible because the enrollment is fresh
    // + has openclaw_lifecycle in allowed_categories + no succeeded
    // install step exists.
    const installBtn = page.locator('[data-action="install-openclaw"]');
    await expect(installBtn, 'Install OpenClaw button visible').toBeVisible({
      timeout: 10_000,
    });

    await installBtn.click();
    const modal = page.locator('[data-helper-install-openclaw-modal]');
    await expect(modal, 'install modal opens').toBeVisible();
    await expect(modal).toHaveAttribute('role', 'dialog');
    await expect(
      modal.locator('[data-helper-install-openclaw-target-path]'),
    ).toContainText('/usr/local/lib/borgee/openclaw');
    await expect(
      modal.locator('[data-helper-install-openclaw-plugin-id]'),
    ).toContainText('openclaw');
    await page.screenshot({ path: path.join(SCREENSHOT_DIR, '02-modal-open.png') });

    // Confirm → wait for the enqueue POST + the post-success reload.
    const enqueuePromise = page.waitForResponse(
      (resp) =>
        resp.request().method() === 'POST' &&
        /\/api\/v1\/helper\/enrollments\/.+\/jobs$/.test(resp.url()) &&
        resp.status() === 201,
      { timeout: 10_000 },
    );
    await page.locator('[data-action="confirm-install-openclaw"]').click();
    const enqueueResp = await enqueuePromise;
    expect(enqueueResp.ok(), 'enqueue POST 201').toBe(true);

    expect(enqueueRequests, 'one enqueue POST issued').toHaveLength(1);
    expect(enqueueRequests[0].body).toMatchObject({
      job_type: 'openclaw.install_from_manifest',
      schema_version: 1,
      payload: { runtime: 'openclaw' },
    });
    expect(String(enqueueRequests[0].body.idempotency_key)).toContain(
      'install-openclaw-',
    );

    // After enqueue the panel refreshes; the install button is now
    // replaced by an "Installing OpenClaw" progress badge.
    await expect(page.locator('[data-helper-install-openclaw-modal]')).toHaveCount(0);
    await expect(
      page.locator('[data-helper-openclaw-badge="progress"]'),
      'progress badge appears after enqueue',
    ).toContainText('Installing OpenClaw', { timeout: 10_000 });
    await page.screenshot({ path: path.join(SCREENSHOT_DIR, '03-progress.png') });

    // Flip the mock to "installed" and click Refresh to simulate the
    // helper-job state transition arriving (the real WS handler triggers
    // the same fetch path). The button must not return, the badge flips
    // to "OpenClaw installed".
    configureView = {
      state: 'succeeded',
      label: 'Configure OpenClaw complete',
      audit_refs: [],
      log_refs: [],
      steps: [
        {
          job_type: 'openclaw.install_from_manifest',
          status: 'succeeded',
          audit_refs: [],
          log_refs: [],
        },
      ],
    };

    const refreshBtn = page.getByRole('button', { name: 'Refresh' });
    await refreshBtn.click();
    await expect(
      page.locator('[data-helper-openclaw-badge="installed"]'),
      'install succeeded → installed badge',
    ).toContainText('OpenClaw installed', { timeout: 10_000 });
    await expect(page.locator('[data-action="install-openclaw"]')).toHaveCount(0);
    await page.screenshot({ path: path.join(SCREENSHOT_DIR, '04-installed.png') });
  });
});
