// ArtifactCommentsMentionBadge.test.tsx — CV-9.2 vitest acceptance.
//
// 锚: cv-9-stance-checklist.md §4 + content-lock §1+§2.

import React from 'react';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react';
import ArtifactCommentsMentionBadge from '../components/ArtifactCommentsMentionBadge';
import { dispatchMentionPushed } from '../hooks/useWsHubFrames';
import type { MentionPushedFrame } from '../types/ws-frames';

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
});

async function render(node: React.ReactElement) {
  root = createRoot(container!);
  await act(async () => {
    root!.render(node);
  });
}

function makeFrame(targetId: string, cursor = 1): MentionPushedFrame {
  return {
    type: 'mention_pushed',
    cursor,
    message_id: `msg-${cursor}`,
    channel_id: 'ch-1',
    sender_id: 'sender',
    mention_target_id: targetId,
    body_preview: 'hello',
    created_at: 1700000000000 + cursor,
  };
}

describe('ArtifactCommentsMentionBadge — CV-9.2 client', () => {
  it('count==0 时不渲染 (反向断)', async () => {
    await render(<ArtifactCommentsMentionBadge currentUserId="u-1" />);
    expect(container!.querySelector('[data-cv9-unread-count]')).toBeNull();
    expect(container!.querySelector('[data-cv9-mention-toast]')).toBeNull();
  });

  it('设计 ④ 文案 "你被 @ 在 N 条评论中" byte-identical + DOM data-attr', async () => {
    await render(<ArtifactCommentsMentionBadge currentUserId="u-1" />);
    await act(async () => {
      dispatchMentionPushed(makeFrame('u-1', 1));
      await Promise.resolve();
    });
    await act(async () => {
      dispatchMentionPushed(makeFrame('u-1', 2));
      await Promise.resolve();
    });
    const badge = container!.querySelector('[data-cv9-unread-count]') as HTMLButtonElement;
    expect(badge).not.toBeNull();
    expect(badge.getAttribute('data-cv9-unread-count')).toBe('2');
    expect(badge.hasAttribute('data-cv9-mention-toast')).toBe(true);
    expect(badge.title).toBe('你被 @ 在 2 条评论中');
    expect(badge.textContent).toContain('你被 @ 在 2 条评论中');
  });

  it('设计 ④ frame for OTHER user does not increment count (反向)', async () => {
    await render(<ArtifactCommentsMentionBadge currentUserId="u-1" />);
    await act(async () => {
      dispatchMentionPushed(makeFrame('u-2', 1));
      await Promise.resolve();
    });
    expect(container!.querySelector('[data-cv9-unread-count]')).toBeNull();
  });

  it('click handler resets count + fires onClick callback', async () => {
    const onClick = vi.fn();
    await render(<ArtifactCommentsMentionBadge currentUserId="u-1" onClick={onClick} />);
    await act(async () => {
      dispatchMentionPushed(makeFrame('u-1', 1));
      await Promise.resolve();
    });
    const badge = container!.querySelector('[data-cv9-unread-count]') as HTMLButtonElement;
    expect(badge).not.toBeNull();
    await act(async () => {
      badge.click();
    });
    expect(onClick).toHaveBeenCalledTimes(1);
    // After click, count==0 → component unrenders.
    expect(container!.querySelector('[data-cv9-unread-count]')).toBeNull();
  });
});
