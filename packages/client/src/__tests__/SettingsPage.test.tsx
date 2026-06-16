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
import { NavigationProvider, useNavigation } from '../components/Navigation/NavigationContext';

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
    root.render(<NavigationProvider initial="settings">{node}</NavigationProvider>);
  });
}

// 用 ref probe 直接拿到栈内 nav 实例, 验证 push 后栈顶动.
function NavProbe({ navRef }: { navRef: { current: ReturnType<typeof useNavigation> | null } }) {
  navRef.current = useNavigation();
  return null;
}

describe('SettingsPage — default runtime tab (#975 privacy UI removed)', () => {
  it('renders settings page with runtime tab active by default', async () => {
    await render(<SettingsPage />);
    expect(container!.querySelector('[data-page="settings"]')).toBeTruthy();
    const runtimeTab = container!.querySelector('[data-tab="runtime"]');
    expect(runtimeTab).toBeTruthy();
    expect(runtimeTab!.className).toContain('active');
    expect(runtimeTab!.getAttribute('aria-current')).toBe('page');
  });

  it('privacy tab is not rendered (#975)', async () => {
    await render(<SettingsPage />);
    expect(container!.querySelector('[data-tab="privacy"]')).toBeNull();
    // 反向 grep — 中文标签也不应出现.
    const tabsText = container!.querySelector('.settings-tabs')?.textContent ?? '';
    expect(tabsText).not.toContain('隐私');
  });

  it('does not embed the privacy/compliance UI markers (#975)', async () => {
    await render(<SettingsPage />);
    expect(container!.querySelector('.privacy-promise')).toBeNull();
    expect(container!.querySelector('[data-section="impersonate-grant"]')).toBeNull();
    expect(container!.querySelector('[data-section="admin-actions-history"]')).toBeNull();
  });

  it('back button calls nav.back', async () => {
    const navRef: { current: ReturnType<typeof useNavigation> | null } = { current: null };
    const root = createRoot(container!);
    await act(async () => {
      root.render(
        <NavigationProvider initial="channel">
          <NavProbe navRef={navRef} />
          <PushAndRender />
        </NavigationProvider>,
      );
    });
    // PushAndRender 已 push 'settings' 进栈, 现在栈 = [channel, settings], canGoBack = true
    expect(navRef.current!.current).toBe('settings');
    expect(navRef.current!.canGoBack).toBe(true);

    const btn = container!.querySelector('.page-header-back') as HTMLButtonElement;
    expect(btn).toBeTruthy();
    await act(async () => {
      btn.click();
    });
    expect(navRef.current!.current).toBe('channel');
  });

  it('runtime tab renders PermissionsView empty state', async () => {
    await render(<SettingsPage />);
    await act(async () => {
      await Promise.resolve();
    });

    expect(fetch).toHaveBeenCalledWith('/api/v1/me/permissions', { credentials: 'include' });
    expect(container!.querySelector('[data-ap2-empty]')?.textContent).toBe('暂无授权');
  });

  it('renders single Remote Nodes runtime entry, no Helper Status (t2)', async () => {
    const navRef: { current: ReturnType<typeof useNavigation> | null } = { current: null };
    const root = createRoot(container!);
    await act(async () => {
      root.render(
        <NavigationProvider initial="settings">
          <NavProbe navRef={navRef} />
          <SettingsPage />
        </NavigationProvider>,
      );
    });

    const runtime = container!.querySelector('[data-settings-runtime-surface="true"]')!;
    const remoteEntry = runtime.querySelector('[data-runtime-entry="remote-nodes"]') as HTMLButtonElement;

    // Remote Nodes entry present (EV-3 getByText('Remote Nodes') equivalent).
    expect(remoteEntry).toBeTruthy();
    expect(remoteEntry.textContent).toContain('Remote Nodes');
    expect(remoteEntry.getAttribute('data-authority-rail')).toBe('remote-agent');

    // Helper Status entry gone (EV-3 queryByText('Helper Status') === null equivalent).
    expect(runtime.querySelector('[data-runtime-entry="helper-status"]')).toBeNull();
    expect(runtime.textContent).not.toContain('Helper Status');

    // Runtime section now has exactly one entry (AC-3).
    expect(runtime.querySelectorAll('.settings-runtime-entry')).toHaveLength(1);

    await act(async () => {
      remoteEntry.click();
    });
    expect(navRef.current!.current).toBe('remote-nodes');
  });

  it('channels tab is still reachable next to runtime', async () => {
    await render(<SettingsPage />);
    const channelsTab = container!.querySelector('[data-tab="channels"]') as HTMLButtonElement;
    expect(channelsTab).toBeTruthy();
    expect(channelsTab.textContent).toBe('频道');
  });

  // RM-2 page-level scope widening (#975 skeptic-owner WARN-3): the prior
  // assertion scoped to `.settings-tabs` would miss a stray '隐私' in the
  // page header, breadcrumb, aria-label, or side content. Walk the whole
  // `[data-page="settings"]` root and assert none of the deleted Chinese
  // labels appear anywhere. Labels harvested from the deleted components:
  //   - PrivacyPromise.tsx: <h2>"隐私承诺"</h2>
  //   - AdminActionsList.tsx: <h3>"admin 对你的影响记录"</h3>
  //     + "(最近 50 条)" / "从未被 admin 影响过 — 你的隐私边界完整。"
  it('no deleted privacy/admin-actions labels appear anywhere on SettingsPage (#975 page-level)', async () => {
    await render(<SettingsPage />);
    const pageRoot = container!.querySelector('[data-page="settings"]');
    expect(pageRoot).toBeTruthy();
    const pageText = pageRoot!.textContent ?? '';
    for (const label of ['隐私', '隐私承诺', '影响记录', '你的影响记录']) {
      expect(pageText).not.toContain(label);
    }
  });
});

