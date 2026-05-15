// DMThread.test.tsx — DM-6.2 DM thread reply UI 文案锁 + 同义词反向
// + 空 thread null + reply submit + thread depth 1 层 反断.
import React from 'react';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { createRoot } from 'react-dom/client';
import { act } from 'react';
import { DMThread } from '../components/DMThread';
import type { Message } from '../types';

let container: HTMLDivElement | null = null;

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
});

afterEach(() => {
  if (container) {
    document.body.removeChild(container);
    container = null;
  }
});

function makeMsg(id: string, content: string): Message {
  return {
    id,
    channel_id: 'ch-1',
    sender_id: 'u-1',
    content,
    created_at: 1700000000000,
    reply_to_id: 'parent-1',
  } as unknown as Message;
}

describe('DMThread — DM-6.2 文案锁 + DOM byte-identical', () => {
  it('折叠态 toggle 文案=`▶ 显示 N 条回复` byte-identical', () => {
    const root = createRoot(container!);
    act(() => {
      root.render(
        <DMThread
          parentId="parent-1"
          replies={[makeMsg('r-1', 'reply A'), makeMsg('r-2', 'reply B')]}
        />,
      );
    });
    const btn = container!.querySelector('[data-testid="dm6-thread-toggle"]') as HTMLButtonElement;
    expect(btn).not.toBeNull();
    expect(btn.textContent).toBe('▶ 显示 2 条回复');
  });

  it('展开态 toggle 文案=`▼ 隐藏 N 条回复` byte-identical (after click)', () => {
    const root = createRoot(container!);
    act(() => {
      root.render(
        <DMThread
          parentId="parent-1"
          replies={[makeMsg('r-1', 'reply A')]}
        />,
      );
    });
    const btn = container!.querySelector('[data-testid="dm6-thread-toggle"]') as HTMLButtonElement;
    act(() => {
      btn.click();
    });
    expect(btn.textContent).toBe('▼ 隐藏 1 条回复');
  });

  it('reply input + submit DOM byte-identical (placeholder + 文案)', () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    const root = createRoot(container!);
    act(() => {
      root.render(
        <DMThread
          parentId="parent-1"
          replies={[makeMsg('r-1', 'a')]}
          onSubmit={onSubmit}
        />,
      );
    });
    const btn = container!.querySelector('[data-testid="dm6-thread-toggle"]') as HTMLButtonElement;
    act(() => {
      btn.click();
    });
    const input = container!.querySelector('[data-testid="dm6-reply-input"]') as HTMLTextAreaElement;
    expect(input).not.toBeNull();
    expect(input.placeholder).toBe('回复...');
    const submit = container!.querySelector('[data-testid="dm6-reply-submit"]') as HTMLButtonElement;
    expect(submit).not.toBeNull();
    expect(submit.textContent).toBe('发送');
  });

  it('submit → onSubmit(content, parentId)', async () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    const root = createRoot(container!);
    act(() => {
      root.render(
        <DMThread
          parentId="parent-1"
          replies={[makeMsg('r-1', 'a')]}
          onSubmit={onSubmit}
        />,
      );
    });
    const toggle = container!.querySelector('[data-testid="dm6-thread-toggle"]') as HTMLButtonElement;
    act(() => {
      toggle.click();
    });
    const input = container!.querySelector('[data-testid="dm6-reply-input"]') as HTMLTextAreaElement;
    // Use native input setter so React picks up the change event.
    const setter = Object.getOwnPropertyDescriptor(window.HTMLTextAreaElement.prototype, 'value')!.set!;
    await act(async () => {
      setter.call(input, 'hello reply');
      input.dispatchEvent(new Event('input', { bubbles: true }));
    });
    const submit = container!.querySelector('[data-testid="dm6-reply-submit"]') as HTMLButtonElement;
    await act(async () => {
      submit.click();
      await new Promise(r => setTimeout(r, 0));
    });
    expect(onSubmit).toHaveBeenCalledWith('hello reply', 'parent-1');
  });

  it('空 thread (replies.length === 0) 不渲染 (return null)', () => {
    const root = createRoot(container!);
    act(() => {
      root.render(<DMThread parentId="parent-1" replies={[]} />);
    });
    const btn = container!.querySelector('[data-testid="dm6-thread-toggle"]');
    expect(btn).toBeNull();
  });

  it('反向断言 — 同义词 0 出现 user-visible text', () => {
    const onSubmit = vi.fn().mockResolvedValue(undefined);
    const root = createRoot(container!);
    act(() => {
      root.render(
        <DMThread
          parentId="parent-1"
          replies={[makeMsg('r-1', 'a')]}
          onSubmit={onSubmit}
        />,
      );
    });
    const btn = container!.querySelector('[data-testid="dm6-thread-toggle"]') as HTMLButtonElement;
    act(() => {
      btn.click();
    });
    const visibleText = (container!.textContent || '');
    const forbidden = ['comment', 'discussion', '讨论', '评论', '评论区', '跟帖'];
    for (const f of forbidden) {
      expect(visibleText).not.toContain(f);
    }
    expect(visibleText.toLowerCase()).not.toContain('reply');
    // After expand the placeholder appears as attribute (not in textContent),
    // so we check innerHTML contains the placeholder literal.
    const html = container!.innerHTML;
    expect(html).toContain('回复...');
  });

  it('thread depth 1 层强制 — reply 行内不渲染 sub-thread toggle', () => {
    const root = createRoot(container!);
    act(() => {
      root.render(
        <DMThread
          parentId="parent-1"
          replies={[makeMsg('r-1', 'a'), makeMsg('r-2', 'b')]}
        />,
      );
    });
    const toggle = container!.querySelector('[data-testid="dm6-thread-toggle"]') as HTMLButtonElement;
    act(() => {
      toggle.click();
    });
    // Only ONE toggle should exist (the parent one), no nested toggles
    // inside <li class="dm-thread-reply">.
    const allToggles = container!.querySelectorAll('[data-testid="dm6-thread-toggle"]');
    expect(allToggles.length).toBe(1);
    const repliesItems = container!.querySelectorAll('.dm-thread-reply');
    repliesItems.forEach(item => {
      expect(item.querySelector('[data-testid="dm6-thread-toggle"]')).toBeNull();
    });
  });
});
