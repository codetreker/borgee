// ReadonlyToggle.test.tsx — CHN-15 acceptance §3.2 + content-lock §1+§2.1.

import React from 'react';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react';
import ReadonlyToggle from '../components/ReadonlyToggle';

let container: HTMLDivElement | null = null;
let root: Root | null = null;

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
  (globalThis as any).fetch = vi.fn(async () =>
    new Response(JSON.stringify({
      channel_id: 'chan-1',
      collapsed: 16,
      readonly: true,
    }), { status: 200, headers: { 'Content-Type': 'application/json' } }),
  );
});

afterEach(() => {
  act(() => {
    root?.unmount();
  });
  if (container) {
    document.body.removeChild(container);
    container = null;
  }
  vi.restoreAllMocks();
});

function render(node: React.ReactElement) {
  act(() => {
    root!.render(node);
  });
}

describe('ReadonlyToggle', () => {
  it('renders with data-readonly=false + label "已设为只读" by default', () => {
    render(<ReadonlyToggle channelId="chan-1" />);
    const btn = container!.querySelector('[data-testid="readonly-toggle"]');
    expect(btn).toBeTruthy();
    expect(btn?.getAttribute('data-readonly')).toBe('false');
    expect(btn?.getAttribute('aria-pressed')).toBe('false');
    expect(btn?.getAttribute('title')).toBe('已设为只读');
    expect(btn?.textContent).toBe('已设为只读');
  });

  it('renders bookmarked state when initialReadonly=true', () => {
    render(<ReadonlyToggle channelId="chan-1" initialReadonly={true} />);
    const btn = container!.querySelector('[data-testid="readonly-toggle"]');
    expect(btn?.getAttribute('data-readonly')).toBe('true');
    expect(btn?.getAttribute('aria-pressed')).toBe('true');
    expect(btn?.getAttribute('title')).toBe('已恢复编辑');
    expect(btn?.textContent).toBe('已恢复编辑');
  });

  it('click toggles to readonly=true (PUT) and fires onChange', async () => {
    const onChange = vi.fn();
    render(<ReadonlyToggle channelId="chan-1" onChange={onChange} />);
    const btn = container!.querySelector('[data-testid="readonly-toggle"]') as HTMLButtonElement;
    await act(async () => {
      btn.click();
      await Promise.resolve();
      await new Promise((r) => setTimeout(r, 0));
    });
    expect(onChange).toHaveBeenCalledWith(true);
    const after = container!.querySelector('[data-testid="readonly-toggle"]');
    expect(after?.getAttribute('data-readonly')).toBe('true');
    expect(after?.textContent).toBe('已恢复编辑');
  });

  it('click toggles unset (DELETE) when initialReadonly=true', async () => {
    (globalThis as any).fetch = vi.fn(async () =>
      new Response(JSON.stringify({
        channel_id: 'chan-1',
        collapsed: 0,
        readonly: false,
      }), { status: 200, headers: { 'Content-Type': 'application/json' } }),
    );
    const onChange = vi.fn();
    render(<ReadonlyToggle channelId="chan-1" initialReadonly={true} onChange={onChange} />);
    const btn = container!.querySelector('[data-testid="readonly-toggle"]') as HTMLButtonElement;
    await act(async () => {
      btn.click();
      await Promise.resolve();
      await new Promise((r) => setTimeout(r, 0));
    });
    expect(onChange).toHaveBeenCalledWith(false);
    const after = container!.querySelector('[data-testid="readonly-toggle"]');
    expect(after?.getAttribute('data-readonly')).toBe('false');
    expect(after?.textContent).toBe('已设为只读');
  });

  it('on error fires onError with toast literal', async () => {
    (globalThis as any).fetch = vi.fn(async () =>
      new Response(JSON.stringify({ error: 'channel.readonly_no_send' }), {
        status: 403,
        headers: { 'Content-Type': 'application/json' },
      }),
    );
    const onError = vi.fn();
    render(<ReadonlyToggle channelId="chan-1" onError={onError} />);
    const btn = container!.querySelector('[data-testid="readonly-toggle"]') as HTMLButtonElement;
    await act(async () => {
      btn.click();
      await Promise.resolve();
      await new Promise((r) => setTimeout(r, 0));
    });
    expect(onError).toHaveBeenCalled();
    const arg = onError.mock.calls[0][0];
    expect(arg).toBe('只读频道, 仅创建者可发言');
  });
});
