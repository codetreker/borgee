// tests/comment-search-filter.spec.ts — comment keyword search + cross-channel access control.
//
// Status: skipped with follow-up work tracked in gh#716 + gh#724 §1.
//
// Skip reason: ArtifactCommentSearchBox currently has no production mount in the client SPA,
// so the real UI path is unreachable. This spec currently calls the backend through REST,
// which makes it a backend contract test rather than an e2e test. After the v2 ArtifactComments mount lands,
// unskip and convert it to page.fill plus DOM assertions on the results list.
//
// 3 cases to verify after v2 unskip:
//   - Seed 3 messages → search "needle" → 1 hit
//   - 搜 "absent-xyz" → 0 hit
//   - Cross-channel non-member search → 403
//
// Related docs:
//   - Acceptance: docs/_archive/qa/acceptance-templates/cv-12.md §3
//   - Follow-up: gh#724 §1 (mount)
//
// Implementation constraints after unskip:
//   - Browser-driven UI path (search input + DOM assertions on results).
//   - Do not use fs.*, page.evaluate(fetch), API-only checks, or empty placeholder tests.
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
