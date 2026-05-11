// tests/smoke-app-loads.spec.ts — dual-service (server-go + vite) smoke check (INFRA-2).
//
// 测试范围:
//   - server-go /health endpoint 返回 200 (Go 二进制启动成功)
//   - client SPA 根 / 返回包含 Borgee title 的 HTML 文档 (vite 启动 + index.html 服务)
//   - vite dev proxy 转发 /health 到 server-go (代理 override env 端到端生效)
//
// 不在范围:
//   - auth / 产品功能 (走 RT-0 / chat-first-time-onboarding 等专项 spec)
//
// 关联文档:
//   - 验收: INFRA-2 spec (双服务 harness CI 跑通)
//
// 实施约束:
//   - Browser-driven path (request.get + page.goto)
//   - Smoke failures should indicate infrastructure issues (port conflict / server crash / proxy misconfiguration), not product bugs
//   - 不允许 fs.* / page.evaluate(fetch) / 只打 API 不开浏览器 / empty placeholder tests
import { test, expect } from '@playwright/test';

test.describe('INFRA-2 smoke', () => {
  test('server-go /health returns ok', async ({ request }) => {
    // server-go was booted on E2E_SERVER_PORT (4901) by playwright.config.
    // We hit it directly (not via proxy) to isolate "is the server up?"
    // from "is the proxy working?".
    const port = process.env.E2E_SERVER_PORT ?? '4901';
    const res = await request.get(`http://127.0.0.1:${port}/health`);
    expect(res.ok()).toBe(true);
  });

  test('client SPA root serves index.html', async ({ page }) => {
    // baseURL = client url (5174) per playwright.config.
    await page.goto('/');
    await expect(page).toHaveTitle(/Borgee/);
  });

  test('vite dev proxy forwards /health to server-go', async ({ request }) => {
    // Hit the *client* port so the request goes through vite's proxy.
    // If VITE_E2E_API_TARGET wiring is wrong, vite proxies to localhost:4900
    // (the dev default), which is either nothing in CI (502) or a stale
    // dev binary on a developer's machine (would still 200 but for the
    // wrong reason — that's why we'd want a marker, but for the smoke
    // test 200 is enough).
    const clientPort = process.env.E2E_CLIENT_PORT ?? '5174';
    const res = await request.get(`http://127.0.0.1:${clientPort}/health`);
    expect(res.ok()).toBe(true);
  });
});
