// reaction-reducer-race.test.ts — gh#686 §4 #11b race 修法的单测.
//
// 锁这一条: WS 比 API fail 早到 (常见情况) 时 reactions 已被整列替换为
// 服务器版含别人 reaction 时, REMOVE_REACTION_OPTIMISTIC 必须按 user_id
// 移除当前用户而非按 emoji 删整条 (反误删别人 reaction).
//
// 4 步路径 (按 design §4 #11b):
//   ① ADD_REACTION_OPTIMISTIC (用户立刻看到 pill)
//   ② UPDATE_REACTIONS (WS 推到, 服务器版含别人 + 当前用户)
//   ③ REMOVE_REACTION_OPTIMISTIC (API fail, 失败撤回)
//   ④ 期望: 别人的 reaction 仍在, 当前用户已移除
import { describe, it, expect } from 'vitest';
import { reducer, initialState, type AppState } from '../context/AppContext';
import type { Message } from '../types';

const CH = 'ch-1';
const MSG = {
  id: 'm-1',
  channel_id: CH,
  sender_id: 'u-current',
  content: 'hi',
  content_type: 'text',
  created_at: 1,
  reactions: [],
} as unknown as Message;

function withMsg(state: AppState): AppState {
  const messages = new Map(state.messages);
  messages.set(CH, [MSG]);
  return { ...state, messages };
}

function getReactions(state: AppState) {
  return state.messages.get(CH)?.[0]?.reactions ?? [];
}

describe('reaction reducer — gh#686 §4 #11b race 路径', () => {
  it('① ADD_REACTION_OPTIMISTIC: 没 reaction 时新建 emoji 条目, 含当前用户', () => {
    const s0 = withMsg(initialState);
    const s1 = reducer(s0, {
      type: 'ADD_REACTION_OPTIMISTIC',
      channelId: CH,
      messageId: 'm-1',
      emoji: '👍',
      userId: 'u-current',
    });
    const r = getReactions(s1);
    expect(r).toEqual([{ emoji: '👍', count: 1, user_ids: ['u-current'] }]);
  });

  it('① ADD_REACTION_OPTIMISTIC: 已有别人加了同 emoji 时, 加当前用户进 user_ids', () => {
    const s0 = withMsg(initialState);
    const s_existing = reducer(s0, {
      type: 'UPDATE_REACTIONS',
      channelId: CH,
      messageId: 'm-1',
      reactions: [{ emoji: '👍', count: 2, user_ids: ['A', 'B'] }],
    });
    const s1 = reducer(s_existing, {
      type: 'ADD_REACTION_OPTIMISTIC',
      channelId: CH,
      messageId: 'm-1',
      emoji: '👍',
      userId: 'u-current',
    });
    const r = getReactions(s1);
    expect(r).toEqual([{ emoji: '👍', count: 3, user_ids: ['A', 'B', 'u-current'] }]);
  });

  it('① ADD_REACTION_OPTIMISTIC: 当前用户已在 user_ids 里时幂等不重复加', () => {
    const s0 = withMsg(initialState);
    const s_existing = reducer(s0, {
      type: 'UPDATE_REACTIONS',
      channelId: CH,
      messageId: 'm-1',
      reactions: [{ emoji: '👍', count: 1, user_ids: ['u-current'] }],
    });
    const s1 = reducer(s_existing, {
      type: 'ADD_REACTION_OPTIMISTIC',
      channelId: CH,
      messageId: 'm-1',
      emoji: '👍',
      userId: 'u-current',
    });
    const r = getReactions(s1);
    expect(r).toEqual([{ emoji: '👍', count: 1, user_ids: ['u-current'] }]);
  });

  it('③④ 完整 race 路径: ADD → UPDATE_REACTIONS (WS 含别人) → REMOVE → 期望别人的 reaction 没被误删', () => {
    const s0 = withMsg(initialState);

    // ① 用户点 thumbs-up, 立刻看到 pill
    const s1 = reducer(s0, {
      type: 'ADD_REACTION_OPTIMISTIC',
      channelId: CH,
      messageId: 'm-1',
      emoji: '👍',
      userId: 'u-current',
    });
    expect(getReactions(s1)).toEqual([
      { emoji: '👍', count: 1, user_ids: ['u-current'] },
    ]);

    // ② WS 推到 (比 API fail 早到), 整列替换为服务器版含 A B 加当前用户
    const s2 = reducer(s1, {
      type: 'UPDATE_REACTIONS',
      channelId: CH,
      messageId: 'm-1',
      reactions: [{ emoji: '👍', count: 3, user_ids: ['A', 'B', 'u-current'] }],
    });
    expect(getReactions(s2)).toEqual([
      { emoji: '👍', count: 3, user_ids: ['A', 'B', 'u-current'] },
    ]);

    // ③ API 这时 fail (其实服务器是收到了, 但 client 收到 5xx); dispatch
    // REMOVE_REACTION_OPTIMISTIC 撤回当前用户. 关键: 不能误删别人 (A B).
    const s3 = reducer(s2, {
      type: 'REMOVE_REACTION_OPTIMISTIC',
      channelId: CH,
      messageId: 'm-1',
      emoji: '👍',
      userId: 'u-current',
    });
    const r = getReactions(s3);

    // ④ 期望: A B 的 reaction 仍在, 当前用户已移除, count = 2
    expect(r).toEqual([
      { emoji: '👍', count: 2, user_ids: ['A', 'B'] },
    ]);
    // 反向断言: A 的 user_id 仍在, count 不是 0
    expect(r[0]!.user_ids).toContain('A');
    expect(r[0]!.user_ids).toContain('B');
    expect(r[0]!.user_ids).not.toContain('u-current');
    expect(r[0]!.count).toBe(2);
  });

  it('REMOVE_REACTION_OPTIMISTIC: 只剩当前用户时删整条', () => {
    const s0 = withMsg(initialState);
    const s1 = reducer(s0, {
      type: 'ADD_REACTION_OPTIMISTIC',
      channelId: CH,
      messageId: 'm-1',
      emoji: '👍',
      userId: 'u-current',
    });
    const s2 = reducer(s1, {
      type: 'REMOVE_REACTION_OPTIMISTIC',
      channelId: CH,
      messageId: 'm-1',
      emoji: '👍',
      userId: 'u-current',
    });
    expect(getReactions(s2)).toEqual([]);
  });

  it('REMOVE_REACTION_OPTIMISTIC: emoji 不存在时不动 (e.g. WS 已收掉)', () => {
    const s0 = withMsg(initialState);
    const s1 = reducer(s0, {
      type: 'REMOVE_REACTION_OPTIMISTIC',
      channelId: CH,
      messageId: 'm-1',
      emoji: '👍',
      userId: 'u-current',
    });
    expect(getReactions(s1)).toEqual([]);
  });

  it('REMOVE_REACTION_OPTIMISTIC: 当前用户不在 user_ids 时不动 (race 中 WS 已经清掉)', () => {
    const s0 = withMsg(initialState);
    const s1 = reducer(s0, {
      type: 'UPDATE_REACTIONS',
      channelId: CH,
      messageId: 'm-1',
      reactions: [{ emoji: '👍', count: 1, user_ids: ['A'] }],
    });
    const s2 = reducer(s1, {
      type: 'REMOVE_REACTION_OPTIMISTIC',
      channelId: CH,
      messageId: 'm-1',
      emoji: '👍',
      userId: 'u-current',
    });
    // A 的 reaction 不动, 当前用户本来就不在 → 无操作
    expect(getReactions(s2)).toEqual([{ emoji: '👍', count: 1, user_ids: ['A'] }]);
  });
});