// AP-2 #970 — BundleSelector is wired into the runtime-tab permissions
// section, and onConfirm fans out one PUT /api/v1/permissions per selected
// capability (caller-driven; no bundle endpoint).
describe('SettingsPage — BundleSelector wiring (#970)', () => {
  interface FetchCall {
    url: string;
    method: string;
    body: unknown;
  }
  let calls: FetchCall[] = [];

  beforeEach(() => {
    calls = [];
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation((url: string, init?: RequestInit) => {
        const method = (init?.method ?? 'GET').toUpperCase();
        calls.push({
          url,
          method,
          body: init?.body ? JSON.parse(init.body as string) : undefined,
        });
        if (method === 'PUT') {
          return Promise.resolve({ ok: true, status: 200, json: () => Promise.resolve({ granted: true }) });
        }
        return Promise.resolve({
          ok: true,
          json: () =>
            Promise.resolve({ user_id: 'user-1', permissions: [], details: [], capabilities: [] }),
        });
      }),
    );
  });

  it('renders BundleSelector inside the mounted runtime permissions surface', async () => {
    await render(<SettingsPage />);
    const surface = container!.querySelector('[data-settings-permissions-surface="true"]');
    expect(surface).toBeTruthy();
    // BundleSelector mounted next to PermissionsView in the same surface.
    expect(surface!.querySelector('[data-ap2-bundle-selector]')).toBeTruthy();
    // PermissionsView still present in the same surface.
    expect(surface!.querySelector('[data-ap2-empty],[data-ap2-permissions-view],[data-ap2-loading]')).toBeTruthy();
  });

  it('onConfirm dispatches one PUT /api/v1/permissions per selected capability', async () => {
    await render(<SettingsPage />);

    // Expand the reader bundle (3 capabilities: channel.read/artifact.read/dm.read).
    const expand = container!.querySelector(
      '[data-ap2-bundle-expand][data-bundle-name="reader"]',
    ) as HTMLButtonElement;
    expect(expand).toBeTruthy();
    await act(async () => {
      expand.click();
    });

    // Uncheck one capability so the fan-out grants the remaining two.
    const checkboxes = container!.querySelectorAll('[data-ap2-bundle-checkbox]');
    expect(checkboxes.length).toBe(3);
    await act(async () => {
      (checkboxes[0] as HTMLInputElement).click();
    });

    const confirm = container!.querySelector('[data-ap2-bundle-confirm]') as HTMLButtonElement;
    expect(confirm).toBeTruthy();
    await act(async () => {
      confirm.click();
    });
    await act(async () => {
      await Promise.resolve();
      await Promise.resolve();
    });

    const puts = calls.filter((c) => c.method === 'PUT' && c.url === '/api/v1/permissions');
    // 3 in the bundle minus 1 unchecked == 2 grants.
    expect(puts.length).toBe(2);
    for (const p of puts) {
      expect((p.body as { permission: string }).permission).toBeTruthy();
      expect((p.body as { scope: string }).scope).toBe('*');
    }
  });

  it('surfaces a forbidden state when the grant PUT returns 403', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation((url: string, init?: RequestInit) => {
        const method = (init?.method ?? 'GET').toUpperCase();
        if (method === 'PUT') {
          return Promise.resolve({ ok: false, status: 403, json: () => Promise.resolve({}) });
        }
        return Promise.resolve({
          ok: true,
          json: () =>
            Promise.resolve({ user_id: 'user-1', permissions: [], details: [], capabilities: [] }),
        });
      }),
    );

    await render(<SettingsPage />);
    const expand = container!.querySelector(
      '[data-ap2-bundle-expand][data-bundle-name="mention"]',
    ) as HTMLButtonElement;
    await act(async () => {
      expand.click();
    });
    const confirm = container!.querySelector('[data-ap2-bundle-confirm]') as HTMLButtonElement;
    await act(async () => {
      confirm.click();
    });
    await act(async () => {
      await Promise.resolve();
      await Promise.resolve();
    });

    expect(container!.querySelector('[data-ap2-bundle-forbidden]')).toBeTruthy();
    expect(container!.querySelector('[data-ap2-bundle-error]')).toBeNull();
  });
});


// Helper component — useEffect 触发 push('settings') 让栈 = [channel, settings].
function PushAndRender() {
  const nav = useNavigation();
  React.useEffect(() => {
    nav.push('settings');
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
  if (nav.current !== 'settings') return null;
  return <SettingsPage />;
}
