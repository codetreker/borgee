// CS-2.2 — FailureCenter 团队栏聚合 (cs-2-content-lock §2 第 4 层).
import React from 'react';
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react';
import FailureCenter from '../components/FailureCenter';

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

describe('CS-2.2 — FailureCenter (4 层 UX 第 4 层)', () => {
  it('TestCS22_ZeroOnSingleFailure — 单 agent 故障 → 不渲染', async () => {
    await render(
      <FailureCenter
        failedAgents={[{ id: 'a1', name: 'Foo', reason: 'api_key_invalid' }]}
      />,
    );
    expect(container!.querySelector('[data-cs2-failure-center]')).toBeNull();
  });

  it('TestCS22_ButtonRendered — ≥2 故障 agent → 按钮渲染', async () => {
    await render(
      <FailureCenter
        failedAgents={[
          { id: 'a1', name: 'Foo', reason: 'api_key_invalid' },
          { id: 'a2', name: 'Bar', reason: 'network_unreachable' },
        ]}
      />,
    );
    const toggle = container!.querySelector('[data-cs2-failure-center-toggle]');
    expect(toggle).toBeTruthy();
    expect(toggle!.textContent).toBe('故障中心 (2)');
  });

  it('TestCS22_ExpandsOnClick — 点按钮 → 展开列表', async () => {
    await render(
      <FailureCenter
        failedAgents={[
          { id: 'a1', name: 'Foo', reason: 'api_key_invalid' },
          { id: 'a2', name: 'Bar', reason: 'network_unreachable' },
        ]}
      />,
    );
    expect(container!.querySelector('[data-cs2-failure-center-list]')).toBeNull();
    const toggle = container!.querySelector(
      '[data-cs2-failure-center-toggle]',
    ) as HTMLButtonElement;
    await act(async () => {
      toggle.click();
    });
    expect(container!.querySelector('[data-cs2-failure-center-list]')).toBeTruthy();
    // plain language label byte-identical
    const items = Array.from(
      container!.querySelectorAll('.cs2-failure-center-agent-reason'),
    ).map((n) => n.textContent);
    expect(items).toContain('Bar 跟 OpenClaw 失联');
    expect(items).toContain('API key 已失效, 需要重新填写');
  });
});
