// tests/ap-2-bundle.spec.ts — AP-2 #970 BundleSelector mounted-in-production
// proof, real browser.
//
// What this proves (memory `e2e_no_curl_only_ui` + `e2e_full_smoke_regression`):
//   - MOUNTED: a real logged-in user clicks the Sidebar gear
//     [data-action="open-settings"], lands on the 运行时 (runtime) tab, and the
//     orphaned BundleSelector ([data-ap2-bundle-selector]) is visible in the
//     adjacent permissions surface (next to PermissionsView). Before #970
//     nothing rendered it; its visibility here is the mounting proof.
//   - WIRED: expanding a bundle shows default-checked capability checkboxes;
//     unchecking one then clicking confirm fans out one real
//     PUT /api/v1/permissions per remaining capability (caller-driven; no
//     bundle endpoint). page.waitForResponse asserts each PUT fires and the
//     server grants it (200) — i.e. the grants land.
//
// Implementation constraints:
//   - Real page.click on real UI affordances (gear, expand, checkbox,
//     confirm). No page.evaluate(fetch) / cURL / route-mock for the grant.
//   - Seed via REST (admin invite + register) + cookie injection, same as
//     settings-nav-stack.spec.ts.
//   - data-testid / data-attr selectors only.
import { test, expect, request as apiRequest } from '@playwright/test';
import path from 'node:path';

const ADMIN_LOGIN = 'e2e-admin';
const ADMIN_PASSWORD = 'e2e-admin-pass-12345';

const SCREENSHOT_DIR = path.join(
  process.env.PLAYWRIGHT_HTML_REPORT ?? 'playwright-report',
  'ap-2-bundle',
);

function serverURL(): string {
  return `http://127.0.0.1:${process.env.E2E_SERVER_PORT ?? '4901'}`;
}

async function seedUserAndCookie(page: import('@playwright/test').Page, baseURL: string) {
  const ctx = await apiRequest.newContext({ baseURL: serverURL() });

  const loginRes = await ctx.post('/admin-api/auth/login', {
    data: { login: ADMIN_LOGIN, password: ADMIN_PASSWORD },
  });
  expect(loginRes.ok(), `admin login failed: ${loginRes.status()}`).toBe(true);

  const inviteRes = await ctx.post('/admin-api/v1/invites', {
    data: { note: 'ap-2-bundle-e2e' },
  });
  expect(inviteRes.ok(), `mint invite failed: ${inviteRes.status()}`).toBe(true);
  const inviteCode = ((await inviteRes.json()) as { invite: { code: string } }).invite.code;

  const stamp = Date.now();
  const email = `ap2-bundle-${stamp}-${Math.random().toString(36).slice(2, 8)}@example.test`;
  const password = 'p@ssw0rd-ap2-bundle';
  const displayName = `BundleGrantor ${stamp}`;
  const regCtx = await apiRequest.newContext({ baseURL: serverURL() });
  const regRes = await regCtx.post('/api/v1/auth/register', {
    data: { invite_code: inviteCode, email, password, display_name: displayName },
  });
  expect(regRes.ok(), `register failed: ${regRes.status()} ${await regRes.text()}`).toBe(true);

  const cookies = await regCtx.storageState();
  const tokenCookie = cookies.cookies.find((c) => c.name === 'borgee_token');
  expect(tokenCookie, 'borgee_token cookie should be set by register').toBeTruthy();
  const url = new URL(baseURL);
  await page.context().addCookies([
    {
      name: 'borgee_token',
      value: tokenCookie!.value,
      domain: url.hostname,
      path: '/',
      httpOnly: true,
      secure: false,
      sameSite: 'Lax',
    },
  ]);

  await ctx.dispose();
  await regCtx.dispose();
}

async function openSettings(page: import('@playwright/test').Page) {
  const gear = page.locator('[data-action="open-settings"]');
  await expect(gear, 'settings gear icon (sidebar)').toBeVisible();
  await gear.click();
  await expect(page.locator('[data-page="settings"]')).toBeVisible();
  // Default tab = runtime (运行时).
  await expect(page.locator('[data-tab="runtime"]')).toHaveClass(/active/);
}

