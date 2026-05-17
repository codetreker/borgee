// tests/artifact-renderer-types.spec.ts — artifact three-kind rendering + XSS defense + G3.4 demo screenshot.
//
// Test scope:
//   - code artifact (Go) renders the Prism syntax-highlight class.
//   - image_link artifact renders <img loading="lazy"> for an https URL.
//   - URL protocol rejection: javascript: / data: / http:.
//   - mention preview kind has three modes, with expansion waiting for the list endpoint.
//   - G3.4 demo screenshot archive; markdown path is the baseline until code/image_link can use the list endpoint.
//
// Related docs:
//   - Acceptance: docs/_archive/qa/acceptance-templates/cv-3.md §3.1-§3.4
//   - Negative checks: image src https only / link rel="noopener noreferrer" three-part lock (XSS boundary)
//
// Implementation constraints:
//   - Browser-driven UI path: page.goto, create artifact through the UI, and assert DOM.
//   - Markdown uses UI creation because panel defaults to type=markdown; code/image_link use direct REST but panel v1 does not render them.
//   - Renderer correctness is covered by Vitest (CodeRenderer / ImageLinkRenderer / MentionArtifactPreview / ArtifactPanel-kind-switch).
//   - Server protocol rejection uses REST with javascript:/data:/http: and expects 400 (CV-3.2 ValidateImageLinkURL).
//   - Do not use fs.*, page.evaluate(fetch), API-only checks, or empty placeholder tests.
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
 * Same pattern as CV-1.3 cv-1-3-canvas.spec.ts::createArtifactViaUI: the UI
 * creation path defaults to type='markdown' (server CV-3.2 default, #400
 * byte-identical). After creation, panel local state holds the artifact; commit
 * once more with a markdown code block to exercise embedded-code rendering.
 */
async function createArtifactViaUI(page: Page, title: string): Promise<string> {
  // gh#691: creation path moved to the in-app modal. Guard with a flag and final assertion.
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
    // Server-side guard, independent of UI rendering. Send the three rejected
    // protocols through REST and expect all to return 400.
    // Same source as CV-3.2 #400 ValidateImageLinkURL: server-side https-only lock.
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

    // Sanity check: https URL passes.
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
    test.skip(
      process.env.E2E_EVIDENCE_SCREENSHOTS !== '1',
      'signoff screenshot archive runs only when E2E_EVIDENCE_SCREENSHOTS=1',
    );

    // ArtifactPanel v1 only renders the artifact created in the user's UI session
    // (CV-1.3 spec §3 literal; no list endpoint). The markdown path uses UI creation
    // plus a commit body containing a code block, which exercises the existing
    // markdown.ts hljs path. That path coexists with the CV-3.3 Prism CodeRenderer:
    // lib/markdown.ts supports code blocks inside markdown, while CodeRenderer is
    // the separate artifact-kind=code path.
    //
    // Note: g3.4-cv3-{code-go-highlight,image-embed} screenshots wait for the
    // list endpoint (CV-5+) before switching to the real path. This PR only emits
    // the markdown screenshot as the CV-3 three-state baseline.
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

    // Commit body contains a markdown code block for the CV-3 spec §0 ② three-renderer breadth demo.
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

    // Trigger panel reload and wait for v2 to render.
    await expect(page.locator('.artifact-version-tag')).toHaveText('v2', { timeout: 10_000 });

    // DOM `data-artifact-kind="markdown"` stays byte-identical (CV-3.3 §2.1 lock).
    // ArtifactPanel wrapper and ArtifactBody both carry this attr. Grep count is
    // 3: markdown has two layers, code/image_link have one each; this check only
    // needs the outer wrapper.
    await expect(page.locator('.artifact-panel[data-artifact-kind="markdown"]')).toBeVisible();

    // §3.4 G3.4 demo screenshot: markdown baseline.
    if (process.env.E2E_EVIDENCE_SCREENSHOTS === '1') await page.screenshot({
      path: path.join(SCREENSHOT_DIR, 'g3.4-cv3-markdown.png'),
      fullPage: false,
    });
  });
});
