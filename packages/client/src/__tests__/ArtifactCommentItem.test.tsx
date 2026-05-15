// ArtifactCommentItem.test.tsx — CV-7.2 vitest acceptance.
//
// 锚: docs/qa/cv-7-stance-checklist.md §4 + content-lock §1+§2.
// 4 case (cv-7.md §2):
//   1. own comment 渲染 data-cv7-edit-btn (sender==current user)
//   2. other comment 不渲染 data-cv7-edit-btn (reverse constraint)
//   3. delete confirm 文案 exact-match "确认删除这条评论?"
//   4. reaction button 渲染 data-cv7-reaction-target

import React from 'react';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react';
import ArtifactCommentItem from '../components/ArtifactCommentItem';
import * as api from '../lib/api';

let container: HTMLDivElement | null = null;
let root: Root | null = null;

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
  vi.restoreAllMocks();
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

describe('ArtifactCommentItem — CV-7.2 client', () => {
  it('设计 ② own comment renders data-cv7-edit-btn (sender==current user)', async () => {
    await render(
      <ArtifactCommentItem
        commentId="msg-1"
        authorId="u-1"
        authorRole="human"
        body="own comment"
        currentUserId="u-1"
      />,
    );
    const btn = container!.querySelector('[data-cv7-edit-btn]');
    expect(btn).not.toBeNull();
    expect(btn!.getAttribute('data-cv7-edit-btn-target')).toBe('msg-1');
  });

  it('设计 ② other comment does NOT render data-cv7-edit-btn (reverse constraint)', async () => {
    await render(
      <ArtifactCommentItem
        commentId="msg-2"
        authorId="u-2"
        authorRole="human"
        body="other comment"
        currentUserId="u-1"
      />,
    );
    const btn = container!.querySelector('[data-cv7-edit-btn]');
    expect(btn).toBeNull();
  });

  it('设计 ④ delete confirm 文案 exact-match "确认删除这条评论?"', async () => {
    const confirmSpy = vi.spyOn(window, 'confirm').mockReturnValue(false);
    const delSpy = vi.spyOn(api, 'deleteMessage').mockResolvedValue();
    await render(
      <ArtifactCommentItem
        commentId="msg-3"
        authorId="u-1"
        authorRole="human"
        body="own"
        currentUserId="u-1"
      />,
    );
    const delBtn = container!.querySelector('[data-cv7-delete-btn]') as HTMLButtonElement;
    expect(delBtn).not.toBeNull();
    await act(async () => {
      delBtn.click();
      await Promise.resolve();
    });
    expect(confirmSpy).toHaveBeenCalledWith('确认删除这条评论?');
    expect(delSpy).not.toHaveBeenCalled(); // confirm returned false → no delete

    // confirm true → delete called
    confirmSpy.mockReturnValue(true);
    await act(async () => {
      delBtn.click();
      await Promise.resolve();
    });
    expect(delSpy).toHaveBeenCalledWith('msg-3');
  });

  it('设计 ④ reaction button renders data-cv7-reaction-target + click → addReaction', async () => {
    const reactSpy = vi.spyOn(api, 'addReaction').mockResolvedValue();
    await render(
      <ArtifactCommentItem
        commentId="msg-4"
        authorId="u-2"
        authorRole="agent"
        body="agent comment"
        currentUserId="u-1"
      />,
    );
    const reactBtn = container!.querySelector('[data-cv7-reaction-target="msg-4"]') as HTMLButtonElement;
    expect(reactBtn).not.toBeNull();
    await act(async () => {
      reactBtn.click();
      await Promise.resolve();
    });
    expect(reactSpy).toHaveBeenCalledWith('msg-4', '👍');
  });

  it('设计 ③ thinking 5-pattern reject — surfaces errcode exact-match CV-5', async () => {
    const ApiErrCtor = api.ApiError;
    vi.spyOn(api, 'editMessage').mockRejectedValue(
      new ApiErrCtor(400, 'comment.thinking_subject_required: thinking-only body rejected'),
    );
    await render(
      <ArtifactCommentItem
        commentId="msg-5"
        authorId="u-1"
        authorRole="agent"
        body="initial"
        currentUserId="u-1"
      />,
    );
    const editBtn = container!.querySelector('[data-cv7-edit-btn]') as HTMLButtonElement;
    await act(async () => {
      editBtn.click();
      await Promise.resolve();
    });
    const ta = container!.querySelector('[data-testid="cv7-edit-textarea"]') as HTMLTextAreaElement;
    await act(async () => {
      ta.value = 'AI is thinking...';
      ta.dispatchEvent(new Event('input', { bubbles: true }));
    });
    const save = container!.querySelector('[data-testid="cv7-edit-save"]') as HTMLButtonElement;
    await act(async () => {
      save.click();
      await Promise.resolve();
      await Promise.resolve();
    });
    const err = container!.querySelector('[data-testid="cv7-edit-error"]');
    expect(err?.textContent).toBe('comment.thinking_subject_required');
  });
});
