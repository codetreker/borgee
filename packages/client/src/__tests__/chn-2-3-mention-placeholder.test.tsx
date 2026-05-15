// chn-2-3-mention-placeholder.test.tsx — CHN-2.3 (#357 §1.2 + #354 §1 ⑤)
// MentionList DM-only placeholder lock test.
//
// Pins #354 §1 ⑤ byte-identical placeholder + DM 反约束 (#338 cross-grep
// 反模式遵守: byte-identical 跟 docs/qa/chn-2-content-lock.md §1 ⑤
// 字面 — 这是字串唯一定义, 改 = 改两边).

import React from 'react';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react';
import MentionList, { DM_MENTION_THIRD_PARTY_PLACEHOLDER } from '../components/MentionList';

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
  if (container) document.body.removeChild(container);
  container = null;
  root = null;
});

// Build a minimal SuggestionProps shape — the test exercises only the
// items/channelType branching, not the suggestion lifecycle.
function makeProps(overrides: {
  items?: { id: string; label: string; role: string }[];
  channelType?: 'dm' | 'channel';
  // 'undefined-channel-type' lets a test pass channelType: undefined explicitly.
}) {
  return {
    items: overrides.items ?? [],
    command: vi.fn(),
    editor: {} as never,
    range: { from: 0, to: 0 },
    query: '',
    text: '@',
    decorationNode: null,
    clientRect: null,
    channelType: overrides.channelType,
  } as React.ComponentProps<typeof MentionList>;
}

function renderList(props: React.ComponentProps<typeof MentionList>) {
  act(() => {
    root!.render(<MentionList {...props} />);
  });
}

describe('MentionList — CHN-2.3 DM third-party placeholder', () => {
  it('renders DM placeholder byte-identical when items empty + DM context', () => {
    renderList(makeProps({ items: [], channelType: 'dm' }));
    // Byte-identical 跟 #354 §1 ⑤ — 字串原样.
    expect(container!.textContent).toContain(DM_MENTION_THIRD_PARTY_PLACEHOLDER);
    expect(DM_MENTION_THIRD_PARTY_PLACEHOLDER).toBe('私信仅限两人, 想加人请新建频道');
    // DOM marker for e2e反查锚.
    const empty = container!.querySelector('[data-mention-empty="dm-third-party"]');
    expect(empty).toBeTruthy();
    expect(empty?.getAttribute('data-channel-type')).toBe('dm');
  });

  it('renders nothing when items empty + channel context (既有行为不破)', () => {
    renderList(makeProps({ items: [], channelType: 'channel' }));
    // 既有 channel 路径: items.length===0 → 浮层关闭.
    expect(container!.firstChild).toBeNull();
  });

  it('renders nothing when items empty + no channelType (default 既有行为)', () => {
    renderList(makeProps({ items: [], channelType: undefined }));
    expect(container!.firstChild).toBeNull();
  });

  it('反约束: placeholder 不含同义词 (升级为频道 / Convert / Upgrade)', () => {
    // 字面锁 #354 §1 ⑤ 反约束 — 蓝图 §1.2 "想加人就**新建** channel
    // 把双方拉进去" 是新建不是 DM 转换. 同义词漂移防御.
    for (const forbidden of ['升级为频道', 'Convert to channel', 'Upgrade DM', '转为频道']) {
      expect(DM_MENTION_THIRD_PARTY_PLACEHOLDER).not.toContain(forbidden);
    }
  });

  it('renders items with data-channel-type attr (e2e DOM grep 锚)', () => {
    const items = [
      { id: 'u-alice', label: 'Alice', role: 'member' },
      { id: 'u-bob', label: 'Bob', role: 'member' },
    ];
    renderList(makeProps({ items, channelType: 'dm' }));
    const list = container!.querySelector('.mention-picker');
    expect(list?.getAttribute('data-channel-type')).toBe('dm');
    expect(container!.textContent).toContain('Alice');
    expect(container!.textContent).toContain('Bob');

    // Channel context 同样渲染候选, 但 attr 切.
    renderList(makeProps({ items, channelType: 'channel' }));
    const list2 = container!.querySelector('.mention-picker');
    expect(list2?.getAttribute('data-channel-type')).toBe('channel');
  });
});
