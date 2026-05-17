// tests/chat-realtime-message-fanout.spec.ts — agent 邀请实时推送 owner 端 latency 守门 (≤ 3s).
//
// 测试范围 (1 case):
//   - requester POST /api/v1/agent_invitations 创建邀请
//   - owner 浏览器 sidebar More badge 从 0 个变为可见 (websocket 推 agent_invitation_pending 帧 → dispatchInvitationPending → Sidebar listener)
//   - latency 实测 ≤ 3s (硬性指标, 60s 轮询不算)
//
// 关联文档:
//   - 蓝图: docs/blueprint/current/realtime.md (websocket push 走 agent_invitation_pending 帧)
//   - 验收: docs/_archive/qa/acceptance-templates/cm-4.md (REG-RT0-008 / G2.4 latency proof)
//   - 单测: server-side push hub (internal/ws/hub.go) Go 单元测覆盖路由逻辑
//   - 客户端: useWebSocket switch + dispatchInvitationPending → Sidebar invitation-badge re-fetch
//
// 实施约束:
//   - 真 UI: owner page.goto + page.locator('[data-testid=sidebar-footer-more-badge]') + waitFor visible 是真浏览器路径
//   - REST seed: requester 端创建邀请没有 production UI 入口 (createAgentInvitation 仅在 lib/api.ts, 反向 grep 0 production caller), 用 REST 直调合规作 seed
//   - INFRA-2 依赖: 双 server 编排 (server-go + vite) 已 ship
//   - 不允许 fs.* / page.evaluate(fetch) / noop

import { test, expect, request as apiRequest } from '@playwright/test';
import { stopwatch } from '../fixtures/stopwatch';

const ADMIN_LOGIN = 'e2e-admin';
const ADMIN_PASSWORD = 'e2e-admin-pass-12345';

test.describe('RT-0 invitation push latency (≤ 3s)', () => {
  test('owner More badge updates within 3s of POST /agent_invitations', async ({
    browser,
    baseURL,
  }, testInfo) => {
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

    // Owner: registers + creates an agent. The agent's owner_id is
    // the owner user — the push hub routes the pending frame to that
    // user's ws connection (server-go internal/ws/hub.go).
    const ownerCtx = await apiRequest.newContext({ baseURL: serverURL });
    const ownerReg = await ownerCtx.post('/api/v1/auth/register', {
      data: {
        invite_code: await mintInvite('rt0-owner'),
        email: `rt0-owner-${stamp}@example.test`,
        password: 'p@ssw0rd-owner',
        display_name: `Owner ${stamp}`,
      },
    });
    expect(ownerReg.ok(), `owner register: ${ownerReg.status()}`).toBe(true);
    const agentRes = await ownerCtx.post('/api/v1/agents', {
      data: { display_name: `Agent ${stamp}` },
    });
    expect(agentRes.ok(), `agent create: ${agentRes.status()}`).toBe(true);
    const agentId = ((await agentRes.json()) as { agent: { id: string } }).agent.id;

    // Requester: registers + creates a channel they own (auto-member).
    const requesterCtx = await apiRequest.newContext({ baseURL: serverURL });
    const requesterReg = await requesterCtx.post('/api/v1/auth/register', {
      data: {
        invite_code: await mintInvite('rt0-requester'),
        email: `rt0-requester-${stamp}@example.test`,
        password: 'p@ssw0rd-requester',
        display_name: `Requester ${stamp}`,
      },
    });
    expect(requesterReg.ok(), `requester register: ${requesterReg.status()}`).toBe(true);
    const chRes = await requesterCtx.post('/api/v1/channels', {
      data: { name: `rt0-${stamp}`, visibility: 'private' },
    });
    expect(chRes.ok(), `channel create: ${chRes.status()}`).toBe(true);
    const channelId = ((await chRes.json()) as { channel: { id: string } }).channel.id;

    // Open the owner's SPA so /ws connects with their token.
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

    const badge = ownerPage.locator('[data-testid=sidebar-footer-more-badge]');
    await expect(badge).toHaveCount(0);

    const sw = stopwatch();
    const inviteRes = await requesterCtx.post('/api/v1/agent_invitations', {
      data: { agent_id: agentId, channel_id: channelId },
    });
    expect(inviteRes.ok(), `invite create: ${inviteRes.status()}`).toBe(true);

    await badge.waitFor({ state: 'visible', timeout: 5000 });
    sw.stop();

    await sw.attach(testInfo, '邀请→通知 latency');

    expect(sw.ms, `latency ${sw.ms}ms exceeds 3s hardline`).toBeLessThanOrEqual(3000);
  });
});
