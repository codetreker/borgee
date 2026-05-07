// ReactionAddButton.test.tsx — gh#686 §6 测试清单 4 类:
//   1. 两种 variant 各自 className + ➕ 文字 + title + 点击开关 picker
//   2. 失败时调 showToast + 撤回乐观 pill
//   3. busy 期间二次 click 不发第二次请求 (双击防御)
//   4. aria-label / aria-haspopup / aria-expanded 字面 + §6.1 文案锁
import React from 'react';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react-dom/test-utils';

// mock @emoji-mart/react 避免拉重依赖
vi.mock('@emoji-mart/react', () => ({
  default: ({ onEmojiSelect }: { onEmojiSelect: (e: { native: string }) => void }) =>
    React.createElement('div', {
      'data-test': 'emoji-mart',
      onClick: () => onEmojiSelect({ native: '👍' }),
    }, 'mock-picker'),
}));
vi.mock('@emoji-mart/data', () => ({ default: {} }));

vi.mock('../lib/api', async () => {
  const actual = await vi.importActual<typeof import('../lib/api')>('../lib/api');
  return {
    ...actual,
    addReaction: vi.fn().mockResolvedValue(undefined),
    removeReaction: vi.fn().mockResolvedValue(undefined),
  };
});

import ReactionAddButton from '../components/ReactionAddButton';
import * as api from '../lib/api';

// 替 useAppContext + useToast 用 jest.spyOn 模式不行 (它们是 hook), 走
// vi.mock 模块替换更稳.
const mockDispatch = vi.fn();
const mockShowToast = vi.fn();
vi.mock('../context/AppContext', () => ({
  useAppContext: () => ({ dispatch: mockDispatch }),
}));
vi.mock('../components/Toast', () => ({
  useToast: () => ({ showToast: mockShowToast }),
}));

let container: HTMLDivElement | null = null;
let root: Root | null = null;

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
  mockDispatch.mockClear();
  mockShowToast.mockClear();
  vi.mocked(api.addReaction).mockClear();
  vi.mocked(api.addReaction).mockResolvedValue(undefined);
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

const baseProps = {
  channelId: 'ch-1',
  messageId: 'm-1',
  currentUserId: 'u-current',
};

