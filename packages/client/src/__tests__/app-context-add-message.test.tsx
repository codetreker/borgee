// app-context-add-message.test.tsx — #687 自己消息未读 Layer 3 client reducer 单测.
//
// 4 case 对应 design doc §7.2:
//
// 1. own message in non-current channel → 不 bump unread (Layer 3 主修).
// 2. peer message in non-current channel → 正常 bump unread (反 Layer 3
//    误伤别人消息).
// 3. 多设备 own message in non-current channel: 设备 B (同 user) 收到
//    设备 A 在 channel C 发的 own message, 设备 B 当前在 channel A.
//    Layer 3 看 sender_id == currentUser.id → 跳 unread bump (UI 不闪).
//    反过度: 设备 B 收到别人在 channel C 发的消息仍正常 bump.
// 4. currentChannel == messageChannel → 任何人发都不 bump (现有行为不变).
//
// 走 __test_reducer / __test_initialState (test-only export, 反 production
// import). 不起 React 树, 直接 reducer(state, action) 验.

import { describe, it, expect } from 'vitest';
import { __test_reducer, __test_initialState } from '../context/AppContext';
import type { Channel, DmChannel, Message, User } from '../types';

const owner: User = {
  id: 'user-owner',
  display_name: 'Owner',
  role: 'member',
  avatar_url: null,
  created_at: 1000,
};

const peer: User = {
  id: 'user-peer',
  display_name: 'Peer',
  role: 'member',
  avatar_url: null,
  created_at: 1000,
};

function makeChannel(id: string, overrides: Partial<Channel> = {}): Channel {
  return {
    id,
    name: id,
    topic: '',
    type: 'channel',
    visibility: 'public',
    created_at: 1000,
    created_by: owner.id,
    member_count: 2,
    unread_count: 0,
    is_member: true,
    last_message_at: null,
    ...overrides,
  };
}

function makeMessage(channelID: string, senderID: string, id = 'msg-1'): Message {
  return {
    id,
    channel_id: channelID,
    sender_id: senderID,
    content: 'hi',
    content_type: 'text',
    reply_to_id: null,
    created_at: 2000,
    edited_at: null,
  };
}

function baseState(currentChannelId: string | null, channels: Channel[], dmChannels: DmChannel[] = []) {
  return {
    ...__test_initialState,
    currentUser: owner,
    currentChannelId,
    channels,
    dmChannels,
  };
}

