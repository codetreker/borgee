// tests/adm-1-privacy-promise.spec.ts — ADM-1 e2e + G4.1 demo 双截屏.
//
// 闭环 docs/_archive/qa/acceptance-templates/adm-1.md:
//   §1 三承诺字面 1:1 + §2 八行 ✅/❌ 表格 byte-identical + §3 三色锁
//   §2 反约束 — privacy section 默认展开不可折叠 (反 details-element 包裹)
//   §4 闸 4 demo 双截屏: g4.1-adm1-{privacy-promise,privacy-table}.png
//
// 立场反查 (admin-model.md §0):
//   - "强权但不窥视" — admin 是平台运维, 不是协作者
//   - 用户首次进设置页就读懂 admin 跟自己关系 (§4.1 字面)
//
// 实现说明: 真 server-go(4901) + vite(5174), 不 mock (跟 cm-onboarding /
// cv-3-3-renderers / chn-4-collab-skeleton 同模式).
import {
  test,
  expect,
  request as apiRequest,
  type APIRequestContext,
  type Page,
  type BrowserContext,
} from '@playwright/test';
import * as path from 'path';
import { fileURLToPath } from 'url';

const ADMIN_LOGIN = 'e2e-admin';
const ADMIN_PASSWORD = 'e2e-admin-pass-12345';
const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const SCREENSHOT_DIR = path.resolve(__dirname, '../../../docs/qa/screenshots');

// ADM-1 三承诺字面 byte-identical (admin-model.md §4.1 + spec §2).
// 改 = 改 admin-model.md §4.1 + PRIVACY_PROMISES 常量 + 此 e2e 三处 (drift CI 拦).
const PRIVACY_PROMISE_FRAGMENTS = [
  'Admin 是平台运维, 不是协作者',
  '永不出现在 channel / DM / 团队列表里',
  'Admin 看不到消息 / 文件 / artifact 内容',
  '24h 时窗, 顶部红色横幅常驻, 可随时撤销',
  'Admin 能看的是元数据',
  '看不到正文',
];

// 八行表格 category 字面 byte-identical (PRIVACY_TABLE_ROWS 同源).
const TABLE_CATEGORIES = [
  '用户名 / 邮箱',
  'channel 名 / 列表',
  '消息条数 / 登录时间',
  '消息正文 (channel / DM)',
  'artifact / 文件内容',
  '你和 owner-agent 内置 DM',
  'API key 原值',
  '授权 impersonate 后 24h 实时入站',
];

