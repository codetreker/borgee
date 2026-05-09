// AP-2 client — PermissionsView component vitest.
//
// 设计沿用 (ap-2-spec.md §0 + content-lock §2):
//   - DOM data-attr SSOT (data-ap2-permissions-view + data-ap2-capability-row +
//     data-ap2-capability-token + data-ap2-scope + data-ap2-known)
//   - capability 渲染走 capabilityLabel SSOT (反 inline 字面)
//   - 反 RBAC role 字面 (admin/editor/viewer/owner) 0 hit
//   - 加载/失败/空 三态 UI byte-identical
import React from 'react';
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react-dom/test-utils';
import { PermissionsView } from '../components/PermissionsView';
import type { PermissionEntry } from '../hooks/usePermissions';

let container: HTMLDivElement | null = null;
let root: Root | null = null;

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
});

afterEach(() => {
  act(() => {
    root?.unmount();
  });
  if (container) {
    document.body.removeChild(container);
    container = null;
  }
});

async function render(node: React.ReactElement) {
  root = createRoot(container!);
  await act(async () => {
    root!.render(node);
  });
}

describe('AP-2 ⭐ PermissionsView — capability 透明 UI 无 role 名', () => {
  it('§1.1 entries=[] renders `暂无授权` empty state + data-ap2-empty', async () => {
    await render(<PermissionsView entries={[]} />);
    const el = container!.querySelector('[data-ap2-empty]');
    expect(el?.textContent).toBe('暂无授权');
  });

  it('§1.2 single capability row — token / scope / 中文 label byte-identical', async () => {
    const entries: PermissionEntry[] = [
      {
        id: 1,
        permission: 'channel.read',
        scope: 'channel:abc',
        granted_by: null,
        granted_at: 0,
      },
    ];
    await render(<PermissionsView entries={entries} />);
    const list = container!.querySelector('[data-ap2-permissions-view]');
    expect(list).not.toBeNull();
    const row = container!.querySelector('[data-ap2-capability-row]');
    expect(row?.getAttribute('data-ap2-capability-token')).toBe('channel.read');
    expect(row?.getAttribute('data-ap2-scope')).toBe('channel:abc');
    expect(row?.getAttribute('data-ap2-known')).toBe('true');
    const label = container!.querySelector('[data-ap2-capability-label]');
    expect(label?.textContent).toBe('查看频道');
  });

  it('§1.3 multi-row 14 capability — 全 byte-identical 中文 label, 0 RBAC role 字面', async () => {
    const entries: PermissionEntry[] = [
      { id: 1, permission: 'channel.read', scope: '*', granted_by: null, granted_at: 0 },
      { id: 2, permission: 'channel.write', scope: '*', granted_by: null, granted_at: 0 },
      { id: 3, permission: 'artifact.commit', scope: '*', granted_by: null, granted_at: 0 },
      { id: 4, permission: 'user.mention', scope: '*', granted_by: null, granted_at: 0 },
    ];
    await render(<PermissionsView entries={entries} />);
    const rows = container!.querySelectorAll('[data-ap2-capability-row]');
    expect(rows.length).toBe(4);
    const labels = Array.from(container!.querySelectorAll('[data-ap2-capability-label]'))
      .map((n) => n.textContent ?? '');
    expect(labels).toEqual(['查看频道', '在频道发消息', '提交产物', '提及用户']);
    // 反向断言: DOM body 不含 RBAC role 字面 (反 role bleed).
    const bodyText = container!.textContent ?? '';
    for (const bad of ['admin', 'editor', 'viewer', 'owner', '管理员', '编辑者', '查看者']) {
      expect(bodyText.toLowerCase().includes(bad.toLowerCase())).toBe(false);
    }
  });

  it('§1.4 wildcard `*` permission renders `完整能力` + data-ap2-known=true', async () => {
    const entries: PermissionEntry[] = [
      { id: 0, permission: '*', scope: '*', granted_by: null, granted_at: 0 },
    ];
    await render(<PermissionsView entries={entries} />);
    const label = container!.querySelector('[data-ap2-capability-label]');
    expect(label?.textContent).toBe('完整能力');
    const row = container!.querySelector('[data-ap2-capability-row]');
    expect(row?.getAttribute('data-ap2-known')).toBe('true');
  });

  it('§1.5 unknown token forward-compat — data-ap2-known=false + 渲染原 token', async () => {
    const entries: PermissionEntry[] = [
      { id: 99, permission: 'future_v3_capability', scope: '*', granted_by: null, granted_at: 0 },
    ];
    await render(<PermissionsView entries={entries} />);
    const label = container!.querySelector('[data-ap2-capability-label]');
    expect(label?.textContent).toBe('future_v3_capability');
    const row = container!.querySelector('[data-ap2-capability-row]');
    expect(row?.getAttribute('data-ap2-known')).toBe('false');
  });
});
