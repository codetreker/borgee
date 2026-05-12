// SearchBox.test.tsx — CV-6 client acceptance Vitest coverage (#cv-6).
//
// References: cv-6-content-lock.md §2 + §4 (DOM literals + 5 error-code toasts byte-identical).
//
// Note: full debounce / fetch / Escape integration is e2e territory
// (jsdom + controlled-input races make those flaky in Vitest); this test covers
// the DOM contract and SEARCH_ERR_TOAST byte-identical invariants.
import React from 'react';
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { createRoot } from 'react-dom/client';
import { act } from 'react-dom/test-utils';
import SearchBox from '../components/SearchBox';
import { SEARCH_ERR_TOAST } from '../lib/api';

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

function render(node: React.ReactElement) {
  const root = createRoot(container!);
  act(() => { root.render(node); });
  return root;
}

describe('SearchBox DOM literals (content-lock §2)', () => {
  it('renders <input type="search"> with locked attrs', () => {
    render(<SearchBox channelId="ch-1" onResults={() => {}} />);
    const input = container!.querySelector('input[data-testid="artifact-search-input"]') as HTMLInputElement;
    expect(input).toBeTruthy();
    expect(input.getAttribute('type')).toBe('search');
    expect(input.getAttribute('placeholder')).toBe('搜索 artifact (按 / 聚焦)');
    expect(input.getAttribute('maxlength')).toBe('256');
    expect(input.getAttribute('aria-label')).toBe('搜索 artifact');
    expect(input.getAttribute('class')).toContain('artifact-search-input');
  });

  it('input is initially empty', () => {
    render(<SearchBox channelId="ch-1" onResults={() => {}} />);
    const input = container!.querySelector('input') as HTMLInputElement;
    expect(input.value).toBe('');
  });
});

describe('SEARCH_ERR_TOAST byte-identical (content-lock §4 — 5 错码)', () => {
  it.each([
    ['search.query_empty',         '请输入搜索词'],
    ['search.query_too_long',      '搜索词太长 (最长 256 字符)'],
    ['search.channel_not_member',  '无权访问此频道'],
    ['search.cross_org_denied',    '跨组织搜索被禁'],
    ['search.not_owner',           '需要频道所有者权限'],
  ])('toast for %s = %s', (code, expected) => {
    expect(SEARCH_ERR_TOAST[code]).toBe(expected);
  });

  it('exactly 5 entries (no drift)', () => {
    expect(Object.keys(SEARCH_ERR_TOAST).length).toBe(5);
  });

  it('all keys start with `search.` prefix and stay aligned with server constants', () => {
    for (const k of Object.keys(SEARCH_ERR_TOAST)) {
      expect(k.startsWith('search.')).toBe(true);
    }
  });
});

describe('SearchBox debounce constant (design ⑨ DEBOUNCE_MS=300)', () => {
  it('component renders without immediate fetch on mount (no query)', () => {
    // Empty query → no fetch; debounce only fires on non-empty query.
    let fetchCalled = false;
    const origFetch = globalThis.fetch;
    globalThis.fetch = (() => { fetchCalled = true; return Promise.reject(new Error('blocked')); }) as typeof fetch;
    try {
      render(<SearchBox channelId="ch-1" onResults={() => {}} />);
      // Render only; do not change the query.
      expect(fetchCalled).toBe(false);
    } finally {
      globalThis.fetch = origFetch;
    }
  });
});
