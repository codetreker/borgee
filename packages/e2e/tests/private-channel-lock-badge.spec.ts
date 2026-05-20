// tests/private-channel-lock-badge.spec.ts — sidebar 私有频道锁 badge 真浏览器视觉验证
//
// PR #1023 (attempt 4) 跟前 3 次踩坑反向断言:
//   - 锁 badge 是真 SVG (.channel-lock-badge), 不再是"锁"汉字
//   - lock bounding box 真叠在 .channel-hash slot 内的右上角 (不在外面占独立水平槽)
//   - 公开频道 # 仍存在, 不丢底图
//   - archived + private 双叠 (📦 base + lock overlay)
//
// 真 UI: 走 SPA 渲染, 通过 REST seed 用户 + 私有频道 + 公开频道 + 归档,
// 然后浏览器视觉断 bounding box / DOM 结构.
import { test, expect, request as apiRequest, type APIRequestContext, type Page } from '@playwright/test';
import path from 'node:path';

const ADMIN_LOGIN = 'e2e-admin';
const ADMIN_PASSWORD = 'e2e-admin-pass-12345';

const SCREENSHOT_DIR = path.join(
  process.env.PLAYWRIGHT_HTML_REPORT ?? 'playwright-report',
  'private-channel-lock-badge',
);

interface RegisteredUser {
  email: string;
  password: string;
  displayName: string;
  token: string;
  userId: string;
}

function serverURL(): string {
  const port = process.env.E2E_SERVER_PORT ?? '4901';
  return `http://127.0.0.1:${port}`;
}

async function adminCtx(): Promise<APIRequestContext> {
  const ctx = await apiRequest.newContext({ baseURL: serverURL() });
  const res = await ctx.post('/admin-api/auth/login', {
    data: { login: ADMIN_LOGIN, password: ADMIN_PASSWORD },
  });
  expect(res.ok(), `admin login: ${res.status()}`).toBe(true);
  return ctx;
}

async function mintInvite(admin: APIRequestContext, note: string): Promise<string> {
  const res = await admin.post('/admin-api/v1/invites', { data: { note } });
  expect(res.ok()).toBe(true);
  const body = (await res.json()) as { invite: { code: string } };
  return body.invite.code;
}

async function registerUser(inviteCode: string, suffix: string): Promise<RegisteredUser> {
  const ctx = await apiRequest.newContext({ baseURL: serverURL() });
  const stamp = Date.now();
  const email = `lockbadge-${suffix}-${stamp}@example.test`;
  const password = 'p@ssw0rd-lockbadge';
  const displayName = `Lock ${suffix} ${stamp}`;
  const res = await ctx.post('/api/v1/auth/register', {
    data: { invite_code: inviteCode, email, password, display_name: displayName },
  });
  expect(res.ok(), `register ${suffix}: ${res.status()} ${await res.text()}`).toBe(true);
  const body = (await res.json()) as { user: { id: string } };
  const cookies = await ctx.storageState();
  const tokenCookie = cookies.cookies.find(c => c.name === 'borgee_token');
  expect(tokenCookie, 'borgee_token cookie missing').toBeTruthy();
  await ctx.dispose();
  return { email, password, displayName, token: tokenCookie!.value, userId: body.user.id };
}

async function attachToken(page: Page, baseURL: string, token: string) {
  const url = new URL(baseURL);
  await page.context().clearCookies();
  await page.context().addCookies([{
    name: 'borgee_token',
    value: token,
    domain: url.hostname,
    path: '/',
    httpOnly: true,
    secure: false,
    sameSite: 'Lax',
  }]);
}

interface CreatedChannel { id: string; name: string }

async function createChannel(
  token: string,
  body: Record<string, unknown>,
): Promise<CreatedChannel> {
  const ctx = await apiRequest.newContext({
    baseURL: serverURL(),
    extraHTTPHeaders: { Cookie: `borgee_token=${token}` },
  });
  const res = await ctx.post('/api/v1/channels', { data: body });
  expect(res.ok(), `create channel: ${res.status()} ${await res.text()}`).toBe(true);
  const json = (await res.json()) as { channel: CreatedChannel };
  await ctx.dispose();
  return json.channel;
}

async function archiveChannel(token: string, channelId: string) {
  const ctx = await apiRequest.newContext({
    baseURL: serverURL(),
    extraHTTPHeaders: { Cookie: `borgee_token=${token}` },
  });
  const res = await ctx.put(`/api/v1/channels/${channelId}`, { data: { archived: true } });
  expect(res.ok(), `archive: ${res.status()} ${await res.text()}`).toBe(true);
  await ctx.dispose();
}

