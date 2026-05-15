// EmojiPickerPopover.test.tsx — DM-9.2 vitest acceptance (7 case).
//
// 锚: docs/qa/dm-9-stance-checklist.md §1-§5 + content-lock §1+§2+§3.

import React from 'react';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react';

vi.mock('../lib/api', async () => {
  const actual = await vi.importActual<typeof import('../lib/api')>('../lib/api');
  return {
    ...actual,
    addReaction: vi.fn().mockResolvedValue(undefined),
  };
});

import EmojiPickerPopover from '../components/EmojiPickerPopover';
import * as api from '../lib/api';

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

describe('EmojiPickerPopover — DM-9.2 client', () => {
  it('§2.1 TestDM9_ToggleByteIdentical — toggle "+" + title "添加表情"', async () => {
    await render(<EmojiPickerPopover messageId="m1" />);
    const toggle = container!.querySelector('[data-dm9-emoji-picker-toggle]') as HTMLButtonElement;
    expect(toggle).not.toBeNull();
    expect(toggle.textContent).toBe('+');
    expect(toggle.title).toBe('添加表情');
    expect(toggle.getAttribute('data-dm9-popover-open')).toBe('false');
  });

  it('§2.2 TestDM9_DefaultClosed — popover 默认 closed', async () => {
    await render(<EmojiPickerPopover messageId="m1" />);
    expect(container!.querySelector('[data-dm9-emoji-picker-popover]')).toBeNull();
  });

  it('§2.3 TestDM9_OpenAndPresetOrder — toggle click → 5 emoji 顺序 byte-identical', async () => {
    await render(<EmojiPickerPopover messageId="m1" />);
    const toggle = container!.querySelector('[data-dm9-emoji-picker-toggle]') as HTMLButtonElement;
    await act(async () => {
      toggle.click();
    });
    const popover = container!.querySelector('[data-dm9-emoji-picker-popover]');
    expect(popover).not.toBeNull();
    const options = container!.querySelectorAll('[data-dm9-emoji-option]');
    expect(options.length).toBe(5);
    const emojis = Array.from(options).map((o) => o.getAttribute('data-dm9-emoji-option'));
    expect(emojis).toEqual(['👍', '❤️', '😄', '🎉', '🚀']);
    // toggle reflects open state.
    expect(toggle.getAttribute('data-dm9-popover-open')).toBe('true');
  });

  it('§2.4 TestDM9_EmojiClickTriggersAddReaction — click → addReaction + close + onChanged', async () => {
    const onChanged = vi.fn();
    await render(<EmojiPickerPopover messageId="m1" onChanged={onChanged} />);
    const toggle = container!.querySelector('[data-dm9-emoji-picker-toggle]') as HTMLButtonElement;
    await act(async () => {
      toggle.click();
    });
    const heart = container!.querySelector('[data-dm9-emoji-option="❤️"]') as HTMLButtonElement;
    await act(async () => {
      heart.click();
      await Promise.resolve();
    });
    // Wait microtask flush
    await act(async () => {
      await Promise.resolve();
    });
    expect(api.addReaction).toHaveBeenCalledWith('m1', '❤️');
    expect(container!.querySelector('[data-dm9-emoji-picker-popover]')).toBeNull();
    expect(onChanged).toHaveBeenCalledTimes(1);
  });

  it('§2.5 TestDM9_EscapeCloses — Escape key 关 popover', async () => {
    await render(<EmojiPickerPopover messageId="m1" />);
    const toggle = container!.querySelector('[data-dm9-emoji-picker-toggle]') as HTMLButtonElement;
    await act(async () => {
      toggle.click();
    });
    expect(container!.querySelector('[data-dm9-emoji-picker-popover]')).not.toBeNull();
    await act(async () => {
      document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape' }));
    });
    expect(container!.querySelector('[data-dm9-emoji-picker-popover]')).toBeNull();
  });

  it('§2.5b TestDM9_OutsideClickCloses — outside mousedown 关 popover', async () => {
    await render(<EmojiPickerPopover messageId="m1" />);
    const toggle = container!.querySelector('[data-dm9-emoji-picker-toggle]') as HTMLButtonElement;
    await act(async () => {
      toggle.click();
    });
    await act(async () => {
      // Click on body (outside popover)
      document.body.dispatchEvent(new MouseEvent('mousedown', { bubbles: true }));
    });
    expect(container!.querySelector('[data-dm9-emoji-picker-popover]')).toBeNull();
  });

  it('§2.6 TestDM9_DOMAttrs — 4 data-attr 锚 byte-identical', async () => {
    await render(<EmojiPickerPopover messageId="m1" />);
    const toggle = container!.querySelector('[data-dm9-emoji-picker-toggle]') as HTMLButtonElement;
    expect(toggle.hasAttribute('data-dm9-popover-open')).toBe(true);
    await act(async () => {
      toggle.click();
    });
    expect(container!.querySelectorAll('[data-dm9-emoji-picker-popover]').length).toBe(1);
    expect(container!.querySelectorAll('[data-dm9-emoji-option]').length).toBe(5);
  });

  it('§2.7 TestDM9_NoStorageWrite — 不写 sessionStorage / localStorage', async () => {
    const sessionSpy = vi.spyOn(Storage.prototype, 'setItem');
    await render(<EmojiPickerPopover messageId="m1" />);
    const toggle = container!.querySelector('[data-dm9-emoji-picker-toggle]') as HTMLButtonElement;
    await act(async () => {
      toggle.click();
    });
    const heart = container!.querySelector('[data-dm9-emoji-option="❤️"]') as HTMLButtonElement;
    await act(async () => {
      heart.click();
      await Promise.resolve();
    });
    const dm9Calls = sessionSpy.mock.calls.filter((c) =>
      String(c[0]).toLowerCase().includes('dm9'),
    );
    expect(dm9Calls.length).toBe(0);
    sessionSpy.mockRestore();
  });
});
