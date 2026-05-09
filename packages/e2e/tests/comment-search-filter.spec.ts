// tests/comment-search-filter.spec.ts — comment 搜索关键字过滤 + 跨频道防越权.
//
// 状态: SKIP+followup (gh#716 + gh#724 §1).
//
// 跳过原因: ArtifactCommentSearchBox 在 client SPA 当前没有 production
// mount, 走真 UI 路径不可达. 现 spec 走 REST 直调后端 (反模式 F3),
// 不算 e2e. v2 ArtifactComments mount 落地后 unskip + 改 page.fill +
// DOM 断结果列表.
//
// 3 case (v2 unskip 时验):
//   - seed 3 条消息 → 搜 "needle" → 1 hit
//   - 搜 "absent-xyz" → 0 hit
//   - 跨频道非成员搜索 → 403
//
// 关联文档:
//   - 验收: docs/_archive/qa/acceptance-templates/cv-12.md §3
//   - 后续: gh#724 §1 (mount)
//
// 实施约束 (unskip 后):
//   - 真 UI 走浏览器 (search input + DOM 断结果)
//   - 不允许 fs.* / page.evaluate(fetch) / 只打 API / noop
import { test, expect, request as apiRequest, type APIRequestContext } from '@playwright/test';

const ADMIN_LOGIN = 'e2e-admin';
const ADMIN_PASSWORD = 'e2e-admin-pass-12345';

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
  expect(res.ok()).toBe(true);
  return ctx;
}

async function mintInvite(adminCtx: APIRequestContext, note: string): Promise<string> {
  const res = await adminCtx.post('/admin-api/v1/invites', { data: { note } });
  expect(res.ok()).toBe(true);
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
  const email = `cv12-${suffix}-${stamp}-${Math.floor(Math.random() * 10000)}@example.test`;
  const password = 'p@ssw0rd-cv12';
  const res = await ctx.post('/api/v1/auth/register', {
    data: { invite_code: inviteCode, email, password, display_name: `CV12 ${suffix} ${stamp}` },
  });
  expect(res.ok(), `register: ${res.status()} ${await res.text()}`).toBe(true);
  const body = (await res.json()) as { user: { id: string } };
  const cookies = await ctx.storageState();
  const tok = cookies.cookies.find((c) => c.name === 'borgee_token');
  return { email, token: tok!.value, userId: body.user.id, ctx };
}

async function createChannel(user: RegisteredUser, name: string): Promise<string> {
  const r = await user.ctx.post('/api/v1/channels', {
    data: { name, visibility: 'private' },
  });
  expect(r.ok()).toBe(true);
  const j = (await r.json()) as { channel: { id: string } };
  return j.channel.id;
}

async function postMessage(user: RegisteredUser, channelId: string, content: string): Promise<void> {
  const r = await user.ctx.post(`/api/v1/channels/${channelId}/messages`, {
    data: { content },
  });
  expect(r.ok()).toBe(true);
}

function serverURL(): string {
  return `http://127.0.0.1:${process.env.E2E_SERVER_PORT ?? '4901'}`;
}

test.describe.skip('comment 搜索过滤 (gh#716 SKIP+followup, 等 v2 mount 后 unskip — gh#724 §1)', () => {
  test('§3.1 seed 3 messages → search "needle" → 1 hit', async () => {
    const adminCtx = await adminLogin(serverURL());
    const inv = await mintInvite(adminCtx, 'cv12-hit');
    const owner = await registerUser(serverURL(), inv, 'hit');
    const chId = await createChannel(owner, `cv12-hit-${Date.now()}`);

    await postMessage(owner, chId, 'first review note about lock TTL');
    await postMessage(owner, chId, 'this contains the needle keyword for search');
    await postMessage(owner, chId, 'third comment unrelated');

    const r = await owner.ctx.get(`/api/v1/channels/${chId}/messages/search?q=needle`);
    expect(r.ok(), await r.text()).toBe(true);
    const j = (await r.json()) as { messages: Array<{ content: string }> };
    expect(j.messages.length).toBe(1);
    expect(j.messages[0].content).toContain('needle');
  });

  test('§3.2 search "absent-xyz" → 0 hit', async () => {
    const adminCtx = await adminLogin(serverURL());
    const inv = await mintInvite(adminCtx, 'cv12-no');
    const owner = await registerUser(serverURL(), inv, 'no');
    const chId = await createChannel(owner, `cv12-no-${Date.now()}`);
    await postMessage(owner, chId, 'sample comment');

    const r = await owner.ctx.get(`/api/v1/channels/${chId}/messages/search?q=absent-xyz-not-real`);
    expect(r.ok()).toBe(true);
    const j = (await r.json()) as { messages: any[] };
    expect(j.messages.length).toBe(0);
  });

  test('§3.3 cross-channel reject — non-member 不能 search (fail-closed 404/403)', async () => {
    const adminCtx = await adminLogin(serverURL());
    const ownerInv = await mintInvite(adminCtx, 'cv12-x-owner');
    const owner = await registerUser(serverURL(), ownerInv, 'x-owner');
    const otherInv = await mintInvite(adminCtx, 'cv12-x-other');
    const other = await registerUser(serverURL(), otherInv, 'x-other');

    const chId = await createChannel(owner, `cv12-x-${Date.now()}`);
    await postMessage(owner, chId, 'private comment');

    const r = await other.ctx.get(`/api/v1/channels/${chId}/messages/search?q=private`);
    // Private channel hidden from non-member: 404 or 403 — both fail-closed.
    expect([403, 404]).toContain(r.status());
  });
});
