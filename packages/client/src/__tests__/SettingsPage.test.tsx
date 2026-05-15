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

describe('SettingsPage — privacy tab 默认展开不可折叠 (acceptance §2.1)', () => {
  it('renders settings page with privacy tab active by default', () => {
    render(<SettingsPage onBack={() => {}} />);
    expect(container!.querySelector('[data-page="settings"]')).toBeTruthy();
    const privacyTab = container!.querySelector('[data-tab="privacy"]');
    expect(privacyTab).toBeTruthy();
    expect(privacyTab!.className).toContain('active');
    expect(privacyTab!.getAttribute('aria-current')).toBe('page');
  });

  it('PrivacyPromise section is always visible (反 <details> 包裹)', () => {
    render(<SettingsPage onBack={() => {}} />);
    const promise = container!.querySelector('.privacy-promise');
    expect(promise).toBeTruthy();
    // No <details> wrapper anywhere in settings page (野马 R3 反约束).
    expect(container!.querySelectorAll('details')).toHaveLength(0);
  });

  it('back button calls onBack handler', () => {
    const onBack = vi.fn();
    render(<SettingsPage onBack={onBack} />);
    const btn = container!.querySelector('.settings-back-btn') as HTMLButtonElement;
    expect(btn).toBeTruthy();
    act(() => {
      btn.click();
    });
    expect(onBack).toHaveBeenCalledTimes(1);
  });

  it('tab label "隐私" byte-identical (中文文案锁)', () => {
    render(<SettingsPage onBack={() => {}} />);
    const tab = container!.querySelector('[data-tab="privacy"]');
    expect(tab!.textContent).toBe('隐私');
  });
});
