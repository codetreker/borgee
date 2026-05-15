// ReactionSummary.test.tsx — DM-5.2 vitest acceptance.
//
// 锚: dm-5-stance-checklist.md §4 + content-lock §1+§2.
// 5 case: chip DOM anchor / count anchor + 文案 / mine highlight / click toggle / empty state.

import React from 'react';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react';

vi.mock('../lib/api', async () => {
  const actual = await vi.importActual<typeof import('../lib/api')>('../lib/api');
  return {
    ...actual,
    addReaction: vi.fn().mockResolvedValue(undefined),
    removeReaction: vi.fn().mockResolvedValue(undefined),
  };
});

import ReactionSummary from '../components/ReactionSummary';
import * as api from '../lib/api';
import type { AggregatedReaction } from '../lib/api';

let container: HTMLDivElement | null = null;
let root: Root | null = null;

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
  vi.clearAllMocks();
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

describe('ReactionSummary — DM-5.2 client', () => {
  it('设计 ④ chip DOM `data-dm5-reaction-chip` 锚', async () => {
    const reactions: AggregatedReaction[] = [
      { emoji: '👍', count: 3, user_ids: ['u-1', 'u-2', 'u-3'] },
      { emoji: '🔥', count: 1, user_ids: ['u-2'] },
    ];
    await render(<ReactionSummary messageId="m-1" reactions={reactions} currentUserId="u-1" />);
    const chips = container!.querySelectorAll('[data-dm5-reaction-chip]');
    expect(chips.length).toBe(2);
    expect(chips[0].getAttribute('data-dm5-reaction-chip')).toBe('👍');
    expect(chips[1].getAttribute('data-dm5-reaction-chip')).toBe('🔥');
  });

  it('设计 ④ count anchor + 文案 byte-identical `{emoji} {count}`', async () => {
    const reactions: AggregatedReaction[] = [
      { emoji: '👍', count: 5, user_ids: ['u-1'] },
    ];
    await render(<ReactionSummary messageId="m-2" reactions={reactions} currentUserId="u-1" />);
    const chip = container!.querySelector('[data-dm5-reaction-chip]') as HTMLButtonElement;
    expect(chip.getAttribute('data-dm5-reaction-count')).toBe('5');
    expect(chip.textContent).toBe('👍 5');
  });

  it('设计 ④ current user reacted → `data-dm5-reaction-mine` highlight', async () => {
    const reactions: AggregatedReaction[] = [
      { emoji: '👍', count: 2, user_ids: ['u-1', 'u-2'] },
      { emoji: '🔥', count: 1, user_ids: ['u-2'] },
    ];
    await render(<ReactionSummary messageId="m-3" reactions={reactions} currentUserId="u-1" />);
    const mine = container!.querySelector('[data-dm5-reaction-mine]');
    const allChips = container!.querySelectorAll('[data-dm5-reaction-chip]');
    expect(mine).not.toBeNull();
    expect(mine!.getAttribute('data-dm5-reaction-chip')).toBe('👍');
    // 反向 sanity: 🔥 is NOT mine (u-1 not in user_ids).
    const fireChip = allChips[1];
    expect(fireChip.hasAttribute('data-dm5-reaction-mine')).toBe(false);
  });

  it('设计 ④ click toggle — mine: DELETE; not-mine: PUT', async () => {
    const reactions: AggregatedReaction[] = [
      { emoji: '👍', count: 2, user_ids: ['u-1', 'u-2'] },
      { emoji: '🔥', count: 1, user_ids: ['u-2'] },
    ];
    await render(<ReactionSummary messageId="m-4" reactions={reactions} currentUserId="u-1" />);
    const chips = container!.querySelectorAll('[data-dm5-reaction-chip]');
    // Click my reaction (👍) → DELETE.
    await act(async () => {
      (chips[0] as HTMLButtonElement).click();
    });
    for (let i = 0; i < 5; i++) {
      await act(async () => {
        await Promise.resolve();
      });
    }
    expect(api.removeReaction).toHaveBeenCalledWith('m-4', '👍');
    expect(api.addReaction).not.toHaveBeenCalled();

    // Click not-mine reaction (🔥) → PUT.
    await act(async () => {
      (chips[1] as HTMLButtonElement).click();
    });
    for (let i = 0; i < 5; i++) {
      await act(async () => {
        await Promise.resolve();
      });
    }
    expect(api.addReaction).toHaveBeenCalledWith('m-4', '🔥');
  });

  it('设计 ④ empty reactions — 0 chip 渲染 (返 null)', async () => {
    await render(<ReactionSummary messageId="m-5" reactions={[]} currentUserId="u-1" />);
    expect(container!.querySelector('[data-testid="dm5-reaction-summary"]')).toBeNull();
    expect(container!.querySelectorAll('[data-dm5-reaction-chip]').length).toBe(0);
  });
});
