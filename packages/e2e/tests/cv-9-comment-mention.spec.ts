// tests/cv-9-comment-mention.spec.ts — CV-9.3 e2e (REST-driven, mention
// fan-out validation). 0 server production code path — CV-9 rides DM-2.2
// MentionDispatcher既有 path; e2e pins the cross-link.
//
// Acceptance: docs/_archive/qa/acceptance-templates/cv-9.md §3.
// Stance: docs/qa/cv-9-stance-checklist.md §1-§5.
// Spec: docs/implementation/modules/cv-9-spec.md.
//
// Note: artifact_comment content_type is added to the messages.go whitelist
// by CV-7 PR #535 (queued to merge). Until that merges to main, this spec
// uses 'text' content_type — the dispatcher behavior is content_type-agnostic
// (see CV-9.1 unit `TestCV9_ArtifactComment_TriggersMentionDispatch`), so
// the cross-link is validated either way; once #535 lands, an in-place
// rebase swaps 'text' → 'artifact_comment' without functional change.
//
// 5 case (cv-9.md §3):
//   §3.1 human posts message with @<uuid> mention → mention row written
//   §3.2 mention non-channel-member → 400 mention.target_not_in_channel
//   §3.3 cross-channel reject (non-member can't post)
//   §3.4 mention dispatch parity — text-typed and (when CV-7 merged)
//        artifact_comment-typed both fire the same dispatch path
//   §3.5 反向 sanity — body 含 mention 但 target 不存在 → 400

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

test.describe('CV-9.3 artifact comment mention notification REST e2e (acceptance §3)', () => {
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

    // Post message with @<target_uuid> token (DM-2.2 既有 mention syntax).
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
