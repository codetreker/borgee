// adm-2-admin-spa-cross-end.test.ts — ADM-2.2 跨端字面拆死 content-lock.
//
// Spec: docs/qa/adm-2-content-lock.md §5 (admin SPA 跨端字面拆死) +
// docs/current/admin/README.md §6 (admin-rail audit-log).
// Blueprint: docs/blueprint/current/admin-model.md §1.4 红线 (admin/user 路径分叉,
// ADM-0 §1.3 同模式).
//
// 跨端字面拆死立场 (ADM-2 NEG-010):
//   - admin SPA AdminAuditLogPage 走英文 enum action 字面 (delete_channel /
//     suspend_user / change_role / reset_password / start_impersonation)
//   - 用户 SPA Settings/AdminActionsList 走中文动词字面 (ACTION_VERBS map
//     `delete_channel: '删除了你的 channel'` 等)
//   - 改 enum = 改 server admin_actions CHECK constraint + 此 admin SPA +
//     用户 SPA AdminActionsList 三处同步
//
// DOM 锚 (e2e 反查):
//   - admin SPA: `[data-page="admin-audit-log"]` + 每行 `[data-action-row]`
//     + 每 filter `[data-filter="{actor|action|target}"]`
//   - 用户 SPA: `[data-section="admin-actions-history"]` (已锁)
//
// 反约束:
//   - admin SPA 不渲染中文动词 (admin 视角看英文 enum 直查)
//   - 用户 SPA 不渲染 actor_id raw (走 admin lookup 翻 admin_username)

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
const USER_PAGE = nodePath.join(HERE, '..', 'components', 'Settings', 'AdminActionsList.tsx');

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

// 5 中文动词 — 用户 SPA AdminActionsList ACTION_VERBS byte-identical.
const ACTION_VERBS_ZH = [
  '删除了你的 channel',
  '暂停了你的账号',
  '调整了你的账号角色',
  '重置了你的登录密码',
  '开启了对你账号的 24h impersonate',
];

describe('ADM-2.2 跨端字面拆死 (admin SPA vs user SPA)', () => {
  const adminPage = read(ADMIN_PAGE);
  const userPage = read(USER_PAGE);

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

  it('admin SPA 不渲染用户端中文动词 (跨端字面拆死)', () => {
    // 反约束: admin 视角看英文 enum 直查, 不混中文动词字面.
    for (const verb of ACTION_VERBS_ZH) {
      expect(adminPage).not.toContain(verb);
    }
  });

  it('user SPA 含 5 中文动词字面 byte-identical', () => {
    for (const verb of ACTION_VERBS_ZH) {
      expect(userPage).toContain(verb);
    }
  });

  it('user SPA 不渲染 actor_id raw 字段 (立场 ④ user 只见自己, actor 走 admin lookup)', () => {
    // 反约束: user SPA AdminActionsList 不读 actor_id 字段 (server sanitize
    // adminView=false 不返此字段, client 也不应假设它存在).
    // 反向 grep `actor_id` literal 在 user page 0 hit (除注释里说 actor_id
    // 被 server 故意省去).
    // 我们用更严格的检查: AdminActionRow type 不含 actor_id 属性访问.
    expect(userPage).not.toContain('row.actor_id');
    expect(userPage).not.toContain('.actor_id;');
    expect(userPage).not.toContain('"actor_id"');
  });

  it('admin SPA 渲染 actor_id (立场 ③ admin 互可见)', () => {
    // admin SPA 显式读 actor_id (UUID 字符串, 走 <code> 渲染).
    expect(adminPage).toContain('row.actor_id');
  });

  it('admin/user 同源 enum 长度 = 5 (ACTION_ENUM byte-identical 跟 user ACTION_VERBS map keys 同长)', () => {
    // 锁两端 enum 数量同步 — 加 enum 时必须改两端.
    expect(ACTION_ENUM.length).toBe(5);
    expect(ACTION_VERBS_ZH.length).toBe(5);
    // 验 user SPA ACTION_VERBS map 含 5 keys (字面承袭).
    for (const action of ACTION_ENUM) {
      expect(userPage).toContain(`${action}: '`);
    }
  });
});
