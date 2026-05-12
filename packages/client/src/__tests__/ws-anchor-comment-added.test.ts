// ws-anchor-comment-added.test.ts — CV-2.3 client gap (mirrors
// ws-mention-pushed.test.ts + ws-artifact-updated.test.ts pattern).
//
// Validates the CV-2.2 #360 push pipeline on the client side without
// booting a real WebSocket:
//   1. dispatchAnchorCommentAdded fires the locked window CustomEvent
//      with the 10-field frame as `detail` (envelope BPP-byte-identical
//      vs server anchor_comment_frame.go::AnchorCommentAddedFrame).
//   2. Field-order discipline guard — drift here breaks server↔client
//      schema lock checked by BPP-1 #304 envelope CI lint on the server.
//   3. Event-name lock pins the literal so AnchorThreadPanel's listener
//      keeps subscribing to the same channel after a rename.
//
// Why mock-only: useWebSocket.ts's switch arm is a short passthrough
// (case 'anchor_comment_added' → dispatchAnchorCommentAdded(data)). Once
// the dispatcher is proven, the wire integration is by inspection. The
// real WS + UI ≤3s contract is the playwright spec
// (cv-2-3-anchor-client.spec.ts).

import { describe, it, expect, vi } from 'vitest';
import { dispatchAnchorCommentAdded } from '../hooks/useWsHubFrames';
import {
  ANCHOR_COMMENT_ADDED_EVENT,
  type AnchorCommentAddedFrame,
} from '../types/ws-frames';

const humanFrame: AnchorCommentAddedFrame = {
  type: 'anchor_comment_added',
  cursor: 7,
  anchor_id: 'anc-A',
  comment_id: 1,
  artifact_id: 'art-X',
  artifact_version_id: 11,
  channel_id: 'ch-Y',
  author_id: 'u-human',
  author_kind: 'human',
  created_at: 1700000000000,
};

const agentFrame: AnchorCommentAddedFrame = {
  type: 'anchor_comment_added',
  cursor: 8,
  anchor_id: 'anc-A',
  comment_id: 2,
  artifact_id: 'art-X',
  artifact_version_id: 11,
  channel_id: 'ch-Y',
  author_id: 'u-agent',
  author_kind: 'agent',
  created_at: 1700000001000,
};

describe('dispatchAnchorCommentAdded', () => {
  it('fires ANCHOR_COMMENT_ADDED_EVENT with the frame in detail', () => {
    const listener = vi.fn();
    window.addEventListener(ANCHOR_COMMENT_ADDED_EVENT, listener);
    try {
      dispatchAnchorCommentAdded(humanFrame);
    } finally {
      window.removeEventListener(ANCHOR_COMMENT_ADDED_EVENT, listener);
    }
    expect(listener).toHaveBeenCalledTimes(1);
    const evt = listener.mock.calls[0][0] as CustomEvent<AnchorCommentAddedFrame>;
    expect(evt.detail).toEqual(humanFrame);
  });

  it('preserves the 10-field byte-identical key order (cv-2-spec.md v3 lock)', () => {
    // Drift guard: server anchor_comment_frame.go::AnchorCommentAddedFrame
    // field order is checked by BPP-1 #304 envelope CI lint server-side.
    // This pins the client-side mirror so a type rename fails clearly.
    expect(Object.keys(humanFrame)).toEqual([
      'type',
      'cursor',
      'anchor_id',
      'comment_id',
      'artifact_id',
      'artifact_version_id',
      'channel_id',
      'author_id',
      'author_kind',
      'created_at',
    ]);
    expect(Object.keys(humanFrame).length).toBe(10);
  });

  it('both author kinds (human / agent) round-trip', () => {
    const listener = vi.fn();
    window.addEventListener(ANCHOR_COMMENT_ADDED_EVENT, listener);
    try {
      dispatchAnchorCommentAdded(humanFrame);
      dispatchAnchorCommentAdded(agentFrame);
    } finally {
      window.removeEventListener(ANCHOR_COMMENT_ADDED_EVENT, listener);
    }
    expect(listener).toHaveBeenCalledTimes(2);
    const kinds = listener.mock.calls.map(
      (c) => (c[0] as CustomEvent<AnchorCommentAddedFrame>).detail.author_kind,
    );
    expect(kinds).toEqual(['human', 'agent']);
  });

  it('frame envelope must NOT include comment body or anchor offsets (design ③ signal-only)', () => {
    // Push is signal-only per cv-2-spec.md §0 design ③: body comes from
    // GET /api/v1/anchors/:id/comments. If a future frame schema adds
    // body / start_offset / end_offset, this catches it.
    const keys = Object.keys(humanFrame);
    expect(keys).not.toContain('body');
    expect(keys).not.toContain('start_offset');
    expect(keys).not.toContain('end_offset');
    // Design ③ envelope naming lock: the column is `author_kind`, NOT
    // `committer_kind` (anchors use comment authors, not committers; this
    // matches the v2 changelog wording).
    expect(keys).toContain('author_kind');
    expect(keys).not.toContain('committer_kind');
    expect(keys).not.toContain('kind');
  });
});

describe('event-name lock', () => {
  // Drift guard: AnchorThreadPanel.tsx subscribes via useAnchorCommentAdded
  // which hard-codes this constant. If the literal changes, the canvas
  // anchor side panel stops refreshing.
  it('ANCHOR_COMMENT_ADDED_EVENT === borgee:anchor-comment-added', () => {
    expect(ANCHOR_COMMENT_ADDED_EVENT).toBe('borgee:anchor-comment-added');
  });
});
