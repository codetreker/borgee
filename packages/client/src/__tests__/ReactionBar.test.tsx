// ReactionBar.test.tsx — gh#686 §4 关键锁: 没 reaction 时 ReactionBar
// 直接 return null 不渲染容器, 不再用空 .reaction-bar-empty 占位撑容器
// ~40px (#686 报的空间浪费).
import React from 'react';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react-dom/test-utils';

vi.mock('@emoji-mart/react', () => ({
  default: () => React.createElement('div', { 'data-test': 'emoji-mart' }),
}));
vi.mock('@emoji-mart/data', () => ({ default: {} }));
vi.mock('../context/AppContext', () => ({
  useAppContext: () => ({ dispatch: vi.fn() }),
}));
vi.mock('../components/Toast', () => ({
  useToast: () => ({ showToast: vi.fn() }),
}));

import ReactionBar from '../components/ReactionBar';

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

function render(node: React.ReactElement) {
  root = createRoot(container!);
  act(() => { root!.render(node); });
}

const userMap = new Map<string, string>([['u-1', 'Alice'], ['u-2', 'Bob']]);

describe('ReactionBar — gh#686 没 reaction 不渲染容器', () => {
  it('reactions=[]: 直接 return null (反 .reaction-bar-empty 占位)', () => {
    render(
      <ReactionBar
        reactions={[]}
        channelId="ch-1"
        messageId="m-1"
        currentUserId="u-1"
        userMap={userMap}
      />,
    );
    // 反向: 不留空 bar, 也不留 .reaction-add-hidden 占位
    expect(container!.querySelector('.reaction-bar')).toBeNull();
    expect(container!.querySelector('.reaction-bar-empty')).toBeNull();
    expect(container!.querySelector('.reaction-add-hidden')).toBeNull();
  });

  it('reactions 有 1 条: 渲染 1 个 reaction-pill + 末尾 ➕ (inline-pill variant)', () => {
    render(
      <ReactionBar
        reactions={[{ emoji: '👍', count: 1, user_ids: ['u-1'] }]}
        channelId="ch-1"
        messageId="m-1"
        currentUserId="u-1"
        userMap={userMap}
      />,
    );
    const bar = container!.querySelector('.reaction-bar');
    expect(bar).not.toBeNull();
    const pills = container!.querySelectorAll('.reaction-pill');
    // 1 reaction button + 1 add button = 2
    expect(pills.length).toBe(2);
    const addBtn = container!.querySelector('button[data-reaction-add-variant="inline-pill"]') as HTMLButtonElement;
    expect(addBtn).not.toBeNull();
    expect(addBtn.textContent).toBe('➕');
  });

  it('reactions 多条: 每条渲染一个 pill, 当前用户已 react 的标 reaction-active', () => {
    render(
      <ReactionBar
        reactions={[
          { emoji: '👍', count: 2, user_ids: ['u-1', 'u-2'] },
          { emoji: '❤️', count: 1, user_ids: ['u-2'] },
        ]}
        channelId="ch-1"
        messageId="m-1"
        currentUserId="u-1"
        userMap={userMap}
      />,
    );
    const buttons = Array.from(container!.querySelectorAll('.reaction-pill')) as HTMLButtonElement[];
    // 2 reaction + 1 add = 3
    expect(buttons.length).toBe(3);
    expect(buttons[0]!.classList.contains('reaction-active')).toBe(true); // 👍 含当前
    expect(buttons[1]!.classList.contains('reaction-active')).toBe(false); // ❤️ 不含当前
  });

  it('没 currentUserId: 不渲染 + 按钮 (反 server 401)', () => {
    render(
      <ReactionBar
        reactions={[{ emoji: '👍', count: 1, user_ids: ['u-2'] }]}
        channelId="ch-1"
        messageId="m-1"
        currentUserId={undefined}
        userMap={userMap}
      />,
    );
    // 仍渲染 reaction pill (展示别人的)
    expect(container!.querySelectorAll('.reaction-pill').length).toBe(1);
    // 但不渲染 + 按钮
    expect(container!.querySelector('button[data-reaction-add-variant]')).toBeNull();
  });
});
