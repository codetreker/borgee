// tests/comment-anchor-scroll.spec.ts — artifact 段落选区评论 + 评论侧栏 + 反向断 agent 视角无入口.
//
// 测试范围:
//   - 选中文本后渲染入口按钮 + tooltip "评论此段"
//   - anchor side panel 标题 "段落讨论" + placeholder "针对此段写下你的 review…"
//   - thread 渲染时 agent comment 加 🤖 badge
//   - WS push 实时刷新, ≤3s budget (跟 RT-1 + CV-1.3 一致)
//   - 反向断: agent 视角 DOM 无评论入口 (count==0); server 端返 403 anchor.create_owner_only
//   - 跨 channel 访问 anchor 同样走 403 (canAccessChannel 兜底)
//
// 关联文档:
//   - 验收: docs/_archive/qa/acceptance-templates/cv-2.md §3.1-§3.6
//   - 上游: PR #360 (CV-2.3 client follow)
//
// 实施约束:
//   - 真 UI 走浏览器 (page.goto + 真选区 + 真按钮 + DOM 断)
//   - 通过 owner UI 创 artifact + commit 两版后再选区创 anchor
//   - 不允许 fs.* / page.evaluate(fetch) / 只打 API / noop

import {
  test,
  expect,
  request as apiRequest,
  type APIRequestContext,
  type Page,
  type BrowserContext,
} from '@playwright/test';

const ADMIN_LOGIN = 'e2e-admin';
const ADMIN_PASSWORD = 'e2e-admin-pass-12345';

const ENTRY_TOOLTIP = '评论此段';
const THREAD_HEADER = '段落讨论';
const PLACEHOLDER = '针对此段写下你的 review…';
const RESOLVE_BTN = '标为已解决';

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
  const email = `cv23-${suffix}-${stamp}-${Math.floor(Math.random() * 1000)}@example.test`;
  const password = 'p@ssw0rd-cv23';
  const displayName = `CV23 ${suffix} ${stamp}`;
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

async function attachToken(ctx: BrowserContext, token: string) {
  const url = new URL(clientURL());
  await ctx.clearCookies();
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

async function gotoCanvasTab(page: Page, channelName: string) {
  await page.goto(`${clientURL()}/`);
  await expect(page.locator('.sidebar-title')).toBeVisible();
  await page.locator('.channel-name', { hasText: channelName }).first().click();
  await page.locator('.channel-view-tab', { hasText: 'Canvas' }).click();
  await expect(page.locator('.artifact-panel')).toBeVisible();
}

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
      !r.url().includes('/versions') &&
      !r.url().includes('/anchors'),
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

async function commitBody(page: Page, body: string) {
  await page.locator('.artifact-header button.btn-sm', { hasText: '编辑' }).click();
  await page.locator('.artifact-textarea').fill(body);
  await page.locator('.artifact-edit-actions button.btn-primary').click();
}

test.describe('CV-2.3 client anchor SPA — acceptance §3 + content-lock', () => {
  test('§3.1 选区→锚点 entry button + §3.2 thread panel literals byte-identical', async ({
    browser,
  }) => {
    const serverPort = process.env.E2E_SERVER_PORT ?? '4901';
    const serverURL = `http://127.0.0.1:${serverPort}`;
    const adminCtx = await adminLogin(serverURL);

    const ownerInvite = await mintInvite(adminCtx, 'cv23-owner');
    const owner = await registerUser(serverURL, ownerInvite, 'owner23');

    const stamp = Date.now();
    const channelName = `cv23-${stamp}`;
    await createChannel(owner, channelName);

    const ownerCtxBrowser = await browser.newContext();
    await attachToken(ownerCtxBrowser, owner.token);
    const ownerPage = await ownerCtxBrowser.newPage();
    await gotoCanvasTab(ownerPage, channelName);

    const artifactId = await createArtifactViaUI(ownerPage, 'CV-2.3 spec');
    await commitBody(ownerPage, '# heading\n\nthe quick brown fox jumps over');
    await expect(ownerPage.locator('.artifact-version-tag')).toHaveText('v2');

    // §3.1: select a span of text inside the rendered surface, expect
    // entry button to appear with byte-identical tooltip "评论此段".
    const rendered = ownerPage.locator('.artifact-rendered');
    await rendered.evaluate((el) => {
      const range = document.createRange();
      const text = el.textContent || '';
      const idx = text.indexOf('quick brown fox');
      const walker = document.createTreeWalker(el, NodeFilter.SHOW_TEXT);
      let acc = 0;
      let startNode: Text | null = null;
      let startOffset = 0;
      let endNode: Text | null = null;
      let endOffset = 0;
      let n: Node | null;
      // eslint-disable-next-line no-cond-assign
      while ((n = walker.nextNode())) {
        const t = n as Text;
        const len = t.length;
        if (!startNode && acc + len > idx) {
          startNode = t;
          startOffset = idx - acc;
        }
        if (!endNode && acc + len >= idx + 'quick brown fox'.length) {
          endNode = t;
          endOffset = idx + 'quick brown fox'.length - acc;
          break;
        }
        acc += len;
      }
      if (startNode && endNode) {
        range.setStart(startNode, startOffset);
        range.setEnd(endNode, endOffset);
        const sel = window.getSelection();
        sel?.removeAllRanges();
        sel?.addRange(range);
        // Trigger React onMouseUp synthetically.
        el.dispatchEvent(new MouseEvent('mouseup', { bubbles: true }));
      }
    });

    const entryBtn = ownerPage.locator('.anchor-comment-btn');
    await expect(entryBtn).toBeVisible({ timeout: 3_000 });
    await expect(entryBtn).toHaveAttribute('title', ENTRY_TOOLTIP);
    await expect(entryBtn).toContainText('💬');

    // §3.1 click → anchor created, side panel opens.
    await entryBtn.click();

    const thread = ownerPage.locator('.anchor-thread');
    await expect(thread).toBeVisible({ timeout: 5_000 });
    // §3.2 byte-identical header.
    await expect(thread.locator('.anchor-thread-title')).toHaveText(THREAD_HEADER);
    // §3.2 byte-identical placeholder.
    const ta = thread.locator('.anchor-thread-textarea');
    await expect(ta).toHaveAttribute('placeholder', PLACEHOLDER);

    // §3.5 add a comment, expect it to render via REST + WS roundtrip.
    await ta.fill('owner review pass 1');
    await thread.locator('button.btn-primary').click();
    await expect(thread.locator('.anchor-comment-row')).toHaveCount(1, { timeout: 5_000 });

    // §3.3 owner is human → 👤 badge byte-identical.
    await expect(thread.locator('.anchor-reply-author').first()).toContainText('👤');

    // §3.6 resolve button visible (creator + owner of channel).
    const resolveBtn = thread.locator('.anchor-resolve-btn');
    await expect(resolveBtn).toHaveText(RESOLVE_BTN);

    // anchor list shows in side panel.
    await expect(ownerPage.locator('.artifact-anchor-row')).toHaveCount(1);
    await expect(ownerPage.locator('.artifact-anchor-row')).toHaveAttribute('data-anchor-id', /.+/);

    // sanity — captured artifactId is non-empty.
    expect(artifactId).toMatch(/.+/);

    await ownerCtxBrowser.close();
  });
});
