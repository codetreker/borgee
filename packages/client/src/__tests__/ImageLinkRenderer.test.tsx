// ImageLinkRenderer.test.tsx — CV-3.3 acceptance §2.4 §2.5 vitest 锁.
//
// 锚: docs/qa/cv-3-content-lock.md §1 ④⑤ + acceptance §2.4 §2.5 +
//     spec §0 设计 ① + ④ XSS 红线第一道 (https only) + ⑤ 第二道
//     (rel="noopener noreferrer" strictly assert byte-identical).
import React from 'react';
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { createRoot } from 'react-dom/client';
import { act } from 'react-dom/test-utils';
import ImageLinkRenderer, { isHttpsURL } from '../components/ImageLinkRenderer';

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
  act(() => {
    root.render(node);
  });
  return root;
}

describe('isHttpsURL — XSS 红线第一道 (反约束 §2.4)', () => {
  it('accepts https URLs', () => {
    expect(isHttpsURL('https://example.com/a.png')).toBe(true);
    expect(isHttpsURL('https://x.example.test/path?q=1')).toBe(true);
    // case-insensitive scheme — allow per RFC 3986.
    expect(isHttpsURL('HTTPS://example.com/')).toBe(true);
  });

  it.each([
    'http://example.com/',
    'javascript:alert(1)',
    'data:image/png;base64,AAAA',
    'data:text/html,<script>',
    'file:///etc/passwd',
    '//example.com/scheme-relative',
    '',
    'not a url',
  ])('rejects unsafe scheme: %s', (raw) => {
    expect(isHttpsURL(raw)).toBe(false);
  });
});

describe('ImageLinkRenderer image branch (设计 ④)', () => {
  it('renders <img loading="lazy" class="artifact-image" src=https>', () => {
    render(<ImageLinkRenderer body="https://example.com/a.png" title="Hi" subKind="image" />);
    const img = container!.querySelector('img.artifact-image') as HTMLImageElement;
    expect(img).toBeTruthy();
    expect(img.getAttribute('loading')).toBe('lazy');
    expect(img.getAttribute('src')).toBe('https://example.com/a.png');
    expect(img.getAttribute('alt')).toBe('Hi');
  });

  it('rejects non-https — does NOT emit <img> with unsafe src', () => {
    render(<ImageLinkRenderer body="javascript:alert(1)" title="x" subKind="image" />);
    expect(container!.querySelector('img')).toBeNull();
    expect(container!.querySelector('.artifact-image-link-invalid')).toBeTruthy();
  });

  it('rejects http: + data:image — XSS 红线第一道', () => {
    for (const bad of ['http://example.com/x.png', 'data:image/png;base64,AAA']) {
      render(<ImageLinkRenderer body={bad} title="x" subKind="image" />);
      expect(container!.querySelector('img')).toBeNull();
    }
  });
});

describe('ImageLinkRenderer link branch (设计 ⑤ — XSS 红线第二道)', () => {
  it('STRICTLY ASSERTS rel="noopener noreferrer" 字串原样 byte-identical', () => {
    render(<ImageLinkRenderer body="https://example.com/" title="Click me" subKind="link" />);
    const a = container!.querySelector('a.artifact-link') as HTMLAnchorElement;
    expect(a).toBeTruthy();
    // 字串原样, 不 includes — 漏 = reverse-tab XSS leak.
    const rel = a.getAttribute('rel');
    expect(rel).toBe('noopener noreferrer');
  });

  it('target="_blank" byte-identical (反 _self SPA 跳走)', () => {
    render(<ImageLinkRenderer body="https://example.com/" title="x" subKind="link" />);
    const a = container!.querySelector('a.artifact-link') as HTMLAnchorElement;
    expect(a.getAttribute('target')).toBe('_blank');
  });

  it('href is the trimmed https URL', () => {
    render(<ImageLinkRenderer body="  https://example.com/path  " title="x" subKind="link" />);
    const a = container!.querySelector('a.artifact-link') as HTMLAnchorElement;
    expect(a.getAttribute('href')).toBe('https://example.com/path');
  });

  it('does not render <img> on link branch (kind 二元拆死)', () => {
    render(<ImageLinkRenderer body="https://example.com/" title="x" subKind="link" />);
    expect(container!.querySelector('img')).toBeNull();
  });

  it('rejects non-https on link branch too', () => {
    render(<ImageLinkRenderer body="javascript:alert(1)" title="x" subKind="link" />);
    expect(container!.querySelector('a')).toBeNull();
  });
});
