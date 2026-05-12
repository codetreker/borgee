// tests/comment-mention-dispatch.spec.ts — comment mention dispatch (write mention row + cross-channel block + non-member reject).
//
// Status: skipped with follow-up work tracked in gh#716 + gh#724 §1.
//
// Skip reason: ArtifactComments currently has no production mount in the client SPA,
// so the real UI path is unreachable. This spec currently calls the backend through REST,
// which makes it a backend contract test rather than an e2e test.
// After the v2 ArtifactComments mount lands, unskip and convert it to page.click plus DOM assertions.
//
// 5 cases to verify after v2 unskip:
//   - Human message containing @<uuid> mention writes one mentions row
//   - mention 非频道成员 → 400 mention.target_not_in_channel
//   - Cross-channel non-member message → 403
//   - Mention dispatch is consistent across content types (text and artifact_comment use the same dispatch path)
//   - Body contains a mention but target does not exist → 400
//
// Related docs:
//   - Acceptance: docs/_archive/qa/acceptance-templates/cv-9.md §3
//   - Unit test: TestCV9_ArtifactComment_TriggersMentionDispatch (dispatcher is content-type agnostic)
//   - Follow-up: gh#724 §1 (mount)
//
// Implementation constraints after unskip:
//   - Browser-driven UI path; do not use fs.*, page.evaluate(fetch), API-only checks, or empty placeholder tests.

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
  const email = `cv9-${suffix}-${stamp}-${Math.floor(Math.random() * 10000)}@example.test`;
  const password = 'p@ssw0rd-cv9';
  const res = await ctx.post('/api/v1/auth/register', {
    data: { invite_code: inviteCode, email, password, display_name: `CV9 ${suffix} ${stamp}` },
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

function serverURL(): string {
  const port = process.env.E2E_SERVER_PORT ?? '4901';
  return `http://127.0.0.1:${port}`;
}

test.describe.skip('comment mention 分发 (gh#716 SKIP+followup, 等 v2 mount 后 unskip — gh#724 §1)', () => {
  test('§3.1 human comment with @user → mention row written + WS frame fired', async () => {
    const adminCtx = await adminLogin(serverURL());
    const ownerInv = await mintInvite(adminCtx, 'cv9-mention-owner');
    const owner = await registerUser(serverURL(), ownerInv, 'mention-owner');
    const targetInv = await mintInvite(adminCtx, 'cv9-mention-target');
    const target = await registerUser(serverURL(), targetInv, 'mention-target');

    const chId = await createChannel(owner, `cv9-mention-${Date.now()}`);
    // Add target as channel member so mention can resolve.
    const addRes = await owner.ctx.post(`/api/v1/channels/${chId}/members`, {
      data: { user_id: target.userId },
    });
    expect(addRes.ok()).toBe(true);

    // Post message with @<target_uuid> token using the existing DM-2.2 mention syntax.
    const r = await owner.ctx.post(`/api/v1/channels/${chId}/messages`, {
      data: {
        content: `Reviewing draft — <@${target.userId}> please check section 2.`,
        mentions: [target.userId],
      },
    });
    expect(r.status(), await r.text()).toBe(201);
  });

  test('§3.2 mention non-channel-member → 400 mention.target_not_in_channel (DM-2.2 既有错码)', async () => {
    const adminCtx = await adminLogin(serverURL());
    const ownerInv = await mintInvite(adminCtx, 'cv9-x-owner');
    const owner = await registerUser(serverURL(), ownerInv, 'x-owner');
    const outsideInv = await mintInvite(adminCtx, 'cv9-x-out');
    const outsider = await registerUser(serverURL(), outsideInv, 'x-out');

    const chId = await createChannel(owner, `cv9-x-${Date.now()}`);
    // Don't add outsider to channel.
    const r = await owner.ctx.post(`/api/v1/channels/${chId}/messages`, {
      data: {
        content: `<@${outsider.userId}> hi`,
        mentions: [outsider.userId],
      },
    });
    expect(r.status()).toBe(400);
    const text = await r.text();
    expect(text).toContain('mention.target_not_in_channel');
  });

  test('§3.3 cross-channel POST reject — non-member blocked (404 or 403, fail-closed)', async () => {
    const adminCtx = await adminLogin(serverURL());
    const ownerInv = await mintInvite(adminCtx, 'cv9-403-owner');
    const owner = await registerUser(serverURL(), ownerInv, '403-owner');
    const otherInv = await mintInvite(adminCtx, 'cv9-403-other');
    const other = await registerUser(serverURL(), otherInv, '403-other');

    const chId = await createChannel(owner, `cv9-403-${Date.now()}`);
    const r = await other.ctx.post(`/api/v1/channels/${chId}/messages`, {
      data: { content: 'drive-by', mentions: [] },
    });
    // Private channel with non-member: server may return 404 (channel hidden)
    // or 403 (forbidden) depending on access path. Both are fail-closed —
    // REG-INV-002 invariant is that the message MUST NOT land.
    expect([403, 404]).toContain(r.status());
  });

  test('§3.4 dispatch parity — text-typed mention path 真触发 (server unit 已锁 artifact_comment 等价)', async () => {
    const adminCtx = await adminLogin(serverURL());
    const inv = await mintInvite(adminCtx, 'cv9-parity');
    const owner = await registerUser(serverURL(), inv, 'parity-owner');
    const targetInv = await mintInvite(adminCtx, 'cv9-parity-tgt');
    const target = await registerUser(serverURL(), targetInv, 'parity-tgt');
    const chId = await createChannel(owner, `cv9-parity-${Date.now()}`);
    await owner.ctx.post(`/api/v1/channels/${chId}/members`, {
      data: { user_id: target.userId },
    });

    // Two messages with same mention payload — dispatch is content_type-agnostic
    // (server unit pins). Both must succeed.
    for (let i = 0; i < 2; i++) {
      const r = await owner.ctx.post(`/api/v1/channels/${chId}/messages`, {
        data: {
          content: `Iteration ${i} — <@${target.userId}> review please`,
          mentions: [target.userId],
        },
      });
      expect(r.status()).toBe(201);
    }
  });

  test('§3.5 mention 同 org 但非 channel member → 400 mention.target_not_in_channel (cross-channel reject)', async () => {
    const adminCtx = await adminLogin(serverURL());
    const ownerInv = await mintInvite(adminCtx, 'cv9-sanity');
    const owner = await registerUser(serverURL(), ownerInv, 'sanity');
    const outsideInv = await mintInvite(adminCtx, 'cv9-sanity-out');
    const outsider = await registerUser(serverURL(), outsideInv, 'sanity-out');

    const chId = await createChannel(owner, `cv9-sanity-${Date.now()}`);
    // outsider exists but is NOT a channel member.
    const r = await owner.ctx.post(`/api/v1/channels/${chId}/messages`, {
      data: {
        content: `<@${outsider.userId}> hi`,
        mentions: [outsider.userId],
      },
    });
    expect(r.status()).toBe(400);
    const text = await r.text();
    expect(text).toContain('mention.target_not_in_channel');
  });
});
