// AdminActionsList.test.tsx — ADM-2.2 acceptance §行为不变量 4.1.c +
// content-lock §4 字面锁.
//
// Pins:
//   - `[data-section="admin-actions-history"]` 始终渲染
//   - 空数据 → 空态字面 byte-identical 跟 content-lock §4 同源
//   - 有数据 → 每行 `[data-action-row]` + 中文动词字面 byte-identical
//   - 反向: 不渲染 raw UUID actor_id (sanitizeAdminAction admin_view=false
//     已 server 端 omitted, client 兜底也不能渲染)
import React from 'react';
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { createRoot } from 'react-dom/client';
import { act } from 'react';
import AdminActionsList from '../components/Settings/AdminActionsList';

let container: HTMLDivElement | null = null;

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
});

afterEach(() => {
  if (container) {
    document.body.removeChild(container);
    container = null;
  }
});

function render(node: React.ReactElement) {
  const root = createRoot(container!);
  act(() => {
    root.render(node);
  });
}

async function flushPromises() {
  await act(async () => {
    await new Promise((r) => setTimeout(r, 0));
  });
}

describe('AdminActionsList — ADM-2 影响记录 (acceptance §行为不变量 4.1.c)', () => {
  it('renders empty state with content-lock §4 byte-identical 字面', async () => {
    render(<AdminActionsList fetchActions={async () => []} />);
    await flushPromises();
    const section = container!.querySelector('[data-section="admin-actions-history"]');
    expect(section).not.toBeNull();
    const empty = container!.querySelector('.admin-actions-empty');
    expect(empty?.textContent).toBe('从未被 admin 影响过 — 你的隐私边界完整。');
  });

  it('renders rows with 5 action verbs byte-identical 跟 content-lock §4', async () => {
    render(
      <AdminActionsList
        fetchActions={async () => [
          {
            id: 'a1',
            target_user_id: 'u1',
            action: 'delete_channel',
            metadata: '{}',
            created_at: 1700000000000,
          },
          {
            id: 'a2',
            target_user_id: 'u1',
            action: 'reset_password',
            metadata: '{}',
            created_at: 1700100000000,
          },
        ]}
      />,
    );
    await flushPromises();
    const rows = container!.querySelectorAll('[data-action-row]');
    expect(rows.length).toBe(2);
    const text = container!.textContent ?? '';
    expect(text).toContain('删除了你的 channel');
    expect(text).toContain('重置了你的登录密码');
  });

  it('does not render raw actor_id even if accidentally provided (反约束 ADM2-NEG-001)', async () => {
    render(
      <AdminActionsList
        fetchActions={async () => [
          {
            id: 'a1',
            target_user_id: 'u1',
            action: 'delete_channel',
            metadata: '{}',
            created_at: 1700000000000,
            // @ts-ignore — server 不返 actor_id 给 user-rail, 此测试断
            // 言即使填了客户端也不渲染. (用 ts-ignore 不 expect-error 因 actor_id 字段无类型存在)
            actor_id: 'admin-uuid-leak',
          },
        ]}
      />,
    );
    await flushPromises();
    const text = container!.textContent ?? '';
    expect(text).not.toContain('admin-uuid-leak');
  });
});
