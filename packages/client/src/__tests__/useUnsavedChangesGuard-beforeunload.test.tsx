// useUnsavedChangesGuard-beforeunload.test.tsx — gh#703 PR-2/2 锁
// hook 内 beforeunload 监听器: dirty 时调 preventDefault + 设 returnValue,
// 干净时 handler 早返回 (不阻 unload).
//
// 跟 ArtifactCommentDraftInput 既有 beforeunload 测试 (CV-10) 同模式
// (vi.spyOn(evt, 'preventDefault') 检查是否真调到 — jsdom 不暴 returnValue
// 为 '' 字面, 所以走 spy 验调用).
import React, { useState, useImperativeHandle, forwardRef } from 'react';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react-dom/test-utils';
import {
  useUnsavedChangesGuard,
  _clearUnsavedGuardsForTest,
} from '../hooks/useUnsavedChangesGuard';

type Probe = { setDirty: (d: boolean) => void };

const FormProbe = forwardRef<Probe>(function FormProbe(_props, ref) {
  const [dirty, setDirty] = useState(false);
  useImperativeHandle(ref, () => ({ setDirty }), []);
  useUnsavedChangesGuard(() => dirty, 'test form');
  return <div data-test="form" data-dirty={dirty} />;
});

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
  vi.restoreAllMocks();
});

describe('useUnsavedChangesGuard — gh#703 beforeunload 监听器', () => {
  it('干净状态: handler 早返回, 不调 preventDefault', () => {
    const probeRef = React.createRef<Probe>();
    act(() => {
      root!.render(<FormProbe ref={probeRef} />);
    });
    // dirty=false (mount 默认), 触发 beforeunload
    const evt = new Event('beforeunload', { cancelable: true }) as BeforeUnloadEvent;
    const preventSpy = vi.spyOn(evt, 'preventDefault');
    window.dispatchEvent(evt);
    expect(preventSpy).not.toHaveBeenCalled();
  });

  it('dirty 状态: handler 调 preventDefault', () => {
    const probeRef = React.createRef<Probe>();
    act(() => {
      root!.render(<FormProbe ref={probeRef} />);
    });
    act(() => {
      probeRef.current!.setDirty(true);
    });
    const evt = new Event('beforeunload', { cancelable: true }) as BeforeUnloadEvent;
    const preventSpy = vi.spyOn(evt, 'preventDefault');
    window.dispatchEvent(evt);
    expect(preventSpy).toHaveBeenCalled();
  });

  it('unmount 后 beforeunload handler 自动反注册 (不再 preventDefault)', () => {
    const probeRef = React.createRef<Probe>();
    act(() => {
      root!.render(<FormProbe ref={probeRef} />);
    });
    act(() => {
      probeRef.current!.setDirty(true);
    });
    // 验一次 dirty 真起作用
    let evt = new Event('beforeunload', { cancelable: true }) as BeforeUnloadEvent;
    let preventSpy = vi.spyOn(evt, 'preventDefault');
    window.dispatchEvent(evt);
    expect(preventSpy).toHaveBeenCalled();
    // unmount
    act(() => {
      root!.unmount();
    });
    // 反注册后再触发 — 不应再 preventDefault
    evt = new Event('beforeunload', { cancelable: true }) as BeforeUnloadEvent;
    preventSpy = vi.spyOn(evt, 'preventDefault');
    window.dispatchEvent(evt);
    expect(preventSpy).not.toHaveBeenCalled();
  });
});
