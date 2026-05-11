// tests/chat-name-display-regression.spec.ts — agent invitation 显示名回归 e2e (防 PR #198 修过的 bug-029 重发).
//
// 测试范围:
//   - owner GET /api/v1/agent_invitations?role=owner 返回行, agent_name / requester_name / channel_name
//     字段不为空 (跟用户 display_name / channel slug 一致)
//   - 每个 *_name 字段必须配对的 *_id 字段同时存在 (schema 稳定, 名称缺失场景被覆盖)
//   - 收件箱 DOM (Sidebar 铃铛 → InvitationsInbox) 渲染显示名, 不渲染 raw UUID
//
// 关联文档:
//   - 上游修复: PR #198 (sanitizeAgentInvitation JOIN users + channels)
//   - 单测: api/agent_invitations_test.go (resolver hit / miss / nil store)
//   - 回归项: REG-CM4-NAMES
//
// 实施约束:
//   - 真 UI 走浏览器 (page.goto + 真铃铛点击 + DOM 断显示名)
//   - seed 用 REST: admin invite + 双 user register + 互发 agent invitation
//   - 测试主体走 SPA InvitationsInbox 视图
//   - 不允许 fs.* / page.evaluate(fetch) / 只打 API / noop

import { test, expect, request as apiRequest } from '@playwright/test';

const ADMIN_LOGIN = 'e2e-admin';
const ADMIN_PASSWORD = 'e2e-admin-pass-12345';

// ID_RE matches either UUID v4 (RFC 4122 8-4-4-4-12 lowercase hex) for
// legacy rows or ULID (Crockford base32, 26 upper-alnum chars) for
// post-ULID-MIGRATION rows. The bug-029 regression intent is: invitation
// DTO returns opaque IDs (UUID *or* ULID) — both are non-name-shaped, so
// names cannot leak through ID fields.
const UUID_RE = /^([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}|[0-9A-HJKMNP-TV-Z]{26})$/i;

interface InvitationDTO {
  id: string;
  channel_id: string;
  agent_id: string;
  requested_by: string;
  agent_name: string;
  channel_name: string;
  requester_name: string;
  state: string;
  created_at: number;
}

