// tests/cv-3-3-renderers.spec.ts — CV-3.3 client kind renderers e2e.
//
// 闭环 cv-3.md acceptance §3.1-§3.4:
//   §3.1 code artifact (Go) → prism syntax highlight class hit
//   §3.2 image_link artifact (https URL) → <img loading="lazy"> + URL
//        协议反向 reject (javascript: / data: / http:)
//   §3.3 mention preview kind 三模式 (留 stub)
//   §3.4 G3.4 demo 截屏归档 (撑章程 Phase 3 退出公告)
//
// 立场反查 (cv-3-content-lock.md):
//   ① 三 enum DOM data-artifact-kind byte-identical
//   ④ image src https only (XSS 红线第一道)
//   ⑤ link rel="noopener noreferrer" 三联锁 (XSS 红线第二道)
//
// 实现说明: ArtifactPanel v1 没有 list endpoint (CV-1.3 #346 spec §3 字面),
// 只显示 user 当 session 创的 artifact. UI 路径仅创 markdown (handleCreate
// 默认 type='markdown'), code/image_link kind 走 REST 直接创但 panel 不渲染.
//   - 渲染层正确性: 走 vitest 146/146 全闭 (CodeRenderer / ImageLinkRenderer /
//     MentionArtifactPreview / ArtifactPanel-kind-switch DOM 字面锁)
//   - server 协议反向断言: 走 REST 直发 javascript:/data:/http: → 400
//     (XSS 红线第一道 server 端守, CV-3.2 #400 ValidateImageLinkURL 已闸)
//   - G3.4 demo 截屏: 走 markdown UI 创建路径 — markdown panel 渲染 byte-identical
//     代表 CV-3 三态收口的 baseline; code/image_link 截屏待 list endpoint
//     (CV-5+ 留账) 后切真路径.
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
  const email = `cv33-${suffix}-${stamp}-${Math.floor(Math.random() * 1000)}@example.test`;
  const password = 'p@ssw0rd-cv33';
  const displayName = `CV33 ${suffix} ${stamp}`;
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

async function createChannel(user: RegisteredUser, name: string): Promise<string> {
  const r = await user.ctx.post('/api/v1/channels', {
    data: { name, visibility: 'private' },
  });
  expect(r.ok(), `channel create: ${r.status()} ${await r.text()}`).toBe(true);
  const j = (await r.json()) as { channel: { id: string } };
  return j.channel.id;
}

async function gotoCanvasTab(page: Page, channelName: string): Promise<void> {
  await page.goto(`${clientURL()}/`);
  await expect(page.locator('.sidebar-title')).toBeVisible();
  await page.locator('.channel-name', { hasText: channelName }).first().click();
  await page.locator('.channel-view-tab', { hasText: 'Canvas' }).click();
  await expect(page.locator('.artifact-panel')).toBeVisible();
}

/**
 * Drive the empty-state create button on the owner's UI. Returns the
 * artifact id captured from the POST /channels/{id}/artifacts response.
 *
 * 跟 CV-1.3 cv-1-3-canvas.spec.ts::createArtifactViaUI 同模式 — UI 创 path
 * 默认走 type='markdown' (server CV-3.2 默认值, #400 byte-identical). 创建
 * 后 panel local state 持有 artifact, 再 commit 一次让 body 含 markdown
 * code block 触发 markdown 内嵌代码渲染.
 */
async function createArtifactViaUI(page: Page, title: string): Promise<string> {
  // gh#691: 创建路径改为应用内 modal. 守卫: 标志位 + 末尾断言.
  let nativeDialogTriggered = false;
  page.on('dialog', async (d) => {
    nativeDialogTriggered = true;
    await d.dismiss();
  });
  const respPromise = page.waitForResponse(
    (r) =>
      r.request().method() === 'POST' &&
      r.url().includes('/artifacts') &&
      !r.url().includes('/commits') &&
      !r.url().includes('/rollback') &&
      !r.url().includes('/versions'),
  );
  await page.locator('.artifact-empty button.btn-primary').click();
  const modal = page.locator('[data-testid="artifact-create-modal"]');
  await expect(modal).toBeVisible({ timeout: 3_000 });
  await modal.locator('input.input-field').fill(title);
  await modal.locator('button[type="submit"]').click();
  const resp = await respPromise;
  const j = (await resp.json()) as { id: string };
  await expect(page.locator('.artifact-version-tag')).toHaveText('v1', { timeout: 5_000 });
  expect(nativeDialogTriggered, 'gh#691 回归: 触发了浏览器原生 dialog').toBe(false);
  return j.id;
}

async function commitBody(
  user: RegisteredUser,
  artifactId: string,
  expectedVersion: number,
  body: string,
): Promise<void> {
  const r = await user.ctx.post(`/api/v1/artifacts/${artifactId}/commits`, {
    data: { expected_version: expectedVersion, body },
  });
  expect(r.ok(), `commit: ${r.status()}`).toBe(true);
}