test.describe('private-channel lock badge — sidebar 真浏览器视觉断言', () => {
  test('public / private / archived-private 三态: # 保留, 私有锁 badge 真叠在 hash 槽内右上角', async ({ page, baseURL }) => {
    const admin = await adminCtx();
    const invite = await mintInvite(admin, 'lock-badge-e2e');
    const user = await registerUser(invite, 'owner');
    await admin.dispose();

    const stamp = Date.now().toString(36);
    const pub = await createChannel(user.token, { name: `lockbadge-pub-${stamp}`, visibility: 'public' });
    const priv = await createChannel(user.token, { name: `lockbadge-priv-${stamp}`, visibility: 'private' });
    const archivedPriv = await createChannel(user.token, {
      name: `lockbadge-archpriv-${stamp}`,
      visibility: 'private',
    });
    await archiveChannel(user.token, archivedPriv.id);

    await attachToken(page, baseURL!, user.token);
    await page.goto('/');
    await expect(page.locator('.sidebar-title')).toBeVisible();

    // Wait for all three channel rows to render in the sidebar.
    const sidebar = page.locator('.sidebar');
    const pubRow = sidebar.locator('.channel-item', { hasText: pub.name }).first();
    const privRow = sidebar.locator('.channel-item', { hasText: priv.name }).first();
    const archivedRow = sidebar
      .locator('.channel-item[data-archived="true"]', { hasText: archivedPriv.name })
      .first();
    await expect(pubRow).toBeVisible({ timeout: 15_000 });
    await expect(privRow).toBeVisible({ timeout: 15_000 });
    await expect(archivedRow).toBeVisible({ timeout: 15_000 });

    // -------------------- Scenario A: 公开频道保留 # 底图, 无锁 badge --------------------
    const pubHashBase = pubRow.locator('.channel-hash .channel-hash-base');
    await expect(pubHashBase).toHaveText('#');
    await expect(pubRow.locator('[data-private-indicator="true"]')).toHaveCount(0);
    await expect(pubRow.locator('.channel-lock-badge')).toHaveCount(0);
    await expect(pubRow.locator('[data-private-lock="true"]')).toHaveCount(0);
    await page.screenshot({ path: path.join(SCREENSHOT_DIR, 'public-no-lock.png') });

    // -------------------- Scenario B: 私有活跃 # 底图 + lock 叠在右上角 --------------------
    const privHashSlot = privRow.locator('.channel-hash[data-private-indicator="true"]');
    await expect(privHashSlot).toHaveCount(1);
    await expect(privHashSlot.locator('.channel-hash-base')).toHaveText('#');

    const privLock = privHashSlot.locator('svg.channel-lock-badge');
    await expect(privLock).toHaveCount(1);
    await expect(privLock).toHaveAttribute('data-private-lock', 'true');
    await expect(privLock).toHaveAttribute('aria-hidden', 'true');

    const hashBox = await privHashSlot.boundingBox();
    const lockBox = await privLock.boundingBox();
    expect(hashBox, 'hash slot bounding box').not.toBeNull();
    expect(lockBox, 'lock badge bounding box').not.toBeNull();

    const hash = hashBox!;
    const lock = lockBox!;

    // Width/height sanity — lock must be smaller than slot.
    expect(lock.width).toBeGreaterThan(0);
    expect(lock.height).toBeGreaterThan(0);
    expect(lock.width).toBeLessThan(hash.width + 2);
    expect(lock.height).toBeLessThan(hash.height + 2);

    // Horizontal overlap (lock's box intersects the hash slot horizontally).
    expect(lock.x + lock.width).toBeGreaterThan(hash.x);
    expect(lock.x).toBeLessThan(hash.x + hash.width);

    // Vertical overlap (lock's box intersects the hash slot vertically).
    expect(lock.y + lock.height).toBeGreaterThan(hash.y);
    expect(lock.y).toBeLessThan(hash.y + hash.height);

    // Lock center sits in the upper-right quadrant of the hash slot.
    const lockCx = lock.x + lock.width / 2;
    const lockCy = lock.y + lock.height / 2;
    const hashCx = hash.x + hash.width / 2;
    const hashCy = hash.y + hash.height / 2;
    expect(lockCx, 'lock center should be on the right half of hash slot').toBeGreaterThan(hashCx);
    expect(lockCy, 'lock center should be on the upper half of hash slot').toBeLessThan(hashCy);

    // Reverse-X: the literal CJK character "锁" must not be present in the sidebar.
    const lockChar = await sidebar.locator('text=锁').count();
    expect(lockChar, 'sidebar must not contain the "锁" CJK character (反 PR #952)').toBe(0);

    await page.screenshot({ path: path.join(SCREENSHOT_DIR, 'lock-badge-on-hash.png') });

    // -------------------- Scenario C: 归档私有 — 📦 base + lock overlay --------------------
    const archHashSlot = archivedRow.locator('.channel-hash[data-private-indicator="true"]');
    await expect(archHashSlot).toHaveCount(1);
    await expect(archHashSlot.locator('.channel-hash-base')).toHaveText('📦');

    const archLock = archHashSlot.locator('svg.channel-lock-badge');
    await expect(archLock).toHaveCount(1);

    const archHashBox = await archHashSlot.boundingBox();
    const archLockBox = await archLock.boundingBox();
    expect(archHashBox).not.toBeNull();
    expect(archLockBox).not.toBeNull();

    // Same overlap/quadrant invariants for the archived row.
    const ah = archHashBox!;
    const al = archLockBox!;
    expect(al.x + al.width).toBeGreaterThan(ah.x);
    expect(al.x).toBeLessThan(ah.x + ah.width);
    expect(al.y + al.height).toBeGreaterThan(ah.y);
    expect(al.y).toBeLessThan(ah.y + ah.height);
    expect(al.x + al.width / 2).toBeGreaterThan(ah.x + ah.width / 2);
    expect(al.y + al.height / 2).toBeLessThan(ah.y + ah.height / 2);

    await page.screenshot({ path: path.join(SCREENSHOT_DIR, 'archived-private-lock.png') });
  });
});
