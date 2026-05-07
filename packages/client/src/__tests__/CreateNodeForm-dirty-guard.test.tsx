// CreateNodeForm-dirty-guard.test.tsx — gh#703 PR-2/2 锁
// useUnsavedChangesGuard 在 CreateNodeForm 的接入正确性 (1 字段创建 form).
//
// 4 case (按 design 摘要):
//   ① mount 时 name='' 不算 dirty
//   ② 输入 name 算 dirty
//   ③ trim 边界: 空格 only 不算 dirty
//   ④ unmount 后守卫不再触发
import React from 'react';
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react-dom/test-utils';

import { CreateNodeForm } from '../components/NodeManager';
import { runUnsavedGuards, _clearUnsavedGuardsForTest } from '../hooks/useUnsavedChangesGuard';

// React 18 受控 input — 必须用 native setter 才能让 React 拿到新 value.
function setInputValue(input: HTMLInputElement, value: string) {
  const proto = Object.getPrototypeOf(input);
  const nativeSetter = Object.getOwnPropertyDescriptor(proto, 'value')!.set!;
  nativeSetter.call(input, value);
  input.dispatchEvent(new Event('input', { bubbles: true }));
}

let container: HTMLDivElement | null = null;
let root: Root | null = null;

beforeEach(() => {
  _clearUnsavedGuardsForTest();
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
});

afterEach(() => {
  act(() => { root?.unmount(); });
  container?.remove();
  root = null;
  container = null;
  _clearUnsavedGuardsForTest();
});

describe('CreateNodeForm — gh#703 PR-2/2 useUnsavedChangesGuard 接入', () => {
  it('① mount 时 name=空: 不算 dirty', () => {
    act(() => {
      root!.render(<CreateNodeForm onSubmit={() => {}} onCancel={() => {}} />);
    });
    expect(runUnsavedGuards()).toBe(true);
  });

  it('② 输入 name: 算 dirty 守卫拦切换', () => {
    act(() => {
      root!.render(<CreateNodeForm onSubmit={() => {}} onCancel={() => {}} />);
    });
    const input = container!.querySelector('input[type="text"]') as HTMLInputElement;
    expect(input).not.toBeNull();
    act(() => {
      setInputValue(input, 'my-server');
    });
    expect(runUnsavedGuards(() => false)).toBe(false);
  });

  it('③ trim 边界: 空格 only 不算 dirty', () => {
    act(() => {
      root!.render(<CreateNodeForm onSubmit={() => {}} onCancel={() => {}} />);
    });
    const input = container!.querySelector('input[type="text"]') as HTMLInputElement;
    act(() => {
      setInputValue(input, '   ');
    });
    expect(runUnsavedGuards()).toBe(true);
  });

  it('④ unmount 后守卫反注册, 不再触发', () => {
    act(() => {
      root!.render(<CreateNodeForm onSubmit={() => {}} onCancel={() => {}} />);
    });
    const input = container!.querySelector('input[type="text"]') as HTMLInputElement;
    act(() => {
      setInputValue(input, 'still dirty');
    });
    // dirty 状态先验
    expect(runUnsavedGuards(() => false)).toBe(false);
    // unmount
    act(() => {
      root!.unmount();
    });
    // 反注册后没守卫了
    expect(runUnsavedGuards()).toBe(true);
  });
});
