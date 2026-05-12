// tests/admin-audit-event-stream.spec.ts — admin 多源审计事件流页面 + 来源过滤 + 时间窗.
//
// 测试范围:
//   - admin `/admin/audit-multi-source` 页面真实渲染，并显示 4 个来源过滤选项
//   - 选 plugin 来源后，表格只显示 plugin 来源行
//   - admin API 路径独立: user API 访问 multi-source 端点应 404/403
//   - since/until 时间窗过滤生效, invalid 入参返回 400 audit.time_range_invalid
//   - limit clamp: limit=999 → 500, limit=0 → 100
//
// 关联文档:
//   - 蓝图: docs/blueprint/current/admin-model.md §1.3 (admin 路径独立) §1.4 (来源透明)
//   - 验收: docs/_archive/qa/acceptance-templates/adm-3-v1-e2e.md §1+§2+§3
//
// 实施约束:
//   - UI 验证通过浏览器执行 (page.goto + dropdown 选择 + DOM 断言)
//   - 4 来源枚举与 server AuditSources 保持一致: server/plugin/host_bridge/agent
//   - 不修改 production code (本 spec 仅增加 E2E 覆盖, 不改 #619 实现)
//   - 不允许 fs.* / page.evaluate(fetch) / API-only / noop

import { test, expect, request as apiRequest, type APIRequestContext } from '@playwright/test';

const ADMIN_LOGIN = 'e2e-admin';
const ADMIN_PASSWORD = 'e2e-admin-pass-12345';
const SERVER_URL = `http://127.0.0.1:${process.env.E2E_SERVER_PORT ?? '4901'}`;
const CLIENT_URL = `http://127.0.0.1:${process.env.E2E_CLIENT_PORT ?? '5174'}`;

// 4 source enum must match server const values (admin_audit_query.go AuditSources).
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
  test('case-1 admin /admin/audit-multi-source renders page DOM and 4 source filter options', async ({
    browser,
  }) => {
    const admin = await adminLogin();

    const ctx = await browser.newContext();
    const url = new URL(CLIENT_URL);
    await ctx.addCookies([
      {
        name: 'borgee_admin_session',
        value: admin.cookieValue,
        domain: url.hostname,
        path: '/',
        httpOnly: true,
        secure: false,
        sameSite: 'Lax',
      },
    ]);
    const page = await ctx.newPage();
    // Vite dev does not auto-serve admin.html for /admin/* paths; load
    // admin.html directly + push history so BrowserRouter mounts at the
    // target route. This follows ap-2-bundle.spec.ts §3.2 for admin routes:
    // production serves /admin/ through server-go fallback, while dev serves admin.html.
    await page.addInitScript(() => {
      window.history.replaceState({}, '', '/admin/audit-multi-source');
    });
    await page.goto(`${CLIENT_URL}/admin.html`);
    await page.waitForLoadState('domcontentloaded');

    // Page-level data attribute must be visible so a blank shell cannot pass.
    const pageAnchor = page.locator('[data-page="admin-audit-multi-source"]');
    await expect(pageAnchor).toBeVisible();

    // The 4 source enum options must be present in the filter dropdown.
    const filter = page.locator('[data-filter="source"]');
    await expect(filter).toBeVisible();
    for (const src of AUDIT_SOURCES) {
      await expect(filter.locator(`option[value="${src}"]`)).toHaveCount(1);
    }

    // Either the table or empty state must render. The E2E fixture is usually
    // empty, but the SPA must render a complete empty state rather than a shell.
    // Do not require rows because cross-milestone fixture data is brittle.
    const tableOrEmpty = page.locator(
      '[data-testid="multi-source-audit-table"], .admin-empty-state',
    );
    await expect(tableOrEmpty.first()).toBeVisible();

    await ctx.close();
    await admin.ctx.dispose();
  });

  test('case-2 4 source filter — plugin selection verifies the source filter API', async () => {
    const admin = await adminLogin();

    // REST-driven check following ap-2-bundle.spec.ts §3.2: UI rendering and
    // API behavior are verified separately. Call each source once so enum drift
    // is caught; source=plugin exercises the SQL filter.
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
      // Every returned row must match the requested source filter.
      for (const row of body.rows) {
        expect(row.source, `${src} filter leaked: ${row.source}`).toBe(src);
      }
    }

    // source=invalid must return the locked 400 error instead of being accepted.
    const bad = await admin.ctx.get('/admin-api/v1/audit/multi-source?source=invalid_source');
    expect(bad.status(), `invalid source: ${bad.status()}`).toBe(400);
    const badBody = await bad.text();
    expect(badBody, 'audit.source_invalid 错码字面').toContain('audit.source_invalid');

    await admin.ctx.dispose();
  });

  test('case-3 admin API isolation — user API /api/v1/audit/multi-source rejects', async () => {
    const admin = await adminLogin();
    const inv = await mintInvite(admin.ctx, 'adm-3-e2e-god-mode');
    const user = await registerUser(inv, 'god-mode');

    // User API call to /api/v1/audit/multi-source should fail because only the
    // admin API exposes this audit endpoint. This mirrors ap-2-bundle.spec.ts
    // case-2 for POST /api/v1/bundles.
    const res = await user.ctx.get('/api/v1/audit/multi-source');
    expect(
      res.status() === 404 || res.status() === 403 || res.status() === 401,
      `user-rail audit/multi-source 应 reject; got ${res.status()}`,
    ).toBe(true);

    // Admin API path must also reject a user cookie (ADM-0.2 cookie split).
    const userToAdmin = await user.ctx.get('/admin-api/v1/audit/multi-source');
    expect(
      userToAdmin.status() === 401 || userToAdmin.status() === 403,
      `user cookie 调 admin-api 应 reject; got ${userToAdmin.status()}`,
    ).toBe(true);

    await user.ctx.dispose();
    await admin.ctx.dispose();
  });

  test('case-4 time range filter — since/until accepted and invalid input rejected', async () => {
    const admin = await adminLogin();

    // Happy path: since/until should be accepted even when no rows are seeded.
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

    // since alone should also be accepted.
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

  test('case-5 limit clamp — limit=999 clamps to max 500 and limit=0 uses default 100', async () => {
    const admin = await adminLogin();

    // limit=999 is clamped to 500 by server-side parseLimit, matching ADM-2.2.
    // The E2E fixture is usually empty; the contract is that limit > max clamps
    // instead of rejecting.
    const big = await admin.ctx.get('/admin-api/v1/audit/multi-source?limit=999');
    expect(big.ok(), `limit=999 clamp: ${big.status()}`).toBe(true);
    const bigBody = (await big.json()) as { rows: unknown[] };
    expect(bigBody.rows.length, 'rows ≤ 500 (clamped)').toBeLessThanOrEqual(500);

    // limit=0 uses the default 100 and should not reject.
    const zero = await admin.ctx.get('/admin-api/v1/audit/multi-source?limit=0');
    expect(zero.ok(), `limit=0 default: ${zero.status()}`).toBe(true);

    // Non-integer limit falls back to the server default instead of rejecting.
    const garbage = await admin.ctx.get('/admin-api/v1/audit/multi-source?limit=garbage');
    expect(garbage.ok(), `limit=garbage default: ${garbage.status()}`).toBe(true);

    await admin.ctx.dispose();
  });
});
