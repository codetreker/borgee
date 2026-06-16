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

import MessageItem from '../components/MessageItem';
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
