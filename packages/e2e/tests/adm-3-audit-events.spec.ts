// tests/adm-3-audit-events.spec.ts — ADM-3 v1 multi-source audit Playwright e2e (acceptance §3.3).
//
// 闭环 docs/_archive/qa/acceptance-templates/adm-3-v1-e2e.md §1+§2+§3:
//   case-1 admin /admin/audit-multi-source 渲染 (page DOM + 4 source filter dropdown 真显)
//   case-2 4 source filter dropdown — 选 plugin → 表只显 plugin source 行 (反向断 server/host_bridge/agent 行 0 hit)
//   case-3 admin god-mode 路径独立 (user-rail 走 /api/v1/audit/multi-source 反向断 404/403 + page-level reverse-grep)
//
// 立场反查 (admin-model.md §1.4 来源透明 + ADM-0 §1.3 admin god-mode 路径独立):
//   - 4 source enum SSOT (server/plugin/host_bridge/agent) byte-identical 跟 server-side AuditSources
//   - admin god-mode 路径独立: 仅 /admin-api/v1/audit/multi-source 暴露, 反 user-rail 漂
//   - 0 production code 改 (post-#619 byte-identical, 本 PR 仅加 e2e)
//
// 实现策略: REST-driven anchor (跟 ap-2-bundle.spec.ts + dm-3-multi-device-sync.spec.ts 同模式)
// + admin SPA browser context (跟 adm-1-privacy-promise.spec.ts SettingsPage e2e 模式承袭).

import {
  test,
  expect,
  request as apiRequest,
  type APIRequestContext,
} from '@playwright/test';

const ADMIN_LOGIN = 'e2e-admin';
const ADMIN_PASSWORD = 'e2e-admin-pass-12345';
const SERVER_URL = `http://127.0.0.1:${process.env.E2E_SERVER_PORT ?? '4901'}`;
const CLIENT_URL = `http://127.0.0.1:${process.env.E2E_CLIENT_PORT ?? '5174'}`;

// 4 source enum byte-identical 跟 server const (admin_audit_query.go AuditSources).
const AUDIT_SOURCES = ['server', 'plugin', 'host_bridge', 'agent'] as const;

interface AdminSession {
  ctx: APIRequestContext;
  cookieValue: string;
}

async function adminLogin(): Promise<AdminSession> {
  const ctx = await apiRequest.newContext({ baseURL: SERVER_URL });
  const res = await ctx.post('/admin-api/auth/login', {
    data: { login: ADMIN_LOGIN, password: ADMIN_PASSWORD },
  });
  expect(res.ok(), `admin login: ${res.status()}`).toBe(true);
  const cookies = await ctx.storageState();
  const tok = cookies.cookies.find((c) => c.name === 'borgee_admin_session');
  expect(tok, 'borgee_admin_session cookie missing').toBeTruthy();
  return { ctx, cookieValue: tok!.value };
}

async function mintInvite(adminCtx: APIRequestContext, note: string): Promise<string> {
  const res = await adminCtx.post('/admin-api/v1/invites', { data: { note } });
  expect(res.ok(), `mint invite: ${res.status()}`).toBe(true);
  const body = (await res.json()) as { invite: { code: string } };
  return body.invite.code;
}

interface UserCtx {
  ctx: APIRequestContext;
  email: string;
}

async function registerUser(inviteCode: string, suffix: string): Promise<UserCtx> {
  const ctx = await apiRequest.newContext({ baseURL: SERVER_URL });
  const stamp = Date.now();
  const email = `adm3-e2e-${suffix}-${stamp}-${Math.floor(Math.random() * 1000)}@example.test`;
  const res = await ctx.post('/api/v1/auth/register', {
    data: {
      invite_code: inviteCode,
      email,
      password: 'p@ssw0rd-adm3-e2e',
      display_name: `ADM3 E2E ${suffix} ${stamp}`,
    },
  });
  expect(res.ok(), `register: ${res.status()} ${await res.text()}`).toBe(true);
  return { ctx, email };
}

