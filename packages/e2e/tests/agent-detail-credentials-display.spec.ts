// tests/agent-detail-credentials-display.spec.ts — Agent 详情 Manage 卡片 + Credentials 凭据展示 (gh#684).
//
// 测试范围:
//   - Manage 展开后 6 卡 section 渲染齐全
//   - Credentials 卡显 mask 形式 `bgr_...{last4}` (非完整 plaintext)
//   - 反向断: 没有 Show 按钮 (防回归)
//   - 复制按钮 aria-label="复制 API Key" + title 文案一致
//   - Prompt textarea rows=8
//
// 不在范围:
//   - clipboard.readText 跨环境 permission 验证 (走 vitest 单测覆盖 writeText 调用 + auto-clear)
//   - 完整复制流程 (走 QA 6 步手工真验, 见 brief §5 行 167)
//
// 关联文档:
//   - 需求: gh#684 brief §5 (e2e) + §3 (文案锁) + §2.2 (Prompt rows)
//
// 实施约束:
//   - 真 UI 走浏览器 (page.goto + 真展开 + DOM 断)
//   - 跟 agent-config-form-layout.spec.ts 共用 helpers 模式
//   - 不允许 fs.* / page.evaluate(fetch) / 只打 API / noop

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
  const email = `gh684-${suffix}-${stamp}-${Math.floor(Math.random() * 1000)}@example.test`;
  const password = 'p@ssw0rd-gh684';
  const displayName = `GH684 ${suffix} ${stamp}`;
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

/** Open Agents page + expand first agent's Manage. 跟 gh-698 同模式. */
async function openAgentManage(page: Page) {
  await page.goto('/');
  await expect(
    page.locator('.hamburger-btn, .sidebar-title').first(),
  ).toBeVisible({ timeout: 10_000 });
  const hamburger = page.locator('.hamburger-btn');
  const isMobile = await hamburger.count() > 0 && await hamburger.isVisible();
  if (isMobile) {
    await hamburger.click();
  }
  await page.locator('[data-testid="sidebar-nav-agents"]').click();
  await expect(page.locator('.agent-page')).toBeVisible({ timeout: 10_000 });
  if (isMobile) {
    await page.evaluate(() => {
      const overlay = document.querySelector('.sidebar-overlay') as HTMLElement | null;
      if (overlay) overlay.click();
    });
    await expect(page.locator('.sidebar-overlay')).toHaveCount(0, { timeout: 5_000 });
  }
  const manageBtn = page.locator('.agent-card button.btn-sm', { hasText: 'Manage' }).first();
  await manageBtn.click();
  // Credentials 卡渲染锚.
  await expect(page.locator('.agent-detail-card-credentials')).toBeVisible({ timeout: 5_000 });
}

test.describe('gh#684 Agents 详情排版重组 (6 卡 + mask + 复制)', () => {
  test.beforeEach(async ({ page, baseURL }) => {
    const serverURL = `http://127.0.0.1:${process.env.E2E_SERVER_PORT ?? '4901'}`;
    const adminCtx = await adminLogin(serverURL);
    const inviteCode = await mintInvite(adminCtx, 'gh684-detail');
    const owner = await registerUser(serverURL, inviteCode, 'owner');
    await createAgent(serverURL, owner.token, `gh684-agent-${Date.now().toString(36)}`);
    await attachToken(page.context(), baseURL!, owner.token);
  });

  test('§2.1 6 卡 section 出现 (Identity / Credentials / Runtime / Config / Permissions / Channels)', async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 800 });
    await openAgentManage(page);

    // Runtime 卡如果 fetchAgentRuntime 返 null 会 graceful degrade omit, 不强求.
    // 其它 5 卡必出现.
    await expect(page.locator('.agent-detail-card-identity')).toBeVisible();
    await expect(page.locator('.agent-detail-card-credentials')).toBeVisible();
    await expect(page.locator('.agent-detail-card-config')).toBeVisible();
    await expect(page.locator('.agent-detail-card-permissions')).toBeVisible();
    await expect(page.locator('.agent-detail-card-channels')).toBeVisible();
  });

  test('§2.3 Credentials 卡显 mask bgr_...{last4} (反完整 plaintext + 反 Show 按钮)', async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 800 });
    await openAgentManage(page);

    // mask 真值锚 testid + 内容格式.
    const mask = page.locator('[data-testid="agent-api-key-mask"]');
    await expect(mask).toBeVisible({ timeout: 10_000 });
    // 等 fetch 完成: 加载中 → bgr_...xxxx.
    await expect(mask).toContainText(/^bgr_\.\.\..{4}$/, { timeout: 10_000 });

    // 反向: Show 按钮不存在.
    const showBtn = page.locator('button', { hasText: /^(Show|Hide)$/ });
    await expect(showBtn).toHaveCount(0);

    // 反向: 完整 plaintext key (bgr_<64 hex>) 不在 DOM.
    const html = await page.content();
    expect(html).not.toMatch(/bgr_[0-9a-f]{64}/);
  });

  test('§3 文案锁: 复制按钮 aria-label + title byte-identical', async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 800 });
    await openAgentManage(page);

    const copyBtn = page.locator('button[aria-label="复制 API Key"]');
    await expect(copyBtn).toBeVisible();
    await expect(copyBtn).toHaveAttribute('title', '复制完整 API Key 到剪贴板');
  });

  test('§3 反 OpenAI 前缀: sk-... 不出现 (反复制粘贴 OpenAI 文档误抄)', async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 800 });
    await openAgentManage(page);

    const html = await page.content();
    // 反 brief §2.4 / §3 grep 守卫 `['"]sk-\.\.\.` 同模式.
    expect(html).not.toMatch(/sk-\.\.\./);
  });

  test('§2.2 Prompt textarea rows=8 (yema brief 拍默认变大)', async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 800 });
    await openAgentManage(page);

    const promptTa = page.locator('[data-agent-config-field="prompt"]');
    await expect(promptTa).toBeVisible();
    await expect(promptTa).toHaveAttribute('rows', '8');
  });

  test('§2.4 反 Rotate API Key 文案丢失 (字面保留 byte-identical)', async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 800 });
    await openAgentManage(page);

    await expect(page.locator('button', { hasText: 'Rotate API Key' })).toBeVisible();
  });
});
