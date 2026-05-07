// GroupHeader.test.tsx
// 锁住 #689 修出来的三条排版行为:
//   1. 按钮跟标题文字对齐: drag-handle 跟 ⋯ 按钮都用 group-header-* 类
//      (现在它们用统一 20px 方块, 不再混用通用 .icon-btn 32x32)
//   2. 折叠时三角朝右: collapsed=true 时字符是 ▶ (右), expanded 是 ▼ (下);
//      .group-header-arrow.collapsed 不应有 transform 把字符再转一次
//   3. 高度紧凑: .group-header 的 padding 现在是 2px 12px (历史是 6px 12px)
//      header 行不再被 32x32 的子元素撑高
import React from 'react';
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { createRoot } from 'react-dom/client';
import { act } from 'react-dom/test-utils';
import { DndContext } from '@dnd-kit/core';
import GroupHeader from '../components/GroupHeader';
import type { ChannelGroup } from '../types';

const sampleGroup: ChannelGroup = {
  id: 'g-1',
  name: '工程',
  created_by: 'u-1',
  created_at: 1,
  position: 'a',
};

function renderHeader(
  container: HTMLDivElement,
  props: Partial<React.ComponentProps<typeof GroupHeader>> = {},
) {
  const root = createRoot(container);
  act(() => {
    root.render(
      <DndContext>
        <GroupHeader
          group={sampleGroup}
          collapsed={props.collapsed ?? false}
          onToggle={props.onToggle ?? (() => {})}
          isOwner={props.isOwner ?? true}
          renaming={props.renaming ?? false}
          onContextMenu={props.onContextMenu}
          onRenameSubmit={props.onRenameSubmit}
          onRenameCancel={props.onRenameCancel}
        />
      </DndContext>,
    );
  });
  return root;
}

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

describe('GroupHeader — #689 排版三条', () => {
  it('折叠时左边三角字符是 ▶ (朝右), 展开时是 ▼ (朝下)', () => {
    renderHeader(container!, { collapsed: true });
    const arrow = container!.querySelector('.group-header-arrow') as HTMLElement;
    expect(arrow.textContent).toBe('▶');
    expect(arrow.classList.contains('collapsed')).toBe(true);
  });

  it('展开时三角字符是 ▼', () => {
    renderHeader(container!, { collapsed: false });
    const arrow = container!.querySelector('.group-header-arrow') as HTMLElement;
    expect(arrow.textContent).toBe('▼');
    expect(arrow.classList.contains('collapsed')).toBe(false);
  });

  it('drag-handle 用 group-header-drag-handle 类 (跟 ⋯ 按钮统一 20px 方块)', () => {
    renderHeader(container!, { isOwner: true });
    const handle = container!.querySelector(
      '.group-header-actions .group-header-drag-handle',
    ) as HTMLElement | null;
    expect(handle).not.toBeNull();
    expect(handle!.textContent).toBe('≡');
    // 反向: 不再用裸 .icon-btn 让 32x32 撑大 header 行
    const oldIconBtn = container!.querySelector(
      '.group-header-actions .icon-btn:not(.group-header-menu-btn)',
    );
    expect(oldIconBtn).toBeNull();
  });

  it('⋯ 按钮用 group-header-menu-btn 类 + aria-label 已写', () => {
    renderHeader(container!, { isOwner: true });
    const menuBtn = container!.querySelector(
      '.group-header-actions .group-header-menu-btn',
    ) as HTMLButtonElement | null;
    expect(menuBtn).not.toBeNull();
    expect(menuBtn!.tagName).toBe('BUTTON');
    expect(menuBtn!.getAttribute('aria-label')).toBe('分组菜单');
    expect(menuBtn!.textContent).toBe('⋯');
    // 反向: 之前是 inline style fontSize/padding, 现在交给 CSS 类管, 不应留
    expect(menuBtn!.getAttribute('style')).toBeNull();
  });

  it('非所有者: 不渲染 actions 区 (drag-handle + ⋯ 按钮都不出)', () => {
    renderHeader(container!, { isOwner: false });
    expect(container!.querySelector('.group-header-actions')).toBeNull();
  });

  it('group-header 用 div 容器 + data-collapsed 反映状态', () => {
    renderHeader(container!, { collapsed: true });
    const header = container!.querySelector('.group-header') as HTMLElement;
    expect(header.tagName).toBe('DIV');
    expect(header.getAttribute('data-collapsed')).toBe('true');
  });
});
