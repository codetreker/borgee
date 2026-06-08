// PageHeader.test.tsx — 通用页头点击行为 + back/close 开关.
import React from 'react';
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react';
import PageHeader from '../components/common/PageHeader';
import { NavigationProvider, useNavigation, type NavigationApi } from '../components/Navigation/NavigationContext';

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
});

function NavSink({ navRef, children }: { navRef: { current: NavigationApi | null }; children: React.ReactNode }) {
  navRef.current = useNavigation();
  return <>{children}</>;
}

function mount(node: React.ReactNode, initial: 'channel' | 'settings' | 'remote-nodes' = 'settings') {
  const navRef: { current: NavigationApi | null } = { current: null };
  act(() => {
    root!.render(
      <NavigationProvider initial={initial}>
        <NavSink navRef={navRef}>{node}</NavSink>
      </NavigationProvider>,
    );
  });
  return navRef;
}

describe('PageHeader — 点击行为', () => {
  it('默认渲染 ← 和 × 按钮 (各带 aria-label)', () => {
    mount(<PageHeader title="测试" />);
    const back = container!.querySelector('.page-header-back') as HTMLButtonElement;
    const close = container!.querySelector('.page-header-close') as HTMLButtonElement;
    expect(back).toBeTruthy();
    expect(back.getAttribute('aria-label')).toBe('返回');
    expect(close).toBeTruthy();
    expect(close.getAttribute('aria-label')).toBe('关闭');
  });

  it('标题正确渲染', () => {
    mount(<PageHeader title="My Page" />);
    expect(container!.querySelector('.page-header-title')?.textContent).toBe('My Page');
  });

  it('点 ← 调 nav.back (栈 >1 时出一层)', () => {
    const navRef = mount(<PageHeader title="子页" />, 'settings');
    act(() => navRef.current!.push('remote-nodes'));
    expect(navRef.current!.current).toBe('remote-nodes');
    const back = container!.querySelector('.page-header-back') as HTMLButtonElement;
    act(() => back.click());
    expect(navRef.current!.current).toBe('settings');
  });

  it('点 × 调 nav.close (清栈回 channel)', () => {
    const navRef = mount(<PageHeader title="子页" />, 'settings');
    act(() => navRef.current!.push('remote-nodes'));
    act(() => navRef.current!.push('agents'));
    const close = container!.querySelector('.page-header-close') as HTMLButtonElement;
    act(() => close.click());
    expect(navRef.current!.current).toBe('channel');
    expect(navRef.current!.canGoBack).toBe(false);
  });

  it('back=false 时不渲染 ←', () => {
    mount(<PageHeader title="只关闭" back={false} />);
    expect(container!.querySelector('.page-header-back')).toBeNull();
    expect(container!.querySelector('.page-header-close')).toBeTruthy();
  });

  it('close=false 时不渲染 ×', () => {
    mount(<PageHeader title="只返回" close={false} />);
    expect(container!.querySelector('.page-header-back')).toBeTruthy();
    expect(container!.querySelector('.page-header-close')).toBeNull();
  });

  it('actions 渲染在 × 左侧', () => {
    mount(
      <PageHeader
        title="Has Actions"
        actions={<button data-testid="custom-action">新建</button>}
      />,
    );
    expect(container!.querySelector('[data-testid="custom-action"]')).toBeTruthy();
  });
});
