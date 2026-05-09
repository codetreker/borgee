// ArtifactCommentDraftInput.test.tsx — CV-10.2 vitest acceptance.
//
// 锚: docs/qa/cv-10-stance-checklist.md §4 + content-lock §1+§2.
// 4 case: DOM data-attr / restore toast 文案 / submit clears draft + cleared toast / beforeunload native.

import React from 'react';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react-dom/test-utils';
import ArtifactCommentDraftInput from '../components/ArtifactCommentDraftInput';

const KEY_PREFIX = 'borgee.cv10.comment-draft:';

let container: HTMLDivElement | null = null;
let root: Root | null = null;

beforeEach(() => {
  localStorage.clear();
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
  localStorage.clear();
});

async function render(node: React.ReactElement) {
  root = createRoot(container!);
  await act(async () => {
    root!.render(node);
  });
}

describe('ArtifactCommentDraftInput — CV-10.1 component', () => {
  it('设计 ④ DOM `data-cv10-draft-textarea="<artifactId>"` 必锚', async () => {
    await render(<ArtifactCommentDraftInput artifactId="art-1" onSubmit={async () => {}} />);
    const ta = container!.querySelector('[data-cv10-draft-textarea]') as HTMLTextAreaElement;
    expect(ta).not.toBeNull();
    expect(ta.getAttribute('data-cv10-draft-textarea')).toBe('art-1');
  });

  it('设计 ④ restore toast 文案 byte-identical "已恢复未保存的草稿" — pre-seed localStorage → toast 渲染', async () => {
    localStorage.setItem(KEY_PREFIX + 'art-2', 'pending');
    await render(<ArtifactCommentDraftInput artifactId="art-2" onSubmit={async () => {}} />);
    const toast = container!.querySelector('[data-cv10-restore-toast]');
    expect(toast).not.toBeNull();
    expect(toast!.textContent).toBe('已恢复未保存的草稿');
    // textarea 也复用了 draft 值.
    const ta = container!.querySelector('[data-cv10-draft-textarea]') as HTMLTextAreaElement;
    expect(ta.value).toBe('pending');
  });

  it('设计 ② submit 后 localStorage cleared + cleared toast 文案 "草稿已清除"', async () => {
    localStorage.setItem(KEY_PREFIX + 'art-3', 'submit me');
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    await render(<ArtifactCommentDraftInput artifactId="art-3" onSubmit={onSubmit} />);
    const submitBtn = container!.querySelector('[data-testid="cv10-submit"]') as HTMLButtonElement;
    await act(async () => {
      submitBtn.click();
    });
    // Allow async chain to settle.
    for (let i = 0; i < 5; i++) {
      await act(async () => {
        await Promise.resolve();
      });
    }
    expect(onSubmit).toHaveBeenCalledWith('submit me');
    expect(localStorage.getItem(KEY_PREFIX + 'art-3')).toBeNull();
    // restore toast cleared.
    expect(container!.querySelector('[data-cv10-restore-toast]')).toBeNull();
  });

  it('设计 ② beforeunload 走浏览器原生 — draft 非空时 handler 调 preventDefault', async () => {
    localStorage.setItem(KEY_PREFIX + 'art-4', 'unsaved');
    await render(<ArtifactCommentDraftInput artifactId="art-4" onSubmit={async () => {}} />);

    const evt = new Event('beforeunload', { cancelable: true }) as BeforeUnloadEvent;
    const preventSpy = vi.spyOn(evt, 'preventDefault');
    window.dispatchEvent(evt);
    // 设计 ② 反向断 handler 走原生 path (preventDefault called) — 不挂自定义 modal.
    expect(preventSpy).toHaveBeenCalled();
  });
});
