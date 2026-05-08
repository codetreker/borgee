// tests/ap-5-messages-acl-matrix.spec.ts — AP-5 e2e cross-channel ACL
// matrix for messages PUT/DELETE/PATCH (post-removal fail-closed gap
// 闭合, 跟 AP-4 reactions ACL 同模式 + DM-5 #549 §3.3 反向断同源).
//
// Acceptance: docs/_archive/qa/acceptance-templates/ap-5.md §2.
// Spec: docs/implementation/modules/ap-5-spec.md §1 AP-5.2.

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
  const email = `ap5-${suffix}-${stamp}-${Math.floor(Math.random() * 10000)}@example.test`;
  const password = 'p@ssw0rd-ap5';
  const res = await ctx.post('/api/v1/auth/register', {
    data: { invite_code: inviteCode, email, password, display_name: `AP5 ${suffix} ${stamp}` },
  });
  expect(res.ok(), `register: ${res.status()} ${await res.text()}`).toBe(true);
  const body = (await res.json()) as { user: { id: string } };
  const cookies = await ctx.storageState();
  const tok = cookies.cookies.find((c) => c.name === 'borgee_token');
  return { email, token: tok!.value, userId: body.user.id, ctx };
}

async function createChannel(
  user: RegisteredUser,
  name: string,
  visibility: 'public' | 'private' = 'public',
): Promise<string> {
  const r = await user.ctx.post('/api/v1/channels', { data: { name, visibility } });
  expect(r.ok()).toBe(true);
  const j = (await r.json()) as { channel: { id: string } };
  return j.channel.id;
}

async function postMessage(
  user: RegisteredUser,
  channelId: string,
  content: string,
): Promise<string> {
  const r = await user.ctx.post(`/api/v1/channels/${channelId}/messages`, { data: { content } });
  expect(r.ok(), `post: ${r.status()}`).toBe(true);
  const j = (await r.json()) as { message: { id: string } };
  return j.message.id;
}

function serverURL(): string {
  const port = process.env.E2E_SERVER_PORT ?? '4901';
  return `http://127.0.0.1:${port}`;
}

test.describe('AP-5 messages ACL matrix — post-removal fail-closed (acceptance §2)', () => {
  test('§2.1 PUT post-removal → 404 fail-closed', async () => {
    const adminCtx = await adminLogin(serverURL());
    const owner = await registerUser(serverURL(), await mintInvite(adminCtx, 'ap5-put'), 'put-owner');
    const sender = await registerUser(serverURL(), await mintInvite(adminCtx, 'ap5-put-s'), 'put-sender');

    const chId = await createChannel(owner, `ap5-put-${Date.now()}`, 'public');
    const join = await sender.ctx.post(`/api/v1/channels/${chId}/join`);
    expect(join.ok()).toBe(true);

    const msgId = await postMessage(sender, chId, 'before removal');

    // Owner removes sender from channel.
    const rm = await owner.ctx.delete(`/api/v1/channels/${chId}/members/${sender.userId}`);
    expect([200, 204]).toContain(rm.status());

    // Sender now tries to PUT-edit own message → 404.
    const r = await sender.ctx.put(`/api/v1/messages/${msgId}`, { data: { content: 'after' } });
    expect(r.status()).toBe(404);
  });

  test('§2.2 DELETE post-removal → 404 fail-closed', async () => {
    const adminCtx = await adminLogin(serverURL());
    const owner = await registerUser(serverURL(), await mintInvite(adminCtx, 'ap5-del'), 'del-owner');
    const sender = await registerUser(serverURL(), await mintInvite(adminCtx, 'ap5-del-s'), 'del-sender');

    const chId = await createChannel(owner, `ap5-del-${Date.now()}`, 'public');
    const join = await sender.ctx.post(`/api/v1/channels/${chId}/join`);
    expect(join.ok()).toBe(true);

    const msgId = await postMessage(sender, chId, 'to delete');

    const rm = await owner.ctx.delete(`/api/v1/channels/${chId}/members/${sender.userId}`);
    expect([200, 204]).toContain(rm.status());

    const r = await sender.ctx.delete(`/api/v1/messages/${msgId}`);
    expect(r.status()).toBe(404);
  });

  test('§2.3 PATCH DM post-removal → 404 fail-closed (DM-4 path)', async () => {
    const adminCtx = await adminLogin(serverURL());
    const owner = await registerUser(serverURL(), await mintInvite(adminCtx, 'ap5-pdm-o'), 'pdm-owner');
    const peer = await registerUser(serverURL(), await mintInvite(adminCtx, 'ap5-pdm-p'), 'pdm-peer');

    // Open DM peer→owner (peer is in DM).
    const open = await peer.ctx.post(`/api/v1/dm/${owner.userId}`);
    expect(open.ok()).toBe(true);
    const oj = (await open.json()) as {
      channel_id?: string;
      channel?: { id: string };
    };
    const chId = oj.channel_id ?? oj.channel?.id;
    expect(chId, `dm channel id: ${JSON.stringify(oj)}`).toBeTruthy();

    const msgId = await postMessage(peer, chId!, 'dm before removal');

    // Owner removes peer from DM channel (admin-style mutation; test the
    // effect rather than the exact path — fall back to leave if remove
    // not permitted, since DM membership ops vary).
    const rm = await owner.ctx.delete(`/api/v1/channels/${chId}/members/${peer.userId}`);
    if (!rm.ok()) {
      // alt: peer leaves (DM-leave path)
      const leave = await peer.ctx.post(`/api/v1/channels/${chId}/leave`);
      expect([200, 204, 403, 404]).toContain(leave.status());
    }

    const r = await peer.ctx.patch(`/api/v1/channels/${chId}/messages/${msgId}`, {
      data: { content: 'after' },
    });
    // Either DM gate (403 dm.edit_only_in_dm if channel kind shifts) or
    // channel-member 404. 404 is the AP-5 lock; 403 acceptable as
    // pre-existing DM-only path. We accept both fail-closed shapes.
    expect([403, 404]).toContain(r.status());
  });

  test('§2.4 cross-org sanity — third-party cannot PUT/DELETE foreign msg', async () => {
    const adminCtx = await adminLogin(serverURL());
    const owner = await registerUser(serverURL(), await mintInvite(adminCtx, 'ap5-x-o'), 'x-owner');
    const stranger = await registerUser(serverURL(), await mintInvite(adminCtx, 'ap5-x-s'), 'x-stranger');

    const chId = await createChannel(owner, `ap5-x-${Date.now()}`, 'private');
    const msgId = await postMessage(owner, chId, 'private msg');

    // Stranger never joined the private channel.
    const put = await stranger.ctx.put(`/api/v1/messages/${msgId}`, { data: { content: 'evil' } });
    expect([403, 404]).toContain(put.status());

    const del = await stranger.ctx.delete(`/api/v1/messages/${msgId}`);
    expect([403, 404]).toContain(del.status());
  });
});
