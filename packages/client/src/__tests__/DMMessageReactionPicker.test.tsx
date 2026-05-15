// DMMessageReactionPicker.test.tsx — DM-12.2 vitest acceptance.

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
    getMessageReactions: vi.fn().mockResolvedValue({ reactions: [] }),
  };
});

import DMMessageReactionPicker from '../components/DMMessageReactionPicker';
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
  root = null;
});

async function render(node: React.ReactElement) {
  root = createRoot(container!);
  await act(async () => {
    root!.render(node);
  });
}

describe('DMMessageReactionPicker — DM-12.2 composite', () => {
  it('§2.1 renders root + picker toggle on mount', async () => {
    await render(<DMMessageReactionPicker messageId="m1" currentUserId="u1" initialReactions={[]} />);
    expect(container!.querySelector('[data-dm12-reaction-picker]')).not.toBeNull();
    expect(container!.querySelector('[data-dm9-emoji-picker-toggle]')).not.toBeNull();
  });

  it('§2.2 with empty initialReactions does NOT render ReactionSummary', async () => {
    await render(<DMMessageReactionPicker messageId="m1" currentUserId="u1" initialReactions={[]} />);
    expect(container!.querySelector('[data-dm5-reaction-chip]')).toBeNull();
  });

  it('§2.3 with non-empty initialReactions renders ReactionSummary chips', async () => {
    const initial: AggregatedReaction[] = [
      { emoji: '👍', count: 2, user_ids: ['u2', 'u3'] },
    ];
    await render(<DMMessageReactionPicker messageId="m1" currentUserId="u1" initialReactions={initial} />);
    const chip = container!.querySelector('[data-dm5-reaction-chip]');
    expect(chip).not.toBeNull();
    expect(chip!.getAttribute('data-dm5-reaction-chip')).toBe('👍');
  });

  it('§2.4 auto-fetches reactions on mount when initialReactions undefined', async () => {
    (api.getMessageReactions as ReturnType<typeof vi.fn>).mockResolvedValueOnce({
      reactions: [{ emoji: '🎉', count: 1, user_ids: ['u4'] }],
    });
    await render(<DMMessageReactionPicker messageId="m1" currentUserId="u1" />);
    await act(async () => {
      await Promise.resolve();
      await Promise.resolve();
    });
    expect(api.getMessageReactions).toHaveBeenCalledWith('m1');
    expect(container!.querySelector('[data-dm5-reaction-chip="🎉"]')).not.toBeNull();
  });

  it('§2.5 emoji picker click → addReaction + refetch chain', async () => {
    (api.getMessageReactions as ReturnType<typeof vi.fn>)
      .mockResolvedValueOnce({ reactions: [] })
      .mockResolvedValueOnce({ reactions: [{ emoji: '❤️', count: 1, user_ids: ['u1'] }] });

    await render(<DMMessageReactionPicker messageId="m1" currentUserId="u1" initialReactions={[]} />);
    const toggle = container!.querySelector('[data-dm9-emoji-picker-toggle]') as HTMLButtonElement;
    await act(async () => {
      toggle.click();
    });
    const heart = container!.querySelector('[data-dm9-emoji-option="❤️"]') as HTMLButtonElement;
    await act(async () => {
      heart.click();
      await Promise.resolve();
      await Promise.resolve();
    });
    expect(api.addReaction).toHaveBeenCalledWith('m1', '❤️');
    expect(api.getMessageReactions).toHaveBeenCalledWith('m1');
  });

  it('§2.6 DOM data-attr 锁 (data-dm12-reaction-picker root + delegated DM-9/DM-5)', async () => {
    const initial: AggregatedReaction[] = [{ emoji: '👍', count: 1, user_ids: ['u1'] }];
    await render(<DMMessageReactionPicker messageId="m1" currentUserId="u1" initialReactions={initial} />);
    const root = container!.querySelector('[data-dm12-reaction-picker]')!;
    expect(root.getAttribute('data-dm12-loading')).toBe('false');
    // Delegated children — DM-9 + DM-5 anchors present (反向不重复 attr).
    expect(root.querySelector('[data-dm9-emoji-picker-toggle]')).not.toBeNull();
    expect(root.querySelector('[data-dm5-reaction-chip]')).not.toBeNull();
  });

  it('§2.7 不写 sessionStorage / localStorage', async () => {
    const setItemSpy = vi.spyOn(Storage.prototype, 'setItem');
    await render(<DMMessageReactionPicker messageId="m1" currentUserId="u1" initialReactions={[]} />);
    const dm12Calls = setItemSpy.mock.calls.filter((c) =>
      String(c[0]).toLowerCase().includes('dm12'),
    );
    expect(dm12Calls.length).toBe(0);
    setItemSpy.mockRestore();
  });
});
