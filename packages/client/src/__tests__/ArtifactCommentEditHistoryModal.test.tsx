// ArtifactCommentEditHistoryModal.test.tsx — CV-15 acceptance §3.2.

import React from 'react';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react';
import ArtifactCommentEditHistoryModal from '../components/ArtifactCommentEditHistoryModal';

let container: HTMLDivElement | null = null;
let root: Root | null = null;

function stubFetch(history: any[]) {
  (globalThis as any).fetch = vi.fn(async () =>
    new Response(JSON.stringify({ history }), {
      status: 200,
      headers: { 'Content-Type': 'application/json' },
    }),
  );
}

async function flush() {
  await act(async () => {
    await Promise.resolve();
    await new Promise((r) => setTimeout(r, 0));
  });
}

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
});

afterEach(() => {
  act(() => {
    root?.unmount();
  });
  if (container) {
    document.body.removeChild(container);
    container = null;
  }
  vi.restoreAllMocks();
});

function render(node: React.ReactElement) {
  act(() => {
    root!.render(node);
  });
}

describe('ArtifactCommentEditHistoryModal', () => {
  it('renders modal container with byte-identical title + aria-label', () => {
    stubFetch([]);
    const onClose = vi.fn();
    render(<ArtifactCommentEditHistoryModal channelID="ch-1" messageID="msg-1" onClose={onClose} />);
    const modal = container!.querySelector('[data-testid="comment-edit-history-modal"]');
    expect(modal).toBeTruthy();
    expect(modal?.getAttribute('aria-label')).toBe('编辑历史');
    const title = container!.querySelector('.comment-edit-history-title');
    expect(title?.textContent).toBe('编辑历史');
  });

  it('shows empty state "暂无编辑记录" when history is empty', async () => {
    stubFetch([]);
    const onClose = vi.fn();
    render(<ArtifactCommentEditHistoryModal channelID="ch-1" messageID="msg-1" onClose={onClose} />);
    await flush();
    const empty = container!.querySelector('.comment-edit-history-empty');
    expect(empty?.textContent).toBe('暂无编辑记录');
    const count = container!.querySelector('.comment-edit-history-count');
    expect(count?.textContent).toBe('共 0 次编辑');
  });

  it('renders entries with data-ts RFC3339 + count "共 N 次编辑"', async () => {
    stubFetch([
      { old_content: 'first version', ts: 1700000000000, reason: 'unknown' },
      { old_content: 'second version', ts: 1700000001000, reason: 'unknown' },
    ]);
    const onClose = vi.fn();
    render(<ArtifactCommentEditHistoryModal channelID="ch-1" messageID="msg-1" onClose={onClose} />);
    await flush();

    const count = container!.querySelector('.comment-edit-history-count');
    expect(count?.textContent).toBe('共 2 次编辑');

    const entries = container!.querySelectorAll('[data-testid="comment-edit-history-entry"]');
    expect(entries.length).toBe(2);
    expect(entries[0].getAttribute('data-ts')).toBe(new Date(1700000000000).toISOString());
    expect(entries[1].getAttribute('data-ts')).toBe(new Date(1700000001000).toISOString());

    const old1 = entries[0].querySelector('.comment-edit-history-old-content');
    expect(old1?.textContent).toBe('first version');
  });

  it('clicking close fires onClose', async () => {
    stubFetch([]);
    const onClose = vi.fn();
    render(<ArtifactCommentEditHistoryModal channelID="ch-1" messageID="msg-1" onClose={onClose} />);
    const closeBtn = container!.querySelector('.comment-edit-history-close') as HTMLButtonElement;
    act(() => {
      closeBtn.click();
    });
    expect(onClose).toHaveBeenCalled();
  });

  it('on fetch error fires onError with toast literal mapped from server code', async () => {
    (globalThis as any).fetch = vi.fn(async () =>
      new Response(JSON.stringify({ error: 'comment.not_owner' }), {
        status: 403,
        headers: { 'Content-Type': 'application/json' },
      }),
    );
    const onClose = vi.fn();
    const onError = vi.fn();
    render(<ArtifactCommentEditHistoryModal channelID="ch-1" messageID="msg-1" onClose={onClose} onError={onError} />);
    await flush();
    expect(onError).toHaveBeenCalledWith('仅评论作者可查看历史');
  });
});
