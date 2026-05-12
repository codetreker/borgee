// MutedChannelIndicator.test.tsx — CHN-7.2 indicator DOM byte-identical checks,
// MuteBit byte-identical alignment, and synonym rejection.
import React from 'react';
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { createRoot } from 'react-dom/client';
import { act } from 'react-dom/test-utils';
import { MutedChannelIndicator } from '../components/MutedChannelIndicator';
import { MUTE_BIT, isMuted } from '../lib/mute';

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
});

describe('MutedChannelIndicator — CHN-7.2 DOM and locked labels', () => {
  it('muted=true renders the indicator with `已静音` text and 🔕 emoji', () => {
    const root = createRoot(container!);
    act(() => {
      root.render(<MutedChannelIndicator muted={true} />);
    });
    const ind = container!.querySelector('[data-testid="muted-channel-indicator"]') as HTMLElement;
    expect(ind).not.toBeNull();
    expect(ind.textContent).toContain('已静音');
    expect(ind.textContent).toContain('🔕');
    expect(ind.getAttribute('title')).toBe('已静音');
  });

  it('muted=false renders nothing', () => {
    const root = createRoot(container!);
    act(() => {
      root.render(<MutedChannelIndicator muted={false} />);
    });
    const ind = container!.querySelector('[data-testid="muted-channel-indicator"]');
    expect(ind).toBeNull();
  });

  it('MUTE_BIT stays byte-identical with isMuted behavior', () => {
    expect(MUTE_BIT).toBe(2);
    expect(isMuted(0)).toBe(false);
    expect(isMuted(1)).toBe(false); // collapsed only
    expect(isMuted(2)).toBe(true); // muted only
    expect(isMuted(3)).toBe(true); // collapsed + muted
    expect(isMuted(null)).toBe(false);
    expect(isMuted(undefined)).toBe(false);
  });

  it('synonym rejection: forbidden mute labels do not appear in user-visible text', () => {
    const root = createRoot(container!);
    act(() => {
      root.render(<MutedChannelIndicator muted={true} />);
    });
    const ind = container!.querySelector('[data-testid="muted-channel-indicator"]') as HTMLElement;
    const text = ind.textContent || '';
    const forbidden = ['silence', 'dnd', 'disturb', 'quiet', '屏蔽', '关闭通知', '勿扰'];
    for (const f of forbidden) {
      expect(text).not.toContain(f);
    }
    expect(text.toLowerCase()).not.toContain('mute');
  });
});
