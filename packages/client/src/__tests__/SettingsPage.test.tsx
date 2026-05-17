// SettingsPage.test.tsx — user 设置页 DOM 锁 (post-#975 privacy UI removal).
//
// 反约束:
//   - privacy tab 已删, 不应再出现 (#975).
//   - 默认 tab = runtime (用户进 Settings 最常见动机是看运行时状态).
//   - 仍保留 runtime + channels 两个 tab, 切换 UI 不变.
import React from 'react';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { createRoot } from 'react-dom/client';
import { act } from 'react';

import SettingsPage from '../components/Settings/SettingsPage';

let container: HTMLDivElement | null = null;

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
  vi.stubGlobal(
    'fetch',
    vi.fn().mockResolvedValue({
      ok: true,
      json: () =>
        Promise.resolve({
          user_id: 'user-1',
          permissions: [],
          details: [],
          capabilities: [],
        }),
    }),
  );
});

afterEach(() => {
  vi.unstubAllGlobals();
  if (container) {
    document.body.removeChild(container);
    container = null;
  }
});

async function render(node: React.ReactElement) {
  const root = createRoot(container!);
  await act(async () => {
    root.render(node);
  });
}

describe('SettingsPage — default runtime tab (#975 privacy UI removed)', () => {
  it('renders settings page with runtime tab active by default', async () => {
    await render(<SettingsPage onBack={() => {}} />);
    expect(container!.querySelector('[data-page="settings"]')).toBeTruthy();
    const runtimeTab = container!.querySelector('[data-tab="runtime"]');
    expect(runtimeTab).toBeTruthy();
    expect(runtimeTab!.className).toContain('active');
    expect(runtimeTab!.getAttribute('aria-current')).toBe('page');
  });

  it('privacy tab is not rendered (#975)', async () => {
    await render(<SettingsPage onBack={() => {}} />);
    expect(container!.querySelector('[data-tab="privacy"]')).toBeNull();
    // 反向 grep — 中文标签也不应出现.
    const tabsText = container!.querySelector('.settings-tabs')?.textContent ?? '';
    expect(tabsText).not.toContain('隐私');
  });

  it('does not embed the privacy/compliance UI markers (#975)', async () => {
    await render(<SettingsPage onBack={() => {}} />);
    expect(container!.querySelector('.privacy-promise')).toBeNull();
    expect(container!.querySelector('[data-section="impersonate-grant"]')).toBeNull();
    expect(container!.querySelector('[data-section="admin-actions-history"]')).toBeNull();
  });

  it('back button calls onBack handler', async () => {
    const onBack = vi.fn();
    await render(<SettingsPage onBack={onBack} />);
    const btn = container!.querySelector('.settings-back-btn') as HTMLButtonElement;
    expect(btn).toBeTruthy();
    act(() => {
      btn.click();
    });
    expect(onBack).toHaveBeenCalledTimes(1);
  });

  it('runtime tab renders PermissionsView empty state', async () => {
    await render(<SettingsPage onBack={() => {}} />);
    await act(async () => {
      await Promise.resolve();
    });

    expect(fetch).toHaveBeenCalledWith('/api/v1/me/permissions', { credentials: 'include' });
    expect(container!.querySelector('[data-ap2-empty]')?.textContent).toBe('暂无授权');
  });

  it('places Remote Nodes and Helper Status as separate runtime Settings entries', async () => {
    const onRemoteNodesOpen = vi.fn();
    const onHelperStatusOpen = vi.fn();
    await render(
      <SettingsPage
        onBack={() => {}}
        onRemoteNodesOpen={onRemoteNodesOpen}
        onHelperStatusOpen={onHelperStatusOpen}
      />,
    );

    const runtime = container!.querySelector('[data-settings-runtime-surface="true"]')!;
    const remoteEntry = runtime.querySelector('[data-runtime-entry="remote-nodes"]') as HTMLButtonElement;
    const helperEntry = runtime.querySelector('[data-runtime-entry="helper-status"]') as HTMLButtonElement;

    expect(remoteEntry).toBeTruthy();
    expect(remoteEntry.textContent).toContain('Remote Nodes');
    expect(remoteEntry.getAttribute('data-authority-rail')).toBe('remote-agent');
    expect(helperEntry).toBeTruthy();
    expect(helperEntry.textContent).toContain('Helper Status');
    expect(helperEntry.getAttribute('data-authority-rail')).toBe('helper-actuator');
    expect(runtime.textContent).not.toContain('Helper/Remote Nodes');

    await act(async () => {
      remoteEntry.click();
      helperEntry.click();
    });

    expect(onRemoteNodesOpen).toHaveBeenCalledTimes(1);
    expect(onHelperStatusOpen).toHaveBeenCalledTimes(1);
  });

  it('channels tab is still reachable next to runtime', async () => {
    await render(<SettingsPage onBack={() => {}} />);
    const channelsTab = container!.querySelector('[data-tab="channels"]') as HTMLButtonElement;
    expect(channelsTab).toBeTruthy();
    expect(channelsTab.textContent).toBe('频道');
  });
});
