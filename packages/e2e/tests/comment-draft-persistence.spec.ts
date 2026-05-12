// tests/comment-draft-persistence.spec.ts — comment 草稿 localStorage 持久化 + 离开提醒.
//
// 状态: SKIP+followup (gh#716 + gh#724 §1).
//
// 跳过原因: ArtifactCommentDraftInput 在 client SPA 当前没有 production
// mount, 走真 UI 路径不可达. 现 spec 走 page.evaluate(localStorage)
// 模拟浏览器层 unsaved-state, 不是真 UI input/click 路径 (反模式 F2 边界).
// v2 ArtifactComments mount 落地后 unskip + 改真 textarea input + reload
// + DOM 断.
//
// 3 case (v2 unskip 时验):
//   - 输入 → reload → localStorage 仍持有 draft
//   - submit → localStorage 清空
//   - 含未保存草稿离开页面: 单测层覆盖 prompt, e2e 仅断 key 仍存在
//
// 关联文档:
//   - 验收: docs/_archive/qa/acceptance-templates/cv-10.md §2
//   - 单测: vitest ArtifactCommentDraftInput.test.tsx
//   - 后续: gh#724 §1 (mount)
//
// 实施约束 (unskip 后):
//   - 真 textarea input + reload + DOM 断
//   - 不允许 fs.* / page.evaluate(fetch) / 只打 API / noop

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
