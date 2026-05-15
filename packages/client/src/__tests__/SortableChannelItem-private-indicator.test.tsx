import React from 'react';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react';
import { DndContext } from '@dnd-kit/core';
import { SortableContext } from '@dnd-kit/sortable';
import SortableChannelItem, { ChannelItemStatic } from '../components/SortableChannelItem';
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
    id: 'c-private',
    name: 'private-room',
    topic: '',
    type: 'channel',
    created_by: 'u-1',
    visibility: 'private',
    is_member: true,
    unread_count: 0,
    created_at: 1700000000000,
    ...overrides,
  };
}

function renderSortable(ch: Channel, props: Partial<React.ComponentProps<typeof SortableChannelItem>> = {}) {
  act(() => {
    root.render(
      <DndContext>
        <SortableContext items={[ch.id]}>
          <SortableChannelItem
            channel={ch}
            active={false}
            isOwner={true}
            onClick={() => undefined}
            {...props}
          />
        </SortableContext>
      </DndContext>,
    );
  });
}

describe('SortableChannelItem private indicator visual treatment', () => {
  it('keeps private identity in the leading slot while unread, pinned, and active states stay visible', () => {
    renderSortable(channel({ unread_count: 12 }), { active: true, pinned: true });

    const row = container.querySelector('.channel-item') as HTMLButtonElement;
    expect(row.classList.contains('channel-item-active')).toBe(true);
    expect(row.getAttribute('data-private')).toBe('true');
    expect(row.getAttribute('data-kind')).toBe('channel');

    const indicator = row.querySelector('[data-private-indicator="true"]') as HTMLElement;
    expect(indicator).not.toBeNull();
    expect(indicator.classList.contains('channel-hash')).toBe(true);
    expect(indicator.classList.contains('channel-private-indicator')).toBe(true);
    expect(indicator.getAttribute('aria-label')).toBe('私有频道');
    expect(indicator.textContent).toBe('锁');

    const name = row.querySelector('.channel-name') as HTMLElement;
    expect(indicator.compareDocumentPosition(name) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(container.querySelector('.channel-pinned-indicator')?.textContent).toBe('📌');
    expect(container.querySelector('.unread-badge')?.textContent).toBe('12');
    expect(container.innerHTML).not.toContain('🔒');
    expect(container.querySelector('[data-presence]')).toBeNull();
    expect(container.querySelector('[data-failure-badge]')).toBeNull();
  });

  it('leaves public rows as the baseline hash without private metadata', () => {
    renderSortable(channel({ visibility: 'public', unread_count: 2 }));

    const row = container.querySelector('.channel-item') as HTMLButtonElement;
    expect(row.getAttribute('data-private')).toBeNull();
    expect(container.querySelector('.channel-hash')?.textContent).toBe('#');
    expect(container.querySelector('[data-private-indicator="true"]')).toBeNull();
    expect(container.querySelector('.unread-badge')?.textContent).toBe('2');
  });

  it('lets archived state override private and suppress unread attention', () => {
    renderSortable(channel({ archived_at: 1700000001000, unread_count: 9 }));

    const row = container.querySelector('.channel-item') as HTMLButtonElement;
    expect(row.getAttribute('data-private')).toBeNull();
    expect(row.getAttribute('data-archived')).toBe('true');
    expect(container.querySelector('.channel-hash')?.textContent).toBe('📦');
    expect(container.querySelector('[data-private-indicator="true"]')).toBeNull();
    expect(container.querySelector('.archived-badge')?.textContent).toBe('已归档');
    expect(container.querySelector('.unread-badge')).toBeNull();
  });

  it('applies the same quiet private marker to static channel rows if the server returns one', () => {
    act(() => {
      root.render(<ChannelItemStatic channel={channel()} active={false} onClick={() => undefined} />);
    });

    const row = container.querySelector('.channel-item') as HTMLButtonElement;
    expect(row.getAttribute('data-private')).toBe('true');
    expect(container.querySelector('[data-private-indicator="true"]')?.textContent).toBe('锁');
    expect(container.querySelector('.preview-badge')).toBeNull();
  });
});
