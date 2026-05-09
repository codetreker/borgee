// ArtifactComments.test.tsx — CV-5.2 vitest acceptance.
//
// 锚: docs/qa/cv-5-stance-checklist.md §2 + spec §0 设计 ② (frame 信号
// + 增量 append). 反约束: 不用 frame.body_preview 渲染 comment text.

import React from 'react';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react-dom/test-utils';
import ArtifactComments from '../components/ArtifactComments';
import { dispatchArtifactCommentAdded } from '../hooks/useWsHubFrames';
import * as api from '../lib/api';
import type { ArtifactComment } from '../lib/api';
import type { ArtifactCommentAddedFrame } from '../types/ws-frames';

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

const sampleHuman: ArtifactComment = {
  id: 'msg-1',
  artifact_id: 'art-X',
  channel_id: 'ch-Y',
  sender_id: 'u-human',
  sender_role: 'human',
  body: 'looks good',
  created_at: 1700000000000,
};

const sampleAgent: ArtifactComment = {
  id: 'msg-2',
  artifact_id: 'art-X',
  channel_id: 'ch-Y',
  sender_id: 'u-agent',
  sender_role: 'agent',
  body: 'I propose tightening section 2.',
  created_at: 1700000001000,
};

describe('ArtifactComments — CV-5.2 client', () => {
  it('renders empty state when no comments', async () => {
    vi.spyOn(api, 'listArtifactComments').mockResolvedValue({ comments: [] });
    await render(<ArtifactComments artifactId="art-X" />);
    // wait microtask
    await act(async () => {
      await Promise.resolve();
    });
    const empty = container!.querySelector('[data-testid="cv5-empty"]');
    expect(empty).not.toBeNull();
  });

  it('renders human + agent comments with hover anchor data-cv5-author-link', async () => {
    vi.spyOn(api, 'listArtifactComments').mockResolvedValue({
      comments: [sampleHuman, sampleAgent],
    });
    await render(<ArtifactComments artifactId="art-X" />);
    await act(async () => {
      await Promise.resolve();
    });
    const authors = container!.querySelectorAll('[data-cv5-author-link]');
    expect(authors.length).toBe(2);
    const roles = Array.from(authors).map((el) =>
      (el as HTMLElement).getAttribute('data-cv5-author-role'),
    );
    expect(roles).toEqual(['human', 'agent']);
  });

  it('设计 ② WS frame triggers refetch (incremental append, no body_preview render)', async () => {
    const listSpy = vi
      .spyOn(api, 'listArtifactComments')
      .mockResolvedValueOnce({ comments: [sampleHuman] })
      .mockResolvedValueOnce({ comments: [sampleHuman, sampleAgent] });
    await render(<ArtifactComments artifactId="art-X" />);
    await act(async () => {
      await Promise.resolve();
    });
    expect(listSpy).toHaveBeenCalledTimes(1);

    // Frame for current artifact → refetch
    const frame: ArtifactCommentAddedFrame = {
      type: 'artifact_comment_added',
      cursor: 100,
      comment_id: 'msg-2',
      artifact_id: 'art-X',
      channel_id: 'ch-Y',
      sender_id: 'u-agent',
      sender_role: 'agent',
      body_preview: 'I propose tightening section 2.',
      created_at: 1700000001000,
    };
    await act(async () => {
      dispatchArtifactCommentAdded(frame);
      await Promise.resolve();
      await Promise.resolve();
    });
    expect(listSpy).toHaveBeenCalledTimes(2);

    // 反约束: rendered text comes from REST `body`, not frame.body_preview.
    const rows = container!.querySelectorAll('[data-cv5-comment-id]');
    expect(rows.length).toBe(2);
  });

  it('设计 ② frame for OTHER artifact does not refetch', async () => {
    const listSpy = vi.spyOn(api, 'listArtifactComments').mockResolvedValue({
      comments: [sampleHuman],
    });
    await render(<ArtifactComments artifactId="art-X" />);
    await act(async () => {
      await Promise.resolve();
    });
    expect(listSpy).toHaveBeenCalledTimes(1);

    const otherFrame: ArtifactCommentAddedFrame = {
      type: 'artifact_comment_added',
      cursor: 200,
      comment_id: 'unrelated',
      artifact_id: 'art-OTHER',
      channel_id: 'ch-Other',
      sender_id: 'u-human',
      sender_role: 'human',
      body_preview: 'unrelated',
      created_at: 1700000002000,
    };
    await act(async () => {
      dispatchArtifactCommentAdded(otherFrame);
      await Promise.resolve();
    });
    expect(listSpy).toHaveBeenCalledTimes(1);
  });
});
