// tests/helper-enrollment-create.spec.ts — operator UI to mint a helper
// enrollment token from the web (closes the curl-only gap flagged during
// PR-4 (#1042) Stage 2 e2e prep).
//
// Flow under test (real browser, no fetch / page.evaluate per
// e2e_no_curl_only_ui):
//   1. Admin login + invite + register a fresh user (member role; passes
//      isHelperHumanOwner middleware).
//   2. Navigate to /, open Settings, click the helper-status entry.
//   3. Click the "Add host" button in the HelperStatusPanel.
//   4. Fill host_label, keep the default categories.
//   5. Click Create.
//   6. Assert the reveal view shows a non-empty token + an install_command
//      that contains `--server <ws|wss>://...` and `--token <token>`.
//   7. Click Done.
//   8. Assert the modal closed AND the new host appears in the helper list.

import { test, expect, request as apiRequest } from '@playwright/test';
import path from 'node:path';

const ADMIN_LOGIN = 'e2e-admin';
const ADMIN_PASSWORD = 'e2e-admin-pass-12345';

const SCREENSHOT_DIR = path.join(
  process.env.PLAYWRIGHT_HTML_REPORT ?? 'playwright-report',
  'helper-enrollment-create',
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
    data: { note: 'helper-enrollment-create-e2e' },
  });
  expect(inviteRes.ok(), `mint invite: ${inviteRes.status()}`).toBe(true);
  const inviteJson = (await inviteRes.json()) as { invite: { code: string } };
  const inviteCode = inviteJson.invite.code;

  const stamp = Date.now();
  const email = `helper-create-${stamp}-${Math.random().toString(36).slice(2, 8)}@example.test`;
  const regCtx = await apiRequest.newContext({ baseURL: serverURL });
  const regRes = await regCtx.post('/api/v1/auth/register', {
    data: {
      invite_code: inviteCode,
      email,
      password: 'p@ssw0rd-helper-create',
      display_name: `HelperCreate ${stamp}`,
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

test.describe('Operator UI mints a helper enrollment token (no curl)', () => {
  test.beforeEach(async ({ page, baseURL }) => {
    await seedUserAndCookie(page, baseURL!);
    await page.goto('/');
    await waitChannelLanded(page);
  });

  test('Add host → form → Create → reveal token + install command → Done refreshes list', async ({
    page,
  }) => {
    // Open Settings → Helper Status.
    await page.locator('[data-action="open-settings"]').click();
    await expect(page.locator('[data-page="settings"]')).toBeVisible();
    await page.locator('[data-runtime-entry="helper-status"]').click();
    await expect(page.locator('[data-page="helper-status"]')).toBeVisible();
    await page.screenshot({ path: path.join(SCREENSHOT_DIR, '01-helper-panel.png') });

    // Click "Add host".
    await page.locator('[data-action="add-helper-host"]').click();
    await expect(page.locator('[data-helper-create-modal]')).toBeVisible();
    await page.screenshot({ path: path.join(SCREENSHOT_DIR, '02-modal-open.png') });

    // Fill the form. Default-on categories are openclaw_config + status_collect,
    // so this scenario picks: explicitly leave defaults, plus tick a third one
    // to prove multi-select also flows through the POST.
    const labelInput = page.locator('[data-helper-host-label]');
    await labelInput.fill('stage2-test-host');

    const openclawLifecycle = page.locator(
      '[data-helper-category-checkbox="openclaw_lifecycle"] input',
    );
    await openclawLifecycle.check();

    // Click Create.
    await page.locator('[data-action="submit-helper-create"]').click();

    // Reveal view appears with non-empty token + install_command.
    const installCmdEl = page.locator('[data-helper-install-command]');
    const tokenEl = page.locator('[data-helper-enrollment-token]');
    await expect(installCmdEl).toBeVisible({ timeout: 10_000 });
    await expect(tokenEl).toBeVisible();
    await expect(page.locator('[data-helper-create-warning]')).toContainText('shown ONCE');

    const installCmd = await installCmdEl.inputValue();
    const token = await tokenEl.inputValue();
    expect(token.length, 'enrollment token is non-empty').toBeGreaterThan(0);
    expect(token, 'token has <id>.<secret> shape').toContain('.');
    expect(installCmd, 'install command embeds wss://-or-ws:// scheme').toMatch(
      /--server wss?:\/\//,
    );
    expect(installCmd, 'install command embeds the rendered token').toContain(`--token ${token}`);
    expect(installCmd, 'install command is the npx one-liner').toContain(
      'sudo npx @codetreker/borgee-remote-agent install',
    );

    await page.screenshot({ path: path.join(SCREENSHOT_DIR, '03-reveal.png') });

    // Click Done.
    await page.locator('[data-action="close-helper-create-modal"]').click();
    await expect(page.locator('[data-helper-create-modal]')).toHaveCount(0);

    // The new host appears in the panel list (proves the list-refresh ran).
    await expect(page.locator('.helper-status-panel')).toContainText('stage2-test-host', {
      timeout: 10_000,
    });
    await page.screenshot({ path: path.join(SCREENSHOT_DIR, '04-list-after.png') });
  });
});