describe('ReactionAddButton — gh#686 §6 4 类断言', () => {
  describe('① variant 类名 + 字面', () => {
    it('inline-pill variant: 用 reaction-pill + reaction-add 类', () => {
      render(<ReactionAddButton {...baseProps} variant="inline-pill" />);
      const btn = container!.querySelector('button[data-reaction-add-variant="inline-pill"]') as HTMLButtonElement;
      expect(btn).not.toBeNull();
      expect(btn.classList.contains('reaction-pill')).toBe(true);
      expect(btn.classList.contains('reaction-add')).toBe(true);
      expect(btn.classList.contains('message-action-btn')).toBe(false);
    });

    it('toolbar-btn variant: 用 message-action-btn + message-action-react 类', () => {
      render(<ReactionAddButton {...baseProps} variant="toolbar-btn" />);
      const btn = container!.querySelector('button[data-reaction-add-variant="toolbar-btn"]') as HTMLButtonElement;
      expect(btn).not.toBeNull();
      expect(btn.classList.contains('message-action-btn')).toBe(true);
      expect(btn.classList.contains('message-action-react')).toBe(true);
      expect(btn.classList.contains('reaction-pill')).toBe(false);
    });

    it('文字 ➕ + title 添加表情', () => {
      render(<ReactionAddButton {...baseProps} variant="toolbar-btn" />);
      const btn = container!.querySelector('button[data-reaction-add-variant]') as HTMLButtonElement;
      expect(btn.textContent).toBe('➕');
      expect(btn.getAttribute('title')).toBe('添加表情');
    });

    it('点击 button 渲染 picker, 再点关 (toggle)', () => {
      render(<ReactionAddButton {...baseProps} variant="toolbar-btn" />);
      expect(container!.querySelector('.reaction-picker-popover')).toBeNull();
      const btn = container!.querySelector('button[data-reaction-add-variant]') as HTMLButtonElement;
      act(() => { btn.click(); });
      expect(container!.querySelector('.reaction-picker-popover')).not.toBeNull();
      act(() => { btn.click(); });
      expect(container!.querySelector('.reaction-picker-popover')).toBeNull();
    });
  });

  describe('② 失败时撤回乐观 + showToast (§4 #11)', () => {
    it('addReaction reject → dispatch REMOVE_REACTION_OPTIMISTIC + showToast 字面 byte-identical', async () => {
      vi.mocked(api.addReaction).mockRejectedValueOnce(new Error('5xx'));
      render(<ReactionAddButton {...baseProps} variant="toolbar-btn" />);
      const btn = container!.querySelector('button[data-reaction-add-variant]') as HTMLButtonElement;
      act(() => { btn.click(); });
      const picker = container!.querySelector('[data-test="emoji-mart"]') as HTMLElement;
      await act(async () => {
        picker.click();
        await new Promise(r => setTimeout(r, 0));
      });

      // 先 dispatch ADD_REACTION_OPTIMISTIC, 后 dispatch REMOVE_REACTION_OPTIMISTIC
      const dispatches = mockDispatch.mock.calls.map(c => c[0].type);
      expect(dispatches).toEqual([
        'ADD_REACTION_OPTIMISTIC',
        'REMOVE_REACTION_OPTIMISTIC',
      ]);
      expect(mockDispatch.mock.calls[0]![0]).toMatchObject({
        type: 'ADD_REACTION_OPTIMISTIC',
        channelId: 'ch-1',
        messageId: 'm-1',
        emoji: '👍',
        userId: 'u-current',
      });
      expect(mockDispatch.mock.calls[1]![0]).toMatchObject({
        type: 'REMOVE_REACTION_OPTIMISTIC',
        channelId: 'ch-1',
        messageId: 'm-1',
        emoji: '👍',
        userId: 'u-current',
      });

      // §6.1 文案锁: byte-identical "添加 reaction 失败, 请重试"
      expect(mockShowToast).toHaveBeenCalledWith('添加 reaction 失败, 请重试');
    });

    it('addReaction resolve → dispatch ADD_REACTION_OPTIMISTIC, 不撤回, 不 toast', async () => {
      vi.mocked(api.addReaction).mockResolvedValueOnce(undefined);
      render(<ReactionAddButton {...baseProps} variant="toolbar-btn" />);
      const btn = container!.querySelector('button[data-reaction-add-variant]') as HTMLButtonElement;
      act(() => { btn.click(); });
      const picker = container!.querySelector('[data-test="emoji-mart"]') as HTMLElement;
      await act(async () => {
        picker.click();
        await new Promise(r => setTimeout(r, 0));
      });
      const dispatches = mockDispatch.mock.calls.map(c => c[0].type);
      expect(dispatches).toEqual(['ADD_REACTION_OPTIMISTIC']);
      expect(mockShowToast).not.toHaveBeenCalled();
    });
  });

  describe('③ busy 期间防双击 (§4 #12)', () => {
    it('addReaction 进行中时第二次 click 不发第二次请求', async () => {
      // 让 addReaction pending 不 resolve, 模拟"还在飞"
      let resolveFirst!: () => void;
      vi.mocked(api.addReaction).mockImplementationOnce(
        () => new Promise<void>((res) => { resolveFirst = res; }),
      );
      render(<ReactionAddButton {...baseProps} variant="toolbar-btn" />);
      const btn = container!.querySelector('button[data-reaction-add-variant]') as HTMLButtonElement;
      act(() => { btn.click(); });
      const picker = container!.querySelector('[data-test="emoji-mart"]') as HTMLElement;
      // 第一次 select
      act(() => { picker.click(); });
      // busy 期间 button 应 disabled
      expect(btn.disabled).toBe(true);
      // 第二次 click: 不应再发请求
      act(() => { btn.click(); });
      expect(api.addReaction).toHaveBeenCalledTimes(1);
      // resolve 让组件解开 busy
      await act(async () => {
        resolveFirst();
        await new Promise(r => setTimeout(r, 0));
      });
    });
  });

  describe('④ a11y 字面 (§4 #15)', () => {
    it('aria-label / aria-haspopup / aria-expanded 字面 byte-identical', () => {
      render(<ReactionAddButton {...baseProps} variant="toolbar-btn" />);
      const btn = container!.querySelector('button[data-reaction-add-variant]') as HTMLButtonElement;
      expect(btn.getAttribute('aria-label')).toBe('添加表情');
      expect(btn.getAttribute('aria-haspopup')).toBe('dialog');
      expect(btn.getAttribute('aria-expanded')).toBe('false');
      act(() => { btn.click(); });
      expect(btn.getAttribute('aria-expanded')).toBe('true');
    });
  });

  describe('反向断言 — §6.1 文案锁 byte-identical', () => {
    it('showToast 收到的字面 byte-identical 跟 design §6.1 一致', () => {
      // §6.1 文案锁: 改 = 改三处 (design + 这里 + ReactionAddButton.tsx).
      // 反向 grep 防近义词漂移那一条是 source-file 层面的 grep, 不是 runtime
      // 字符串 substring (例如 "reaction 失败" 自然是 "添加 reaction 失败,
      // 请重试" 的子串, 不能用 not.toContain). 这里只锁 byte-identical 全等.
      const ALLOWED = '添加 reaction 失败, 请重试';
      vi.mocked(api.addReaction).mockRejectedValueOnce(new Error('5xx'));
      render(<ReactionAddButton {...baseProps} variant="toolbar-btn" />);
      const btn = container!.querySelector('button[data-reaction-add-variant]') as HTMLButtonElement;
      act(() => { btn.click(); });
      const picker = container!.querySelector('[data-test="emoji-mart"]') as HTMLElement;
      return act(async () => {
        picker.click();
        await new Promise(r => setTimeout(r, 0));
      }).then(() => {
        const arg = mockShowToast.mock.calls[0]?.[0] as string;
        expect(arg).toBe(ALLOWED);
      });
    });
  });
});
