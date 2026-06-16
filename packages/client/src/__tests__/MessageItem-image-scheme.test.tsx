// MessageItem-image-scheme.test.tsx — borgee #1108 F5
//
// content_type=image must only render an <img src>/<a href> for URLs that
// pass the allowlist (http(s):// or a single-leading-slash same-origin path).
// javascript:/data:/protocol-relative content is rendered as inert plain text
// (no <a href>, no <img src>) so a stored row can't become a phishing /
// script-anchor vector. Valid URLs / relative paths still render (no
// regression).
import React from 'react';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react';

vi.mock('@emoji-mart/react', () => ({
  default: () => React.createElement('div', { 'data-test': 'emoji-mart' }),
}));
vi.mock('@emoji-mart/data', () => ({ default: {} }));
vi.mock('../lib/api', async () => {
  const actual = await vi.importActual<typeof import('../lib/api')>('../lib/api');
  return {
    ...actual,
    addReaction: vi.fn().mockResolvedValue(undefined),
    removeReaction: vi.fn().mockResolvedValue(undefined),
    editMessage: vi.fn().mockResolvedValue({ content: '', edited_at: 0 }),
    deleteMessage: vi.fn().mockResolvedValue(undefined),
  };
});
vi.mock('../context/AppContext', () => ({
  useAppContext: () => ({ dispatch: vi.fn() }),
}));
vi.mock('../components/Toast', () => ({
  useToast: () => ({ showToast: vi.fn() }),
}));
vi.mock('../hooks/useCommandTracking', () => ({
  trackCommand: vi.fn(),
  getCommandStatus: vi.fn().mockReturnValue(null),
}));
vi.mock('../hooks/useLongPress', () => ({
  useLongPress: () => ({}),
}));

// Drive the edit-save guard without the real tiptap editor: replace EditEditor
// with a stub that fires onSave with whatever content the test stashed. The
// stub also exposes onCancel so the component's cancel path still works.
let pendingEditContent = '';
vi.mock('../components/EditEditor', () => ({
  default: ({ onSave }: { onSave: (c: string) => void }) =>
    React.createElement(
      'button',
      {
        'data-test': 'edit-save',
        onClick: () => onSave(pendingEditContent),
      },
      'save',
    ),
}));

import MessageItem, { isAllowedImageContentURL } from '../components/MessageItem';
import * as api from '../lib/api';
import type { Message } from '../types';

let container: HTMLDivElement | null = null;
let root: Root | null = null;

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
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

function makeImageMsg(content: string): Message {
  return {
    id: 'm-img',
    channel_id: 'ch-1',
    sender_id: 'u-other',
    content,
    content_type: 'image',
    created_at: 1700000000000,
    reply_to_id: null,
    edited_at: null,
    reactions: [],
  } as Message;
}

function render(message: Message) {
  root = createRoot(container!);
  act(() => {
    root!.render(
      <MessageItem
        message={message}
        userMap={new Map([['u-current', 'Me'], ['u-other', 'Other']])}
        members={[]}
        memberMap={new Map()}
        currentUserId="u-current"
        currentUserRole="member"
      />,
    );
  });
}

function hasAnchorOrImgWith(value: string): boolean {
  const anchor = Array.from(container!.querySelectorAll('a[href]')).some(
    (a) => (a as HTMLAnchorElement).getAttribute('href') === value,
  );
  const img = Array.from(container!.querySelectorAll('img[src]')).some(
    (i) => (i as HTMLImageElement).getAttribute('src') === value,
  );
  return anchor || img;
}

