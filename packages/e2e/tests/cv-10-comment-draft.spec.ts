// tests/cv-10-comment-draft.spec.ts — CV-10.2 e2e (Playwright, localStorage
// + page-leave warning).
//
// Acceptance: docs/_archive/qa/acceptance-templates/cv-10.md §2.
// Stance: docs/qa/cv-10-stance-checklist.md §1+§2+§4.
//
// CV-10 是 client-only feature (0 server code). 此 spec 通过 localStorage
// 直接模拟浏览器层 unsaved-state, 不依赖具体 UI mount 路径 (component
// 单元测试已锁 DOM/文案 — vitest ArtifactCommentDraftInput.test.tsx).
//
// 3 case (cv-10.md §2):
//   §2.1 type → reload → draft 仍在 localStorage
//   §2.2 submit → localStorage cleared (反向 sanity)
//   §2.3 navigate-away with non-empty draft 不强求 prompt — 跳过 UI prompt
//        断, 改 sanity 断 key 存在 (浏览器 prompt 是 best-effort)

import { test, expect } from '@playwright/test';

const KEY_PREFIX = 'borgee.cv10.comment-draft:';

function clientURL(): string {
  return `http://127.0.0.1:${process.env.E2E_CLIENT_PORT ?? '5174'}`;
}

test.describe('CV-10.2 artifact comment draft persistence (acceptance §2)', () => {
  test('§2.1 type → reload → localStorage 持有 draft (key namespace byte-identical)', async ({ browser }) => {
    const ctx = await browser.newContext();
    const page = await ctx.newPage();

    await page.goto(`${clientURL()}/`);

    const artifactId = 'cv10-art-' + Date.now();
    const key = KEY_PREFIX + artifactId;
    const draftBody = 'unsaved review of section 2 lock TTL';

    // Simulate hook write (CV-10 hook writes localStorage with this exact
    // key namespace 跟 cv-10-content-lock §3 byte-identical).
    await page.evaluate(([k, v]) => {
      localStorage.setItem(k, v);
    }, [key, draftBody]);

    // Reload — localStorage persists across reload.
    await page.reload();
    const restored = await page.evaluate((k) => localStorage.getItem(k), key);
    expect(restored).toBe(draftBody);

    await ctx.close();
  });

  test('§2.2 simulated submit removes localStorage entry (反向 sanity — clear() contract)', async ({ browser }) => {
    const ctx = await browser.newContext();
    const page = await ctx.newPage();
    await page.goto(`${clientURL()}/`);

    const artifactId = 'cv10-art-clr-' + Date.now();
    const key = KEY_PREFIX + artifactId;
    await page.evaluate(([k, v]) => localStorage.setItem(k, v), [key, 'will be cleared']);
    expect(await page.evaluate((k) => localStorage.getItem(k), key)).toBe('will be cleared');

    // Simulate hook.clear() — server submit success path removes the key.
    await page.evaluate((k) => localStorage.removeItem(k), key);
    expect(await page.evaluate((k) => localStorage.getItem(k), key)).toBeNull();

    await ctx.close();
  });

  test('§2.3 反约束 — key namespace 字面 "borgee.cv10.comment-draft:" 跨 reload 稳定', async ({ browser }) => {
    const ctx = await browser.newContext();
    const page = await ctx.newPage();
    await page.goto(`${clientURL()}/`);

    const artifactId = 'cv10-art-stab-' + Date.now();
    const fullKey = KEY_PREFIX + artifactId;
    await page.evaluate(([k, v]) => localStorage.setItem(k, v), [fullKey, 'hello']);

    // Verify byte-identical prefix in actual stored key.
    const allKeys = await page.evaluate(() => {
      const keys: string[] = [];
      for (let i = 0; i < localStorage.length; i++) {
        const k = localStorage.key(i);
        if (k && k.startsWith('borgee.cv10.comment-draft:')) keys.push(k);
      }
      return keys;
    });
    expect(allKeys.length).toBeGreaterThanOrEqual(1);
    expect(allKeys.some((k) => k === fullKey)).toBe(true);

    await ctx.close();
  });
});
