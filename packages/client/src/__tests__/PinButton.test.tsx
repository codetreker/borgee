// PinButton.test.tsx — CHN-6.2 PinButton DOM byte-identical checks,
// locked Chinese labels, synonym rejection, and click → pinChannel API calls.
import React from 'react';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { createRoot } from 'react-dom/client';
import { act } from 'react-dom/test-utils';
import { PinButton } from '../components/PinButton';
import * as api from '../lib/api';

let container: HTMLDivElement | null = null;

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
});

afterEach(() => {
  if (container) {
    document.body.removeChild(container);
    container = null;
  }
  vi.restoreAllMocks();
});

describe('PinButton — CHN-6.2 locked labels and DOM literals', () => {
  it('unpinned state renders label=`置顶` and data-action="pin"', () => {
    const root = createRoot(container!);
    act(() => {
      root.render(<PinButton channelId="c-1" pinned={false} />);
    });
    const btn = container!.querySelector('button') as HTMLButtonElement;
    expect(btn.textContent).toBe('置顶');
    expect(btn.getAttribute('data-action')).toBe('pin');
  });

  it('pinned state renders label=`取消置顶` and data-action="unpin"', () => {
    const root = createRoot(container!);
    act(() => {
      root.render(<PinButton channelId="c-1" pinned={true} />);
    });
    const btn = container!.querySelector('button') as HTMLButtonElement;
    expect(btn.textContent).toBe('取消置顶');
    expect(btn.getAttribute('data-action')).toBe('unpin');
  });

  it('click → pinChannel(id, true) + onChange(true)', async () => {
    const spy = vi
      .spyOn(api, 'pinChannel')
      .mockResolvedValue({ position: -1, pinned: true });
    const onChange = vi.fn();
    const root = createRoot(container!);
    act(() => {
      root.render(
        <PinButton channelId="c-1" pinned={false} onChange={onChange} />,
      );
    });
    const btn = container!.querySelector('button') as HTMLButtonElement;
    await act(async () => {
      btn.click();
      await new Promise(r => setTimeout(r, 0));
    });
    expect(spy).toHaveBeenCalledWith('c-1', true);
    expect(onChange).toHaveBeenCalledWith(true);
  });

  it('click on pinned → pinChannel(id, false)', async () => {
    const spy = vi
      .spyOn(api, 'pinChannel')
      .mockResolvedValue({ position: 1, pinned: false });
    const root = createRoot(container!);
    act(() => {
      root.render(<PinButton channelId="c-1" pinned={true} />);
    });
    const btn = container!.querySelector('button') as HTMLButtonElement;
    await act(async () => {
      btn.click();
      await new Promise(r => setTimeout(r, 0));
    });
    expect(spy).toHaveBeenCalledWith('c-1', false);
  });

  it('synonym rejection: forbidden pin labels do not appear in the DOM', () => {
    const root = createRoot(container!);
    act(() => {
      root.render(<PinButton channelId="c-1" pinned={false} />);
    });
    const html = container!.innerHTML;
    const forbidden = ['收藏', '标星', 'star', 'favorite', '顶置', '钉住'];
    for (const f of forbidden) {
      expect(html).not.toContain(f);
    }
    // `top` needs an exact-text check because ancestor class names may contain it.
    expect(html.toLowerCase()).not.toContain('>top<');
  });
});
