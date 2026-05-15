// MessageItem-reaction-toggle.test.tsx — gh#686 集成测覆盖组合点
// (按 feima R1 #686 review: 单测两个子组件不够, 需要集成测).
//
// 锁两路径:
//   1. reactions=[] → 工具栏 ➕ 出 (toolbar-btn variant), ReactionBar 不渲染
//   2. reactions=[一条] → 工具栏 ➕ 不出 (避免重复), ReactionBar 渲染 + 末尾 ➕
import React from 'react';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react';

vi.mock('@emoji-mart/react', () => ({
  default: () => React.createElement('div', { 'data-test': 'emoji-mart' }),
}));
vi.mock('@emoji-mart/data', () => ({ default: {} }));
vi.mock('../lib/api', async () => {
  const actual = await vi.importActual<typeof import('../lib/api')>('../lib/api');
  return {
    ...actual,
    addReaction: vi.fn().mockResolvedValue(undefined),
    removeReaction: vi.fn().mockResolvedValue(undefined),
    editMessage: vi.fn().mockResolvedValue({ content: '', edited_at: 0 }),
    deleteMessage: vi.fn().mockResolvedValue(undefined),
  };
});
vi.mock('../context/AppContext', () => ({
  useAppContext: () => ({ dispatch: vi.fn() }),
}));
vi.mock('../components/Toast', () => ({
  useToast: () => ({ showToast: vi.fn() }),
}));
vi.mock('../hooks/useCommandTracking', () => ({
  trackCommand: vi.fn(),
  getCommandStatus: vi.fn().mockReturnValue(null),
}));
vi.mock('../hooks/useLongPress', () => ({
  useLongPress: () => ({}),
}));

import MessageItem from '../components/MessageItem';
import type { Message } from '../types';

let container: HTMLDivElement | null = null;
let root: Root | null = null;

beforeEach(() => {
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
  root = null;
});

function makeMsg(reactions: { emoji: string; count: number; user_ids: string[] }[]): Message {
  return {
    id: 'm-1',
    channel_id: 'ch-1',
    sender_id: 'u-other',
    content: 'hello',
    content_type: 'text',
    created_at: 1700000000000,
    reactions,
  } as Message;
}

function render(message: Message) {
  root = createRoot(container!);
  act(() => {
    root!.render(
      <MessageItem
        message={message}
        userMap={new Map([['u-current', 'Me'], ['u-other', 'Other']])}
        members={[]}
        memberMap={new Map()}
        currentUserId="u-current"
        currentUserRole="member"
      />,
    );
  });
}

describe('MessageItem ↔ ReactionAddButton/ReactionBar 组合 — gh#686', () => {
  it('reactions=[]: 工具栏渲染 ➕ (toolbar-btn variant), ReactionBar 不渲染', () => {
    render(makeMsg([]));
    // 工具栏的 ➕ 在 .message-actions 下面
    const toolbarBtn = container!.querySelector(
      '.message-actions button[data-reaction-add-variant="toolbar-btn"]',
    ) as HTMLButtonElement | null;
    expect(toolbarBtn).not.toBeNull();
    expect(toolbarBtn!.textContent).toBe('➕');

    // ReactionBar 不渲染 (reactions=[] 直接 return null)
    expect(container!.querySelector('.reaction-bar')).toBeNull();

    // 反向: 不应有 inline-pill variant 的 ➕
    expect(container!.querySelector('button[data-reaction-add-variant="inline-pill"]')).toBeNull();
  });

  it('reactions=[一条]: 工具栏 ➕ 不出 (避免重复), ReactionBar 渲染 + 末尾 ➕ (inline-pill variant)', () => {
    render(makeMsg([{ emoji: '👍', count: 1, user_ids: ['u-other'] }]));
    // ReactionBar 渲染了
    expect(container!.querySelector('.reaction-bar')).not.toBeNull();

    // 末尾的 ➕ 是 inline-pill variant
    const inlineBtn = container!.querySelector(
      'button[data-reaction-add-variant="inline-pill"]',
    ) as HTMLButtonElement | null;
    expect(inlineBtn).not.toBeNull();

    // 反向: 工具栏的 ➕ (toolbar-btn) 不出, 避免双 ➕ 重复
    expect(
      container!.querySelector('.message-actions button[data-reaction-add-variant="toolbar-btn"]'),
    ).toBeNull();
  });

  it('消息已删除: 不出工具栏 ➕ 也不出 ReactionBar', () => {
    const msg = makeMsg([]);
    (msg as Message & { deleted_at: number }).deleted_at = 1700000001000;
    render(msg as Message);
    expect(container!.querySelector('.message-actions')).toBeNull();
    expect(container!.querySelector('.reaction-bar')).toBeNull();
    expect(container!.querySelector('button[data-reaction-add-variant]')).toBeNull();
  });

  it('消息发送中 (_pending): 不出工具栏 ➕', () => {
    const msg = { ...makeMsg([]), _pending: true } as Message;
    render(msg);
    // _pending + 没 reaction → canAddReaction false → 没 toolbar
    expect(
      container!.querySelector('button[data-reaction-add-variant="toolbar-btn"]'),
    ).toBeNull();
  });
});
