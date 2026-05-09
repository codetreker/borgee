// useArtifactCommentDraft.test.ts — CV-10.2 vitest acceptance.
//
// 锚: docs/qa/cv-10-stance-checklist.md §1 + content-lock §3.
// 4 case: load empty / save → reload restore / clear after submit / debounce.

import React from 'react';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react-dom/test-utils';
import { useArtifactCommentDraft, type UseArtifactCommentDraftResult } from '../hooks/useArtifactCommentDraft';

const KEY_PREFIX = 'borgee.cv10.comment-draft:';

let container: HTMLDivElement | null = null;
let root: Root | null = null;
let captured: UseArtifactCommentDraftResult | null = null;

function HookProbe({ artifactId }: { artifactId: string }) {
  captured = useArtifactCommentDraft(artifactId);
  return null;
}

async function mount(artifactId: string) {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
  await act(async () => {
    root!.render(<HookProbe artifactId={artifactId} />);
  });
}

beforeEach(() => {
  localStorage.clear();
  vi.useFakeTimers();
  captured = null;
});

afterEach(() => {
  vi.useRealTimers();
  if (container) {
    act(() => {
      root?.unmount();
    });
    document.body.removeChild(container);
    container = null;
  }
  localStorage.clear();
});

describe('useArtifactCommentDraft — CV-10.1 hook', () => {
  it('设计 ① empty initial state — no localStorage entry → draft="" + restored=false', async () => {
    await mount('art-1');
    expect(captured!.draft).toBe('');
    expect(captured!.restored).toBe(false);
  });

  it('设计 ① save → reload restore — pre-seed localStorage, mount → draft populated + restored=true', async () => {
    localStorage.setItem(KEY_PREFIX + 'art-2', 'pending review note');
    await mount('art-2');
    expect(captured!.draft).toBe('pending review note');
    expect(captured!.restored).toBe(true);
  });

  it('设计 ② clear() — submit 后 localStorage.removeItem → 后续 getItem returns null', async () => {
    localStorage.setItem(KEY_PREFIX + 'art-3', 'will be cleared');
    await mount('art-3');
    expect(captured!.draft).toBe('will be cleared');
    await act(async () => {
      captured!.clear();
    });
    expect(captured!.draft).toBe('');
    expect(localStorage.getItem(KEY_PREFIX + 'art-3')).toBeNull();
  });

  it('设计 ② debounced save — setDraft writes localStorage only after 500ms idle', async () => {
    await mount('art-4');
    await act(async () => {
      captured!.setDraft('first keystroke');
    });
    // Before timer fires, localStorage NOT yet written.
    expect(localStorage.getItem(KEY_PREFIX + 'art-4')).toBeNull();
    await act(async () => {
      vi.advanceTimersByTime(499);
    });
    expect(localStorage.getItem(KEY_PREFIX + 'art-4')).toBeNull();
    // After 500ms idle, write fires.
    await act(async () => {
      vi.advanceTimersByTime(2);
    });
    expect(localStorage.getItem(KEY_PREFIX + 'art-4')).toBe('first keystroke');
  });
});
