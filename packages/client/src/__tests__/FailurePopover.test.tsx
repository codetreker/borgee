// CS-2.2 — FailurePopover 浮层 (cs-2-stance-checklist 设计 ② + content-lock §2).
import React from 'react';
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react';
import FailurePopover from '../components/FailurePopover';

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

describe('CS-2.2 — FailurePopover (4 层 UX 第 2 层)', () => {
  it('TestCS22_PopoverHiddenWhenClosed — open=false → null', async () => {
    await render(<FailurePopover open={false} reason="api_key_invalid" agentName="DevAgent" />);
    expect(container!.querySelector('[data-cs2-failure-popover]')).toBeNull();
  });

  it('TestCS22_PopoverRendersWhenOpen — open=true → DOM', async () => {
    await render(<FailurePopover open={true} reason="api_key_invalid" agentName="DevAgent" />);
    const popover = container!.querySelector('[data-cs2-failure-popover="open"]');
    expect(popover).toBeTruthy();
    expect(popover!.getAttribute('role')).toBe('dialog');
  });

  it('TestCS22_3ButtonsLiteralByteIdentical — 文案 byte-identical 跟蓝图', async () => {
    await render(<FailurePopover open={true} reason="api_key_invalid" agentName="DevAgent" />);
    const buttons = Array.from(container!.querySelectorAll('button'));
    const labels = buttons.map((b) => b.textContent);
    expect(labels).toEqual(['重连', '重填 API key', '查日志']);
    expect(container!.querySelector('[data-action="reconnect"]')).toBeTruthy();
    expect(container!.querySelector('[data-action="refill_api_key"]')).toBeTruthy();
    expect(container!.querySelector('[data-action="view_logs"]')).toBeTruthy();
  });

  it('TestCS22_ReasonTextRendered — plain language byte-identical', async () => {
    await render(
      <FailurePopover open={true} reason="network_unreachable" agentName="BugAgent" />,
    );
    expect(container!.querySelector('[data-cs2-failure-reason]')!.textContent).toBe(
      'BugAgent 跟 OpenClaw 失联',
    );
  });

  it('TestCS22_RepairCallback — onRepair fires with action', async () => {
    let captured = '';
    await render(
      <FailurePopover
        open={true}
        reason="api_key_invalid"
        agentName="DevAgent"
        onRepair={(a) => {
          captured = a;
        }}
      />,
    );
    const reconnectBtn = container!.querySelector('[data-action="reconnect"]') as HTMLButtonElement;
    await act(async () => {
      reconnectBtn.click();
    });
    expect(captured).toBe('reconnect');
  });
});