describe('MessageItem ImageContent scheme guard — borgee #1108 F5', () => {
  it('javascript: scheme → inert plain text, no <a href> / <img src>', () => {
    const url = 'javascript:alert(1)';
    render(makeImageMsg(url));
    expect(hasAnchorOrImgWith(url)).toBe(false);
    // shown as inert text
    expect(container!.textContent).toContain(url);
  });

  it('data: scheme → inert plain text, no <a href> / <img src>', () => {
    const url = 'data:text/html,<script>alert(1)</script>';
    render(makeImageMsg(url));
    expect(hasAnchorOrImgWith(url)).toBe(false);
  });

  it('protocol-relative //host → inert plain text, no <a href> / <img src>', () => {
    const url = '//evil.com/x.png';
    render(makeImageMsg(url));
    expect(hasAnchorOrImgWith(url)).toBe(false);
  });

  it('https URL → renders <img> wrapped in <a> (no regression)', () => {
    const url = 'https://example.com/x.png';
    render(makeImageMsg(url));
    const img = container!.querySelector('img.message-image') as HTMLImageElement | null;
    expect(img).not.toBeNull();
    expect(img!.getAttribute('src')).toBe(url);
    const anchor = container!.querySelector('a[href]') as HTMLAnchorElement | null;
    expect(anchor).not.toBeNull();
    expect(anchor!.getAttribute('href')).toBe(url);
    expect(anchor!.getAttribute('rel')).toContain('noopener');
  });

  it('http URL (case-insensitive) → renders <img>', () => {
    const url = 'HTTP://example.com/x.png';
    render(makeImageMsg(url));
    const img = container!.querySelector('img.message-image') as HTMLImageElement | null;
    expect(img).not.toBeNull();
    expect(img!.getAttribute('src')).toBe(url);
  });

  it('same-origin relative path /api/uploads/x.png → renders <img>', () => {
    const url = '/api/uploads/x.png';
    render(makeImageMsg(url));
    const img = container!.querySelector('img.message-image') as HTMLImageElement | null;
    expect(img).not.toBeNull();
    expect(img!.getAttribute('src')).toBe(url);
  });
});

// borgee #1108 F5 (edit rail): editing an image message to a banned scheme
// must be rejected client-side (no PUT) and surface an error, mirroring the
// server's 400 INVALID_CONTENT. Valid edits still go through.
function makeOwnImageMsg(content: string): Message {
  return {
    id: 'm-own-img',
    channel_id: 'ch-1',
    sender_id: 'u-current',
    content,
    content_type: 'image',
    created_at: 1700000000000,
    reply_to_id: null,
    edited_at: null,
    reactions: [],
  } as Message;
}

function startEditAndSave(message: Message, newContent: string) {
  render(message);
  // Click the ✏️ edit action to enter edit mode.
  const editBtn = Array.from(container!.querySelectorAll('button')).find(
    (b) => b.getAttribute('title') === '编辑',
  ) as HTMLButtonElement | undefined;
  if (!editBtn) throw new Error('edit button not found');
  act(() => { editBtn.click(); });
  // Stash the new content the stubbed EditEditor will pass to onSave, then save.
  pendingEditContent = newContent;
  const saveBtn = container!.querySelector('[data-test="edit-save"]') as HTMLButtonElement | null;
  if (!saveBtn) throw new Error('stub save button not found');
  act(() => { saveBtn.click(); });
}

describe('MessageItem edit-save image scheme guard — borgee #1108 F5', () => {
  beforeEach(() => {
    vi.mocked(api.editMessage).mockClear();
    vi.mocked(api.editMessage).mockResolvedValue({ content: '', edited_at: 0 } as never);
  });

  it('exported allowlist matches server: rejects javascript:/data:/protocol-relative, accepts http(s)/relative', () => {
    expect(isAllowedImageContentURL('javascript:alert(1)')).toBe(false);
    expect(isAllowedImageContentURL('data:text/html,<script>')).toBe(false);
    expect(isAllowedImageContentURL('//evil.com/x.png')).toBe(false);
    expect(isAllowedImageContentURL('https://example.com/x.png')).toBe(true);
    expect(isAllowedImageContentURL('HTTP://example.com/x.png')).toBe(true);
    expect(isAllowedImageContentURL('/api/uploads/x.png')).toBe(true);
    expect(isAllowedImageContentURL('')).toBe(false);
  });

  it('editing an image to javascript: → no PUT, error surfaced', () => {
    startEditAndSave(makeOwnImageMsg('https://example.com/old.png'), 'javascript:alert(1)');
    expect(vi.mocked(api.editMessage)).not.toHaveBeenCalled();
    const err = container!.querySelector('.message-edit-error');
    expect(err).not.toBeNull();
  });

  it('editing an image to protocol-relative //host → no PUT, error surfaced', () => {
    startEditAndSave(makeOwnImageMsg('https://example.com/old.png'), '//evil.com/x.png');
    expect(vi.mocked(api.editMessage)).not.toHaveBeenCalled();
    expect(container!.querySelector('.message-edit-error')).not.toBeNull();
  });

  it('editing an image to a valid https URL → PUT sent (no regression)', () => {
    startEditAndSave(makeOwnImageMsg('https://example.com/old.png'), 'https://cdn.example.com/new.png');
    expect(vi.mocked(api.editMessage)).toHaveBeenCalledWith('m-own-img', 'https://cdn.example.com/new.png');
  });
});
