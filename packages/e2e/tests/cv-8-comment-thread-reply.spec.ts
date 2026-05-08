// tests/cv-8-comment-thread-reply.spec.ts — CV-8.3 e2e (REST-driven, mirrors
// CV-5 / CV-7 e2e pattern).
//
// Acceptance: docs/_archive/qa/acceptance-templates/cv-8.md §3.
// Stance: docs/qa/cv-8-stance-checklist.md §1-§5.
// Spec: docs/implementation/modules/cv-8-spec.md.
//
// 6 case (cv-8.md §3):
//   §3.1 human reply on artifact_comment-typed parent — POST 200 + reply_to_id 链接
//   §3.2 agent reply thinking 4-pattern reject (5-pattern 第 6 处链 byte-identical CV-5/CV-7)
//   §3.3 reply on reply (depth 2) → 400 `comment.thread_depth_exceeded`
//   §3.4 reply on plain-text message → 400 `comment.reply_target_invalid`
//   §3.5 cross-channel reject — non-member → 403
//   §3.6 立场 ④ 反约束 — non-comment-typed message reply 不会创建出 thread (sanity 反向)

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
  const email = `cv8-${suffix}-${stamp}-${Math.floor(Math.random() * 10000)}@example.test`;
  const password = 'p@ssw0rd-cv8';
  const res = await ctx.post('/api/v1/auth/register', {
    data: { invite_code: inviteCode, email, password, display_name: `CV8 ${suffix} ${stamp}` },
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

async function postMessage(
  user: RegisteredUser,
  channelId: string,
  content: string,
  contentType?: string,
  replyToId?: string,
): Promise<{ id: string; status: number; data: any }> {
  const data: Record<string, any> = { content };
  if (contentType) data.content_type = contentType;
  if (replyToId) data.reply_to_id = replyToId;
  const r = await user.ctx.post(`/api/v1/channels/${channelId}/messages`, { data });
  const status = r.status();
  let json: any = {};
  try {
    json = await r.json();
  } catch {
    /* */
  }
  return { id: json?.message?.id ?? '', status, data: json };
}

function serverURL(): string {
  const port = process.env.E2E_SERVER_PORT ?? '4901';
  return `http://127.0.0.1:${port}`;
}

test.describe('CV-8.3 artifact comment thread reply REST e2e (acceptance §3)', () => {
  test('§3.1 human reply on artifact_comment-typed parent — 201 + reply_to_id 链接', async () => {
    const adminCtx = await adminLogin(serverURL());
    const inv = await mintInvite(adminCtx, 'cv8-rt');
    const owner = await registerUser(serverURL(), inv, 'rt');
    const chId = await createChannel(owner, `cv8-rt-${Date.now()}`);

    const parent = await postMessage(owner, chId, 'parent comment', 'artifact_comment');
    expect(parent.status).toBe(201);
    expect(parent.id).toBeTruthy();

    const reply = await postMessage(owner, chId, 'reply body', 'artifact_comment', parent.id);
    expect(reply.status).toBe(201);
    expect(reply.data.message.reply_to_id).toBe(parent.id);
  });

  test('§3.2 agent reply thinking 4-pattern reject (5-pattern 第 6 处链)', async () => {
    const adminCtx = await adminLogin(serverURL());
    const inv = await mintInvite(adminCtx, 'cv8-think');
    const owner = await registerUser(serverURL(), inv, 'think-owner');
    const chId = await createChannel(owner, `cv8-think-${Date.now()}`);
    const parent = await postMessage(owner, chId, 'parent', 'artifact_comment');
    expect(parent.status).toBe(201);

    const agentRes = await owner.ctx.post('/api/v1/agents', {
      data: { display_name: `cv8-agent-${Date.now()}` },
    });
    expect(agentRes.ok()).toBe(true);
    const ab = (await agentRes.json()) as {
      id?: string;
      agent?: { id: string; api_key?: string };
      api_key?: string;
    };
    const apiKey = ab.api_key ?? ab.agent?.api_key;
    const agentId = ab.id ?? ab.agent?.id;
    await owner.ctx.post(`/api/v1/channels/${chId}/members`, { data: { user_id: agentId } });

    const agentCtx = await apiRequest.newContext({
      baseURL: serverURL(),
      extraHTTPHeaders: { Authorization: `Bearer ${apiKey}` },
    });

    const sentinels = [
      'agent thinking',
      'defaultSubject placeholder leak',
      'wrapped fallbackSubject token',
      'AI is thinking...',
    ];
    for (const body of sentinels) {
      const r = await agentCtx.post(`/api/v1/channels/${chId}/messages`, {
        data: { content: body, content_type: 'artifact_comment', reply_to_id: parent.id },
      });
      expect(r.status(), `pattern "${body}"`).toBe(400);
      const j = (await r.json()) as { code?: string };
      expect(j.code).toBe('comment.thinking_subject_required');
    }

    // Sanity: concrete subject succeeds.
    const ok = await agentCtx.post(`/api/v1/channels/${chId}/messages`, {
      data: { content: 'Section 2 tightening proposal v3.', content_type: 'artifact_comment', reply_to_id: parent.id },
    });
    expect(ok.status()).toBe(201);
  });

  test('§3.3 reply on reply (depth 2) → 400 `comment.thread_depth_exceeded`', async () => {
    const adminCtx = await adminLogin(serverURL());
    const inv = await mintInvite(adminCtx, 'cv8-depth');
    const owner = await registerUser(serverURL(), inv, 'depth');
    const chId = await createChannel(owner, `cv8-depth-${Date.now()}`);
    const parent = await postMessage(owner, chId, 'parent', 'artifact_comment');
    const r1 = await postMessage(owner, chId, 'reply 1', 'artifact_comment', parent.id);
    expect(r1.status).toBe(201);
    const r2 = await postMessage(owner, chId, 'reply on reply', 'artifact_comment', r1.id);
    expect(r2.status).toBe(400);
    expect(r2.data.code).toBe('comment.thread_depth_exceeded');
  });

  test('§3.4 reply on plain-text message → 400 `comment.reply_target_invalid`', async () => {
    const adminCtx = await adminLogin(serverURL());
    const inv = await mintInvite(adminCtx, 'cv8-target');
    const owner = await registerUser(serverURL(), inv, 'target');
    const chId = await createChannel(owner, `cv8-target-${Date.now()}`);
    const plain = await postMessage(owner, chId, 'plain chat'); // default text
    expect(plain.status).toBe(201);
    const r = await postMessage(owner, chId, 'reply on plain', 'artifact_comment', plain.id);
    expect(r.status).toBe(400);
    expect(r.data.code).toBe('comment.reply_target_invalid');
  });

  test('§3.5 cross-channel reject — non-member reply → 403', async () => {
    const adminCtx = await adminLogin(serverURL());
    const ownerInv = await mintInvite(adminCtx, 'cv8-x-owner');
    const owner = await registerUser(serverURL(), ownerInv, 'x-owner');
    const otherInv = await mintInvite(adminCtx, 'cv8-x-other');
    const other = await registerUser(serverURL(), otherInv, 'x-other');
    const chId = await createChannel(owner, `cv8-x-${Date.now()}`);
    const parent = await postMessage(owner, chId, 'parent', 'artifact_comment');

    const r = await other.ctx.post(`/api/v1/channels/${chId}/messages`, {
      data: { content: 'drive-by reply', content_type: 'artifact_comment', reply_to_id: parent.id },
    });
    // Private channel non-member: server returns 404 (channel hidden) or 403
    // (forbidden) depending on access path. Both are fail-closed — REG-INV-002
    // invariant is that the message MUST NOT land. (Same shape as CV-9 §3.3.)
    expect([403, 404]).toContain(r.status());
  });

  test('§3.6 立场 ④ sanity — text-typed message can NOT be parent of artifact_comment thread (反向断)', async () => {
    const adminCtx = await adminLogin(serverURL());
    const inv = await mintInvite(adminCtx, 'cv8-sanity');
    const owner = await registerUser(serverURL(), inv, 'sanity');
    const chId = await createChannel(owner, `cv8-sanity-${Date.now()}`);

    // Two text messages; cannot start a thread on either.
    const a = await postMessage(owner, chId, 'plain a');
    const b = await postMessage(owner, chId, 'plain b');
    expect(a.status).toBe(201);
    expect(b.status).toBe(201);
    const r1 = await postMessage(owner, chId, 'reply', 'artifact_comment', a.id);
    expect(r1.status).toBe(400);
    expect(r1.data.code).toBe('comment.reply_target_invalid');
    // Distinct id — not a fluke: same code on b too.
    const r2 = await postMessage(owner, chId, 'reply2', 'artifact_comment', b.id);
    expect(r2.status).toBe(400);
    expect(r2.data.code).toBe('comment.reply_target_invalid');
  });
});
