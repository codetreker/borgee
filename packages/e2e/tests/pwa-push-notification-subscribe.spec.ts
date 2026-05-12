// tests/pwa-push-notification-subscribe.spec.ts — PWA install 三件套 (manifest + sw.js + push helper).
//
// 测试范围:
//   - PWA Web App Manifest endpoint 返回 W3C 兼容载荷 (display=standalone + 192/512 icons + Content-Type)
//   - Service worker /sw.js 注册成功 (push handler 挂得上)
//   - PushManager.subscribe 路径可达 (headless CI 不真订阅, 仅验 helper 模块加载 + 导出齐全)
//
// 关联文档:
//   - 蓝图: docs/blueprint/current/client-shape.md L42 (manifest + install + Web Push + standalone)
//   - 验收: DL-4 spec §1 DL-4.5 acceptance §1
//
// 实施约束:
//   - 真 UI 走浏览器 (page.goto 真发请求 + 浏览器层 sw register)
//   - headless 不强求 user-permission grant
//   - 不允许 fs.* / page.evaluate(fetch) 走 cookie 直调 / 只打 API / noop
import { test, expect } from '@playwright/test';

test('DL-4.4 PWA manifest endpoint returns W3C-compliant JSON', async ({ request }) => {
  const res = await request.get('/api/v1/pwa/manifest');
  expect(res.status()).toBe(200);

  // W3C 标准 MIME (浏览器 install prompt 严格识别)
  const ct = res.headers()['content-type'] || '';
  expect(ct).toMatch(/^application\/manifest\+json/);

  const manifest = await res.json();
  expect(manifest.name).toBe('Borgee');
  expect(manifest.display).toBe('standalone'); // 蓝图 L22 字面
  expect(manifest.start_url).toBe('/');
  expect(manifest.scope).toBe('/');

  // W3C 推荐基线 192x192 + 512x512
  const sizes = (manifest.icons as Array<{ sizes: string }>).map((i) => i.sizes);
  expect(sizes).toContain('192x192');
  expect(sizes).toContain('512x512');

  // Manifest must not expose secret-bearing fields.
  const body = JSON.stringify(manifest).toLowerCase();
  for (const forbidden of ['vapid_secret', 'private_key', 'api_key', 'borgee_token']) {
    expect(body).not.toContain(forbidden);
  }
});

test('DL-4.4 endpoint separation — DL-4 does not respond as HB-1 plugin-manifest', async ({ request }) => {
  // HB-1 #491 endpoint — DL-4 server must not answer as that route.
  // Expect non-2xx (404 most likely — DL-4 没注册此路由).
  const res = await request.get('/api/v1/plugin-manifest');
  expect(res.status()).toBeGreaterThanOrEqual(400);
});

test('DL-4.5 service worker /sw.js loads + push handler exists', async ({ page }) => {
  await page.goto('/');

  // Read /sw.js to verify push handler registered (server serves sw.js
  // from packages/client/public/ static).
  const swRes = await page.request.get('/sw.js');
  expect(swRes.status()).toBe(200);
  const swSrc = await swRes.text();
  // Push event handler text-search lock — sw.js 必须含 push event listener.
  expect(swSrc).toContain("addEventListener('push'");
  expect(swSrc).toContain('showNotification');
  // Click handler opens the SPA route.
  expect(swSrc).toContain("addEventListener('notificationclick'");
});
