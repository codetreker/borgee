// SystemMessageBubble.al5.test.tsx — AL-5.2 单按钮 "重连" DOM 字面锁
// + 反向断言: 不渲染 BPP-3.2 三按钮 + 不渲染 fallback 按钮.
//
// 锚: al-5-spec.md §1 AL-5.2 byte-identical:
//   <button data-al5-button="recover" data-action="recover">重连</button>
//
// 反约束: AL-5 同义词禁词反向 — 不渲染 "重启"/"reset"/"restart"/
// "重新启动"/"重置".

import React from 'react';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { createRoot } from 'react-dom/client';
import { act } from 'react';
import SystemMessageBubble, {
  isAL5RecoverPayload,
  type AL5RecoverPayload,
} from '../components/SystemMessageBubble';

let container: HTMLDivElement | null = null;

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
});

afterEach(() => {
  if (container) {
    container.remove();
    container = null;
  }
});

const sampleAL5: AL5RecoverPayload = {
  action: 'recover',
  agent_id: 'agent-uuid-1',
  reason: 'api_key_invalid',
  request_id: 'req-trace-1',
};

describe('AL-5.2 SystemMessageBubble — 重连 button', () => {
  it('renders single 重连 button with data-al5-button="recover" + data-action="recover"', async () => {
    const root = createRoot(container!);
    await act(async () => {
      root.render(
        <SystemMessageBubble
          bodyHTML="<p>状态变更: error</p>"
          al5={sampleAL5}
        />,
      );
    });
    const btn = container!.querySelector('button[data-al5-button="recover"]');
    expect(btn, 'recover button rendered').toBeTruthy();
    expect(btn?.getAttribute('data-action')).toBe('recover');
    expect(btn?.textContent).toBe('重连');
    // Container marker present.
    expect(container!.querySelector('[data-al5-recover="true"]')).toBeTruthy();
    await act(async () => { root.unmount(); });
  });

  it('does NOT render BPP-3.2 三按钮 when only AL-5 payload provided', async () => {
    const root = createRoot(container!);
    await act(async () => {
      root.render(
        <SystemMessageBubble bodyHTML="<p>x</p>" al5={sampleAL5} />,
      );
    });
    expect(container!.querySelector('[data-bpp32-grant]')).toBeNull();
    expect(container!.querySelector('[data-action="grant"]')).toBeNull();
    expect(container!.querySelector('[data-action="reject"]')).toBeNull();
    expect(container!.querySelector('[data-action="snooze"]')).toBeNull();
    await act(async () => { root.unmount(); });
  });

  it('reverse — no synonym buttons (重启/reset/restart/重新启动/重置)', async () => {
    const root = createRoot(container!);
    await act(async () => {
      root.render(
        <SystemMessageBubble bodyHTML="<p>x</p>" al5={sampleAL5} />,
      );
    });
    const text = container!.textContent ?? '';
    for (const bad of ['重启', 'reset', 'restart', '重新启动', '重置']) {
      expect(text.includes(bad), `synonym '${bad}' must not appear`).toBe(false);
    }
    await act(async () => { root.unmount(); });
  });

  it('clicking 重连 calls onRecover with full payload byte-identical', async () => {
    const onRecover = vi.fn().mockResolvedValue(undefined);
    const root = createRoot(container!);
    await act(async () => {
      root.render(
        <SystemMessageBubble bodyHTML="<p>x</p>" al5={sampleAL5} onRecover={onRecover} />,
      );
    });
    const btn = container!.querySelector('button[data-al5-button="recover"]') as HTMLButtonElement;
    await act(async () => {
      btn.click();
    });
    expect(onRecover).toHaveBeenCalledTimes(1);
    expect(onRecover).toHaveBeenCalledWith(sampleAL5);
    await act(async () => { root.unmount(); });
  });

  it('isAL5RecoverPayload type guard — accepts shape, rejects 5 invalid cases', () => {
    expect(isAL5RecoverPayload(sampleAL5)).toBe(true);
    // 5 reverse cases:
    expect(isAL5RecoverPayload(null)).toBe(false);
    expect(isAL5RecoverPayload({ ...sampleAL5, action: 'grant' })).toBe(false);
    expect(isAL5RecoverPayload({ ...sampleAL5, agent_id: '' })).toBe(false);
    expect(isAL5RecoverPayload({ ...sampleAL5, reason: '' })).toBe(false);
    expect(isAL5RecoverPayload({ ...sampleAL5, request_id: '' })).toBe(false);
  });
});
