// QuotedCommentBlock.test.tsx — CV-13.2 vitest acceptance (6 case).
//
// 锚: docs/qa/cv-13-stance-checklist.md §1-§5 + content-lock §1+§2.
// 6 case (cv-13.md §2):
//   1. happy-path render (parent existing) — DOM data-attr + author + body
//   2. missing parent (null) → fallback "(原消息已删除)" exact-match
//   3. deleted_at 非 null → 同 missing fallback
//   4. collapse toggle (展开/收起) exact-match 文案
//   5. truncate 200 chars + "…" suffix (collapsed only); expanded 显示完整
//   6. DOM 4 data-attr 锚 exact-match

import React from 'react';
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react';
import QuotedCommentBlock from '../components/QuotedCommentBlock';
import type { Message } from '../types';

let container: HTMLDivElement | null = null;
let root: Root | null = null;

function makeMsg(overrides: Partial<Message> = {}): Message {
  return {
    id: 'msg-parent-1',
    channel_id: 'ch-1',
    sender_id: 'user-a',
    sender_name: 'Alice',
    content: 'parent body',
    content_type: 'text',
    reply_to_id: null,
    created_at: 1700000000000,
    edited_at: null,
    deleted_at: null,
    ...overrides,
  };
}

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
  root = null;
});

describe('CV-13.2 QuotedCommentBlock', () => {
  it('§2.1 TestCV13_HappyPath — parent existing renders author + body + DOM attrs', () => {
    const msg = makeMsg();
    act(() => {
      root = createRoot(container!);
      root.render(<QuotedCommentBlock quotedMessage={msg} />);
    });
    const block = container!.querySelector('[data-cv13-quoted-block]');
    expect(block).not.toBeNull();
    expect(block!.getAttribute('data-cv13-quoted-id')).toBe('msg-parent-1');
    expect(block!.getAttribute('data-cv13-collapsed')).toBe('true');
    const author = container!.querySelector('[data-cv13-quoted-author]');
    expect(author!.textContent).toBe('@Alice');
    expect(container!.textContent).toContain('> parent body');
  });

  it('§2.2 TestCV13_MissingFallback — null parent → "(原消息已删除)" exact-match', () => {
    act(() => {
      root = createRoot(container!);
      root.render(<QuotedCommentBlock quotedMessage={null} />);
    });
    expect(container!.textContent).toContain('(原消息已删除)');
    const block = container!.querySelector('[data-cv13-quoted-block]');
    expect(block!.getAttribute('data-cv13-quoted-id')).toBe('');
  });

  it('§2.2b TestCV13_DeletedFallback — deleted_at set → 同 fallback', () => {
    const msg = makeMsg({ deleted_at: 1700000001000 });
    act(() => {
      root = createRoot(container!);
      root.render(<QuotedCommentBlock quotedMessage={msg} />);
    });
    expect(container!.textContent).toContain('(原消息已删除)');
  });

  it('§2.3 TestCV13_CollapseToggle — 展开/收起 exact-match (long body shows toggle)', () => {
    const longBody = 'x'.repeat(250);
    const msg = makeMsg({ content: longBody });
    act(() => {
      root = createRoot(container!);
      root.render(<QuotedCommentBlock quotedMessage={msg} />);
    });
    const toggle = container!.querySelector('[data-cv13-toggle]') as HTMLButtonElement;
    expect(toggle).not.toBeNull();
    expect(toggle.textContent).toBe('展开');
    act(() => {
      toggle.click();
    });
    const toggle2 = container!.querySelector('[data-cv13-toggle]') as HTMLButtonElement;
    expect(toggle2.textContent).toBe('收起');
    const block = container!.querySelector('[data-cv13-quoted-block]');
    expect(block!.getAttribute('data-cv13-collapsed')).toBe('false');
  });

  it('§2.4 TestCV13_Truncate200 — collapsed body 截 200 + "…" suffix; expanded 完整', () => {
    const longBody = 'a'.repeat(250);
    const msg = makeMsg({ content: longBody });
    act(() => {
      root = createRoot(container!);
      root.render(<QuotedCommentBlock quotedMessage={msg} />);
    });
    // collapsed: 200 + ellipsis
    expect(container!.textContent).toContain('a'.repeat(200) + '…');
    expect(container!.textContent).not.toContain('a'.repeat(250));
    const toggle = container!.querySelector('[data-cv13-toggle]') as HTMLButtonElement;
    act(() => {
      toggle.click();
    });
    expect(container!.textContent).toContain('a'.repeat(250));
  });

  it('§2.5 TestCV13_DOMAttrs — 4 data-attr 锚 exact-match', () => {
    const msg = makeMsg();
    act(() => {
      root = createRoot(container!);
      root.render(<QuotedCommentBlock quotedMessage={msg} />);
    });
    expect(container!.querySelectorAll('[data-cv13-quoted-block]').length).toBe(1);
    expect(container!.querySelectorAll('[data-cv13-quoted-author]').length).toBe(1);
    const block = container!.querySelector('[data-cv13-quoted-block]')!;
    expect(block.hasAttribute('data-cv13-quoted-id')).toBe(true);
    expect(block.hasAttribute('data-cv13-collapsed')).toBe(true);
  });

  it('§2.6 TestCV13_NoStorageWrite — render 不写 sessionStorage / localStorage', () => {
    const sessionSpy = vi.spyOn(Storage.prototype, 'setItem');
    const msg = makeMsg();
    act(() => {
      root = createRoot(container!);
      root.render(<QuotedCommentBlock quotedMessage={msg} />);
    });
    const cv13Calls = sessionSpy.mock.calls.filter((c) =>
      String(c[0]).toLowerCase().includes('cv13'),
    );
    expect(cv13Calls.length).toBe(0);
    sessionSpy.mockRestore();
  });
});

// vi import for the spy above
import { vi } from 'vitest';
