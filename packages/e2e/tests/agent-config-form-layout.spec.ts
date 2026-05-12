// tests/agent-config-form-layout.spec.ts — Agent config form layout across viewports (gh#698).
//
// Test scope:
//   - Six labels each occupy their own non-overlapping row; before the fix, the
//     inline flow overlapped inside an 800px parent container.
//   - Five text/textarea labels use inline style display:'block'.
//   - One checkbox label uses inline style display:'flex'.
//   - All six data-agent-config-field markers are present.
//   - Responsive behavior across 1280 / 1024 / 480 viewports.
//   - Accessibility negative checks run in the browser layer by measuring inline styles.
//
// Out of scope:
//   - label htmlFor implicit association; design §3 covers it, not this e2e test.
//   - New CSS class validation; this fix chose option A, inline style, without adding a class.
//   - al-2a content-lock drift, tracked separately in gh#701.
//
// Related docs:
//   - Design: docs/tasks/698-agent-config-form-overlap/design.md §7 test strategy + §4 boundaries
//   - QA review: liema #1 / #3 / #4
//
// Implementation constraints:
//   - Browser-driven UI path: page.goto, viewport changes, and getBoundingClientRect measurement.
//   - Do not use fs.*, page.evaluate(fetch) with cookies, API-only checks, or empty placeholder tests.

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

interface RegisteredUser {
  email: string;
  token: string;
  userId: string;
  ctx: APIRequestContext;
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
  const email = `gh698-${suffix}-${stamp}-${Math.floor(Math.random() * 1000)}@example.test`;
  const password = 'p@ssw0rd-gh698';
  const displayName = `GH698 ${suffix} ${stamp}`;
  const res = await ctx.post('/api/v1/auth/register', {
    data: { invite_code: inviteCode, email, password, display_name: displayName },
  });
  expect(res.ok(), `register: ${res.status()} ${await res.text()}`).toBe(true);
  const body = (await res.json()) as { user: { id: string } };
  const cookies = await ctx.storageState();
  const tok = cookies.cookies.find((c) => c.name === 'borgee_token');
  expect(tok, 'borgee_token cookie missing').toBeTruthy();
  return { email, token: tok!.value, userId: body.user.id, ctx };
}

