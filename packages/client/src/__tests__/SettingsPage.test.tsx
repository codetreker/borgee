// SettingsPage.test.tsx — ADM-1 acceptance §2 SettingsPage DOM 锁.
//
// ADM-2 mock: SettingsPage 现在嵌入 ImpersonateGrantSection +
// AdminActionsList, 它们 mount 时调 lib/api fetch helpers; jsdom 没真
// fetch endpoint, 这里 mock 整个 module 防止 ERR_INVALID_URL unhandled
// rejection (CI client-vitest 看作 failure).
import React from 'react';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { createRoot } from 'react-dom/client';
import { act } from 'react';

vi.mock('../lib/api', () => ({
  getMyAdminActions: () => Promise.resolve({ actions: [] }),
  getMyImpersonateGrant: () => Promise.resolve({ grant: null }),
  createMyImpersonateGrant: () => Promise.resolve({ grant: null }),
  revokeMyImpersonateGrant: () => Promise.resolve(),
}));

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

describe('SettingsPage — privacy tab 默认展开不可折叠 (acceptance §2.1)', () => {
  it('renders settings page with privacy tab active by default', async () => {
    await render(<SettingsPage onBack={() => {}} />);
    expect(container!.querySelector('[data-page="settings"]')).toBeTruthy();
    const privacyTab = container!.querySelector('[data-tab="privacy"]');
    expect(privacyTab).toBeTruthy();
    expect(privacyTab!.className).toContain('active');
    expect(privacyTab!.getAttribute('aria-current')).toBe('page');
  });

  it('PrivacyPromise section is always visible (反 <details> 包裹)', async () => {
    await render(<SettingsPage onBack={() => {}} />);
    const promise = container!.querySelector('.privacy-promise');
    expect(promise).toBeTruthy();
    // No <details> wrapper anywhere in settings page (野马 R3 反约束).
    expect(container!.querySelectorAll('details')).toHaveLength(0);
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

  it('tab label "隐私" byte-identical (中文文案锁)', async () => {
    await render(<SettingsPage onBack={() => {}} />);
    const tab = container!.querySelector('[data-tab="privacy"]');
    expect(tab!.textContent).toBe('隐私');
  });

  it('renders standalone PermissionsView empty state from user Settings', async () => {
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

    await act(async () => {
      (container!.querySelector('[data-tab="runtime"]') as HTMLButtonElement).click();
    });

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
});
