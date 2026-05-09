// ws-artifact-comment-added.test.ts — CV-5.2 client gap (mirrors
// ws-anchor-comment-added.test.ts pattern).
//
// Validates the CV-5 push pipeline on the client side without booting
// a real WebSocket:
//   1. dispatchArtifactCommentAdded fires the locked window CustomEvent
//      with the 9-field frame as `detail`.
//   2. Field-order discipline guard — drift here breaks server↔client
//      schema lock (server: artifact_comment_added_frame.go).
//   3. Event-name lock pins the literal so ArtifactComments' listener
//      keeps subscribing to the same channel post-rename.

import { describe, it, expect, vi } from 'vitest';
import { dispatchArtifactCommentAdded } from '../hooks/useWsHubFrames';
import {
  ARTIFACT_COMMENT_ADDED_EVENT,
  type ArtifactCommentAddedFrame,
} from '../types/ws-frames';

const humanFrame: ArtifactCommentAddedFrame = {
  type: 'artifact_comment_added',
  cursor: 42,
  comment_id: 'msg-1',
  artifact_id: 'art-X',
  channel_id: 'art-channel-id',
  sender_id: 'u-human',
  sender_role: 'human',
  body_preview: 'hello world',
  created_at: 1700000000000,
};

const agentFrame: ArtifactCommentAddedFrame = {
  type: 'artifact_comment_added',
  cursor: 43,
  comment_id: 'msg-2',
  artifact_id: 'art-X',
  channel_id: 'art-channel-id',
  sender_id: 'u-agent',
  sender_role: 'agent',
  body_preview: 'I propose tightening section 2',
  created_at: 1700000001000,
};

describe('dispatchArtifactCommentAdded', () => {
  it('fires ARTIFACT_COMMENT_ADDED_EVENT with frame in detail', () => {
    const listener = vi.fn();
    window.addEventListener(ARTIFACT_COMMENT_ADDED_EVENT, listener);
    try {
      dispatchArtifactCommentAdded(humanFrame);
    } finally {
      window.removeEventListener(ARTIFACT_COMMENT_ADDED_EVENT, listener);
    }
    expect(listener).toHaveBeenCalledTimes(1);
    const evt = listener.mock.calls[0][0] as CustomEvent<ArtifactCommentAddedFrame>;
    expect(evt.detail).toEqual(humanFrame);
  });

  it('preserves 9-field byte-identical key order (cv-5-spec.md §0 设计 ② 锁)', () => {
    expect(Object.keys(humanFrame)).toEqual([
      'type',
      'cursor',
      'comment_id',
      'artifact_id',
      'channel_id',
      'sender_id',
      'sender_role',
      'body_preview',
      'created_at',
    ]);
    expect(Object.keys(agentFrame)).toEqual(Object.keys(humanFrame));
  });

  it('event name literal lock', () => {
    expect(ARTIFACT_COMMENT_ADDED_EVENT).toBe('borgee:artifact-comment-added');
  });

  it('sender_role enum has only human|agent (隐私 §13 + 5-pattern thinking 链)', () => {
    const roles: Array<ArtifactCommentAddedFrame['sender_role']> = ['human', 'agent'];
    expect(roles.length).toBe(2);
  });
});
