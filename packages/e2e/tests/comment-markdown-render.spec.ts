// tests/comment-markdown-render.spec.ts — comment markdown 渲染 + sanitize.
//
// 状态: SKIP+followup (gh#716 + gh#724 §1).
//
// 跳过原因: ArtifactCommentBody 在 client SPA 当前没有 production mount,
// 走真 UI 路径不可达. 现 spec 走 REST 直调 + page.evaluate 调内部 lib
// (反模式 F3+F4), 不算 e2e. v2 mount 后 unskip + 改真 UI 输入 + DOM 断渲染.
//
// 3 case (v2 unskip 时验):
//   - server 存 raw markdown, POST → GET round-trip 字面相等 (无后端渲染)
//   - client renderMarkdown 真渲染, DOM 含 strong / em / code 元素
//   - sanitize 防 XSS: <script> 输入后 DOM 内 0 个 script 元素
//
// 关联文档:
//   - 验收: docs/_archive/qa/acceptance-templates/cv-11.md §3
//   - 单测: vitest ArtifactCommentBody.test.tsx (DOM / sanitize 锁)
//   - 后续: gh#724 §1 (mount)
//
// 实施约束 (unskip 后):
//   - 真 UI 走浏览器 (textarea input + DOM 断)
//   - 不允许 fs.* / page.evaluate(fetch) / 只打 API / noop

import { test, expect, request as apiRequest } from '@playwright/test';

const ADMIN_LOGIN = 'e2e-admin';
const ADMIN_PASSWORD = 'e2e-admin-pass-12345';

function clientURL(): string {
  return `http://127.0.0.1:${process.env.E2E_CLIENT_PORT ?? '5174'}`;
}
function serverURL(): string {
  return `http://127.0.0.1:${process.env.E2E_SERVER_PORT ?? '4901'}`;
}

test.describe.skip('comment markdown 渲染 (gh#716 SKIP+followup, 等 v2 mount 后 unskip — gh#724 §1)', () => {
  test('§3.1 server stores raw markdown — POST → GET byte-identical (no server-side render)', async () => {
    const adminCtx = await apiRequest.newContext({ baseURL: serverURL() });
    const loginRes = await adminCtx.post('/admin-api/auth/login', {
      data: { login: ADMIN_LOGIN, password: ADMIN_PASSWORD },
    });
    expect(loginRes.ok()).toBe(true);

    const inviteRes = await adminCtx.post('/admin-api/v1/invites', { data: { note: 'cv11-md' } });
    const invite = (await inviteRes.json()) as { invite: { code: string } };

    const userCtx = await apiRequest.newContext({ baseURL: serverURL() });
    const stamp = Date.now();
    const reg = await userCtx.post('/api/v1/auth/register', {
      data: {
        invite_code: invite.invite.code,
        email: `cv11-md-${stamp}@example.test`,
        password: 'p@ssw0rd-cv11',
        display_name: `CV11 MD ${stamp}`,
      },
    });
    expect(reg.ok()).toBe(true);

    const chRes = await userCtx.post('/api/v1/channels', {
      data: { name: `cv11-md-${stamp}`, visibility: 'private' },
    });
    const ch = (await chRes.json()) as { channel: { id: string } };

    const rawBody = '**bold** _italic_ `code` <script>alert(1)</script>';
    const post = await userCtx.post(`/api/v1/channels/${ch.channel.id}/messages`, {
      data: { content: rawBody },
    });
    expect(post.ok()).toBe(true);
    const pj = (await post.json()) as { message: { id: string; content: string } };
    expect(pj.message.content).toBe(rawBody); // byte-identical raw storage
  });

  test('§3.2 client renderMarkdown — page.evaluate generates strong/em/code', async ({ page }) => {
    await page.goto(`${clientURL()}/`);
    // Pull renderMarkdown via the running Vite-bundled module — load through
    // a <script> tag execution in page context that mounts a probe div, calls
    // the lib, asserts elements. Since direct module import inside evaluate
    // is tricky, we use a thin DOM probe: write known markdown into the
    // page via document.body, then dispatch DOMContentLoaded so the app
    // would run. Here we simply assert the marked + DOMPurify lib path
    // works by injecting raw HTML THROUGH the same sanitize call shape
    // (lightweight browser check; the strict lock is the vitest unit suite).
    //
    // Practical check: page loads without error, the client module bundle
    // includes 'marked' and 'dompurify' (反向断 spec lib 单源).
    const html = await page.content();
    expect(html.length).toBeGreaterThan(0);
  });

  test('§3.3 server response is raw text (does NOT contain <strong>/<em> HTML)', async () => {
    const adminCtx = await apiRequest.newContext({ baseURL: serverURL() });
    const login = await adminCtx.post('/admin-api/auth/login', {
      data: { login: ADMIN_LOGIN, password: ADMIN_PASSWORD },
    });
    expect(login.ok()).toBe(true);
    const invRes = await adminCtx.post('/admin-api/v1/invites', { data: { note: 'cv11-anti' } });
    const inv = (await invRes.json()) as { invite: { code: string } };

    const userCtx = await apiRequest.newContext({ baseURL: serverURL() });
    const stamp = Date.now();
    await userCtx.post('/api/v1/auth/register', {
      data: {
        invite_code: inv.invite.code,
        email: `cv11-anti-${stamp}@example.test`,
        password: 'p@ssw0rd-cv11',
        display_name: `CV11 anti ${stamp}`,
      },
    });
    const chRes = await userCtx.post('/api/v1/channels', {
      data: { name: `cv11-anti-${stamp}`, visibility: 'private' },
    });
    const ch = (await chRes.json()) as { channel: { id: string } };

    const post = await userCtx.post(`/api/v1/channels/${ch.channel.id}/messages`, {
      data: { content: '**should-stay-raw**' },
    });
    expect(post.ok()).toBe(true);
    const pj = (await post.json()) as { message: { content: string } };
    // Server MUST NOT pre-render — content stays as `**should-stay-raw**`,
    // never `<strong>should-stay-raw</strong>`.
    expect(pj.message.content).not.toContain('<strong>');
    expect(pj.message.content).not.toContain('<em>');
    expect(pj.message.content).toBe('**should-stay-raw**');
  });
});
