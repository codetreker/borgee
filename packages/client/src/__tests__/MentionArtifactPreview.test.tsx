// MentionArtifactPreview.test.tsx — CV-3.3 acceptance §2.6 vitest 锁.
//
// 锚: docs/qa/cv-3-content-lock.md §1 ⑥ + acceptance §2.6 + spec §0 设计 ③.
import React from 'react';
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { createRoot } from 'react-dom/client';
import { act } from 'react-dom/test-utils';
import MentionArtifactPreview from '../components/MentionArtifactPreview';

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
}

describe('MentionArtifactPreview — kind 三模式 byte-identical', () => {
  it('markdown: 头 80 字符 + ellipsis …', () => {
    const long = 'a'.repeat(120);
    render(<MentionArtifactPreview kind="markdown" title="t" body={long} />);
    const span = container!.querySelector('.artifact-preview') as HTMLElement;
    expect(span.getAttribute('data-artifact-kind')).toBe('markdown');
    expect(span.textContent!.length).toBe(81); // 80 + …
    expect(span.textContent!.endsWith('…')).toBe(true);
  });

  it('markdown: 短 body 不截断, 不加 …', () => {
    render(<MentionArtifactPreview kind="markdown" title="t" body="short" />);
    const span = container!.querySelector('.artifact-preview') as HTMLElement;
    expect(span.textContent).toBe('short');
  });

  it('code: 头 5 行 + 语言徽标 byte-identical 跟 §2.2 同源', () => {
    const body = Array.from({ length: 10 }, (_, i) => `line ${i + 1}`).join('\n');
    render(<MentionArtifactPreview kind="code" title="t" body={body} language="go" />);
    const span = container!.querySelector('.artifact-preview') as HTMLElement;
    expect(span.getAttribute('data-artifact-kind')).toBe('code');
    const code = span.querySelector('.artifact-preview-code') as HTMLElement;
    expect(code.textContent!.split('\n')).toHaveLength(5);
    const badge = span.querySelector('.code-lang-badge') as HTMLElement;
    expect(badge.getAttribute('data-lang')).toBe('go');
    expect(badge.textContent).toBe('GO');
  });

  it('image: 缩略图 max-width 192px byte-identical', () => {
    render(
      <MentionArtifactPreview
        kind="image_link"
        title="pic"
        body="https://example.com/a.png"
      />,
    );
    const span = container!.querySelector('.artifact-preview') as HTMLElement;
    expect(span.getAttribute('data-artifact-kind')).toBe('image_link');
    const img = span.querySelector('img') as HTMLImageElement;
    expect(img).toBeTruthy();
    expect(img.getAttribute('loading')).toBe('lazy');
    expect(img.style.maxWidth).toBe('192px');
  });

  it('image_link: 非 https URL → 不渲染 <img> (XSS 红线)', () => {
    render(<MentionArtifactPreview kind="image_link" title="t" body="javascript:alert(1)" />);
    expect(container!.querySelector('img')).toBeNull();
  });
});
