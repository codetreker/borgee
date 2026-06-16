// SearchResultList.test.tsx — CV-6 client acceptance vitest 锁 (#cv-6).
//
// 锚: cv-6-content-lock.md §3 (单 row DOM 字面 byte-identical).
import React from 'react';
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { createRoot } from 'react-dom/client';
import { act } from 'react';
import SearchResultList from '../components/SearchResultList';
import type { SearchResult } from '../lib/api';

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

const baseResult: SearchResult = {
  artifact_id: 'art-1',
  title: 'Roadmap Q3',
  snippet: '<mark>Hello</mark> world plan',
  kind: 'markdown',
  channel_id: 'ch-A',
  current_version: 1,
};

describe('SearchResultList', () => {
  it('renders nothing when empty', () => {
    render(<SearchResultList results={[]} />);
    expect(container!.querySelector('ul')).toBeNull();
  });

  it('renders single row with locked DOM (data-* attrs + class names)', () => {
    render(<SearchResultList results={[baseResult]} />);
    const ul = container!.querySelector('ul.search-result-list[data-testid="artifact-search-results"]');
    expect(ul).toBeTruthy();
    const li = container!.querySelector('li[data-testid="search-result-row"]') as HTMLLIElement;
    expect(li).toBeTruthy();
    expect(li.getAttribute('data-artifact-id')).toBe('art-1');
    expect(li.getAttribute('data-artifact-kind')).toBe('markdown');
    expect(li.querySelector('.search-result-title')?.textContent).toBe('Roadmap Q3');
    const snippet = li.querySelector('.search-result-snippet') as HTMLDivElement;
    // server-side <mark>...</mark> 字面保留 (走 dangerouslySetInnerHTML).
    expect(snippet.innerHTML).toContain('<mark>Hello</mark>');
  });

  it('renders attacker snippet inert — no HTML executes, highlight preserved (XSS #1030)', () => {
    // Stored XSS vector: artifact body indexed verbatim by FTS5, wrapped in
    // literal <mark>/</mark> by snippet(). Server never HTML-escapes, so an
    // attacker body flows into the snippet. Walk-render must neutralize it.
    render(<SearchResultList results={[{
      ...baseResult,
      snippet: '<mark>kickoff</mark> <img src=x onerror="alert(1)"> <script>alert(2)</script>',
    }]} />);
    const snippet = container!.querySelector('.search-result-snippet') as HTMLDivElement;
    // No live HTML node may exist — attacker markup must be inert text.
    expect(snippet.querySelector('img')).toBeNull();
    expect(snippet.querySelector('script')).toBeNull();
    // The attacker markup appears verbatim as escaped, inert text.
    expect(snippet.textContent).toContain('<img src=x onerror="alert(1)">');
    expect(snippet.textContent).toContain('<script>alert(2)</script>');
    // Genuine server highlight is preserved as a real <mark> element.
    const marks = snippet.querySelectorAll('mark');
    expect(marks.length).toBe(1);
    expect(marks[0].textContent).toBe('kickoff');
  });

  it('renders multiple rows', () => {
    render(<SearchResultList results={[
      baseResult,
      { ...baseResult, artifact_id: 'art-2', kind: 'code', title: 'Snippet' },
    ]} />);
    const rows = container!.querySelectorAll('li[data-testid="search-result-row"]');
    expect(rows.length).toBe(2);
    expect(rows[1].getAttribute('data-artifact-kind')).toBe('code');
  });
});
