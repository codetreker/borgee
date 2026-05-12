// tests/comment-draft-persistence.spec.ts — comment draft localStorage persistence + leave warning.
//
// Status: skipped with follow-up work tracked in gh#716 + gh#724 §1.
//
// Skip reason: ArtifactCommentDraftInput currently has no production mount in the client SPA,
// so the real UI path is unreachable. This spec currently uses page.evaluate(localStorage)
// to simulate browser unsaved-state behavior, which is not a real UI input/click path.
// After the v2 ArtifactComments mount lands, unskip and convert it to real textarea input + reload + DOM assertions.
//
// 3 cases to verify after v2 unskip:
//   - Input → reload → localStorage still contains draft
//   - submit → localStorage 清空
//   - Leaving with an unsaved draft: unit tests cover the prompt; e2e only asserts the key still exists
//
// Related docs:
//   - Acceptance: docs/_archive/qa/acceptance-templates/cv-10.md §2
//   - Unit test: vitest ArtifactCommentDraftInput.test.tsx
//   - Follow-up: gh#724 §1 (mount)
//
// Implementation constraints after unskip:
//   - Real textarea input + reload + DOM assertions.
//   - Do not use fs.*, page.evaluate(fetch), API-only checks, or empty placeholder tests.

import { test, expect } from '@playwright/test';

const KEY_PREFIX = 'borgee.cv10.comment-draft:';

function clientURL(): string {
  return `http://127.0.0.1:${process.env.E2E_CLIENT_PORT ?? '5174'}`;
}

test.describe.skip('comment 草稿持久化 (gh#716 SKIP+followup, 等 v2 mount 后 unskip — gh#724 §1)', () => {
  test('§2.1 type → reload → localStorage 持有 draft (key namespace byte-identical)', async ({ browser }) => {
    const ctx = await browser.newContext();
    const page = await ctx.newPage();

    await page.goto(`${clientURL()}/`);

    const artifactId = 'cv10-art-' + Date.now();
    const key = KEY_PREFIX + artifactId;
    const draftBody = 'unsaved review of section 2 lock TTL';

    // Simulate hook write (CV-10 hook writes localStorage with this exact
    // key namespace remains byte-identical with cv-10-content-lock §3).
    await page.evaluate(([k, v]) => {
      localStorage.setItem(k, v);
    }, [key, draftBody]);

    // Reload — localStorage persists across reload.
    await page.reload();
    const restored = await page.evaluate((k) => localStorage.getItem(k), key);
    expect(restored).toBe(draftBody);

    await ctx.close();
  });

  test('§2.2 simulated submit removes localStorage entry (clear() contract check)', async ({ browser }) => {
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

  test('§2.3 key namespace "borgee.cv10.comment-draft:" stays stable across reload', async ({ browser }) => {
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
