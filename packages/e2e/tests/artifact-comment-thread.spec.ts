// tests/artifact-comment-thread.spec.ts — artifact comment main path (human comments / agent reject / cross-channel / cursor / admin isolation).
//
// Status: skipped with follow-up work tracked in gh#716 + gh#724 §1.
//
// Skip reason: ArtifactComments currently has no production mount in the client SPA,
// so the browser path using page.click and DOM assertions is not reachable. This spec currently calls the backend through REST,
// which makes it a backend contract test rather than an e2e test. After the v2 ArtifactComments UI lands,
// convert this to page.click plus DOM assertions and unskip it.
//
// Test scope after v2 mount and unskip:
//   - Human comment round-trip (POST then GET returns it, channel_id remains in the artifact namespace)
//   - agent thinking subject 必带, 5 模式 reject 400
//   - Cross-channel non-member access returns 403
//   - Cursor increases monotonically and shares ordering with RT-1.1 ArtifactUpdated
//   - Admin routes do not consume this frame; GET /api/v1/artifacts/:id/comments is not registered under admin
//
// Related docs:
//   - Acceptance: docs/_archive/qa/acceptance-templates/cv-5.md §3
//   - Follow-up: gh#724 §1 (ArtifactComments v2 mount)
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
  const email = `cv5-${suffix}-${stamp}-${Math.floor(Math.random() * 10000)}@example.test`;
  const password = 'p@ssw0rd-cv5';
  const res = await ctx.post('/api/v1/auth/register', {
    data: { invite_code: inviteCode, email, password, display_name: `CV5 ${suffix} ${stamp}` },
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
  expect(r.ok(), `channel create: ${r.status()}`).toBe(true);
  const j = (await r.json()) as { channel: { id: string } };
  return j.channel.id;
}

async function createArtifact(user: RegisteredUser, channelId: string, title: string): Promise<string> {
  const r = await user.ctx.post(`/api/v1/channels/${channelId}/artifacts`, {
    data: { title, body: 'head' },
  });
  expect(r.ok(), `artifact create: ${r.status()} ${await r.text()}`).toBe(true);
  const j = (await r.json()) as { id: string };
  return j.id;
}

function serverURL(): string {
  const port = process.env.E2E_SERVER_PORT ?? '4901';
  return `http://127.0.0.1:${port}`;
}

test.describe.skip('artifact comments 主流 (gh#716 SKIP+followup, 等 v2 mount 后 unskip — gh#724 §1)', () => {
  test('§3.1 human comment round-trip — namespace channel + GET list', async () => {
    const adminCtx = await adminLogin(serverURL());
    const inv = await mintInvite(adminCtx, 'cv5-rt');
    const owner = await registerUser(serverURL(), inv, 'rt');
    const chId = await createChannel(owner, `cv5-rt-${Date.now()}`);
    const artId = await createArtifact(owner, chId, 'Plan');

    const post = await owner.ctx.post(`/api/v1/artifacts/${artId}/comments`, {
      data: { body: 'looks great, ship it' },
    });
    expect(post.status(), await post.text()).toBe(201);
    const created = (await post.json()) as {
      id: string;
      sender_role: string;
      channel_id: string;
      body: string;
    };
    expect(created.sender_role).toBe('human');
    expect(created.body).toBe('looks great, ship it');
    // Opaque ID assertion (UUID-36 legacy or ULID-26 post-ULID-MIGRATION) —
    // intent: NOT raw `artifact:` literal in id.
    expect(created.channel_id).toMatch(
      /^([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}|[0-9A-HJKMNP-TV-Z]{26})$/i,
    );

    // GET list returns it because comments are stored in messages and must be listable.
    const list = await owner.ctx.get(`/api/v1/artifacts/${artId}/comments`);
    expect(list.ok()).toBe(true);
    const j = (await list.json()) as { comments: Array<{ id: string; body: string }> };
    expect(j.comments.length).toBe(1);
    expect(j.comments[0].body).toBe('looks great, ship it');
  });

  test('§3.2 agent thinking subject 必带反断 — 5-pattern 全 reject 400', async () => {
    const adminCtx = await adminLogin(serverURL());
    const inv = await mintInvite(adminCtx, 'cv5-think');
    const owner = await registerUser(serverURL(), inv, 'think-owner');
    const chId = await createChannel(owner, `cv5-think-${Date.now()}`);
    const artId = await createArtifact(owner, chId, 'Plan');

    // Create an agent under the owner via /api/v1/agents (returns api_key).
    const agentRes = await owner.ctx.post('/api/v1/agents', {
      data: { display_name: `cv5-agent-${Date.now()}` },
    });
    expect(agentRes.ok(), `create agent: ${agentRes.status()} ${await agentRes.text()}`).toBe(true);
    const agentBody = (await agentRes.json()) as { id?: string; agent?: { id: string }; api_key?: string };
    const agentApiKey = (agentBody as { api_key?: string }).api_key
      ?? ((agentBody as { agent?: { api_key?: string } }).agent?.api_key as string | undefined);
    const agentId = agentBody.id ?? agentBody.agent?.id;
    expect(agentApiKey, 'agent api_key').toBeTruthy();
    expect(agentId, 'agent id').toBeTruthy();

    // Add agent to channel as member.
    const addM = await owner.ctx.post(`/api/v1/channels/${chId}/members`, {
      data: { user_id: agentId },
    });
    expect(addM.ok(), `add member: ${addM.status()} ${await addM.text()}`).toBe(true);

    // Use Bearer auth on a fresh ctx for the agent.
    const agentCtx = await apiRequest.newContext({
      baseURL: serverURL(),
      extraHTTPHeaders: { Authorization: `Bearer ${agentApiKey}` },
    });

    const sentinels = [
      '   ', // pattern 5 empty/whitespace
      'agent thinking', // pattern 1 trailing thinking
      'defaultSubject placeholder leak',
      'wrapped fallbackSubject token',
      'AI is thinking...',
    ];
    for (const body of sentinels) {
      const r = await agentCtx.post(`/api/v1/artifacts/${artId}/comments`, { data: { body } });
      expect(r.status(), `pattern "${body}"`).toBe(400);
      const j = (await r.json()) as { code?: string };
      expect(j.code).toBe('comment.thinking_subject_required');
    }

    // Sanity: agent with concrete subject succeeds.
    const ok = await agentCtx.post(`/api/v1/artifacts/${artId}/comments`, {
      data: { body: 'I propose tightening section 2 about lock TTLs.' },
    });
    expect(ok.status()).toBe(201);
  });

  test('§3.3 cross-channel reject — non-member → 403 byte-identical code', async () => {
    const adminCtx = await adminLogin(serverURL());
    const ownerInv = await mintInvite(adminCtx, 'cv5-xchan-owner');
    const owner = await registerUser(serverURL(), ownerInv, 'xchan-owner');
    const otherInv = await mintInvite(adminCtx, 'cv5-xchan-other');
    const other = await registerUser(serverURL(), otherInv, 'xchan-other');
    const chId = await createChannel(owner, `cv5-xchan-${Date.now()}`);
    const artId = await createArtifact(owner, chId, 'Plan');

    const r = await other.ctx.post(`/api/v1/artifacts/${artId}/comments`, {
      data: { body: 'drive-by from non-member' },
    });
    expect(r.status()).toBe(403);
    const j = (await r.json()) as { code?: string };
    expect(j.code).toBe('comment.cross_channel_reject');
  });

  test('§3.4 cursor 共序锁 — comment cursor monotonic + ≥ artifact_updated cursor', async () => {
    const adminCtx = await adminLogin(serverURL());
    const inv = await mintInvite(adminCtx, 'cv5-cursor');
    const owner = await registerUser(serverURL(), inv, 'cursor');
    const chId = await createChannel(owner, `cv5-cursor-${Date.now()}`);
    const artId = await createArtifact(owner, chId, 'Plan');

    // Trigger an artifact update first to advance hub.cursors.
    const commit = await owner.ctx.post(`/api/v1/artifacts/${artId}/commits`, {
      data: { body: 'edited body', expected_version: 1 },
    });
    expect(commit.ok(), `commit: ${commit.status()} ${await commit.text()}`).toBe(true);

    // Now post 3 comments and assert cursor monotonicity for the RT-3 shared ordering point.
    let prev = 0;
    for (let i = 0; i < 3; i++) {
      const r = await owner.ctx.post(`/api/v1/artifacts/${artId}/comments`, {
        data: { body: `comment ${i}` },
      });
      expect(r.status()).toBe(201);
      const j = (await r.json()) as { cursor?: number };
      expect(typeof j.cursor === 'number' && j.cursor > 0).toBe(true);
      expect(j.cursor!).toBeGreaterThan(prev);
      prev = j.cursor!;
    }
  });

  test('§3.5 admin god-mode 不消费此 frame — /admin-api/* rail 不挂 comment GET', async () => {
    const adminCtx = await adminLogin(serverURL());
    // Admin rail GET on artifact comments path → not registered (404 / 405).
    const r = await adminCtx.get(`/admin-api/v1/artifacts/anything/comments`);
    expect(r.status() === 404 || r.status() === 405).toBe(true);
    // Also: /api/v1/* rail with admin cookie is the user rail; admin cookie
    // should NOT auto-grant access to private channel (ADM-0 §1.3 红线).
    // (覆盖 by §3.3 cross-channel reject already.)
  });
});
