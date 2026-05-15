// AppShell.test.tsx — CS-1.1 三栏 layout + responsive contract.
//
// Tests:
//   - Desktop closed: grid '240px 1fr', 第三栏不渲染
//   - Desktop drawer: grid '240px 1fr 380px', artifact column 渲染
//   - Desktop split: grid '240px 1fr 1fr'
//   - Desktop fullscreen: grid '240px 1fr', overlay 渲染
//   - Mobile (≤768px): grid '1fr' single column, sidebar overlay 在 sidebarOpen
//   - data-artifact-mode + data-mobile attrs byte-identical (DOM 锚)

import React from 'react';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react';
import AppShell, {
  computeGridColumns,
  APP_SHELL_DESKTOP_SIDEBAR,
  APP_SHELL_DESKTOP_DRAWER,
  APP_SHELL_MOBILE_BREAKPOINT,
} from '../components/AppShell';

let container: HTMLDivElement | null = null;
let root: Root | null = null;

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
});

afterEach(() => {
  act(() => {
    root?.unmount();
  });
  if (container) {
    document.body.removeChild(container);
    container = null;
  }
  vi.restoreAllMocks();
});

function render(node: React.ReactElement) {
  act(() => {
    root!.render(node);
  });
}

describe('AppShell — CS-1.1 三栏 layout contract', () => {
  it('byte-identical 蓝图 §1.2 字面常量 (240 / 380 / 768)', () => {
    expect(APP_SHELL_DESKTOP_SIDEBAR).toBe(240);
    expect(APP_SHELL_DESKTOP_DRAWER).toBe(380);
    expect(APP_SHELL_MOBILE_BREAKPOINT).toBe(768);
  });

  it('computeGridColumns: 4 desktop modes byte-identical', () => {
    expect(computeGridColumns('closed', false)).toBe('240px 1fr');
    expect(computeGridColumns('drawer', false)).toBe('240px 1fr 380px');
    expect(computeGridColumns('split', false)).toBe('240px 1fr 1fr');
    expect(computeGridColumns('fullscreen', false)).toBe('240px 1fr');
  });

  it('computeGridColumns: mobile single column 反 desktop multi-col', () => {
    expect(computeGridColumns('closed', true)).toBe('1fr');
    expect(computeGridColumns('drawer', true)).toBe('1fr');
    expect(computeGridColumns('split', true)).toBe('1fr');
    expect(computeGridColumns('fullscreen', true)).toBe('1fr');
  });

  it('Desktop closed: artifact column NOT rendered', () => {
    render(
      <AppShell
        sidebar={<div>S</div>}
        main={<div>M</div>}
        artifactPanel={<div>A</div>}
        artifactMode="closed"
        isMobile={false}
        sidebarOpen={false}
        onSidebarClose={() => {}}
      />,
    );
    const shell = container!.querySelector('[data-testid="app-shell"]');
    expect(shell).toBeTruthy();
    expect(shell?.getAttribute('data-artifact-mode')).toBe('closed');
    expect(container!.querySelector('[data-testid="app-shell-artifact-column"]')).toBeNull();
    expect(container!.querySelector('[data-testid="app-shell-artifact-fullscreen"]')).toBeNull();
  });

  it('Desktop drawer: artifact column rendered with content', () => {
    render(
      <AppShell
        sidebar={<div>S</div>}
        main={<div>M</div>}
        artifactPanel={<div data-testid="art-content">A</div>}
        artifactMode="drawer"
        isMobile={false}
        sidebarOpen={false}
        onSidebarClose={() => {}}
      />,
    );
    const col = container!.querySelector('[data-testid="app-shell-artifact-column"]');
    expect(col).toBeTruthy();
    expect(col?.querySelector('[data-testid="art-content"]')).toBeTruthy();
  });

  it('Desktop fullscreen: overlay rendered (反 column)', () => {
    render(
      <AppShell
        sidebar={<div>S</div>}
        main={<div>M</div>}
        artifactPanel={<div>A</div>}
        artifactMode="fullscreen"
        isMobile={false}
        sidebarOpen={false}
        onSidebarClose={() => {}}
      />,
    );
    expect(container!.querySelector('[data-testid="app-shell-artifact-column"]')).toBeNull();
    const overlay = container!.querySelector('[data-testid="app-shell-artifact-fullscreen"]');
    expect(overlay).toBeTruthy();
    expect(overlay?.getAttribute('role')).toBe('dialog');
    expect(overlay?.getAttribute('aria-modal')).toBe('true');
  });

  it('Mobile sidebar overlay only when sidebarOpen', () => {
    const onClose = vi.fn();
    render(
      <AppShell
        sidebar={<div>S</div>}
        main={<div>M</div>}
        artifactPanel={null}
        artifactMode="closed"
        isMobile={true}
        sidebarOpen={true}
        onSidebarClose={onClose}
      />,
    );
    const overlay = container!.querySelector('[data-testid="app-shell-sidebar-overlay"]') as HTMLElement;
    expect(overlay).toBeTruthy();
    act(() => {
      overlay.click();
    });
    expect(onClose).toHaveBeenCalled();
  });

  it('Mobile sidebarOpen=false: overlay NOT rendered', () => {
    render(
      <AppShell
        sidebar={<div>S</div>}
        main={<div>M</div>}
        artifactPanel={null}
        artifactMode="closed"
        isMobile={true}
        sidebarOpen={false}
        onSidebarClose={() => {}}
      />,
    );
    expect(container!.querySelector('[data-testid="app-shell-sidebar-overlay"]')).toBeNull();
  });
});
