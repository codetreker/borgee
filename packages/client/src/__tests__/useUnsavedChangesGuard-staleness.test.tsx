// useUnsavedChangesGuard-staleness.test.tsx — #695 feima review 抓的 bug
// 回归.
//
// bug: 之前 useUnsavedChangesGuard 用 useEffect(..., []) 空 deps, 把调
// 用方传进的 isDirty 闭包直接存进 module-level Set. 闭包绑定 mount 那
// 一刻的 React state, 用户后续 setState 让 component 重新 render, 但
// Set 里那个 isDirty 还是旧闭包, 永远拿旧 state. 守卫看着注册了, 实际
// 不工作.
//
// 修法: useRef 装一个引用, 每次 render 把最新 isDirty 写进 ref.current,
// useEffect 注册"调 ref" — 这样 Set 里那条永远拿当前 render 的 isDirty.
//
// 这个测试模拟 AgentManager 真实场景: 用 React state 存 displayName, 一
// 开始空 (isDirty 应该返 false), setState 让 displayName 非空 (isDirty
// 应该返 true), 走 runUnsavedGuards 看守卫真的拦得住.
//
// 走 createRoot + act 跟 useArtifactPanel.test.tsx 同模式 (这个仓库不依
// 赖 @testing-library/react).

import React, { useState, useImperativeHandle, forwardRef } from 'react';
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react-dom/test-utils';
import {
  useUnsavedChangesGuard,
  runUnsavedGuards,
  _clearUnsavedGuardsForTest,
} from '../hooks/useUnsavedChangesGuard';

// 用 forwardRef 暴露 setState — 测试外部能调它改 state 触发 re-render.
type Probe = { setName: (s: string) => void };

const FormProbe = forwardRef<Probe>(function FormProbe(_props, ref) {
  const [name, setName] = useState('');
  useImperativeHandle(ref, () => ({ setName }), []);
  // 模拟 AgentManager 写法: isDirty 闭包绑定 React state.
  useUnsavedChangesGuard(
    () => name.trim() !== '',
    'name 表单填了一半',
  );
  return <div data-testid="form" data-name={name} />;
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
  act(() => {
    root?.unmount();
  });
  container?.remove();
  root = null;
  container = null;
  _clearUnsavedGuardsForTest();
});

describe('useUnsavedChangesGuard — React state staleness 回归 (#695 feima)', () => {
  it('mount 时 state 空, 守卫应放行 (不 dirty)', () => {
    const probeRef = React.createRef<Probe>();
    act(() => {
      root!.render(<FormProbe ref={probeRef} />);
    });
    // 没填东西, 守卫返 true (放行).
    expect(runUnsavedGuards()).toBe(true);
  });

  it('mount 后 setState 改 state, 守卫应拿到新 state (拦切换)', () => {
    const probeRef = React.createRef<Probe>();
    act(() => {
      root!.render(<FormProbe ref={probeRef} />);
    });
    // 一开始干净.
    expect(runUnsavedGuards()).toBe(true);
    // setState 改成非空 — 这是 bug 修前的"卡点": 闭包绑定 mount 时 ''.
    act(() => {
      probeRef.current!.setName('张三');
    });
    // 现在 isDirty 应该返 true, runUnsavedGuards 拿到 dirty 弹 confirm.
    // 注入 confirmFn 返 false (用户取消) → runUnsavedGuards 返 false (拦住).
    const result = runUnsavedGuards(() => false);
    expect(result).toBe(false);
  });

  it('setState 来回切, 守卫每次都拿当前 state 不卡死在某次', () => {
    const probeRef = React.createRef<Probe>();
    act(() => {
      root!.render(<FormProbe ref={probeRef} />);
    });
    // 干净 → dirty → 干净 → dirty 来回, 每次守卫都正确反应.
    expect(runUnsavedGuards()).toBe(true);

    act(() => probeRef.current!.setName('a'));
    expect(runUnsavedGuards(() => false)).toBe(false);

    act(() => probeRef.current!.setName(''));
    expect(runUnsavedGuards()).toBe(true);

    act(() => probeRef.current!.setName('b'));
    expect(runUnsavedGuards(() => false)).toBe(false);

    // 用户这次确认离开 → 守卫放行.
    act(() => probeRef.current!.setName('c'));
    expect(runUnsavedGuards(() => true)).toBe(true);
  });

  it('unmount 后守卫不再触发 (反注册生效)', () => {
    const probeRef = React.createRef<Probe>();
    act(() => {
      root!.render(<FormProbe ref={probeRef} />);
    });
    act(() => probeRef.current!.setName('still dirty'));
    // 先确认守卫确实触发.
    expect(runUnsavedGuards(() => false)).toBe(false);
    // unmount.
    act(() => {
      root!.unmount();
    });
    // 再调 — 没守卫了, 直接放行.
    expect(runUnsavedGuards()).toBe(true);
  });
});
