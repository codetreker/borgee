// DescriptionEditor-dirty-guard.test.tsx — gh#703 §6 锁
// useUnsavedChangesGuard 在 DescriptionEditor 的接入正确性.
//
// 4 case:
//   ① 初始 mount, value === initial: 不算 dirty
//   ② 用户改了 textarea: 算 dirty, 守卫拦切换
//   ③ 改回原值: 不算 dirty (来回切)
//   ④ 改完点保存 (busy 中): 不算 dirty (反重复弹)
import React from 'react';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react-dom/test-utils';

vi.mock('../lib/api', async () => {
  const actual = await vi.importActual<typeof import('../lib/api')>('../lib/api');
  return {
    ...actual,
    setChannelDescription: vi.fn(),
  };
});

import { DescriptionEditor } from '../components/DescriptionEditor';
import * as api from '../lib/api';
import type { Channel } from '../types';
import { runUnsavedGuards, _clearUnsavedGuardsForTest } from '../hooks/useUnsavedChangesGuard';

let container: HTMLDivElement | null = null;
let root: Root | null = null;

beforeEach(() => {
  _clearUnsavedGuardsForTest();
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
});

afterEach(() => {
  act(() => {
    root?.unmount();
  });
  container?.remove();
  root = null;
  container = null;
  _clearUnsavedGuardsForTest();
  vi.restoreAllMocks();
});

// React 18 受控 input/textarea — 必须用 native setter 才能让 React 拿到
// 新 value (反 ta.value = X 不通过 React 的属性 setter, onChange 不触发).
function setTextareaValue(ta: HTMLTextAreaElement, value: string) {
  const proto = Object.getPrototypeOf(ta);
  const nativeSetter = Object.getOwnPropertyDescriptor(proto, 'value')!.set!;
  nativeSetter.call(ta, value);
  ta.dispatchEvent(new Event('input', { bubbles: true }));
}

describe('DescriptionEditor — gh#703 useUnsavedChangesGuard 接入', () => {
  it('① 初始 mount value === initial: 不算 dirty', () => {
    act(() => {
      root!.render(
        <DescriptionEditor
          channelID="ch-1"
          initial="原始 description"
          onSaved={() => {}}
          onCancel={() => {}}
        />,
      );
    });
    expect(runUnsavedGuards()).toBe(true);
  });

  it('② 用户改 textarea: 算 dirty, 守卫拦切换', () => {
    act(() => {
      root!.render(
        <DescriptionEditor
          channelID="ch-1"
          initial="原始"
          onSaved={() => {}}
          onCancel={() => {}}
        />,
      );
    });
    const ta = container!.querySelector(
      '[data-testid="description-editor-input"]',
    ) as HTMLTextAreaElement;
    act(() => {
      setTextareaValue(ta, '改了');
    });
    expect(runUnsavedGuards(() => false)).toBe(false);
  });

  it('③ 改回原值: 不算 dirty (来回切)', () => {
    act(() => {
      root!.render(
        <DescriptionEditor
          channelID="ch-1"
          initial="原始"
          onSaved={() => {}}
          onCancel={() => {}}
        />,
      );
    });
    const ta = container!.querySelector(
      '[data-testid="description-editor-input"]',
    ) as HTMLTextAreaElement;
    act(() => {
      setTextareaValue(ta, '改了');
    });
    expect(runUnsavedGuards(() => false)).toBe(false);
    // 改回
    act(() => {
      setTextareaValue(ta, '原始');
    });
    expect(runUnsavedGuards()).toBe(true);
  });

  it('④ 保存中 (busy=true): 不算 dirty', async () => {
    let resolveSave!: (v: Channel) => void;
    vi.mocked(api.setChannelDescription).mockImplementationOnce(
      () => new Promise<Channel>(res => { resolveSave = res; }),
    );
    act(() => {
      root!.render(
        <DescriptionEditor
          channelID="ch-1"
          initial="原始"
          onSaved={() => {}}
          onCancel={() => {}}
        />,
      );
    });
    const ta = container!.querySelector(
      '[data-testid="description-editor-input"]',
    ) as HTMLTextAreaElement;
    act(() => {
      setTextareaValue(ta, '改了');
    });
    // dirty 状态
    expect(runUnsavedGuards(() => false)).toBe(false);
    // 点保存进入 busy
    const saveBtn = container!.querySelector(
      '[data-testid="description-save"]',
    ) as HTMLButtonElement;
    act(() => {
      saveBtn.click();
    });
    // busy=true 不算 dirty
    expect(runUnsavedGuards()).toBe(true);
    // resolve 让组件解开 busy
    await act(async () => {
      resolveSave({ id: 'ch-1' } as Channel);
      await Promise.resolve();
    });
  });
});
