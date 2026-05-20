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

describe('SortableChannelItem private indicator visual treatment (lock badge)', () => {
  it('private active row keeps # as the base glyph with the lock badge overlaid in the same slot', () => {
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

    // Base glyph stays as `#` — private channels are NOT character-replaced.
    const base = indicator.querySelector('.channel-hash-base') as HTMLElement;
    expect(base).not.toBeNull();
    expect(base.textContent).toBe('#');

    // Lock badge is rendered as an inline SVG inside the same .channel-hash slot.
    const lock = indicator.querySelector('svg.channel-lock-badge') as SVGElement | null;
    expect(lock).not.toBeNull();
    expect(lock!.getAttribute('aria-hidden')).toBe('true');
    expect(lock!.getAttribute('data-private-lock')).toBe('true');

    // Reverse-X: the literal CJK character "锁" must NEVER appear in the row
    // (反 PR #952 残留 — 用户拍 icon-on-icon 不字符替换).
    expect(row.textContent).not.toContain('锁');
    expect(container.innerHTML).not.toContain('🔒');

    const name = row.querySelector('.channel-name') as HTMLElement;
    expect(indicator.compareDocumentPosition(name) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(container.querySelector('.channel-pinned-indicator')?.textContent).toBe('📌');
    expect(container.querySelector('.unread-badge')?.textContent).toBe('12');
    expect(container.querySelector('[data-presence]')).toBeNull();
    expect(container.querySelector('[data-failure-badge]')).toBeNull();
  });

  it('public rows keep the bare hash with no lock badge', () => {
    renderSortable(channel({ visibility: 'public', unread_count: 2 }));

    const row = container.querySelector('.channel-item') as HTMLButtonElement;
    expect(row.getAttribute('data-private')).toBeNull();

    const hash = container.querySelector('.channel-hash') as HTMLElement;
    expect(hash).not.toBeNull();
    expect(hash.querySelector('.channel-hash-base')?.textContent).toBe('#');
    expect(hash.classList.contains('channel-private-indicator')).toBe(false);
    expect(container.querySelector('[data-private-indicator="true"]')).toBeNull();
    expect(container.querySelector('.channel-lock-badge')).toBeNull();
    expect(container.querySelector('[data-private-lock="true"]')).toBeNull();
    expect(row.textContent).not.toContain('锁');

    expect(container.querySelector('.unread-badge')?.textContent).toBe('2');
  });

  it('archived public rows render 📦 with no lock badge', () => {
    renderSortable(channel({ visibility: 'public', archived_at: 1700000001000, unread_count: 9 }));

    const row = container.querySelector('.channel-item') as HTMLButtonElement;
    expect(row.getAttribute('data-private')).toBeNull();
    expect(row.getAttribute('data-archived')).toBe('true');

    const hash = container.querySelector('.channel-hash') as HTMLElement;
    expect(hash.querySelector('.channel-hash-base')?.textContent).toBe('📦');
    expect(container.querySelector('[data-private-indicator="true"]')).toBeNull();
    expect(container.querySelector('.channel-lock-badge')).toBeNull();
    expect(row.textContent).not.toContain('锁');

    expect(container.querySelector('.archived-badge')?.textContent).toBe('已归档');
    expect(container.querySelector('.unread-badge')).toBeNull();
  });

  it('archived private rows render 📦 base WITH the lock badge overlaid (用户拍归档私有也叠)', () => {
    renderSortable(channel({ archived_at: 1700000001000, unread_count: 9 }));

    const row = container.querySelector('.channel-item') as HTMLButtonElement;
    // data-private row attribute is suppressed for archived rows by design
    // (other code keys archive flow on data-archived); but the lock badge
    // visual cue is still rendered.
    expect(row.getAttribute('data-archived')).toBe('true');

    const indicator = row.querySelector('[data-private-indicator="true"]') as HTMLElement;
    expect(indicator).not.toBeNull();
    expect(indicator.getAttribute('aria-label')).toBe('私有频道');
    expect(indicator.querySelector('.channel-hash-base')?.textContent).toBe('📦');

    const lock = indicator.querySelector('svg.channel-lock-badge');
    expect(lock).not.toBeNull();

    expect(row.textContent).not.toContain('锁');
    expect(container.querySelector('.archived-badge')?.textContent).toBe('已归档');
    expect(container.querySelector('.unread-badge')).toBeNull();
  });

  it('static channel rows render the same lock-badge treatment for private channels', () => {
    act(() => {
      root.render(<ChannelItemStatic channel={channel()} active={false} onClick={() => undefined} />);
    });

    const row = container.querySelector('.channel-item') as HTMLButtonElement;
    expect(row.getAttribute('data-private')).toBe('true');

    const indicator = container.querySelector('[data-private-indicator="true"]') as HTMLElement;
    expect(indicator).not.toBeNull();
    expect(indicator.querySelector('.channel-hash-base')?.textContent).toBe('#');
    expect(indicator.querySelector('svg.channel-lock-badge')).not.toBeNull();
    expect(row.textContent).not.toContain('锁');
    expect(container.querySelector('.preview-badge')).toBeNull();
  });
});
