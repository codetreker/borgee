// tests/artifact-renderer-types.spec.ts — artifact kind rendering guardrails + XSS defense.
//
// Test scope:
//   - URL protocol rejection: javascript: / data: / http:.
//   - UI renderer breadth stays in canvas-modal-open-close.spec.ts and client Vitest.
//
// Related docs:
//   - Acceptance: docs/_archive/qa/acceptance-templates/cv-3.md §3.1-§3.4
//   - Negative checks: image src https only / link rel="noopener noreferrer" three-part lock (XSS boundary)
//
// Implementation constraints:
//   - Renderer correctness is covered by Vitest (CodeRenderer / ImageLinkRenderer / MentionArtifactPreview / ArtifactPanel-kind-switch)
//     plus canvas-modal-open-close.spec.ts for the mounted markdown UI path.
//   - This spec keeps only the server protocol rejection path with javascript:/data:/http: returning 400.
//   - Do not use fs.*, page.evaluate(fetch), screenshot-only checks, or empty placeholder tests.
import {
  test,
  expect,
  request as apiRequest,
  type APIRequestContext,
} from '@playwright/test';

const ADMIN_LOGIN = 'e2e-admin';
const ADMIN_PASSWORD = 'e2e-admin-pass-12345';

interface RegisteredUser {
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
  expect(res.ok(), `mint invite: ${res.status()}`).toBe(true);
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
  const email = `cv33-${suffix}-${stamp}-${Math.floor(Math.random() * 1000)}@example.test`;
  const password = 'p@ssw0rd-cv33';
  const displayName = `CV33 ${suffix} ${stamp}`;
  const res = await ctx.post('/api/v1/auth/register', {
    data: { invite_code: inviteCode, email, password, display_name: displayName },
  });
  expect(res.ok(), `register: ${res.status()} ${await res.text()}`).toBe(true);
  return { ctx };
}

async function createChannel(user: RegisteredUser, name: string): Promise<string> {
  const r = await user.ctx.post('/api/v1/channels', {
    data: { name, visibility: 'private' },
  });
  expect(r.ok(), `channel create: ${r.status()} ${await r.text()}`).toBe(true);
  const j = (await r.json()) as { channel: { id: string } };
  return j.channel.id;
}

test.describe('CV-3.3 client kind renderers — acceptance §3', () => {
  test('§3.2 image_link 协议反向 reject — javascript:/data:/http: 400 (XSS 红线第一道)', async () => {
    // Server-side guard, independent of UI rendering. Send the three rejected
    // protocols through REST and expect all to return 400.
    // Same source as CV-3.2 #400 ValidateImageLinkURL: server-side https-only lock.
    const serverPort = process.env.E2E_SERVER_PORT ?? '4901';
    const serverURL = `http://127.0.0.1:${serverPort}`;
    const adminCtx = await adminLogin(serverURL);
    const inv = await mintInvite(adminCtx, 'cv33-32');
    const owner = await registerUser(serverURL, inv, 'o32');

    const stamp = Date.now();
    const chName = `cv33-img-${stamp}`;
    const chId = await createChannel(owner, chName);

    for (const bad of [
      { url: 'javascript:alert(1)', label: 'javascript:' },
      { url: 'data:image/png;base64,AAAA', label: 'data:image' },
      { url: 'http://example.com/x.png', label: 'http:' },
    ]) {
      const r = await owner.ctx.post(`/api/v1/channels/${chId}/artifacts`, {
        data: {
          type: 'image_link',
          title: `bad-${bad.label}`,
          body: bad.url,
          metadata: { kind: 'image', url: bad.url },
        },
      });
      expect(r.status(), `${bad.label} should reject 400`).toBe(400);
    }

    // Sanity check: https URL passes.
    const ok = await owner.ctx.post(`/api/v1/channels/${chId}/artifacts`, {
      data: {
        type: 'image_link',
        title: 'good-https',
        body: 'https://example.com/x.png',
        metadata: { kind: 'image', url: 'https://example.com/x.png' },
      },
    });
    const okStatus = ok.status();
    const okText = okStatus >= 400 ? await ok.text() : '';
    expect([200, 201], `https URL should pass; got ${okStatus} body=${okText}`).toContain(okStatus);
  });
});
