// Regression: nested <button> inside <button> fires React validateDOMNesting
// warning and risks hydration breakage in prod. Found by borgee-local-e2e
// skill first-run; console quote:
//   Warning: validateDOMNesting(...): <button> cannot appear as a descendant
//   of <button>.
//
// The fix swaps the inner drag handle from <button> to a <span role="button">
// keyboard-accessible control. This test pins:
//   1) The outer row stays a <button>.
//   2) No <button> element exists inside another <button> in the rendered tree.
//   3) The inner handle remains keyboard-accessible (role=button, tabIndex=0,
//      aria-label preserved).
//   4) Clicking the handle does NOT bubble to the outer onClick handler
//      (stopPropagation preserved).
//   5) Clicking the row body still fires onClick.

import React from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react';
import { DndContext } from '@dnd-kit/core';
import { SortableContext } from '@dnd-kit/sortable';
import SortableChannelItem from '../components/SortableChannelItem';
import type { Channel } from '../types';

let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
});

afterEach(() => {
  act(() => root.unmount());
  document.body.removeChild(container);
});

function channel(overrides: Partial<Channel> = {}): Channel {
  return {
    id: 'c-nest',
    name: 'general',
    topic: '',
    type: 'channel',
    created_by: 'u-1',
    visibility: 'public',
    is_member: true,
    unread_count: 0,
    created_at: 1700000000000,
    ...overrides,
  };
}

function render(ch: Channel, onClick: () => void) {
  act(() => {
    root.render(
      <DndContext>
        <SortableContext items={[ch.id]}>
          <SortableChannelItem
            channel={ch}
            active={false}
            isOwner={true}
            onClick={onClick}
          />
        </SortableContext>
      </DndContext>,
    );
  });
}

describe('SortableChannelItem — no nested <button> (validateDOMNesting fix)', () => {
  it('outer row is a <button>; inner drag handle is NOT a <button>', () => {
    render(channel(), () => undefined);

    const row = container.querySelector('.channel-item') as HTMLElement;
    expect(row).not.toBeNull();
    expect(row.tagName).toBe('BUTTON');

    const handle = row.querySelector('[data-sortable-handle=""]') as HTMLElement;
    expect(handle).not.toBeNull();
    expect(handle.tagName).not.toBe('BUTTON');
  });

  it('no <button> appears as descendant of another <button> anywhere in the tree', () => {
    render(channel({ unread_count: 5 }), () => undefined);

    const outerButtons = container.querySelectorAll('button');
    expect(outerButtons.length).toBeGreaterThan(0);
    for (const b of Array.from(outerButtons)) {
      const inner = b.querySelector('button');
      expect(inner, `<button> nested inside another <button>: ${b.outerHTML}`).toBeNull();
    }
  });

  it('inner handle remains keyboard-accessible (role=button + tabIndex + aria-label preserved)', () => {
    render(channel(), () => undefined);
    const handle = container.querySelector('[data-sortable-handle=""]') as HTMLElement;
    expect(handle).not.toBeNull();
    expect(handle.getAttribute('role')).toBe('button');
    expect(handle.getAttribute('aria-label')).toBe('拖拽调整顺序');
    expect(handle.tabIndex).toBe(0);
    expect(handle.textContent).toContain('⋮⋮');
  });

  it('clicking the drag handle does NOT bubble to the row onClick', () => {
    const onClick = vi.fn();
    render(channel(), onClick);

    const handle = container.querySelector('[data-sortable-handle=""]') as HTMLElement;
    act(() => {
      handle.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true }));
    });
    expect(onClick).not.toHaveBeenCalled();
  });

  it('clicking the row body still fires onClick', () => {
    const onClick = vi.fn();
    render(channel(), onClick);

    const row = container.querySelector('.channel-item') as HTMLElement;
    // Click on a descendant of the row that is NOT the handle (channel name span).
    const name = row.querySelector('.channel-name') as HTMLElement;
    expect(name).not.toBeNull();
    act(() => {
      name.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true }));
    });
    expect(onClick).toHaveBeenCalledTimes(1);
  });
});
