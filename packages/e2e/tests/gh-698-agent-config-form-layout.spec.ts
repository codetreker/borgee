// tests/gh-698-agent-config-form-layout.spec.ts — gh#698 form 排版 e2e
//
// 验 design 698-agent-config-form-overlap-fix.md §7 测试策略 + §4 边界条件
// + liema review 4 条非阻塞建议:
//   ✓ liema #1 a11y 反向断言走浏览器层 (不 vitest), 真量 inline style
//   ✓ liema #3 多 viewport 自适应 (1280 + 1024 + 480)
//   - liema #4 label htmlFor 隐式关联 (设计层 §3 已写, 不需要 e2e 测)
//
// 立场反查 (跟 design 对齐):
//   ① 6 个 label 各占独立行不重叠 (修前 800px 父容器下 inline 流水重叠)
//   ② 5 个 text/textarea label display: 'block' (yema 拍 a 默认)
//   ③ 1 个 checkbox label display: 'flex' inline (yema 拍 b 例外)
//   ④ data-agent-config-field byte-identical 6 个 (REG-AL2A-* 锚保)
//
// 反约束 (本 spec 锚):
//   - 不引入新 CSS class 验证 (方案 A 是内联 style)
//   - 不测 al-2a content lock drift (gh#701 followup, 不在本 PR)

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
 * AgentConfigPanel 在 Manage 展开后渲染.
 *
 * gh#698 e2e CI fail 修: 480px mobile viewport 下 sidebar 默认折叠 (App.tsx
 * isMobile = innerWidth < 768, sidebar-wrapper sidebar-closed), [data-testid=
 * "sidebar-nav-agents"] 在 closed sidebar 里不可点. 加 hamburger 守卫: 如果
 * mobile 路径 .hamburger-btn 可见, 先点开 sidebar 再点 agents nav. desktop
 * (≥768px) hamburger-btn 不渲染 (App.tsx L204 {isMobile && ...}), 直接点 nav.
 * (跟 PR #699 cv-1-3-canvas-modal-a11y.spec.ts mobile case 同款修法.)
 */
async function openAgentConfigPanel(page: Page) {
  await page.goto('/');
  // mobile: 等 hamburger 出来 (mobile 必有); desktop: 等 sidebar-title
  // (desktop 必有). 用 first() 避 strict mode .or() 双命中坑 (PR #699 踩过).
  await expect(
    page.locator('.hamburger-btn, .sidebar-title').first(),
  ).toBeVisible({ timeout: 10_000 });
  // mobile 路径: hamburger 可见 → 点开 sidebar (desktop 路径 hamburger DOM
  // 不渲染, count == 0, isVisible() == false, 跳过).
  const hamburger = page.locator('.hamburger-btn');
  const isMobile = await hamburger.count() > 0 && await hamburger.isVisible();
  if (isMobile) {
    await hamburger.click();
  }
  await page.locator('[data-testid="sidebar-nav-agents"]').click();
  await expect(page.locator('.agent-page')).toBeVisible({ timeout: 10_000 });
  // mobile sidebar 不会自动关 (Sidebar.tsx L316 onClick={onAgentsOpen} 不调
  // onClose, 不像 channel 选择 L106-107 同时调). sidebar-overlay 遮在
  // .main-content 上, click 任何 main-content 内元素被截获. 手动点 overlay
  // 关 sidebar 才能 click .agent-card 内 Manage button. (跟 PR #699 mobile
  // case 不撞这条因为它在 .artifact-empty button — Canvas tab 切换时 sidebar
  // 走 closeAllViews 自动关, gh#698 sidebar-nav-agents 不走 closeAllViews.)
  //
  // gh#698 e2e v4: overlay.click() 没 force 时被 .channel-list 拦 pointer
  // event (即便 overlay 在 DOM 上 visible). 加 { force: true } 跳 hit-testing
  // 直接派 click 到 overlay 元素本身. 用户真路径也是点 overlay 区域
  // (overlay 整 .main-content 几乎全屏), force 模拟没失真.
  if (isMobile) {
    const overlay = page.locator('.sidebar-overlay');
    if (await overlay.count() > 0) {
      await overlay.click({ force: true });
      await expect(overlay).toHaveCount(0);
    }
  }
  // Manage 展开 (展开后才会 mount AgentConfigPanel — design §1 路径).
  const manageBtn = page.locator('.agent-card button.btn-sm', { hasText: 'Manage' }).first();
  await manageBtn.click();
  // AgentConfigPanel 渲染锚: data-agent-config="root".
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

    // 6 个 label 各自占独立行 (y 不同). 用 data-agent-config-field 锚定 input,
    // 然后量每个 input 的 boundingRect: 6 个 input 应占 6 个不同 y.
    const fields = ['name', 'avatar', 'prompt', 'model', 'memory_ref', 'enabled'];
    const ys: number[] = [];
    for (const field of fields) {
      const rect = await page.locator(`[data-agent-config-field="${field}"]`).first().evaluate(
        (el) => el.getBoundingClientRect(),
      );
      ys.push(rect.y);
    }
    // 每个 field 跟下一个 field 的 y 间距 ≥ 8px (marginTop), 反 inline 流水重叠.
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

    // liema review #1 反向断言: 防"加新字段忘加 style".
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

    // checkbox 例外: 跟 CreateAgentModal Permissions 块同款 inline.
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
      // 反溢出: 每个 field 不应超出 viewport.
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
    // header / version / save 锚也不动.
    await expect(page.locator('[data-agent-config="root"]')).toBeVisible();
    await expect(page.locator('[data-agent-config-version]')).toBeVisible();
    await expect(page.locator('[data-agent-config-action="save"]')).toBeVisible();
    // 标题字面 byte-identical "Agent 配置" (al-2a-content-lock.test.ts 现锁此字面).
    await expect(page.locator('[data-agent-config="root"] h3')).toHaveText('Agent 配置');
  });
});
