// CommentUnreadBadge.test.tsx — CV-14.2 vitest acceptance.
//
// 锚: docs/qa/cv-14-stance-checklist.md §1-§5 + content-lock §1+§2.
// 8 case (cv-14.md §2):
//   1. count==0 不渲染 (反向锁)
//   2. sender_id != currentUserId → 计数 +1 + 文案 byte-identical
//   3. sender_id == currentUserId → 不计数 (自己发的不计)
//   4. count > 99 → "99+" overflow display
//   5. click → reset count + 消失
//   6. DOM data-attr 2 锚 byte-identical
//   7. 不写 sessionStorage / localStorage
//   8. 多 frame 累加正确

import React from 'react';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react';
import CommentUnreadBadge from '../components/CommentUnreadBadge';
import { dispatchArtifactCommentAdded } from '../hooks/useWsHubFrames';
import type { ArtifactCommentAddedFrame } from '../types/ws-frames';

let container: HTMLDivElement | null = null;
let root: Root | null = null;

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

async function render(node: React.ReactElement) {
  root = createRoot(container!);
  await act(async () => {
    root!.render(node);
  });
}

function makeFrame(senderId: string, cursor = 1): ArtifactCommentAddedFrame {
  return {
    type: 'artifact_comment_added',
    cursor,
    comment_id: `c-${cursor}`,
    artifact_id: 'art-1',
    channel_id: 'artifact:art-1',
    sender_id: senderId,
    sender_role: 'human',
    body_preview: 'hello',
    created_at: 1700000000000 + cursor,
  };
}

describe('CommentUnreadBadge — CV-14.2 client', () => {
  it('§2.5 TestCV14_ZeroNotRendered — count==0 时不渲染', async () => {
    await render(<CommentUnreadBadge currentUserId="u-1" />);
    expect(container!.querySelector('[data-cv14-comment-unread-badge]')).toBeNull();
  });

  it('§2.1 TestCV14_OtherSenderIncrements + §2.3 TestCV14_LabelByteIdentical', async () => {
    await render(<CommentUnreadBadge currentUserId="u-1" />);
    await act(async () => {
      dispatchArtifactCommentAdded(makeFrame('u-2', 1));
      await Promise.resolve();
    });
    await act(async () => {
      dispatchArtifactCommentAdded(makeFrame('u-3', 2));
      await Promise.resolve();
    });
    const badge = container!.querySelector('[data-cv14-comment-unread-badge]') as HTMLButtonElement;
    expect(badge).not.toBeNull();
    expect(badge.getAttribute('data-cv14-unread-count')).toBe('2');
    expect(badge.title).toBe('2 条新评论');
    expect(badge.textContent).toContain('2 条新评论');
  });

  it('§2.2 TestCV14_SelfSenderNotCounted — 自己发的 frame 不计数', async () => {
    await render(<CommentUnreadBadge currentUserId="u-1" />);
    await act(async () => {
      dispatchArtifactCommentAdded(makeFrame('u-1', 1));
      await Promise.resolve();
    });
    await act(async () => {
      dispatchArtifactCommentAdded(makeFrame('u-1', 2));
      await Promise.resolve();
    });
    expect(container!.querySelector('[data-cv14-comment-unread-badge]')).toBeNull();
  });

  it('§2.4 TestCV14_Overflow99Plus — count > 99 → "99+"', async () => {
    await render(<CommentUnreadBadge currentUserId="u-1" />);
    for (let i = 1; i <= 105; i++) {
      await act(async () => {
        dispatchArtifactCommentAdded(makeFrame('u-other', i));
        await Promise.resolve();
      });
    }
    const badge = container!.querySelector('[data-cv14-comment-unread-badge]') as HTMLButtonElement;
    expect(badge).not.toBeNull();
    expect(badge.getAttribute('data-cv14-unread-count')).toBe('99+');
    expect(badge.title).toBe('99+ 条新评论');
  });

  it('§2.6 TestCV14_ClickResets — click 重置 count, badge 消失', async () => {
    const onClick = vi.fn();
    await render(<CommentUnreadBadge currentUserId="u-1" onClick={onClick} />);
    await act(async () => {
      dispatchArtifactCommentAdded(makeFrame('u-other', 1));
      await Promise.resolve();
    });
    const badge = container!.querySelector('[data-cv14-comment-unread-badge]') as HTMLButtonElement;
    expect(badge).not.toBeNull();
    await act(async () => {
      badge.click();
    });
    expect(container!.querySelector('[data-cv14-comment-unread-badge]')).toBeNull();
    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it('§2.7 TestCV14_DOMAttrs — 2 data-attr 锚 byte-identical', async () => {
    await render(<CommentUnreadBadge currentUserId="u-1" />);
    await act(async () => {
      dispatchArtifactCommentAdded(makeFrame('u-other', 1));
      await Promise.resolve();
    });
    expect(container!.querySelectorAll('[data-cv14-comment-unread-badge]').length).toBe(1);
    const badge = container!.querySelector('[data-cv14-comment-unread-badge]')!;
    expect(badge.hasAttribute('data-cv14-unread-count')).toBe(true);
  });

  it('§2.8 TestCV14_NoStorageWrite — render + frame 不写 sessionStorage / localStorage', async () => {
    const sessionSpy = vi.spyOn(Storage.prototype, 'setItem');
    await render(<CommentUnreadBadge currentUserId="u-1" />);
    await act(async () => {
      dispatchArtifactCommentAdded(makeFrame('u-other', 1));
      await Promise.resolve();
    });
    const cv14Calls = sessionSpy.mock.calls.filter((c) =>
      String(c[0]).toLowerCase().includes('cv14'),
    );
    expect(cv14Calls.length).toBe(0);
    sessionSpy.mockRestore();
  });
});
