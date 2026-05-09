// admin-spa-archived-ui-followup.test.ts — REG-ASAUI-001..005 file-source
// content lock — AL-8 §0 设计 ③ archived 三态 UI 真兑现 (#633 D4-A 漏的
// client filter UI 真补).
//
// 反约束 (file-source byte-identical 守门, 跟 admin-api-shape.test.ts 同模式):
//   - AdminAuditLogPage.tsx 必含 data-filter="archived" select + 3 option
//     ("active" / "archived" / "all") byte-identical 跟 server enum 同源
//   - data-archived-state row attr + admin-audit-row-{active,archived} className
//     已在 #633 加 — grep 检查 守 不破
//   - api.ts AuditLogFilters interface 必含 archived?: 'active'|'archived'|'all'
//   - fetchAdminAuditLog 必传 ?archived= URLSearchParams (跟 server query
//     param byte-identical, drift = 改两处)
//   - 反约束: 0 server / 0 endpoint URL 改 (本 PR 仅 client UI 真兑现)

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

describe('ADMIN-SPA-ARCHIVED-UI-FOLLOWUP — AL-8 §0 设计③ 三态 UI 真兑现', () => {
  test('REG-ASAUI-001 — AdminAuditLogPage 加 data-filter="archived" select + 3 option byte-identical', () => {
    const src = read('pages/AdminAuditLogPage.tsx');
    expect(src).toMatch(/data-filter="archived"/);
    // 3 option value byte-identical 跟 server enum 同源 (admin_endpoints.go).
    expect(src).toMatch(/<option\s+value="active">Active<\/option>/);
    expect(src).toMatch(/<option\s+value="archived">Archived<\/option>/);
    expect(src).toMatch(/<option\s+value="all">All<\/option>/);
  });

  test('REG-ASAUI-002 — row 三态 data-archived-state attr + className 不破 (#633 已加)', () => {
    const src = read('pages/AdminAuditLogPage.tsx');
    expect(src).toMatch(/data-archived-state=\{row\.archived_at\s*!=\s*null\s*\?\s*'archived'\s*:\s*'active'\}/);
    expect(src).toMatch(/admin-audit-row-archived/);
    expect(src).toMatch(/admin-audit-row-active/);
  });

  test('REG-ASAUI-003 — api.ts AuditLogFilters 加 archived?: union 三态', () => {
    const src = read('api.ts');
    expect(src).toMatch(/archived\?\:\s*'active'\s*\|\s*'archived'\s*\|\s*'all'/);
  });

  test('REG-ASAUI-004 — fetchAdminAuditLog 加 archived URL param', () => {
    const src = read('api.ts');
    expect(src).toMatch(/qs\.set\(\s*'archived'\s*,\s*filters\.archived\s*\)/);
  });

  test('REG-ASAUI-005 — admin-audit-archived-row className 不漂入其他 page (反 cross-page 漏)', () => {
    // grep 检查: 这 className 只该在 AdminAuditLogPage.tsx 内 (避免 cross
    // contamination 到别的 admin page).
    const adminApp = read('AdminApp.tsx');
    expect(adminApp).not.toMatch(/admin-audit-row-archived/);
  });
});
