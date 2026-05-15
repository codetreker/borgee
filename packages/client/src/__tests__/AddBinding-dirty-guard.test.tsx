// AddBinding-dirty-guard.test.tsx — gh#703 PR-2/2 锁
// useUnsavedChangesGuard 在 NodeDetail.AddBinding 的接入正确性 (3 字段创建 form).
//
// 4 case (按 design 摘要):
//   ① showAddBinding=false: 不算 dirty (form 没显)
//   ② showAddBinding=true 但 3 字段全空: 不算 dirty
//   ③ showAddBinding=true + bindPath 非空: 算 dirty
//   ④ showAddBinding=true + bindLabel 非空 (任一字段非空): 算 dirty
import React from 'react';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react';

vi.mock('../lib/api', async () => {
  const actual = await vi.importActual<typeof import('../lib/api')>('../lib/api');
  return {
    ...actual,
    fetchRemoteBindings: vi.fn().mockResolvedValue([]),
    createRemoteBinding: vi.fn(),
    deleteRemoteBinding: vi.fn(),
  };
});

import { NodeDetail } from '../components/NodeManager';
import { runUnsavedGuards, _clearUnsavedGuardsForTest } from '../hooks/useUnsavedChangesGuard';

function setInputValue(input: HTMLInputElement, value: string) {
  const proto = Object.getPrototypeOf(input);
  const nativeSetter = Object.getOwnPropertyDescriptor(proto, 'value')!.set!;
  nativeSetter.call(input, value);
  input.dispatchEvent(new Event('input', { bubbles: true }));
}

let container: HTMLDivElement | null = null;
let root: Root | null = null;

const fakeNode = {
  id: 'n-1',
  name: 'my-server',
  connection_token: 'token-abc',
  created_at: 1,
} as unknown as Parameters<typeof NodeDetail>[0]['node'];

const fakeChannels = [{ id: 'ch-1', name: 'general' }];

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

function render() {
  act(() => {
    root!.render(
      <NodeDetail
        node={fakeNode}
        online={true}
        channels={fakeChannels}
        onDelete={() => {}}
      />,
    );
  });
}

async function clickAddBindingTrigger() {
  // 找 "+ 绑定" 按钮触发 setShowAddBinding(true)
  const buttons = Array.from(container!.querySelectorAll('button')) as HTMLButtonElement[];
  const addBtn = buttons.find(b => b.textContent === '+ 绑定');
  expect(addBtn).toBeDefined();
  act(() => addBtn!.click());
  // 等 useEffect 跑 (loadBindings)
  await act(async () => {
    await Promise.resolve();
  });
}

describe('NodeDetail AddBinding — gh#703 PR-2/2 useUnsavedChangesGuard 接入', () => {
  it('① showAddBinding=false (form 没显): 不算 dirty', async () => {
    render();
    await act(async () => { await Promise.resolve(); });
    expect(runUnsavedGuards()).toBe(true);
  });

  it('② showAddBinding=true 但 3 字段全空: 不算 dirty', async () => {
    render();
    await clickAddBindingTrigger();
    expect(runUnsavedGuards()).toBe(true);
  });

  it('③ showAddBinding=true + bindPath 非空: 算 dirty 守卫拦切换', async () => {
    render();
    await clickAddBindingTrigger();
    // 找 bindPath input (placeholder 含 "远程路径")
    const inputs = Array.from(container!.querySelectorAll('input[type="text"]')) as HTMLInputElement[];
    const bindPathInput = inputs.find(i => (i.placeholder ?? '').includes('远程路径'));
    expect(bindPathInput).toBeDefined();
    act(() => {
      setInputValue(bindPathInput!, '/workspace/foo');
    });
    expect(runUnsavedGuards(() => false)).toBe(false);
  });

  it('④ showAddBinding=true + bindLabel 非空: 算 dirty (任一字段触发)', async () => {
    render();
    await clickAddBindingTrigger();
    const inputs = Array.from(container!.querySelectorAll('input[type="text"]')) as HTMLInputElement[];
    const bindLabelInput = inputs.find(i => (i.placeholder ?? '').includes('标签'));
    expect(bindLabelInput).toBeDefined();
    act(() => {
      setInputValue(bindLabelInput!, '我的标签');
    });
    expect(runUnsavedGuards(() => false)).toBe(false);
  });
});
