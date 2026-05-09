// admin-spa-ui-coverage.test.tsx — REG-ASUC-001..006 + UserDetailPage UI render
//
// 设计: ADM-0 §1.3 admin god-mode 路径独立, CAPABILITY-DOT #628 14 const SSOT
// byte-identical, 0 server / 0 endpoint 改 (server 已挂 endpoint, 仅 client 接 UI).

import React from 'react';
import { describe, expect, test, beforeEach, vi } from 'vitest';
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

describe('ADMIN-SPA-UI-COVERAGE — REG-ASUC content-lock + DOM 锚 + 文案 byte-identical', () => {
  test('REG-ASUC-001 — api.ts exports fetchUserPermissions / grantUserPermission / revokeUserPermission', () => {
    const src = read('api.ts');
    expect(src).toMatch(/export async function fetchUserPermissions/);
    expect(src).toMatch(/export async function grantUserPermission/);
    expect(src).toMatch(/export async function revokeUserPermission/);
    // server endpoint path byte-identical (admin.go:39-41).
    expect(src).toMatch(/\/users\/\$\{encodeURIComponent\(id\)\}\/permissions/);
  });

  test('REG-ASUC-002 — UserPermissionDetail interface 字段 byte-identical 跟 server sanitize', () => {
    const src = read('api.ts');
    // server admin.go:393-403 returns {permission, scope, granted_at, granted_by?}
    expect(src).toMatch(/interface UserPermissionDetail/);
    expect(src).toMatch(/permission: string;/);
    expect(src).toMatch(/scope: string;/);
    expect(src).toMatch(/granted_at: number;/);
    expect(src).toMatch(/granted_by\?: string \| null;/);
  });

  test('REG-ASUC-003 — patchUser 扩 body 字段 (role + require_mention) 跟 server handleUpdateUser', () => {
    const src = read('api.ts');
    // server admin.go:205-211 accepts display_name/password/role/require_mention/disabled.
    expect(src).toMatch(/role\?: 'member' \| 'agent';/);
    expect(src).toMatch(/require_mention\?: boolean;/);
  });

  test('REG-ASUC-004 — UserDetailPage DOM 锚 byte-identical (data-asuc-* SSOT)', () => {
    const src = read('pages/UserDetailPage.tsx');
    // 9 DOM 锚 (grep 守门, 跟 admin-spa-shape-fix data-* 模式承袭).
    const anchors = [
      'data-page="admin-user-detail"',
      'data-asuc-action-msg',
      'data-asuc-account-actions',
      'data-asuc-password-input',
      'data-asuc-reset-password',
      'data-asuc-role-select',
      'data-asuc-set-role',
      'data-asuc-toggle-disabled',
      'data-asuc-grant-form',
      'data-asuc-capability-select',
      'data-asuc-scope-input',
      'data-asuc-grant-button',
      'data-asuc-permissions-list',
      'data-asuc-permission-row',
      'data-asuc-revoke-button',
    ];
    for (const a of anchors) {
      expect(src).toContain(a);
    }
  });

  test('REG-ASUC-005 — UserDetailPage 中文文案 byte-identical (content-lock §1)', () => {
    const src = read('pages/UserDetailPage.tsx');
    // content-lock 14 字面 (UI 锁字面, 改 = 改两处: content-lock + 此组件).
    expect(src).toContain('账号操作');
    expect(src).toContain('能力授权');
    expect(src).toContain('当前授权');
    expect(src).toContain('重置密码');
    expect(src).toContain('改角色');
    expect(src).toContain('账号状态');
    expect(src).toContain('启用账号');
    expect(src).toContain('停用账号');
    expect(src).toContain('授予');
    expect(src).toContain('撤销');
    expect(src).toContain('已授予');
    expect(src).toContain('已撤销');
    expect(src).toContain('暂无授权');
    expect(src).toContain('未知能力');
  });

  test('REG-ASUC-006 — UserDetailPage 走 CAPABILITY-DOT #628 14 const SSOT (反 hardcode)', () => {
    const src = read('pages/UserDetailPage.tsx');
    // grep 检查 — UserDetailPage 必从 lib/capabilities import, 不内嵌字面.
    expect(src).toMatch(/import.*CAPABILITY_TOKENS.*capabilityLabel.*isKnownCapability.*from.*lib\/capabilities/);
    // 反向: 反 hardcode 14 dot-notation 字面 in UserDetailPage (CAPABILITY_TOKENS 单源).
    const hardcodedTokens = src.match(/['"]channel\.read['"]|['"]artifact\.commit['"]|['"]user\.mention['"]/g);
    expect(hardcodedTokens).toBeNull();
  });

  test('REG-ASUC-007 — admin god-mode 路径独立 (ADM-0 §1.3 红线) — UserDetailPage 仅访问 /admin-api/*', () => {
    const src = read('pages/UserDetailPage.tsx');
    // 不直 fetch '/api/v1/' (user-rail), 走 admin api 模块.
    expect(src).not.toMatch(/fetch\(['"`]\/api\/v1/);
    // 也不 import user-rail api (反 cross-rail leak).
    expect(src).not.toMatch(/from ['"]\.\.\/\.\.\/lib\/api['"]/);
  });
});