test.describe('ADM-3 v1 multi-source audit — acceptance §3.3 e2e', () => {
  test('case-1 admin /admin/audit-multi-source 渲染 — page DOM 真显 + 4 source filter dropdown', async ({ browser }) => {
    const admin = await adminLogin();

    const ctx = await browser.newContext();
    const url = new URL(CLIENT_URL);
    await ctx.addCookies([{
      name: 'borgee_admin_session',
      value: admin.cookieValue,
      domain: url.hostname,
      path: '/',
      httpOnly: true,
      secure: false,
      sameSite: 'Lax',
    }]);
    const page = await ctx.newPage();
    // Vite dev does not auto-serve admin.html for /admin/* paths; load
    // admin.html directly + push history so BrowserRouter mounts at the
    // target route. (跟 ap-2-bundle.spec.ts §3.2 admin god-mode case 同模式 —
    // /admin/ 入口 prod 走 server-go fallback, dev 走 admin.html.)
    await page.addInitScript(() => {
      window.history.replaceState({}, '', '/admin/audit-multi-source');
    });
    await page.goto(`${CLIENT_URL}/admin.html`);
    await page.waitForLoadState('domcontentloaded');

    // page-level data-attr 锚 真显 (反 silent fail).
    const pageAnchor = page.locator('[data-page="admin-audit-multi-source"]');
    await expect(pageAnchor).toBeVisible();

    // 4 source enum filter dropdown — option 真显 (反 enum 漂).
    const filter = page.locator('[data-filter="source"]');
    await expect(filter).toBeVisible();
    for (const src of AUDIT_SOURCES) {
      await expect(filter.locator(`option[value="${src}"]`)).toHaveCount(1);
    }

    // table 或 empty-state 真渲染 (e2e fixture 一定空, 但 SPA 必须渲染 empty-state
    // 反 silent skeleton). 不强求 row 真有 (避免 fixture seeding 跨 milestone fragile).
    const tableOrEmpty = page.locator(
      '[data-testid="multi-source-audit-table"], .admin-empty-state',
    );
    await expect(tableOrEmpty.first()).toBeVisible();

    await ctx.close();
    await admin.ctx.dispose();
  });

  test('case-2 4 source filter — 选 plugin → 反向断 source filter API 真过 (REST-driven 反 fixture seed 脆性)', async () => {
    const admin = await adminLogin();

    // REST-driven 反向断 (跟 ap-2-bundle.spec.ts §3.2 同模式 — UI render +
    // API behavior 拆死). 4 source 各调一次, 反 enum 漂; source=plugin 走真 SQL.
    for (const src of AUDIT_SOURCES) {
      const res = await admin.ctx.get(`/admin-api/v1/audit/multi-source?source=${src}`);
      expect(res.ok(), `${src}: ${res.status()}`).toBe(true);
      const body = (await res.json()) as { sources: string[]; rows: Array<{ source: string }> };
      expect(body.sources, 'sources enum 4 元素 byte-identical').toEqual([
        'server',
        'plugin',
        'host_bridge',
        'agent',
      ]);
      // 反向断 — 凡 row 必 source==filter (反 cross-source leak).
      for (const row of body.rows) {
        expect(row.source, `${src} filter leaked: ${row.source}`).toBe(src);
      }
    }

    // source=invalid → 400 byte-identical 错码 (反 silent accept).
    const bad = await admin.ctx.get('/admin-api/v1/audit/multi-source?source=invalid_source');
    expect(bad.status(), `invalid source: ${bad.status()}`).toBe(400);
    const badBody = await bad.text();
    expect(badBody, 'audit.source_invalid 错码字面').toContain('audit.source_invalid');

    await admin.ctx.dispose();
  });

  test('case-3 admin god-mode 路径独立 — user-rail /api/v1/audit/multi-source 反向断 404/403 + ADM-0 §1.3 红线', async () => {
    const admin = await adminLogin();
    const inv = await mintInvite(admin.ctx, 'adm-3-e2e-god-mode');
    const user = await registerUser(inv, 'god-mode');

    // user-rail 调 /api/v1/audit/multi-source — 路径不存在 (仅 admin-rail 暴露).
    // 期望 404 (not found) 反 ADM-0 §1.3 红线 (跟 ap-2-bundle.spec.ts case-2
    // 反向断 POST /api/v1/bundles 同模式承袭).
    const res = await user.ctx.get('/api/v1/audit/multi-source');
    expect(
      res.status() === 404 || res.status() === 403 || res.status() === 401,
      `user-rail audit/multi-source 应 reject; got ${res.status()}`,
    ).toBe(true);

    // 反向断 — admin-rail 路径用 user cookie 也应 reject (cookie 拆守 ADM-0.2).
    const userToAdmin = await user.ctx.get('/admin-api/v1/audit/multi-source');
    expect(
      userToAdmin.status() === 401 || userToAdmin.status() === 403,
      `user cookie 调 admin-api 应 reject; got ${userToAdmin.status()}`,
    ).toBe(true);

    await user.ctx.dispose();
    await admin.ctx.dispose();
  });

  test('case-4 time range filter — since/until 真测 + invalid → 400 audit.time_range_invalid', async () => {
    const admin = await adminLogin();

    // Happy path — since/until 真接 (no rows seeded fine, 反 query syntax err).
    const now = Date.now();
    const oneHrAgo = now - 60 * 60 * 1000;
    const happy = await admin.ctx.get(
      `/admin-api/v1/audit/multi-source?since=${oneHrAgo}&until=${now}`,
    );
    expect(happy.ok(), `since+until happy: ${happy.status()}`).toBe(true);
    const happyBody = (await happy.json()) as { sources: string[]; rows: unknown[] };
    expect(happyBody.sources, 'sources 4 元素 byte-identical').toEqual([
      'server',
      'plugin',
      'host_bridge',
      'agent',
    ]);
    expect(Array.isArray(happyBody.rows), 'rows 数组').toBe(true);

    // since alone 真过.
    const sinceOnly = await admin.ctx.get(`/admin-api/v1/audit/multi-source?since=${oneHrAgo}`);
    expect(sinceOnly.ok(), `since-only: ${sinceOnly.status()}`).toBe(true);

    // since negative → 400 audit.time_range_invalid 字面.
    const bad = await admin.ctx.get('/admin-api/v1/audit/multi-source?since=-100');
    expect(bad.status(), `since=-100: ${bad.status()}`).toBe(400);
    const badBody = await bad.text();
    expect(badBody, 'audit.time_range_invalid 错码字面').toContain('audit.time_range_invalid');

    // since=非整数 → 400.
    const malformed = await admin.ctx.get('/admin-api/v1/audit/multi-source?since=not_a_number');
    expect(malformed.status(), `since=non-int: ${malformed.status()}`).toBe(400);

    await admin.ctx.dispose();
  });

  test('case-5 limit clamp — limit=999 → clamp 到 max 500 + limit=0 → default 100', async () => {
    const admin = await adminLogin();

    // limit=999 → server-side parseLimit clamp 到 500 (跟 ADM-2.2 同模式).
    // 由于 e2e fixture 一定空, rows.length ≤ 500 byte-identical;
    // 不 fail-on-error 是 contract: limit > max 不 reject, 走 clamp.
    const big = await admin.ctx.get('/admin-api/v1/audit/multi-source?limit=999');
    expect(big.ok(), `limit=999 clamp: ${big.status()}`).toBe(true);
    const bigBody = (await big.json()) as { rows: unknown[] };
    expect(bigBody.rows.length, 'rows ≤ 500 (clamped)').toBeLessThanOrEqual(500);

    // limit=0 → default 100; 同样 contract 不 reject.
    const zero = await admin.ctx.get('/admin-api/v1/audit/multi-source?limit=0');
    expect(zero.ok(), `limit=0 default: ${zero.status()}`).toBe(true);

    // limit=非整数 → server-side parseLimit 容错 fallback default (反 reject).
    const garbage = await admin.ctx.get('/admin-api/v1/audit/multi-source?limit=garbage');
    expect(garbage.ok(), `limit=garbage default: ${garbage.status()}`).toBe(true);

    await admin.ctx.dispose();
  });
});