test.describe('bug-029 regression — invitation sanitizer ships display names', () => {
  test('GET /agent_invitations returns names, not raw UUIDs', async ({
    browser,
    baseURL,
  }) => {
    const serverPort = process.env.E2E_SERVER_PORT ?? '4901';
    const serverURL = `http://127.0.0.1:${serverPort}`;
    const adminCtx = await apiRequest.newContext({ baseURL: serverURL });

    const loginRes = await adminCtx.post('/admin-api/auth/login', {
      data: { login: ADMIN_LOGIN, password: ADMIN_PASSWORD },
    });
    expect(loginRes.ok()).toBe(true);

    const mintInvite = async (note: string) => {
      const r = await adminCtx.post('/admin-api/v1/invites', { data: { note } });
      expect(r.ok()).toBe(true);
      return ((await r.json()) as { invite: { code: string } }).invite.code;
    };

    const stamp = Date.now();
    const ownerName = `Owner ${stamp}`;
    const requesterName = `Requester ${stamp}`;
    const channelSlug = `bug029-${stamp}`;

    // Owner registers + creates the agent (display name carried via
    // POST /agents). bug-029 was specifically about *agent* name surfacing.
    const ownerCtx = await apiRequest.newContext({ baseURL: serverURL });
    const ownerReg = await ownerCtx.post('/api/v1/auth/register', {
      data: {
        invite_code: await mintInvite('bug029-owner'),
        email: `bug029-owner-${stamp}@example.test`,
        password: 'p@ssw0rd-bug029',
        display_name: ownerName,
      },
    });
    expect(ownerReg.ok()).toBe(true);
    const agentDisplayName = `Agent ${stamp}`;
    const agentRes = await ownerCtx.post('/api/v1/agents', {
      data: { display_name: agentDisplayName },
    });
    expect(agentRes.ok()).toBe(true);
    const agentId = ((await agentRes.json()) as { agent: { id: string } }).agent.id;

    // Requester registers + creates a private channel; private so the
    // request must explicitly invite the agent (matches the prod path).
    const requesterCtx = await apiRequest.newContext({ baseURL: serverURL });
    const requesterReg = await requesterCtx.post('/api/v1/auth/register', {
      data: {
        invite_code: await mintInvite('bug029-requester'),
        email: `bug029-requester-${stamp}@example.test`,
        password: 'p@ssw0rd-bug029',
        display_name: requesterName,
      },
    });
    expect(requesterReg.ok()).toBe(true);
    const chRes = await requesterCtx.post('/api/v1/channels', {
      data: { name: channelSlug, visibility: 'private' },
    });
    expect(chRes.ok()).toBe(true);
    const channelId = ((await chRes.json()) as { channel: { id: string } }).channel.id;

    const inviteRes = await requesterCtx.post('/api/v1/agent_invitations', {
      data: { agent_id: agentId, channel_id: channelId },
    });
    expect(inviteRes.ok()).toBe(true);

    // Assertion 1: owner-side GET ships the resolved display names.
    const ownerListRes = await ownerCtx.get('/api/v1/agent_invitations?role=owner');
    expect(ownerListRes.ok()).toBe(true);
    const ownerList = (await ownerListRes.json()) as { invitations: InvitationDTO[] };
    expect(ownerList.invitations.length).toBeGreaterThanOrEqual(1);
    const inv = ownerList.invitations.find(i => i.agent_id === agentId)!;
    expect(inv, 'invitation for our agent should be in owner list').toBeTruthy();
    expect(inv.agent_name).toBe(agentDisplayName);
    expect(inv.requester_name).toBe(requesterName);
    expect(inv.channel_name).toBe(channelSlug);

    // Assertion 2: schema is *id + *name, not one-or-the-other. The
    // pre-#198 bug shape was non-empty *_id with empty *_name; assert
    // both populated together so an accidental sanitizer revert fails.
    expect(inv.agent_id, 'agent_id still UUID-shaped').toMatch(UUID_RE);
    expect(inv.channel_id).toMatch(UUID_RE);
    expect(inv.requested_by).toMatch(UUID_RE);
    expect(inv.agent_name).not.toMatch(UUID_RE);
    expect(inv.channel_name).not.toMatch(UUID_RE);
    expect(inv.requester_name).not.toMatch(UUID_RE);
    expect(inv.agent_name.length).toBeGreaterThan(0);
    expect(inv.requester_name.length).toBeGreaterThan(0);
    expect(inv.channel_name.length).toBeGreaterThan(0);

    // Assertion 3: DOM render shows names, not UUIDs. Cookie-injection
    // pattern matches cm-4-realtime.spec.ts (#239).
    const ownerToken = (await ownerCtx.storageState()).cookies.find(
      c => c.name === 'borgee_token',
    );
    expect(ownerToken).toBeTruthy();
    const ownerPage = await browser.newPage();
    const url = new URL(baseURL!);
    await ownerPage.context().addCookies([{
      name: 'borgee_token',
      value: ownerToken!.value,
      domain: url.hostname,
      path: '/',
      httpOnly: true,
      secure: false,
      sameSite: 'Lax',
    }]);
    await ownerPage.goto('/');

    // Open the invitations inbox by clicking the bell. The badge appears
    // because of the pending invitation; click the bell to mount the
    // InvitationsInbox modal (Sidebar.tsx onClick={onInvitationsOpen}).
    const badge = ownerPage.locator('[data-testid=invitation-bell-badge]');
    await badge.waitFor({ state: 'visible', timeout: 5000 });
    await ownerPage.locator('button.invitations-btn').click();

    // Display names must render in the inbox; UUIDs must not.
    // InvitationsInbox.tsx falls back to ID when name is empty (the
    // bug-029 shape) — assert visible name + absent UUID together.
    await expect(ownerPage.getByText(agentDisplayName).first()).toBeVisible({ timeout: 5000 });
    await expect(ownerPage.getByText(requesterName).first()).toBeVisible();
    await expect(ownerPage.getByText(`#${channelSlug}`).first()).toBeVisible();
    const bodyText = (await ownerPage.locator('body').innerText()).trim();
    expect(bodyText, 'agent UUID must not surface in DOM').not.toContain(agentId);
    expect(bodyText, 'channel UUID must not surface in DOM').not.toContain(channelId);
  });
});
