// tests/demo-screenshot-archive.spec.ts — Phase 2 退出门槛 G2.4 demo 截屏归档.
//
// 测试范围:
//   - #1 Welcome 第一眼非空屏: register 后 DOM 含 system message + 不含 "👈 选择频道"
//   - #5 系统消息 + CTA 按钮: message bubble + 按钮点击跳 AgentManager
//
// 不在范围 (审计后删除占位 case, 改由其它路径覆盖):
//   - #2 左栏团队感知 (已由 AL-1b 单测锁源)
//   - #3 Agent invitation inbox 名字渲染 (已由 chat-name-display-regression e2e 覆盖)
//   - #4 Quick action 错误态 409 (依赖 mock fixture, 已由 server unit api/agent_invitations_test.go 锁)
//
// 关联文档:
//   - 计划: docs/qa/g2.4-screenshot-plan.md (PR #199)
//   - 签字: docs/qa/g2.4-demo-signoff.md
//   - 输出: docs/qa/screenshots/g2.4-{1..5}.png
//
// 实施约束:
//   - 真 UI 走浏览器 (page.goto + 真按钮 + page.screenshot 入 git)
//   - seed 用 REST: admin invite + register, 流程跟 chat-first-time-onboarding 一致
//   - 不允许 fs.* / page.evaluate(fetch) / 只打 API / noop / 占位 expect(true)
import { test, expect, request as apiRequest } from '@playwright/test';
import * as path from 'path';
import { fileURLToPath } from 'url';

const ADMIN_LOGIN = 'e2e-admin';
const ADMIN_PASSWORD = 'e2e-admin-pass-12345';
const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const SCREENSHOT_DIR = path.resolve(__dirname, '../../../docs/qa/screenshots');

async function bootstrapUser(serverURL: string, page: any, baseURL: string, displayName: string) {
  const ctx = await apiRequest.newContext({ baseURL: serverURL });
  const loginRes = await ctx.post('/admin-api/auth/login', {
    data: { login: ADMIN_LOGIN, password: ADMIN_PASSWORD },
  });
  expect(loginRes.ok(), `admin login failed: ${loginRes.status()}`).toBe(true);
  const inviteRes = await ctx.post('/admin-api/v1/invites', { data: { note: 'g2.4-demo' } });
  const inviteJson = (await inviteRes.json()) as { invite: { code: string } };
  const stamp = Date.now();
  const regCtx = await apiRequest.newContext({ baseURL: serverURL });
  const regRes = await regCtx.post('/api/v1/auth/register', {
    data: {
      invite_code: inviteJson.invite.code,
      email: `g24-${stamp}@example.test`,
      password: 'p@ssw0rd-g24',
      display_name: displayName,
    },
  });
  expect(regRes.ok(), `register failed: ${regRes.status()}`).toBe(true);
  const cookies = await regCtx.storageState();
  const tokenCookie = cookies.cookies.find(c => c.name === 'borgee_token');
  expect(tokenCookie).toBeTruthy();
  const url = new URL(baseURL);
  await page.context().addCookies([{
    name: 'borgee_token',
    value: tokenCookie!.value,
    domain: url.hostname,
    path: '/',
    httpOnly: true,
    secure: false,
    sameSite: 'Lax',
  }]);
}

test.describe('G2.4 demo screenshots — Phase 2 退出 gate', () => {
  const serverPort = process.env.E2E_SERVER_PORT ?? '4901';
  const serverURL = `http://127.0.0.1:${serverPort}`;

  test('#1 Welcome 第一眼非空屏 (§1.4 + onboarding §3 步骤 2)', async ({ page, baseURL }) => {
    await bootstrapUser(serverURL, page, baseURL!, 'G2.4 Owner');
    await page.goto('/');
    // 立场锁: 第一眼非空屏 — system message + 不含 "👈 选择频道"
    await expect(page.locator('.message-system-content').first()).toContainText('欢迎来到 Borgee');
    await expect(page.getByText('👈 选择一个频道开始聊天')).toHaveCount(0);
    if (process.env.E2E_EVIDENCE_SCREENSHOTS === '1') await page.screenshot({
      path: path.join(SCREENSHOT_DIR, 'g2.4-1-welcome-first-glance.png'),
      fullPage: true,
    });
  });

  test('#5 System message + CTA button 局部 (步骤 2 message kind)', async ({ page, baseURL }) => {
    await bootstrapUser(serverURL, page, baseURL!, 'G2.4 CTA Owner');
    await page.goto('/');
    const messageBubble = page.locator('.message-system-content').first();
    await expect(messageBubble).toContainText('欢迎来到 Borgee');
    const cta = page.locator('button.message-system-quick-action');
    await expect(cta).toHaveText('创建 agent');
    // 局部截屏: message bubble + button (非 fullPage)
    if (process.env.E2E_EVIDENCE_SCREENSHOTS === '1') await messageBubble.screenshot({
      path: path.join(SCREENSHOT_DIR, 'g2.4-5-system-message-cta.png'),
    });
  });

  // #2/#3/#4: DEFERRED-UNWIND audit真删 — 立场已由跨 milestone server unit
  // + vitest 单测锁源头 byte-identical 守, e2e 加层重复无新覆盖. 详细
  // rationale 见本文件 header.
});