describe('#687 Layer 3 — ADD_MESSAGE reducer 反 own-message 误算 unread', () => {
  it('own message in non-current channel: 不 bump unread (Layer 3 主路径)', () => {
    const chA = makeChannel('chan-A', { unread_count: 0 });
    const chB = makeChannel('chan-B', { unread_count: 0 });
    const state = baseState('chan-A', [chA, chB]);

    // owner 在 chan-B 发自己消息 (多设备场景: 别的设备发的, 这台收 ws frame).
    const next = __test_reducer(state, {
      type: 'ADD_MESSAGE',
      channelId: 'chan-B',
      message: makeMessage('chan-B', owner.id),
    });

    const updatedB = next.channels.find(c => c.id === 'chan-B')!;
    expect(updatedB.unread_count).toBe(0);
    // last_message_at 仍然要更 (跟 unread bump 是独立的: 消息事件本身要落).
    expect(updatedB.last_message_at).toBe(2000);
  });

  it('peer message in non-current channel: 正常 bump unread (反 Layer 3 误伤别人)', () => {
    const chA = makeChannel('chan-A', { unread_count: 0 });
    const chB = makeChannel('chan-B', { unread_count: 0 });
    const state = baseState('chan-A', [chA, chB]);

    const next = __test_reducer(state, {
      type: 'ADD_MESSAGE',
      channelId: 'chan-B',
      message: makeMessage('chan-B', peer.id),
    });

    const updatedB = next.channels.find(c => c.id === 'chan-B')!;
    expect(updatedB.unread_count).toBe(1);
  });

  it('多设备 own message in non-current channel: Layer 3 跳 (反 UI 闪) + 反向 peer 仍 bump', () => {
    // 模拟设备 B 当前在 channel A, 设备 A 在 channel C 发 own message,
    // ws frame 把 own message 推给设备 B. 设备 B 视角: 不该让 channel C
    // 在 sidebar 上闪未读.
    const chA = makeChannel('chan-A', { unread_count: 0 });
    const chC = makeChannel('chan-C', { unread_count: 0 });
    const state = baseState('chan-A', [chA, chC]);

    // 多设备 own.
    let next = __test_reducer(state, {
      type: 'ADD_MESSAGE',
      channelId: 'chan-C',
      message: makeMessage('chan-C', owner.id, 'msg-own'),
    });
    expect(next.channels.find(c => c.id === 'chan-C')!.unread_count).toBe(0);

    // 反向: 别人在 chan-C 发, 要正常 bump (没误伤).
    next = __test_reducer(next, {
      type: 'ADD_MESSAGE',
      channelId: 'chan-C',
      message: makeMessage('chan-C', peer.id, 'msg-peer'),
    });
    expect(next.channels.find(c => c.id === 'chan-C')!.unread_count).toBe(1);
  });

  it('currentChannel == messageChannel: 任何人发都不 bump (现有行为不变)', () => {
    const chA = makeChannel('chan-A', { unread_count: 0 });
    const state = baseState('chan-A', [chA]);

    // 自己发.
    let next = __test_reducer(state, {
      type: 'ADD_MESSAGE',
      channelId: 'chan-A',
      message: makeMessage('chan-A', owner.id, 'msg-own'),
    });
    expect(next.channels.find(c => c.id === 'chan-A')!.unread_count).toBe(0);

    // 别人发.
    next = __test_reducer(next, {
      type: 'ADD_MESSAGE',
      channelId: 'chan-A',
      message: makeMessage('chan-A', peer.id, 'msg-peer'),
    });
    expect(next.channels.find(c => c.id === 'chan-A')!.unread_count).toBe(0);
  });

  it('DM channel own message in non-current: 不 bump dm unread (Layer 3 同样应用 DM)', () => {
    const dmChannel: DmChannel = {
      id: 'dm-1',
      name: 'dm-1',
      type: 'dm',
      created_at: 1000,
      peer: { id: peer.id, display_name: 'Peer', avatar_url: null, role: 'member' },
      unread_count: 0,
      last_message: null,
    };
    const state = baseState('chan-A', [makeChannel('chan-A')], [dmChannel]);

    const next = __test_reducer(state, {
      type: 'ADD_MESSAGE',
      channelId: 'dm-1',
      message: makeMessage('dm-1', owner.id, 'dm-own'),
    });

    const updatedDm = next.dmChannels.find(d => d.id === 'dm-1')!;
    expect(updatedDm.unread_count).toBe(0);
  });

  it('currentUser null (auth 未完成): 走原行为, 不 crash', () => {
    const chA = makeChannel('chan-A', { unread_count: 0 });
    const chB = makeChannel('chan-B', { unread_count: 0 });
    const state = { ...baseState('chan-A', [chA, chB]), currentUser: null };

    // 没 currentUser, 任何 sender_id 都不等于 null → isOwnMessage = false →
    // 走原行为 (peer 在 non-current channel bump).
    const next = __test_reducer(state, {
      type: 'ADD_MESSAGE',
      channelId: 'chan-B',
      message: makeMessage('chan-B', owner.id),
    });

    expect(next.channels.find(c => c.id === 'chan-B')!.unread_count).toBe(1);
  });

  it('SET_MESSAGES stale REST load must not drop newer live WS messages', () => {
    const chA = makeChannel('chan-A', { unread_count: 0 });
    const olderRestMessage = makeMessage('chan-A', owner.id, 'msg-rest-old');
    olderRestMessage.created_at = 1_000;
    const liveAgentReply = makeMessage('chan-A', peer.id, 'msg-live-agent');
    liveAgentReply.created_at = 2_000;
    liveAgentReply.sender_name = 'Agent Peer';

    const withLiveReply = __test_reducer(baseState('chan-A', [chA]), {
      type: 'ADD_MESSAGE',
      channelId: 'chan-A',
      message: liveAgentReply,
    });

    const afterStaleLoad = __test_reducer(withLiveReply, {
      type: 'SET_MESSAGES',
      channelId: 'chan-A',
      messages: [olderRestMessage],
      hasMore: false,
      loadedBefore: 1_500,
    });

    expect(afterStaleLoad.messages.get('chan-A')?.map(m => m.id)).toEqual([
      'msg-rest-old',
      'msg-live-agent',
    ]);
  });

  it('SET_MESSAGES empty stale REST load keeps only live messages newer than request start', () => {
    const chA = makeChannel('chan-A', { unread_count: 0 });
    const oldLocalMessage = makeMessage('chan-A', owner.id, 'msg-old-local');
    oldLocalMessage.created_at = 1_000;
    const liveAgentReply = makeMessage('chan-A', peer.id, 'msg-live-agent');
    liveAgentReply.created_at = 2_000;

    const withLocalMessages = {
      ...baseState('chan-A', [chA]),
      messages: new Map([['chan-A', [oldLocalMessage, liveAgentReply]]]),
    };

    const afterStaleEmptyLoad = __test_reducer(withLocalMessages, {
      type: 'SET_MESSAGES',
      channelId: 'chan-A',
      messages: [],
      hasMore: false,
      loadedBefore: 1_500,
    });

    expect(afterStaleEmptyLoad.messages.get('chan-A')?.map(m => m.id)).toEqual([
      'msg-live-agent',
    ]);
  });
});
