// MuteButton.test.tsx — CHN-7.2 MuteButton DOM byte-identical checks,
// locked Chinese labels, synonym rejection, and click → muteChannel API calls.
import React from 'react';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { createRoot } from 'react-dom/client';
import { act } from 'react';
import { MuteButton } from '../components/MuteButton';
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

describe('MuteButton — CHN-7.2 locked labels and DOM literals', () => {
  it('unmuted state renders label=`静音` and data-action="mute"', () => {
    const root = createRoot(container!);
    act(() => {
      root.render(<MuteButton channelId="c-1" muted={false} />);
    });
    const btn = container!.querySelector('button') as HTMLButtonElement;
    expect(btn.textContent).toBe('静音');
    expect(btn.getAttribute('data-action')).toBe('mute');
  });

  it('muted state renders label=`取消静音` and data-action="unmute"', () => {
    const root = createRoot(container!);
    act(() => {
      root.render(<MuteButton channelId="c-1" muted={true} />);
    });
    const btn = container!.querySelector('button') as HTMLButtonElement;
    expect(btn.textContent).toBe('取消静音');
    expect(btn.getAttribute('data-action')).toBe('unmute');
  });

  it('click → muteChannel(id, true) + onChange(true)', async () => {
    const spy = vi
      .spyOn(api, 'muteChannel')
      .mockResolvedValue({ collapsed: 2, muted: true });
    const onChange = vi.fn();
    const root = createRoot(container!);
    act(() => {
      root.render(
        <MuteButton channelId="c-1" muted={false} onChange={onChange} />,
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

  it('click on muted → muteChannel(id, false)', async () => {
    const spy = vi
      .spyOn(api, 'muteChannel')
      .mockResolvedValue({ collapsed: 0, muted: false });
    const root = createRoot(container!);
    act(() => {
      root.render(<MuteButton channelId="c-1" muted={true} />);
    });
    const btn = container!.querySelector('button') as HTMLButtonElement;
    await act(async () => {
      btn.click();
      await new Promise(r => setTimeout(r, 0));
    });
    expect(spy).toHaveBeenCalledWith('c-1', false);
  });

  it('synonym rejection: forbidden mute labels do not appear in user-visible button text', () => {
    const root = createRoot(container!);
    act(() => {
      root.render(<MuteButton channelId="c-1" muted={false} />);
    });
    const btn = container!.querySelector('button') as HTMLButtonElement;
    const text = btn.textContent || '';
    const forbidden = ['silence', 'dnd', 'disturb', 'quiet', '屏蔽', '关闭通知', '勿扰'];
    for (const f of forbidden) {
      expect(text).not.toContain(f);
    }
    // English 'mute' must not appear in user-visible text; data-action is separate.
    expect(text.toLowerCase()).not.toContain('mute');
  });
});
