// tests/openclaw-install-trigger.interaction.spec.ts — Issue #1050 UI
// interaction spec.
//
// SCOPE — this is NOT a live-backend e2e. It is an interaction test that
// exercises the React UI through the real production build (navigation +
// fetch + modal + a11y) with the helper-enrollment GET and the
// helper-jobs POST stubbed via Playwright `page.route()`. The filename
// ends in `.interaction.spec.ts` to make that honest at the disk-tree
// level; the original `.spec.ts` name overstated what this proves.
// Per project memory rule `e2e_no_curl_only_ui` /
// `e2e_full_smoke_regression`: a real e2e must drive a real backend.
//
// What this spec proves:
//   - Real React build serves the helper-status page.
//   - Owner can navigate to it via the real Settings UI.
//   - Install OpenClaw button gates on the visibility predicate
//     (status=connected, fresh, allowed_categories has
//     openclaw_lifecycle, no succeeded install in history).
//   - Modal opens, shows the read-only facts the operator must confirm,
//     and Confirm POSTs the exact envelope the server contract pins.
//   - After enqueue the UI flips to the in-flight progress badge; a
//     subsequent refresh that returns a `succeeded` aggregate flips the
//     surface to the installed badge.
//
// What this spec does NOT prove (acceptance OUT-7 re-scoped):
//   - That `install-butler` actually runs inside the helper container.
//   - That `/usr/local/lib/borgee/openclaw` exists on disk after the
//     POST.
//   - That the lease + WS push end-to-end carries the signed manifest
//     body to the helper.
//
// Why deferral is the right call here:
//   - The dev-stack at `scripts/dev-stack/` runs `borgee-vm` as a
//     `--privileged` Docker container (systemd PID 1). Per
//     `docs/runbooks/local-e2e-helper-container.md` privileged
//     containers are "not safe for hardened CI; intended for dev
//     machines." The Playwright runner used in CI cannot host this.
//   - The `install-butler` executor contract is already pinned by
//     PR-4 (#1042) Stage 2 e2e (8 JobType helper-vm runs via the
//     runbook). Re-running the same coverage from Playwright would
//     add no signal that PR-4 doesn't already give us.
//   - Acceptance file `acceptance-criteria.md` for #1050 has been
//     amended to mark OUT-7 as "compile-time wiring + server contract +
//     UI interaction" with the live-binary assertion deferred and the
//     blocker (`--privileged` in CI) cited.
//
// The server enqueue contract (auth, idempotency, payload shape,
// canonical manifest body) is pinned in
// packages/server-go/internal/api/helper_jobs_install_openclaw_ui_test.go.
// Component-level a11y, visibility, error-path coverage is in
// HelperStatusPanel-install-openclaw.test.tsx.

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
