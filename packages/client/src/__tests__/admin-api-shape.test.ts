// admin-api-shape.test.ts — ADMIN-SPA-SHAPE-FIX REG-ASF-D1..D5 file-source
// content lock (跟 adm-2-followup-audit-page.test.tsx + adm-2-admin-spa-cross-end.test.ts
// 同模式 — 文件源字面 reverse-grep, 守 6 drift).
//
// 反约束: client/admin/{api,auth,AdminApp,SettingsPage,LoginPage,pages/...}.tsx
// 必含 byte-identical fixed shape, 反 next-time silent drift.

import { describe, expect, test } from 'vitest';
// @ts-expect-error — node:module 没 @types/node, vitest node 上下文可达.
import { createRequire } from 'module';

const nodeRequire = createRequire(import.meta.url);
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const fs: any = nodeRequire('fs');
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const nodePath: any = nodeRequire('path');
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const url: any = nodeRequire('url');

const HERE = nodePath.dirname(url.fileURLToPath(import.meta.url));
const ADMIN_DIR = nodePath.resolve(HERE, '../admin');

function read(p: string): string {
  return fs.readFileSync(nodePath.join(ADMIN_DIR, p), 'utf-8');
}

describe('ADMIN-SPA-SHAPE-FIX — D1..D5 client shape lock', () => {
  test('REG-ASF-D1.body — adminLogin 真发 {login, password} body byte-identical 跟 server loginRequest', () => {
    const src = read('api.ts');
    expect(src).toMatch(/JSON\.stringify\(\s*\{\s*login\s*,\s*password\s*\}\s*\)/);
    // 反向: 不再有 {username, password} 字面.
    expect(src).not.toMatch(/JSON\.stringify\(\s*\{\s*username/);
  });

  test('REG-ASF-D1.sig — adminLogin sig 走 login: string 不走 username: string', () => {
    const src = read('api.ts');
    expect(src).toMatch(/adminLogin\s*\(\s*login:\s*string\s*,\s*password:\s*string\s*\)/);
    expect(src).not.toMatch(/adminLogin\s*\(\s*username:\s*string/);
  });

  test('REG-ASF-D2 — AdminSession interface byte-identical 跟 server handleMe writeJSON {id, login}', () => {
    const src = read('api.ts');
    // 必含 id + login 字段.
    expect(src).toMatch(/export\s+interface\s+AdminSession\s*\{[\s\S]*?id:\s*string;[\s\S]*?login:\s*string;[\s\S]*?\}/);
    // 反向: 不再有 role / username / admin_id / expires_at 字面 (D2 真值修订).
    const adminBlock = src.match(/export\s+interface\s+AdminSession\s*\{[^}]*\}/)?.[0] ?? '';
    expect(adminBlock).not.toMatch(/role:\s*['"]admin['"]/);
    expect(adminBlock).not.toMatch(/username:/);
    expect(adminBlock).not.toMatch(/admin_id:/);
    expect(adminBlock).not.toMatch(/expires_at:/);
  });

  test('REG-ASF-D3 — AdminChannel.member_count 死字段真删 (server 不返)', () => {
    const src = read('api.ts');
    // 反向: AdminChannel interface 内不再有 member_count 字段声明 (注释里允许提及).
    const channelBlock = src.match(/export\s+interface\s+AdminChannel\s*\{[^}]*\}/)?.[0] ?? '';
    expect(channelBlock).not.toMatch(/member_count\?\:\s*number/);
    // ChannelsPage.tsx 不再渲染 channel.member_count.
    expect(read('pages/ChannelsPage.tsx')).not.toMatch(/channel\.member_count/);
  });

  test('REG-ASF-D4 — AdminActionRow.archived_at 真补 (AL-8 §0 设计③ 三态)', () => {
    const src = read('api.ts');
    expect(src).toMatch(/archived_at\?:\s*number\s*\|\s*null/);
  });

  test('REG-ASF-D5 — InviteCode.note 收紧 string non-null (server 真返非 null)', () => {
    const src = read('api.ts');
    // 反向: 不再有 note?: string | null.
    expect(src).not.toMatch(/note\?\:\s*string\s*\|\s*null/);
    // 必含 note: string non-null.
    expect(src).toMatch(/note:\s*string;/);
  });

  test('REG-ASF-callsite — auth.ts/AdminApp.tsx/SettingsPage.tsx 0 username 字面 (post-D2)', () => {
    expect(read('auth.ts')).not.toMatch(/username/);
    expect(read('AdminApp.tsx')).not.toMatch(/session\?\.username/);
    expect(read('pages/SettingsPage.tsx')).not.toMatch(/session\?\.username/);
    expect(read('pages/SettingsPage.tsx')).not.toMatch(/session\?\.role/);
  });

  test('REG-ASF-LoginPage — form state 走 login 不走 username (D1 byte-identical)', () => {
    const src = read('pages/LoginPage.tsx');
    expect(src).toMatch(/setLogin\b/);
    expect(src).not.toMatch(/setUsername\b/);
    // UI label "Login" 而不是 "Username" (跟 server SSOT byte-identical).
    expect(src).toMatch(/>\s*Login\s*</);
    expect(src).not.toMatch(/>\s*Username\s*</);
  });

  // RM-5 (#975) — AdminAuditLogPage import-source lock. The admin SPA must
  // import `fetchAdminAuditLog` from the admin-rail `../api` (admin/api.ts),
  // NOT from the user-rail `../../lib/api`. A "re-consolidation accident"
  // that points the admin page back at lib/api would re-couple the two
  // rails the #975 cleanup separated.
  test('REG-ASF-RM5 — AdminAuditLogPage imports fetchAdminAuditLog from admin/api (#975)', () => {
    const src = read('pages/AdminAuditLogPage.tsx');
    // Positive: import from '../api'.
    expect(src).toMatch(/import\s*\{[^}]*\bfetchAdminAuditLog\b[^}]*\}\s*from\s*['"]\.\.\/api['"]/);
    // Negative: no import from the user-rail lib/api.
    expect(src).not.toMatch(/from\s*['"]\.\.\/\.\.\/lib\/api['"]/);
    expect(src).not.toMatch(/from\s*['"]src\/lib\/api['"]/);
    expect(src).not.toContain('lib/api');
  });
});
