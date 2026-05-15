// CS-2.2 — FailureBanner 顶部 banner (cs-2-content-lock §2 第 3 层).
import React from 'react';
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react';
import FailureBanner, { CORE_AGENT_FAILURE_THRESHOLD_MS } from '../components/FailureBanner';

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

describe('CS-2.2 — FailureBanner (4 层 UX 第 3 层)', () => {
  it('TestCS22_AllAgentsFailureTriggers — 全部故障 → banner', async () => {
    await render(
      <FailureBanner
        agents={[
          { id: 'a1', name: 'Foo', isFailed: true },
          { id: 'a2', name: 'Bar', isFailed: true },
        ]}
      />,
    );
    const el = container!.querySelector('[data-cs2-failure-banner="visible"]');
    expect(el).toBeTruthy();
    expect(el!.querySelector('.cs2-failure-banner-body')!.textContent).toBe(
      '全部 agent 故障, 请检查',
    );
  });

  it('TestCS22_PartialFailureNoBanner — 部分故障 → 不渲染', async () => {
    await render(
      <FailureBanner
        agents={[
          { id: 'a1', name: 'Foo', isFailed: true },
          { id: 'a2', name: 'Bar', isFailed: false },
        ]}
      />,
    );
    expect(container!.querySelector('[data-cs2-failure-banner="visible"]')).toBeNull();
  });

  it('TestCS22_CoreAgent5MinTriggers — 核心 agent > 5 min → banner', async () => {
    const now = 1_000_000;
    const failedAt = now - CORE_AGENT_FAILURE_THRESHOLD_MS - 1000;
    await render(
      <FailureBanner
        agents={[
          { id: 'a1', name: 'CoreAgent', isFailed: true, isCore: true, failedAt },
          { id: 'a2', name: 'Other', isFailed: false },
        ]}
        now={now}
      />,
    );
    const body = container!.querySelector('.cs2-failure-banner-body')!.textContent;
    expect(body).toBe('CoreAgent 已故障 5 分钟以上');
  });

  it('TestCS22_CoreAgentBelowThreshold_NoBanner — 核心 agent 4min → 不渲染', async () => {
    const now = 1_000_000;
    const failedAt = now - 4 * 60 * 1000;
    await render(
      <FailureBanner
        agents={[{ id: 'a1', name: 'CoreAgent', isFailed: true, isCore: true, failedAt }]}
        now={now}
      />,
    );
    expect(container!.querySelector('[data-cs2-failure-banner="visible"]')).toBeNull();
  });

  it('TestCS22_DismissButton — 点关闭 → banner 消失', async () => {
    await render(
      <FailureBanner
        agents={[
          { id: 'a1', name: 'Foo', isFailed: true },
          { id: 'a2', name: 'Bar', isFailed: true },
        ]}
      />,
    );
    expect(container!.querySelector('[data-cs2-failure-banner="visible"]')).toBeTruthy();
    const dismissBtn = container!.querySelector(
      '[data-cs2-failure-banner-dismiss]',
    ) as HTMLButtonElement;
    await act(async () => {
      dismissBtn.click();
    });
    expect(container!.querySelector('[data-cs2-failure-banner="visible"]')).toBeNull();
  });
});
