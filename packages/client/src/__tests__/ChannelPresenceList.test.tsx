// ChannelPresenceList.test.tsx — RT-4.3 5 vitest cases pin content-lock.
//
// Cases:
//   ① 文案 `当前在线 N 人` byte-identical (N 动态)
//   ② ≤5 显示头像 + N>5 显示前 5 + `+M` overflow chip 字面
//   ③ 空 onlineUserIds → return null (整个 list 不渲染)
//   ④ data-presence-user-id 行级锚 byte-identical
//   ⑤ 同义词反向 reject — source grep user-visible Chinese 0 hit (className/
//      data-testid 例外); 既有 RT-2 typing path byte-identical (grep 检查
//      `rt_4|rt4|RT4` 在 TypingIndicator.tsx 0 hit).

import React from 'react';
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react-dom/test-utils';
// @ts-expect-error — node:module no @types/node
import { createRequire } from 'module';
import { ChannelPresenceList } from '../components/ChannelPresenceList';

const nodeRequire = createRequire(import.meta.url);
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const fs: any = nodeRequire('fs');
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const nodePath: any = nodeRequire('path');
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const url: any = nodeRequire('url');
const HERE = nodePath.dirname(url.fileURLToPath(import.meta.url));

let container: HTMLDivElement | null = null;
let root: Root | null = null;

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
});

afterEach(() => {
  act(() => {
    root?.unmount();
  });
  container?.remove();
  container = null;
  root = null;
});

describe('RT-4.3 ChannelPresenceList content lock', () => {
  it('① count 文案 `当前在线 N 人` byte-identical', () => {
    act(() => {
      root!.render(<ChannelPresenceList onlineUserIds={['u1', 'u2', 'u3']} />);
    });
    const list = container!.querySelector('[data-testid="channel-presence-list"]');
    expect(list).not.toBeNull();
    expect(list!.querySelector('.channel-presence-count')!.textContent).toBe(
      '当前在线 3 人',
    );
  });

  it('② ≤5 显示头像 / >5 显示前 5 + overflow chip', () => {
    act(() => {
      root!.render(<ChannelPresenceList onlineUserIds={['a', 'b', 'c']} />);
    });
    expect(
      container!.querySelectorAll('.channel-presence-avatar').length,
    ).toBe(3);
    expect(
      container!.querySelector('[data-testid="channel-presence-overflow"]'),
    ).toBeNull();

    // 7 → 5 显示 + +2 overflow.
    act(() => {
      root!.render(
        <ChannelPresenceList
          onlineUserIds={['a', 'b', 'c', 'd', 'e', 'f', 'g']}
        />,
      );
    });
    expect(
      container!.querySelectorAll('.channel-presence-avatar').length,
    ).toBe(5);
    const overflow = container!.querySelector(
      '[data-testid="channel-presence-overflow"]',
    );
    expect(overflow).not.toBeNull();
    expect(overflow!.textContent).toBe('+2');
  });

  it('③ empty onlineUserIds → return null (不渲染)', () => {
    act(() => {
      root!.render(<ChannelPresenceList onlineUserIds={[]} />);
    });
    expect(
      container!.querySelector('[data-testid="channel-presence-list"]'),
    ).toBeNull();
  });

  it('④ data-presence-user-id 行级锚 byte-identical', () => {
    act(() => {
      root!.render(<ChannelPresenceList onlineUserIds={['user-42']} />);
    });
    const li = container!.querySelector(
      '.channel-presence-avatar',
    ) as HTMLElement;
    expect(li.getAttribute('data-presence-user-id')).toBe('user-42');
  });

  it('⑤ 同义词反向 reject + 既有 RT-2 typing byte-identical', () => {
    const compPath = nodePath.resolve(
      HERE,
      '..',
      'components',
      'ChannelPresenceList.tsx',
    );
    const src: string = fs.readFileSync(compPath, 'utf8');
    // user-visible Chinese 反向 reject — 我们用 `当前在线`.
    for (const tok of ['在线状态', '上线', '在线人员', '在线列表']) {
      expect(src.includes(tok)).toBe(false);
    }
    // RT-2 既有 TypingIndicator.tsx 不漂入 RT-4.
    const typingPath = nodePath.resolve(
      HERE,
      '..',
      'components',
      'TypingIndicator.tsx',
    );
    const typingSrc: string = fs.readFileSync(typingPath, 'utf8');
    for (const tok of ['rt_4', 'RT4', 'rt4', 'ChannelPresenceList']) {
      expect(typingSrc.includes(tok)).toBe(false);
    }
  });
});
