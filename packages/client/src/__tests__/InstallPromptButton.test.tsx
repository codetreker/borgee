// CS-3.2 — InstallPromptButton 单测 (cs-3-stance-checklist 设计 ① + content-lock §2).
import React from 'react';
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react-dom/test-utils';
import InstallPromptButton from '../components/InstallPromptButton';

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

describe('CS-3.2 — InstallPromptButton (PWA install UI)', () => {
  it('TestCS32_HiddenWhenUnavailable — 无 beforeinstallprompt event → 不渲染', async () => {
    await render(<InstallPromptButton />);
    expect(container!.querySelector('[data-cs3-install-button]')).toBeNull();
  });

  it('TestCS32_RendersWhenInstallable — beforeinstallprompt event → 渲染 byte-identical label', async () => {
    await render(<InstallPromptButton />);
    const fakeEvent = new Event('beforeinstallprompt');
    Object.assign(fakeEvent, {
      prompt: () => Promise.resolve(),
      userChoice: Promise.resolve({ outcome: 'accepted', platform: 'web' }),
      platforms: ['web'],
    });
    await act(async () => {
      window.dispatchEvent(fakeEvent);
    });
    const btn = container!.querySelector('[data-cs3-install-button]') as HTMLButtonElement | null;
    expect(btn).toBeTruthy();
    expect(btn!.textContent).toBe('安装 Borgee 桌面应用');
    expect(btn!.getAttribute('data-install-state')).toBe('installable');
  });

  it('TestCS32_HiddenWhenInstalled — appinstalled event → 隐藏', async () => {
    await render(<InstallPromptButton />);
    // 先 installable
    const fakeEvent = new Event('beforeinstallprompt');
    Object.assign(fakeEvent, {
      prompt: () => Promise.resolve(),
      userChoice: Promise.resolve({ outcome: 'accepted', platform: 'web' }),
      platforms: ['web'],
    });
    await act(async () => {
      window.dispatchEvent(fakeEvent);
    });
    expect(container!.querySelector('[data-cs3-install-button]')).toBeTruthy();
    // 再 appinstalled
    await act(async () => {
      window.dispatchEvent(new Event('appinstalled'));
    });
    expect(container!.querySelector('[data-cs3-install-button]')).toBeNull();
  });
});