test.describe('CV-3.3 client kind renderers — acceptance §3', () => {
  test('§3.2 image_link 协议反向 reject — javascript:/data:/http: 400 (XSS 红线第一道)', async () => {
    // server-side 守, 不依赖 UI 渲染. 走 REST 直发反约束三协议 → 全 400.
    // 跟 CV-3.2 #400 ValidateImageLinkURL 同源 (server 端 https only 锁).
    const serverPort = process.env.E2E_SERVER_PORT ?? '4901';
    const serverURL = `http://127.0.0.1:${serverPort}`;
    const adminCtx = await adminLogin(serverURL);
    const inv = await mintInvite(adminCtx, 'cv33-32');
    const owner = await registerUser(serverURL, inv, 'o32');

    const stamp = Date.now();
    const chName = `cv33-img-${stamp}`;
    const chId = await createChannel(owner, chName);

    for (const bad of [
      { url: 'javascript:alert(1)', label: 'javascript:' },
      { url: 'data:image/png;base64,AAAA', label: 'data:image' },
      { url: 'http://example.com/x.png', label: 'http:' },
    ]) {
      const r = await owner.ctx.post(`/api/v1/channels/${chId}/artifacts`, {
        data: {
          type: 'image_link',
          title: `bad-${bad.label}`,
          body: bad.url,
          metadata: { kind: 'image', url: bad.url },
        },
      });
      expect(r.status(), `${bad.label} should reject 400`).toBe(400);
    }

    // Sanity — https URL passes 201.
    const ok = await owner.ctx.post(`/api/v1/channels/${chId}/artifacts`, {
      data: {
        type: 'image_link',
        title: 'good-https',
        body: 'https://example.com/x.png',
        metadata: { kind: 'image', url: 'https://example.com/x.png' },
      },
    });
    const okStatus = ok.status();
    const okText = okStatus >= 400 ? await ok.text() : '';
    expect([200, 201], `https URL should pass; got ${okStatus} body=${okText}`).toContain(okStatus);
  });

  test('§3.4 G3.4 demo markdown 截屏 (panel render baseline 撑 Phase 3 退出公告)', async ({ browser }) => {
    // ArtifactPanel v1 仅渲染 user UI session 创的 artifact (CV-1.3 spec §3 字面,
    // 无 list endpoint). markdown 路径走 UI 创建 + commit body 含 code block 字符串
    // 触发 markdown.ts hljs 既有路径 (跟 CV-3.3 prism CodeRenderer 路径并存,
    // #338 cross-grep 反模式遵守: lib/markdown.ts 8 lang hljs 是 markdown 内代码块,
    // CodeRenderer 是 artifact-kind=code 独立路径).
    //
    // 注: g3.4-cv3-{code-go-highlight,image-embed} 截屏待 list endpoint (CV-5+ 留账)
    // 后切真路径; 当前 PR 仅出 markdown 截屏代表 CV-3 三态收口 baseline.
    const serverPort = process.env.E2E_SERVER_PORT ?? '4901';
    const serverURL = `http://127.0.0.1:${serverPort}`;
    const adminCtx = await adminLogin(serverURL);
    const inv = await mintInvite(adminCtx, 'cv33-34');
    const owner = await registerUser(serverURL, inv, 'o34');

    const stamp = Date.now();
    const chName = `cv33-md-${stamp}`;
    await createChannel(owner, chName);

    const ctx = await browser.newContext();
    await attachToken(ctx, owner.token);
    const page = await ctx.newPage();
    await gotoCanvasTab(page, chName);

    const artifactId = await createArtifactViaUI(page, 'CV-3 markdown demo');

    // commit body 含 markdown 代码块 (跟 CV-3 spec §0 ② 三 renderer 渲染广度 demo).
    const md = [
      '# CV-3 D-lite',
      '',
      '三 kind 收口: **markdown / code / image_link**',
      '',
      '- ① data-artifact-kind 三 enum DOM 锁',
      '- ② 11 项语言白名单',
      '- ③ XSS 红线两道闸',
      '',
      '```go',
      'package main',
      'func main() {',
      '  println("hello")',
      '}',
      '```',
      '',
    ].join('\n');
    await commitBody(owner, artifactId, 1, md);

    // 触发 panel reload — 等 v2 渲染.
    await expect(page.locator('.artifact-version-tag')).toHaveText('v2', { timeout: 10_000 });

    // 立场 ① — DOM `data-artifact-kind="markdown"` byte-identical (CV-3.3 §2.1 锁).
    // ArtifactPanel 顶层 wrapper + ArtifactBody div 都带此 attr (二处反约束 grep
    // count==3 — markdown 双层 + code/image_link 各一); 此处验最外层即可.
    await expect(page.locator('.artifact-panel[data-artifact-kind="markdown"]')).toBeVisible();

    // §3.4 G3.4 demo 截屏 — markdown baseline.
    await page.screenshot({
      path: path.join(SCREENSHOT_DIR, 'g3.4-cv3-markdown.png'),
      fullPage: false,
    });
  });
});
