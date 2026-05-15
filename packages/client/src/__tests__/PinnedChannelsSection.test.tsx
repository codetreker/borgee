// PinnedChannelsSection.test.tsx — CHN-6.2 top section DOM byte-identical checks,
// pinned-channel filtering byte-identical invariant, empty state, and synonym rejection.
import React from 'react';
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { createRoot } from 'react-dom/client';
import { act } from 'react';
import { PinnedChannelsSection } from '../components/PinnedChannelsSection';
import { POSITION_PIN_THRESHOLD, isPinned } from '../lib/pin';
import type { Channel } from '../types';

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

function ch(id: string, name: string, position: number): Channel & { position: number } {
  return {
    id,
    name,
    org_id: 'org-1',
    creator_id: 'u-1',
    visibility: 'public',
    type: 'channel',
    created_at: 1700000000000,
    position,
  } as unknown as Channel & { position: number };
}

describe('PinnedChannelsSection — CHN-6.2 DOM, filtering, and empty state', () => {
  it('list rendering + DOM byte-identical', () => {
    const root = createRoot(container!);
    act(() => {
      root.render(
        <PinnedChannelsSection
          channels={[
            ch('c-1', 'pinned-a', -1700000000000),
            ch('c-2', 'pinned-b', -1700000001000),
            ch('c-3', 'normal-x', 1),
          ]}
        />,
      );
    });

    const section = container!.querySelector('[data-testid="pinned-channels-section"]');
    expect(section).not.toBeNull();
    const header = section!.querySelector('header');
    expect(header?.textContent).toBe('已置顶频道');

    // Only 2 pinned items rendered (position < 0 filter).
    const items = container!.querySelectorAll('[data-pinned="true"]');
    expect(items.length).toBe(2);
  });

  it('empty state renders no section', () => {
    const root = createRoot(container!);
    act(() => {
      root.render(
        <PinnedChannelsSection
          channels={[ch('c-1', 'normal', 1), ch('c-2', 'normal', 2)]}
        />,
      );
    });
    const section = container!.querySelector('[data-testid="pinned-channels-section"]');
    expect(section).toBeNull();
  });

  it('POSITION_PIN_THRESHOLD stays byte-identical with isPinned behavior', () => {
    expect(POSITION_PIN_THRESHOLD).toBe(0);
    expect(isPinned(-1)).toBe(true);
    expect(isPinned(0)).toBe(false);
    expect(isPinned(1)).toBe(false);
  });

  it('synonym rejection: forbidden pin labels do not appear in the DOM', () => {
    const root = createRoot(container!);
    act(() => {
      root.render(
        <PinnedChannelsSection
          channels={[ch('c-1', 'pinned', -1700000000000)]}
        />,
      );
    });
    const html = container!.innerHTML;
    const forbidden = ['收藏', '标星', 'star', 'favorite', '顶置', '钉住'];
    for (const f of forbidden) {
      expect(html).not.toContain(f);
    }
  });
});
