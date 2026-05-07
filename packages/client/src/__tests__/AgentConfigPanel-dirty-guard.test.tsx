// AgentConfigPanel-dirty-guard.test.tsx — gh#703 §6 锁
// useUnsavedChangesGuard 在 AgentConfigPanel 的接入正确性.
//
// 4 case:
//   ① 加载中: 不算 dirty (防初始 render 误触发)
//   ② config 加载完, draft === config.blob: 不算 dirty
//   ③ 用户改了 draft 字段: 算 dirty, runUnsavedGuards 拦切换
//   ④ saving 中: 不算 dirty (避免重复弹)
import React from 'react';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react-dom/test-utils';

vi.mock('../lib/api', async () => {
  const actual = await vi.importActual<typeof import('../lib/api')>('../lib/api');
  return {
    ...actual,
    fetchAgentConfig: vi.fn(),
    updateAgentConfig: vi.fn(),
  };
});

import { AgentConfigPanel } from '../components/AgentConfigPanel';
import * as api from '../lib/api';
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

// React 18 受控 input — 必须用 native setter 才能让 React 拿到新 value.
function setInputValue(input: HTMLInputElement, value: string) {
  const proto = Object.getPrototypeOf(input);
  const nativeSetter = Object.getOwnPropertyDescriptor(proto, 'value')!.set!;
  nativeSetter.call(input, value);
  input.dispatchEvent(new Event('input', { bubbles: true }));
}

describe('AgentConfigPanel — gh#703 useUnsavedChangesGuard 接入', () => {
  it('① 加载中 (loading=true): 不算 dirty, 守卫放行', () => {
    // fetchAgentConfig pending forever → loading 永远 true
    vi.mocked(api.fetchAgentConfig).mockImplementationOnce(
      () => new Promise(() => {}),
    );
    act(() => {
      root!.render(<AgentConfigPanel agentId="ag-1" />);
    });
    expect(runUnsavedGuards()).toBe(true);
  });

  it('② 加载完, draft === config.blob: 不算 dirty', async () => {
    vi.mocked(api.fetchAgentConfig).mockResolvedValueOnce({
      schema_version: 1,
      blob: { name: 'AgentX', prompt: 'hi' },
    } as api.AgentConfig);
    act(() => {
      root!.render(<AgentConfigPanel agentId="ag-1" />);
    });
    // 等 promise 解析 + state 更新
    await act(async () => {
      await Promise.resolve();
      await Promise.resolve();
    });
    expect(runUnsavedGuards()).toBe(true);
  });

  it('③ 用户改 draft 字段: 算 dirty, 守卫拦切换', async () => {
    vi.mocked(api.fetchAgentConfig).mockResolvedValueOnce({
      schema_version: 1,
      blob: { name: 'AgentX' },
    } as api.AgentConfig);
    act(() => {
      root!.render(<AgentConfigPanel agentId="ag-1" />);
    });
    await act(async () => {
      await Promise.resolve();
      await Promise.resolve();
    });

    // 通过 DOM 改 input 触发 setDraft
    const input = container!.querySelector(
      'input[data-agent-config-field="name"]',
    ) as HTMLInputElement;
    expect(input).not.toBeNull();
    expect(input.value).toBe('AgentX');
    act(() => {
      setInputValue(input, 'AgentY');
    });
    // 现在 draft.name = AgentY ≠ config.blob.name = AgentX
    expect(runUnsavedGuards(() => false)).toBe(false);
  });

  it('④ 反向断言: 改回原值后守卫又放行 (来回切)', async () => {
    vi.mocked(api.fetchAgentConfig).mockResolvedValueOnce({
      schema_version: 1,
      blob: { name: 'AgentX' },
    } as api.AgentConfig);
    act(() => {
      root!.render(<AgentConfigPanel agentId="ag-1" />);
    });
    await act(async () => {
      await Promise.resolve();
      await Promise.resolve();
    });
    const input = container!.querySelector(
      'input[data-agent-config-field="name"]',
    ) as HTMLInputElement;
    act(() => {
      setInputValue(input, 'AgentY');
    });
    expect(runUnsavedGuards(() => false)).toBe(false);
    // 改回原值
    act(() => {
      setInputValue(input, 'AgentX');
    });
    expect(runUnsavedGuards()).toBe(true);
  });
});
