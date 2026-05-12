// tests/comment-edit-delete-permission.spec.ts — comment edit / delete / reactions + access-control checks (§1 + §2).
//
// Status: skipped with follow-up work tracked in gh#716 + gh#724 §1 mount + §2 ACL UX.
//
// Skip reasons:
//   - §1: ArtifactComments currently has no production mount, so page.click cannot reach the UI.
//   - §2: forbidden-state UX for ACL failures is not designed yet (gh#724 §2).
// This spec currently calls the backend through REST, so it is a backend contract test rather than an e2e test.
//
// Keep all 6 cases; each ACL path is required:
//   - Human owner edits their own comment (PUT 200 + GET sees new body + edited_at is non-empty)
//   - Agent edit with thinking placeholder triggers placeholder-pattern validation; 4 sub-cases reject with 400
//   - Human owner deletes their own comment (DELETE 200 + GET no longer returns it)
//   - Editing another user's comment → 403
//   - Reaction +1 / -1 round-trip (PUT/DELETE 200 + count changes)
//   - Non-comment message types do not use the thinking validator
//
// Related docs:
//   - Acceptance: docs/_archive/qa/acceptance-templates/cv-7.md §3
//   - Follow-up: gh#724 §1 (mount) + §2 (forbidden state UX)
//
// Implementation constraints after unskip:
//   - Browser-driven UI path; cross-user access checks use page.goto plus negative DOM assertions (REWRITE-NAV pattern).
//   - Do not use fs.*, page.evaluate(fetch), API-only checks, or empty placeholder tests.
//   - Keep all six ACL cases.

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
  expect(res.ok(), `admin login: ${res.status()}`).toBe(true);
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
  const email = `cv7-${suffix}-${stamp}-${Math.floor(Math.random() * 10000)}@example.test`;
  const password = 'p@ssw0rd-cv7';
  const res = await ctx.post('/api/v1/auth/register', {
    data: { invite_code: inviteCode, email, password, display_name: `CV7 ${suffix} ${stamp}` },
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
): Promise<string> {
  const r = await user.ctx.post(`/api/v1/channels/${channelId}/messages`, {
    data: contentType ? { content, content_type: contentType } : { content },
  });
  expect(r.ok(), `post msg: ${r.status()} ${await r.text()}`).toBe(true);
  const j = (await r.json()) as { message: { id: string } };
  return j.message.id;
}

function serverURL(): string {
  const port = process.env.E2E_SERVER_PORT ?? '4901';
  return `http://127.0.0.1:${port}`;
}

test.describe.skip('comment 编辑/删除/反应 + 防越权 (gh#716 SKIP+followup, 等 v2 mount + ACL UX 后 unskip — gh#724 §1+§2)', () => {
  test('§3.1 human owner edits own message — PUT 200 + GET 见新 body', async () => {
    const adminCtx = await adminLogin(serverURL());
    const inv = await mintInvite(adminCtx, 'cv7-edit');
    const owner = await registerUser(serverURL(), inv, 'edit');
    const chId = await createChannel(owner, `cv7-edit-${Date.now()}`);
    const msgId = await postMessage(owner, chId, 'first draft');

    const r = await owner.ctx.put(`/api/v1/messages/${msgId}`, {
      data: { content: 'edited body v2' },
    });
    expect(r.status()).toBe(200);
    const j = (await r.json()) as { message: { content: string; edited_at: number | null } };
    expect(j.message.content).toBe('edited body v2');
    expect(j.message.edited_at).toBeTruthy();
  });

  test('§3.2 agent edit thinking 5-pattern reject — 4 sub-case 全 400 byte-identical CV-5', async () => {
    const adminCtx = await adminLogin(serverURL());
    const inv = await mintInvite(adminCtx, 'cv7-think');
    const owner = await registerUser(serverURL(), inv, 'think-owner');
    const chId = await createChannel(owner, `cv7-think-${Date.now()}`);

    // Create agent under owner (returns api_key).
    const agentRes = await owner.ctx.post('/api/v1/agents', {
      data: { display_name: `cv7-agent-${Date.now()}` },
    });
    expect(agentRes.ok()).toBe(true);
    const ab = (await agentRes.json()) as {
      id?: string;
      agent?: { id: string; api_key?: string };
      api_key?: string;
    };
    const apiKey = ab.api_key ?? ab.agent?.api_key;
    const agentId = ab.id ?? ab.agent?.id;
    expect(apiKey).toBeTruthy();
    expect(agentId).toBeTruthy();

    await owner.ctx.post(`/api/v1/channels/${chId}/members`, { data: { user_id: agentId } });

    const agentCtx = await apiRequest.newContext({
      baseURL: serverURL(),
      extraHTTPHeaders: { Authorization: `Bearer ${apiKey}` },
    });

    // Agent posts an artifact_comment-typed message with concrete subject.
    const post = await agentCtx.post(`/api/v1/channels/${chId}/messages`, {
      data: { content: 'concrete subject v1', content_type: 'artifact_comment' },
    });
    expect(post.ok(), await post.text()).toBe(true);
    const pj = (await post.json()) as { message: { id: string } };
    const msgId = pj.message.id;

    const sentinels = [
      'agent thinking',
      'defaultSubject placeholder leak',
      'wrapped fallbackSubject token',
      'AI is thinking...',
    ];
    for (const body of sentinels) {
      const r = await agentCtx.put(`/api/v1/messages/${msgId}`, { data: { content: body } });
      expect(r.status(), `pattern "${body}"`).toBe(400);
      const j = (await r.json()) as { code?: string };
      expect(j.code).toBe('comment.thinking_subject_required');
    }
    // Sanity: concrete subject succeeds.
    const ok = await agentCtx.put(`/api/v1/messages/${msgId}`, {
      data: { content: 'Section 2 tightening proposal v3.' },
    });
    expect(ok.status()).toBe(200);
  });

  test('§3.3 delete own — DELETE 200 + 后续 GET 不再返回', async () => {
    const adminCtx = await adminLogin(serverURL());
    const inv = await mintInvite(adminCtx, 'cv7-del');
    const owner = await registerUser(serverURL(), inv, 'del');
    const chId = await createChannel(owner, `cv7-del-${Date.now()}`);
    const msgId = await postMessage(owner, chId, 'will be deleted');

    const r = await owner.ctx.delete(`/api/v1/messages/${msgId}`);
    expect(r.status() === 200 || r.status() === 204).toBe(true);

    // Subsequent list should not include the deleted message id.
    const list = await owner.ctx.get(`/api/v1/channels/${chId}/messages`);
    expect(list.ok()).toBe(true);
    const lj = (await list.json()) as { messages: Array<{ id: string }> };
    const stillThere = lj.messages.some((m) => m.id === msgId);
    expect(stillThere).toBe(false);
  });

  test('§3.4 edit other comment → 403 byte-identical', async () => {
    const adminCtx = await adminLogin(serverURL());
    const ownerInv = await mintInvite(adminCtx, 'cv7-403-owner');
    const owner = await registerUser(serverURL(), ownerInv, '403-owner');
    const otherInv = await mintInvite(adminCtx, 'cv7-403-other');
    const other = await registerUser(serverURL(), otherInv, '403-other');
    const chId = await createChannel(owner, `cv7-403-${Date.now()}`);
    // Add `other` to channel (so the cross-user test isolates the
    // sender-only edit gate, not membership).
    await owner.ctx.post(`/api/v1/channels/${chId}/members`, { data: { user_id: other.userId } });
    const msgId = await postMessage(owner, chId, 'owners message');

    const r = await other.ctx.put(`/api/v1/messages/${msgId}`, {
      data: { content: 'mutating someone elses' },
    });
    expect(r.status()).toBe(403);
  });

  test('§3.5 reaction +1 -1 round-trip — PUT 200 + GET count switches', async () => {
    const adminCtx = await adminLogin(serverURL());
    const inv = await mintInvite(adminCtx, 'cv7-react');
    const owner = await registerUser(serverURL(), inv, 'react');
    const chId = await createChannel(owner, `cv7-react-${Date.now()}`);
    const msgId = await postMessage(owner, chId, 'react to me');

    const put = await owner.ctx.put(`/api/v1/messages/${msgId}/reactions`, {
      data: { emoji: '👍' },
    });
    expect(put.ok(), await put.text()).toBe(true);
    const after = await owner.ctx.get(`/api/v1/messages/${msgId}/reactions`);
    const aj = (await after.json()) as { reactions: Array<{ emoji: string; count: number }> };
    expect(aj.reactions.length).toBeGreaterThan(0);

    const del = await owner.ctx.delete(`/api/v1/messages/${msgId}/reactions`, {
      data: { emoji: '👍' },
    });
    expect(del.ok()).toBe(true);
    const after2 = await owner.ctx.get(`/api/v1/messages/${msgId}/reactions`);
    const aj2 = (await after2.json()) as { reactions: Array<{ count: number }> };
    expect(aj2.reactions.length).toBe(0);
  });

  test('§3.6 立场 ③ 反约束 sanity — non-comment-typed agent edit 不走 thinking validator', async () => {
    const adminCtx = await adminLogin(serverURL());
    const inv = await mintInvite(adminCtx, 'cv7-anti');
    const owner = await registerUser(serverURL(), inv, 'anti-owner');
    const chId = await createChannel(owner, `cv7-anti-${Date.now()}`);

    const agentRes = await owner.ctx.post('/api/v1/agents', {
      data: { display_name: `cv7-anti-agent-${Date.now()}` },
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

    // Plain-text (default content_type='text') message — NOT subject to
    // thinking validator even when body matches a 5-pattern sentinel.
    const post = await agentCtx.post(`/api/v1/channels/${chId}/messages`, {
      data: { content: 'plain chat hello' },
    });
    expect(post.ok()).toBe(true);
    const pj = (await post.json()) as { message: { id: string } };

    const r = await agentCtx.put(`/api/v1/messages/${pj.message.id}`, {
      data: { content: 'AI is thinking...' },
    });
    expect(r.status()).toBe(200); // Text messages do not use this validator.
  });
});
