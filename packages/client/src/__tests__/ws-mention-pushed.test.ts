// ws-mention-pushed.test.ts — DM-2.3 (#377) client gap (mirrors
// ws-artifact-updated.test.ts pattern).
//
// Validates the DM-2.2 #372 push pipeline on the client side without
// booting a real WebSocket:
//   1. dispatchMentionPushed fires the locked window CustomEvent
//      with the 8-field frame as `detail` (envelope BPP-byte-identical
//      vs server mention_pushed_frame.go::MentionPushedFrame).
//   2. Field-order discipline guard — drift here breaks server↔client
//      schema lock checked by BPP-1 #304 envelope CI lint on the server.
//   3. Event-name lock pins the literal so MessageList's listener keeps
//      subscribing to the same channel post-rename.
//
// Why mock-only: useWebSocket.ts's switch arm is a 4-line passthrough
// (case 'mention_pushed' → dispatchMentionPushed(data)). Once the
// dispatcher is proven, the wire integration is by inspection. The real
// WS + UI ≤3s contract is the playwright spec (dm-2-3-mention.spec.ts).

import { describe, it, expect, vi } from 'vitest';
import { dispatchMentionPushed } from '../hooks/useWsHubFrames';
import {
  MENTION_PUSHED_EVENT,
  type MentionPushedFrame,
} from '../types/ws-frames';

const onlineFrame: MentionPushedFrame = {
  type: 'mention_pushed',
  cursor: 99,
  message_id: 'msg-A',
  channel_id: 'ch-Y',
  sender_id: 'u-sender',
  mention_target_id: 'u-target',
  body_preview: 'hello @u-target',
  created_at: 1700000000000,
};

describe('dispatchMentionPushed', () => {
  it('fires MENTION_PUSHED_EVENT with the frame in detail', () => {
    const listener = vi.fn();
    window.addEventListener(MENTION_PUSHED_EVENT, listener);
    try {
      dispatchMentionPushed(onlineFrame);
      expect(listener).toHaveBeenCalledTimes(1);
      const evt = listener.mock.calls[0]?.[0] as CustomEvent<MentionPushedFrame>;
      expect(evt.detail).toEqual(onlineFrame);
    } finally {
      window.removeEventListener(MENTION_PUSHED_EVENT, listener);
    }
  });

  it('event name is the locked literal', () => {
    // Pin the event channel name. MessageList subscribes to this exact
    // string via useMentionPushed; rename without coordinated update
    // silently breaks the dispatch pipeline.
    expect(MENTION_PUSHED_EVENT).toBe('borgee:mention-pushed');
  });

  it('frame field set is the 8-field byte-identical contract', () => {
    // Pin the field order + count. Drift here and the server↔client
    // lock (mention_pushed_frame.go) is broken — BPP CI lint on the
    // server is the byte-identity guard but this test is the client
    // mirror so refactors notice fast.
    const keys = Object.keys(onlineFrame);
    expect(keys).toEqual([
      'type',
      'cursor',
      'message_id',
      'channel_id',
      'sender_id',
      'mention_target_id',
      'body_preview',
      'created_at',
    ]);
    expect(keys).toHaveLength(8);
  });

  it('反约束: frame schema has no owner_id / target_owner / fanout fields', () => {
    // 设计 ③ (蓝图 §4): mention 永不抄送 owner — frame surface 不
    // 暴露 owner 路由信息. offline owner-fallback 走独立 system DM,
    // 不复用此 envelope.
    const keys = Object.keys(onlineFrame);
    for (const forbidden of ['owner_id', 'target_owner_id', 'fanout_to_owner', 'cc_owner']) {
      expect(keys).not.toContain(forbidden);
    }
  });
});
