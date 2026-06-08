// NavigationContext.test.tsx — App nav 栈契约.
import React, { useImperativeHandle, forwardRef } from 'react';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react';
import {
  NavigationProvider,
  useNavigation,
  type NavigationApi,
} from '../components/Navigation/NavigationContext';
import type { MainView } from '../lib/mainView';

const HookProbe = forwardRef<NavigationApi, {}>(function HookProbe(_props, ref) {
  const nav = useNavigation();
  useImperativeHandle(ref, () => nav, [nav]);
  return null;
});

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

function mount(initial?: MainView): React.RefObject<NavigationApi> {
  const ref = React.createRef<NavigationApi>();
  act(() => {
    root!.render(
      <NavigationProvider initial={initial}>
        <HookProbe ref={ref} />
      </NavigationProvider>,
    );
  });
  return ref as React.RefObject<NavigationApi>;
}

describe('NavigationContext — 栈契约', () => {
  it('initial state: current=channel, canGoBack=false', () => {
    const ref = mount();
    expect(ref.current!.current).toBe('channel');
    expect(ref.current!.canGoBack).toBe(false);
  });

  it('push 入栈, current 变, canGoBack 真', () => {
    const ref = mount();
    act(() => ref.current!.push('settings'));
    expect(ref.current!.current).toBe('settings');
    expect(ref.current!.canGoBack).toBe(true);
  });

  it('push 同一 view 连续两次不重复入栈 (back 一步直接回前一个)', () => {
    const ref = mount();
    act(() => ref.current!.push('settings'));
    act(() => ref.current!.push('settings'));
    expect(ref.current!.current).toBe('settings');
    act(() => ref.current!.back());
    expect(ref.current!.current).toBe('channel');
  });

  it('back 出一层, current 变前一个 view', () => {
    const ref = mount();
    act(() => ref.current!.push('settings'));
    act(() => ref.current!.push('remote-nodes'));
    expect(ref.current!.current).toBe('remote-nodes');
    act(() => ref.current!.back());
    expect(ref.current!.current).toBe('settings');
    expect(ref.current!.canGoBack).toBe(true);
  });

  it('back 栈只剩 1 层时变成 channel (等价 close)', () => {
    const ref = mount('settings');
    expect(ref.current!.current).toBe('settings');
    expect(ref.current!.canGoBack).toBe(false);
    act(() => ref.current!.back());
    expect(ref.current!.current).toBe('channel');
    expect(ref.current!.canGoBack).toBe(false);
  });

  it('close 永远直接回 channel, 不管栈多深', () => {
    const ref = mount();
    act(() => ref.current!.push('settings'));
    act(() => ref.current!.push('remote-nodes'));
    act(() => ref.current!.push('agents'));
    act(() => ref.current!.close());
    expect(ref.current!.current).toBe('channel');
    expect(ref.current!.canGoBack).toBe(false);
  });

  it('canGoBack: 栈 >1 真, ≤1 假', () => {
    const ref = mount();
    expect(ref.current!.canGoBack).toBe(false);
    act(() => ref.current!.push('settings'));
    expect(ref.current!.canGoBack).toBe(true);
    act(() => ref.current!.push('remote-nodes'));
    expect(ref.current!.canGoBack).toBe(true);
    act(() => ref.current!.back());
    expect(ref.current!.canGoBack).toBe(true);
    act(() => ref.current!.back());
    expect(ref.current!.canGoBack).toBe(false);
  });

  it('useNavigation 在 provider 外 throw', () => {
    // React 18 在 jsdom 里 throw 会被 dispatch 成 window error event 噪音, 抑制掉.
    const consoleError = vi.spyOn(console, 'error').mockImplementation(() => {});
    const errorHandler = (e: Event) => e.preventDefault();
    window.addEventListener('error', errorHandler);
    try {
      expect(() => {
        act(() => {
          root!.render(<HookProbe />);
        });
      }).toThrow(/NavigationProvider/);
    } finally {
      window.removeEventListener('error', errorHandler);
      consoleError.mockRestore();
    }
  });

  it('集成: channel → push(settings) → push(remote-nodes) → back 回 settings, close 回 channel', () => {
    const ref = mount();
    act(() => ref.current!.push('settings'));
    act(() => ref.current!.push('remote-nodes'));
    expect(ref.current!.current).toBe('remote-nodes');
    act(() => ref.current!.back());
    expect(ref.current!.current).toBe('settings'); // 真正后退一步 — 不是 channel
    act(() => ref.current!.close());
    expect(ref.current!.current).toBe('channel');
  });
});
