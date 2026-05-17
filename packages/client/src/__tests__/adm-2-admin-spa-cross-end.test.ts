// adm-2-admin-spa-cross-end.test.ts — ADM-2.2 admin SPA audit-log content lock.
//
// Post-#975 update: the user-side audit list was deleted (parent #654 stance:
// 现有合规代码要清掉). Cross-end literal lock now only enforces the admin SPA
// half (AdminAuditLogPage renders English enum actions verbatim and exposes
// the DOM anchors the admin e2e relies on).
//
// Spec: docs/qa/adm-2-content-lock.md §5 (admin SPA literal lock) +
// docs/current/admin/README.md §6 (admin-rail audit-log).

import { describe, it, expect } from 'vitest';
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
const ADMIN_PAGE = nodePath.join(HERE, '..', 'admin', 'pages', 'AdminAuditLogPage.tsx');

function read(file: string): string {
  return fs.readFileSync(file, 'utf-8');
}

// 5 enum action — server admin_actions CHECK constraint byte-identical.
const ACTION_ENUM = [
  'delete_channel',
  'suspend_user',
  'change_role',
  'reset_password',
  'start_impersonation',
];

describe('ADM-2.2 admin SPA audit-log literal lock', () => {
  const adminPage = read(ADMIN_PAGE);

  it('admin SPA 含 5 英文 enum action 字面 byte-identical', () => {
    for (const action of ACTION_ENUM) {
      expect(adminPage).toContain(`'${action}'`);
    }
  });

  it('admin SPA DOM 锚: data-page / data-action-row / data-filter 三 attr', () => {
    expect(adminPage).toContain('data-page="admin-audit-log"');
    expect(adminPage).toContain('data-action-row');
    expect(adminPage).toContain('data-filter="actor"');
    expect(adminPage).toContain('data-filter="action"');
    expect(adminPage).toContain('data-filter="target"');
  });

  it('admin SPA 渲染 actor_id (设计 ③ admin 互可见)', () => {
    expect(adminPage).toContain('row.actor_id');
  });

  it('ACTION_ENUM 长度 = 5 (加 enum 时必须改 server CHECK + admin SPA 同步)', () => {
    expect(ACTION_ENUM.length).toBe(5);
  });
});