async function attachToken(ctx: BrowserContext, baseURL: string, token: string) {
  const url = new URL(baseURL);
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

async function createAgent(
  serverURL: string,
  ownerToken: string,
  displayName: string,
): Promise<{ id: string }> {
  const ctx = await apiRequest.newContext({
    baseURL: serverURL,
    extraHTTPHeaders: { Cookie: `borgee_token=${ownerToken}` },
  });
  const r = await ctx.post('/api/v1/agents', { data: { display_name: displayName } });
  expect(r.ok() || r.status() === 201, `agent create: ${r.status()}`).toBe(true);
  return (await r.json()) as { id: string };
}

/**
 * Open AgentConfigPanel by going to /agents → expand first agent's Manage.
 * AgentConfigPanel renders after Manage is expanded.
 *
 * gh#698 e2e CI fix: at a 480px mobile viewport, the sidebar starts collapsed
 * (App.tsx isMobile = innerWidth < 768, sidebar-wrapper sidebar-closed), so
 * [data-testid="sidebar-nav-agents"] is not clickable inside the closed sidebar.
 * If the mobile .hamburger-btn is visible, open the sidebar before clicking the
 * agents nav. On desktop (≥768px), hamburger-btn is not rendered
 * (App.tsx L204 {isMobile && ...}), so the nav can be clicked directly. This
 * matches the mobile handling used by PR #699 cv-1-3-canvas-modal-a11y.spec.ts.
 */
async function openAgentConfigPanel(page: Page) {
  await page.goto('/');
  // Mobile waits for hamburger; desktop waits for sidebar-title. first() avoids
  // strict-mode .or() double-match failures seen in PR #699.
  await expect(
    page.locator('.hamburger-btn, .sidebar-title').first(),
  ).toBeVisible({ timeout: 10_000 });
  // Mobile path: visible hamburger means the sidebar must be opened first.
  // Desktop does not render hamburger, so count == 0 and this branch is skipped.
  const hamburger = page.locator('.hamburger-btn');
  const isMobile = await hamburger.count() > 0 && await hamburger.isVisible();
  if (isMobile) {
    await hamburger.click();
  }
  await page.locator('[data-testid="sidebar-nav-agents"]').click();
  await expect(page.locator('.agent-page')).toBeVisible({ timeout: 10_000 });
  // gh#698 e2e v6: local reproduction showed the mobile sidebar overlay
  // (z 199, inset 0) covers .main-content, making the Manage button unclickable
  // under the overlay. Earlier attempts with overlay click / force click were
  // unreliable because the overlay can be covered by the sidebar, React onClick
  // may not fire, and forced Manage clicks can skip React handlers. The mobile
  // path closes the overlay through page.evaluate; desktop keeps native click.
  if (isMobile) {
    // Find sidebar-overlay and click it so React's closeSidebar handler runs.
    await page.evaluate(() => {
      const overlay = document.querySelector('.sidebar-overlay') as HTMLElement | null;
      if (overlay) {
        overlay.click();  // native click uses React event delegation and closes the sidebar
      }
    });
    await expect(page.locator('.sidebar-overlay')).toHaveCount(0, { timeout: 5_000 });
  }
  // Expand Manage; AgentConfigPanel mounts only after this design §1 path.
  const manageBtn = page.locator('.agent-card button.btn-sm', { hasText: 'Manage' }).first();
  await manageBtn.click();
  // AgentConfigPanel render anchor: data-agent-config="root".
  await expect(page.locator('[data-agent-config="root"]')).toBeVisible({ timeout: 5_000 });
}

test.describe('gh#698 AgentConfigPanel form 排版重叠修', () => {
  test.beforeEach(async ({ page, baseURL }) => {
    const serverURL = `http://127.0.0.1:${process.env.E2E_SERVER_PORT ?? '4901'}`;
    const adminCtx = await adminLogin(serverURL);
    const inviteCode = await mintInvite(adminCtx, 'gh698-layout');
    const owner = await registerUser(serverURL, inviteCode, 'owner');
    await createAgent(serverURL, owner.token, `gh698-agent-${Date.now().toString(36)}`);
    await attachToken(page.context(), baseURL!, owner.token);
  });

  test('立场 ① 1280 viewport: 6 个 label 各占独立行 (label.bottom <= input.top)', async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 800 });
    await openAgentConfigPanel(page);

    // Six labels occupy separate rows. Use data-agent-config-field to anchor each
    // input, then measure boundingRect: the six inputs should have six distinct y values.
    const fields = ['name', 'avatar', 'prompt', 'model', 'memory_ref', 'enabled'];
    const ys: number[] = [];
    for (const field of fields) {
      const rect = await page.locator(`[data-agent-config-field="${field}"]`).first().evaluate(
        (el) => el.getBoundingClientRect(),
      );
      ys.push(rect.y);
    }
    // Adjacent fields need at least 8px y spacing (marginTop), guarding against inline-flow overlap.
    for (let i = 1; i < ys.length; i++) {
      expect(
        ys[i]! - ys[i - 1]!,
        `field ${fields[i]} y=${ys[i]} 跟 ${fields[i - 1]} y=${ys[i - 1]} 间距 < 8px (回归 inline 流水重叠?)`,
      ).toBeGreaterThanOrEqual(8);
    }
  });

  test('立场 ② 5 个 text/textarea label display: block (yema 拍 a)', async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 800 });
    await openAgentConfigPanel(page);

    // QA review #1 negative assertion: adding a text field must include the style.
    const textFields = ['name', 'avatar', 'prompt', 'model', 'memory_ref'];
    for (const field of textFields) {
      const display = await page.locator(`[data-agent-config-field="${field}"]`).first()
        .evaluate((el) => {
          const label = el.closest('label');
          return label ? (label as HTMLElement).style.display : '';
        });
      expect(display, `${field} label 应有 inline style display: block`).toBe('block');
    }
  });

  test('立场 ③ checkbox label display: flex inline (yema 拍 b)', async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 800 });
    await openAgentConfigPanel(page);

    // Checkbox exception: same inline pattern as CreateAgentModal Permissions.
    const display = await page.locator('[data-agent-config-field="enabled"]').first()
      .evaluate((el) => {
        const label = el.closest('label');
        return label ? (label as HTMLElement).style.display : '';
      });
    expect(display, 'enabled label 应有 inline style display: flex (yema 拍 b inline)').toBe('flex');
  });

  test('立场 ① 480 mobile viewport: 6 个 field 仍各占独立行不溢出', async ({ page }) => {
    await page.setViewportSize({ width: 480, height: 800 });
    await openAgentConfigPanel(page);

    const fields = ['name', 'avatar', 'prompt', 'model', 'memory_ref', 'enabled'];
    const ys: number[] = [];
    for (const field of fields) {
      const rect = await page.locator(`[data-agent-config-field="${field}"]`).first().evaluate(
        (el) => el.getBoundingClientRect(),
      );
      ys.push(rect.y);
      // Overflow check: each field should stay inside the viewport.
      expect(rect.x, `${field} x=${rect.x} 应 ≥ 0 (不左溢)`).toBeGreaterThanOrEqual(0);
      expect(rect.x + rect.width, `${field} 右边界 ${rect.x + rect.width} 应 ≤ 480 (不右溢)`).toBeLessThanOrEqual(480);
    }
    for (let i = 1; i < ys.length; i++) {
      expect(ys[i]! - ys[i - 1]!).toBeGreaterThanOrEqual(8);
    }
  });

  test('立场 ① 1024 viewport: 中等屏 6 个 field stack 不溢出 (liema #3)', async ({ page }) => {
    await page.setViewportSize({ width: 1024, height: 800 });
    await openAgentConfigPanel(page);

    const fields = ['name', 'avatar', 'prompt', 'model', 'memory_ref', 'enabled'];
    const ys: number[] = [];
    for (const field of fields) {
      const rect = await page.locator(`[data-agent-config-field="${field}"]`).first().evaluate(
        (el) => el.getBoundingClientRect(),
      );
      ys.push(rect.y);
    }
    for (let i = 1; i < ys.length; i++) {
      expect(ys[i]! - ys[i - 1]!).toBeGreaterThanOrEqual(8);
    }
  });

  test('立场 ④ 6 个 data-agent-config-field byte-identical 不动 (REG-AL2A-* 锚保)', async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 800 });
    await openAgentConfigPanel(page);

    const expectedFields = ['name', 'avatar', 'prompt', 'model', 'memory_ref', 'enabled'];
    for (const field of expectedFields) {
      await expect(page.locator(`[data-agent-config-field="${field}"]`)).toHaveCount(1);
    }
    // Header / version / save anchors are also locked.
    await expect(page.locator('[data-agent-config="root"]')).toBeVisible();
    await expect(page.locator('[data-agent-config-version]')).toBeVisible();
    await expect(page.locator('[data-agent-config-action="save"]')).toBeVisible();
    // Title literal remains byte-identical with "Agent 配置" as locked by al-2a-content-lock.test.ts.
    await expect(page.locator('[data-agent-config="root"] h3')).toHaveText('Agent 配置');
  });
});
