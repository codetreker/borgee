// SortableChannelItem.test.tsx — bf-wo fix-skill-findings / task button-nesting.
//
// Locks the three invariants from the task spec AC-2:
//   (i)   the rendered DOM tree contains no <button> descendant of another <button>;
//   (ii)  console.error is NOT called with any message matching /validateDOMNesting/
//         during render;
//   (iii) the inner sortable drag handle remains keyboard-activatable
//         (Enter and Space fire its click handler).
//
// Pre-fix codebase: the inner drag handle is a <button>, nested under the row's
// <button>. All three assertions FAIL on pre-fix HEAD (captured as ev-2-pre.txt).
// Post-fix: the handle becomes a non-<button> element with role="button" + key
// handler. All three pass.
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
    attributes: { role: 'button', tabIndex: 0, 'aria-roledescription': 'sortable' },
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

import SortableChannelItem from './SortableChannelItem';

let container: HTMLDivElement;
let root: Root;

function baseChannel(overrides: Partial<Channel> = {}): Channel {
  return {
    id: 'c-1',
    name: 'general',
    topic: '',
    type: 'channel',
    visibility: 'public',
    is_member: true,
    unread_count: 0,
    created_at: 1700000000000,
    created_by: 'u-1',
    ...overrides,
  };
}

function render(ch: Channel, props: Partial<React.ComponentProps<typeof SortableChannelItem>> = {}) {
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
  // NOTE: do NOT call vi.restoreAllMocks() — the module-level
  // consoleErrorSpy (declared below the describe block) must persist across
  // tests so React's dedup cache cannot mask a missed validateDOMNesting
  // warning by intercepting it in test #1 then losing the spy by test #2.
});

// IMPORTANT — install the console.error spy at the module level (before any
// test renders the component) so we observe React's first-occurrence
// validateDOMNesting warning. React internally dedups by stack trace; if
// another test renders the offending DOM first, the warning will be
// suppressed by React's own cache and our spy will see nothing.
const consoleErrorSpy = vi.spyOn(console, 'error');

describe('SortableChannelItem — button-nesting fix (bf task button-nesting)', () => {
  it('does NOT emit a React validateDOMNesting console.error on render', () => {
    // Use a fresh channel id so the rendered DOM differs from any later test
    // — extra defense against React's dedup cache hiding the warning.
    render(baseChannel({ id: 'validateDOMNesting-probe' }));
    const offendingCalls = consoleErrorSpy.mock.calls.filter(args =>
      args.some(a => typeof a === 'string' && /validateDOMNesting/.test(a)),
    );
    expect(
      offendingCalls,
      `console.error was called with validateDOMNesting: ${JSON.stringify(offendingCalls)}`,
    ).toEqual([]);
  });

  it('renders no <button> nested inside another <button>', () => {
    render(baseChannel());
    const outerButtons = Array.from(container.querySelectorAll('button'));
    expect(outerButtons.length).toBeGreaterThan(0);
    for (const btn of outerButtons) {
      const nestedButton = btn.querySelector('button');
      expect(nestedButton, 'no <button> may descend from another <button>').toBeNull();
    }
  });

  it('drag handle is keyboard-activatable: Enter and Space fire its click handler', () => {
    const clicks: string[] = [];
    render(baseChannel(), {
      onClick: () => clicks.push('row'),
    });
    const handle = container.querySelector('[data-sortable-handle]') as HTMLElement | null;
    expect(handle, 'sortable handle must be present for owner').not.toBeNull();
    // The handle must not be a <button> element itself (would re-introduce the
    // nested-button DOM warning) but it MUST expose role="button" and tabIndex
    // so it stays keyboard-reachable.
    expect(handle!.tagName.toLowerCase()).not.toBe('button');
    expect(handle!.getAttribute('role')).toBe('button');
    expect(handle!.tabIndex).toBeGreaterThanOrEqual(0);

    // The handle must catch Enter and Space and NOT bubble to the row's
    // onClick (which would open the channel — the wrong action for a drag
    // handle activation).
    const handleClicks: string[] = [];
    handle!.addEventListener('click', () => handleClicks.push('handle'));

    act(() => {
      handle!.dispatchEvent(
        new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true }),
      );
    });
    act(() => {
      handle!.dispatchEvent(
        new KeyboardEvent('keydown', { key: ' ', bubbles: true, cancelable: true }),
      );
    });

    // Enter + Space must each trigger a click on the handle.
    expect(handleClicks.length).toBeGreaterThanOrEqual(2);
    // Row onClick must NOT be reached via handle keyboard activation (the
    // handle's onClick stops propagation so the channel does not open).
    expect(clicks).not.toContain('row');
  });
});
