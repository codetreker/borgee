import React from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import type { Channel } from '../types';

const sortableState = vi.hoisted(() => ({
  isDragging: false,
  isOver: false,
}));

vi.mock('@dnd-kit/sortable', () => ({
  useSortable: () => ({
    attributes: {},
    listeners: {},
    setNodeRef: vi.fn(),
    transform: null,
    transition: undefined,
    isDragging: sortableState.isDragging,
    isOver: sortableState.isOver,
  }),
}));

vi.mock('@dnd-kit/utilities', () => ({
  CSS: {
    Transform: {
      toString: () => undefined,
    },
  },
}));

import SortableChannelItem, { ChannelItemStatic } from '../components/SortableChannelItem';

// @ts-expect-error - node:module is available in the Vitest node runtime.
import { createRequire } from 'module';

const nodeRequire = createRequire(import.meta.url);
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const fs: any = nodeRequire('fs');
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const nodePath: any = nodeRequire('path');
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const url: any = nodeRequire('url');

const HERE: string = nodePath.dirname(url.fileURLToPath(import.meta.url));
const SRC_ROOT: string = nodePath.resolve(HERE, '..');

let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
  sortableState.isDragging = false;
  sortableState.isOver = false;
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
    name: 'private-regression-room',
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
      <SortableChannelItem
        channel={ch}
        active={false}
        isOwner={true}
        onClick={() => undefined}
        {...props}
      />,
    );
  });
}

function renderStatic(ch: Channel, props: Partial<React.ComponentProps<typeof ChannelItemStatic>> = {}) {
  act(() => {
    root.render(<ChannelItemStatic channel={ch} active={false} onClick={() => undefined} {...props} />);
  });
}

function read(path: string): string {
  return fs.readFileSync(path, 'utf8') as string;
}

describe('M2 Task9 sidebar state collision regression', () => {
  it('keeps private, unread, selected, pinned, and drag-over anchors distinct on one channel row', () => {
    sortableState.isOver = true;
    renderSortable(channel({ unread_count: 128 }), { active: true, pinned: true });

    const row = container.querySelector('.channel-item') as HTMLButtonElement;
    const privateMarker = row.querySelector('[data-private-indicator="true"]') as HTMLElement;
    const name = row.querySelector('.channel-name') as HTMLElement;
    const pinned = row.querySelector('.channel-pinned-indicator') as HTMLElement;
    const unread = row.querySelector('.unread-badge') as HTMLElement;
    const dropIndicator = row.querySelector('.drop-indicator') as HTMLElement;

    expect(row.getAttribute('data-kind')).toBe('channel');
    expect(row.getAttribute('data-private')).toBe('true');
    expect(row.getAttribute('data-pinned')).toBe('true');
    expect(row.classList.contains('channel-item-active')).toBe(true);
    expect(privateMarker.querySelector('.channel-hash-base')?.textContent).toBe('#');
    expect(privateMarker.querySelector('svg.channel-lock-badge')).not.toBeNull();
    expect(row.textContent).not.toContain('锁');
    expect(privateMarker.compareDocumentPosition(name) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(name.compareDocumentPosition(pinned) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(pinned.compareDocumentPosition(unread) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(unread.textContent).toBe('99+');
    expect(dropIndicator).not.toBeNull();
  });

  it('archived rows show the 📦 base + lock overlay for private channels but never unread', () => {
    renderSortable(channel({ archived_at: 1700000001000, unread_count: 3 }));

    let row = container.querySelector('.channel-item') as HTMLButtonElement;
    expect(row.getAttribute('data-private')).toBeNull();
    expect(row.getAttribute('data-archived')).toBe('true');
    // Archived private channels keep the lock overlay (用户拍归档私有也叠锁).
    const archivedPrivate = row.querySelector('[data-private-indicator="true"]') as HTMLElement;
    expect(archivedPrivate).not.toBeNull();
    expect(archivedPrivate.querySelector('.channel-hash-base')?.textContent).toBe('📦');
    expect(archivedPrivate.querySelector('svg.channel-lock-badge')).not.toBeNull();
    expect(row.textContent).not.toContain('锁');
    expect(row.querySelector('.unread-badge')).toBeNull();
    expect(row.querySelector('.archived-badge')?.textContent).toBe('已归档');

    renderStatic(channel({ visibility: 'public', is_member: false, unread_count: 7 }), { active: true });
    row = container.querySelector('.channel-item') as HTMLButtonElement;
    expect(row.getAttribute('data-private')).toBeNull();
    expect(row.querySelector('[data-private-indicator="true"]')).toBeNull();
    expect(row.classList.contains('channel-item-preview')).toBe(true);
    expect(row.querySelector('.preview-badge')?.textContent).toBe('预览');
    expect(row.querySelector('.unread-badge')).toBeNull();
  });

  it('keeps channel private markers separate from DM-only presence and fault semantics', () => {
    const sortableItem = read(nodePath.join(SRC_ROOT, 'components', 'SortableChannelItem.tsx'));
    const sidebar = read(nodePath.join(SRC_ROOT, 'components', 'Sidebar.tsx'));

    expect(sortableItem).not.toContain('PresenceDot');
    expect(sortableItem).not.toContain('data-presence');
    expect(sortableItem).not.toContain('data-failure-badge');
    expect(sidebar).toContain('data-kind="dm"');
    expect(sidebar).toContain('PresenceDot');
  });
});