interface RegisteredUser {
  email: string;
  token: string;
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

async function registerUser(serverURL: string, inviteCode: string): Promise<RegisteredUser> {
  const ctx = await apiRequest.newContext({ baseURL: serverURL });
  const stamp = Date.now();
  const email = `adm1-e2e-${stamp}-${Math.floor(Math.random() * 1000)}@example.test`;
  const res = await ctx.post('/api/v1/auth/register', {
    data: {
      invite_code: inviteCode,
      email,
      password: 'p@ssw0rd-adm1-e2e',
      display_name: `ADM1 E2E ${stamp}`,
    },
  });
  expect(res.ok(), `register: ${res.status()} ${await res.text()}`).toBe(true);
  const cookies = await ctx.storageState();
  const tok = cookies.cookies.find((c) => c.name === 'borgee_token');
  expect(tok, 'borgee_token cookie missing').toBeTruthy();
  return { email, token: tok!.value, ctx };
}

function clientURL(): string {
  return `http://127.0.0.1:${process.env.E2E_CLIENT_PORT ?? '5174'}`;
}

async function attachToken(ctx: BrowserContext, token: string): Promise<void> {
  const url = new URL(clientURL());
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

async function gotoSettings(page: Page): Promise<void> {
  await page.goto(`${clientURL()}/`);
  await expect(page.locator('.sidebar-title')).toBeVisible();
  // ⚙️ 按钮 (Sidebar.tsx data-action="open-settings", 跟 onAgentsOpen
  // /onWorkspacesOpen 同模式).
  await page.locator('[data-action="open-settings"]').click();
  await expect(page.locator('[data-page="settings"]')).toBeVisible();
}

test.describe('ADM-1 PrivacyPromise — acceptance §1+§2+§3 + G4.1 demo', () => {
  test('§1+§2 — 三承诺字面 1:1 + 八行表格 byte-identical + 三色锁 + G4.1 截屏', async ({ browser }) => {
    const serverPort = process.env.E2E_SERVER_PORT ?? '4901';
    const serverURL = `http://127.0.0.1:${serverPort}`;
    const adminCtx = await adminLogin(serverURL);
    const inv = await mintInvite(adminCtx, 'adm-1-e2e');
    const user = await registerUser(serverURL, inv);

    const ctx = await browser.newContext();
    await attachToken(ctx, user.token);
    const page = await ctx.newPage();
    await gotoSettings(page);

    // privacy tab active by default (acceptance §2).
    const privacyTab = page.locator('[data-tab="privacy"]');
    await expect(privacyTab).toBeVisible();
    await expect(privacyTab).toHaveText('隐私');
    await expect(privacyTab).toHaveClass(/active/);

    // §1 三承诺字面 byte-identical 跟 admin-model §4.1 同源 (drift 锁).
    const promiseList = page.locator('.privacy-promise-list');
    await expect(promiseList).toBeVisible();
    const items = page.locator('.privacy-promise-item');
    await expect(items).toHaveCount(3);
    for (const fragment of PRIVACY_PROMISE_FRAGMENTS) {
      await expect(promiseList).toContainText(fragment);
    }

    // G4.1 demo 截屏 #1 — 首屏渲染 (含 3 承诺 + 表格头).
    await page.screenshot({
      path: path.join(SCREENSHOT_DIR, 'g4.1-adm1-privacy-promise.png'),
      fullPage: false,
    });

    // §2 八行表格 category 字面 byte-identical + 顺序不变.
    const rows = page.locator('.privacy-promise-table tbody tr');
    await expect(rows).toHaveCount(8);
    for (let i = 0; i < TABLE_CATEGORIES.length; i++) {
      await expect(rows.nth(i).locator('td').first()).toHaveText(TABLE_CATEGORIES[i]!);
    }

    // §3 三色锁 — allow (gray) / deny (#d33 加粗) / impersonate (#d97706 amber).
    // Class names 反查; computed style 走 vitest unit (此处 class lock 已够).
    await expect(rows.nth(0)).toHaveClass(/privacy-row-allow/);
    await expect(rows.nth(0)).toHaveAttribute('data-row-kind', 'allow');
    await expect(rows.nth(3)).toHaveClass(/privacy-row-deny/);
    await expect(rows.nth(3)).toHaveAttribute('data-row-kind', 'deny');
    await expect(rows.nth(7)).toHaveClass(/privacy-row-impersonate/);
    await expect(rows.nth(7)).toHaveAttribute('data-row-kind', 'impersonate');

    // 八行 mark 字面 byte-identical (✅ × 3 / ❌ × 4 / ✅ (临时) × 1).
    await expect(rows.nth(0).locator('td').nth(1)).toHaveText('✅');
    await expect(rows.nth(3).locator('td').nth(1)).toHaveText('❌');
    await expect(rows.nth(7).locator('td').nth(1)).toHaveText('✅ (临时)');

    // G4.1 demo 截屏 #2 — 八行表格 (滚动到表格视野).
    await page.locator('.privacy-promise-table').scrollIntoViewIfNeeded();
    await page.screenshot({
      path: path.join(SCREENSHOT_DIR, 'g4.1-adm1-privacy-table.png'),
      fullPage: false,
    });
  });

  test('§2 反约束 — privacy section 默认展开不可折叠 (反 details-element)', async ({ browser }) => {
    const serverPort = process.env.E2E_SERVER_PORT ?? '4901';
    const serverURL = `http://127.0.0.1:${serverPort}`;
    const adminCtx = await adminLogin(serverURL);
    const inv = await mintInvite(adminCtx, 'adm-1-e2e-collapse');
    const user = await registerUser(serverURL, inv);

    const ctx = await browser.newContext();
    await attachToken(ctx, user.token);
    const page = await ctx.newPage();
    await gotoSettings(page);

    // 反向断言 — settings page 全树无 details-element 包裹.
    // 野马 R3 + acceptance §2.3 字面: privacy 区块默认展开不可折叠.
    const detailsCount = await page.locator('details').count();
    expect(detailsCount, 'privacy section 不应被 details-element 包裹').toBe(0);

    // privacy section 始终 visible (无 hidden / collapsed 态).
    const promise = page.locator('.privacy-promise');
    await expect(promise).toBeVisible();
  });

  test('§4 反向断言 — admin/user 路径分叉 (跟 admin SPA SettingsPage 共存不混用)', async ({ browser }) => {
    // ADM-0 红线: admin SPA 跟 user SPA cookie 拆 + 路径分叉. 此 test 反向
    // 断言用户 SPA SettingsPage 跟 admin/pages/SettingsPage.tsx 不混用 —
    // user 视角访问 /admin/* 应 reject (admin cookie 缺).
    const serverPort = process.env.E2E_SERVER_PORT ?? '4901';
    const serverURL = `http://127.0.0.1:${serverPort}`;
    const adminCtx = await adminLogin(serverURL);
    const inv = await mintInvite(adminCtx, 'adm-1-e2e-isolation');
    const user = await registerUser(serverURL, inv);

    // user cookie 调 admin-api → 401 (ADM-0.2 cookie 拆守).
    const res = await user.ctx.get('/admin-api/auth/me');
    expect([401, 403], `user cookie 调 admin-api 应 reject; got ${res.status()}`).toContain(
      res.status(),
    );
  });
});