test.describe('AP-2 #970 — BundleSelector mounted in runtime permissions surface', () => {
  test.beforeEach(async ({ page, baseURL }) => {
    await seedUserAndCookie(page, baseURL!);
    await page.goto('/');
    // App auto-selects the welcome channel after init.
    await expect(page.locator('.channel-view')).toBeVisible({ timeout: 15_000 });
  });

  test('gear → 运行时 → BundleSelector visible (MOUNTED) → confirm fans out PUT /api/v1/permissions', async ({
    page,
  }) => {
    await openSettings(page);

    // MOUNTED proof: BundleSelector renders inside the permissions surface,
    // next to PermissionsView. Before #970 this element did not exist in the
    // rendered tree.
    const surface = page.locator('[data-settings-permissions-surface="true"]');
    await expect(surface).toBeVisible();
    const bundleSelector = surface.locator('[data-ap2-bundle-selector]');
    await expect(bundleSelector, 'BundleSelector mounted in production UI').toBeVisible();
    // PermissionsView is mounted in the same surface (self-view).
    await expect(surface.locator('[data-ap2-permissions-view], [data-ap2-empty], [data-ap2-loading]'))
      .toBeVisible();
    await page.screenshot({ path: path.join(SCREENSHOT_DIR, '1-bundle-selector-mounted.png') });

    // Expand the reader bundle → 3 default-checked capability checkboxes.
    await page.locator('[data-ap2-bundle-expand][data-bundle-name="reader"]').click();
    const checkboxes = bundleSelector.locator('[data-ap2-bundle-checkbox]');
    await expect(checkboxes).toHaveCount(3);
    // Default all checked (反偷默认全勾的设计 — user can uncheck).
    for (let i = 0; i < 3; i++) {
      await expect(checkboxes.nth(i)).toBeChecked();
    }
    await page.screenshot({ path: path.join(SCREENSHOT_DIR, '2-bundle-expanded.png') });

    // Uncheck the first capability → fan-out should grant the remaining two.
    await checkboxes.nth(0).uncheck();
    await expect(checkboxes.nth(0)).not.toBeChecked();

    // Capture every self-grant PUT that fires during confirm.
    const grantedPermissions: string[] = [];
    page.on('request', (req) => {
      if (req.method() === 'PUT' && new URL(req.url()).pathname === '/api/v1/permissions') {
        const post = req.postData();
        if (post) {
          try {
            grantedPermissions.push((JSON.parse(post) as { permission: string }).permission);
          } catch {
            /* ignore non-JSON */
          }
        }
      }
    });

    // Arm waitForResponse BEFORE clicking confirm so we never miss the PUTs.
    const isSelfGrantPut = (res: import('@playwright/test').Response) =>
      res.request().method() === 'PUT' &&
      new URL(res.url()).pathname === '/api/v1/permissions';
    const put1 = page.waitForResponse(isSelfGrantPut);

    await page.locator('[data-ap2-bundle-confirm]').click();

    // First PUT fired and the server granted it (grants land).
    const r1 = await put1;
    expect(r1.status(), 'first self-grant PUT should return 200').toBe(200);

    // The fan-out dispatches one PUT per remaining capability (2 here).
    await expect.poll(() => grantedPermissions.length, { timeout: 10_000 }).toBe(2);
    expect(new Set(grantedPermissions).size, 'distinct capability tokens granted').toBe(2);

    await page.screenshot({ path: path.join(SCREENSHOT_DIR, '3-after-confirm.png') });
  });

  test('fan-out fires exactly one PUT per remaining capability (network count proof)', async ({
    page,
  }) => {
    await openSettings(page);

    const surface = page.locator('[data-settings-permissions-surface="true"]');
    const bundleSelector = surface.locator('[data-ap2-bundle-selector]');
    await expect(bundleSelector).toBeVisible();

    // Capture every self-grant PUT request body during this test.
    const grantedPermissions: string[] = [];
    page.on('request', (req) => {
      if (
        req.method() === 'PUT' &&
        new URL(req.url()).pathname === '/api/v1/permissions'
      ) {
        const post = req.postData();
        if (post) {
          try {
            grantedPermissions.push((JSON.parse(post) as { permission: string }).permission);
          } catch {
            /* ignore non-JSON */
          }
        }
      }
    });

    // Expand the workspace bundle (3 capabilities), uncheck one → grant 2.
    await page.locator('[data-ap2-bundle-expand][data-bundle-name="workspace"]').click();
    const checkboxes = bundleSelector.locator('[data-ap2-bundle-checkbox]');
    await expect(checkboxes).toHaveCount(3);
    await checkboxes.nth(0).uncheck();

    // Arm a response wait so we know the network settled before asserting.
    const lastPut = page.waitForResponse(
      (res) =>
        res.request().method() === 'PUT' &&
        new URL(res.url()).pathname === '/api/v1/permissions' &&
        res.status() === 200,
    );
    await page.locator('[data-ap2-bundle-confirm]').click();
    await lastPut;

    // Allow the remaining microtask-driven PUTs to settle.
    await expect.poll(() => grantedPermissions.length, { timeout: 10_000 }).toBe(2);

    // Each PUT carried a distinct, real capability token (not a role name).
    expect(new Set(grantedPermissions).size).toBe(2);
    for (const perm of grantedPermissions) {
      expect(perm).toMatch(/^[a-z]+\.[a-z_]+$/);
      for (const roleWord of ['admin', 'editor', 'viewer', 'owner']) {
        expect(perm).not.toContain(roleWord);
      }
    }

    await page.screenshot({ path: path.join(SCREENSHOT_DIR, '4-fanout-count.png') });
  });
});
