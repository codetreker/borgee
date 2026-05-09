// ArtifactThumbnail.test.tsx — CV-3 v2 acceptance vitest 锁定 (#cv-3-v2).
//
// 出处: docs/implementation/modules/cv-3-v2-spec.md §0 设计 ① 服务端 thumbnail
// 不 inline + ② https only + ③ 二端互斥 跟 PreviewableKinds.
import React from 'react';
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { createRoot } from 'react-dom/client';
import { act } from 'react-dom/test-utils';
import ArtifactThumbnail, {
  isThumbnailableKind,
  THUMBNAILABLE_KINDS,
} from '../components/ArtifactThumbnail';

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

describe('THUMBNAILABLE_KINDS — 设计 ③ (server byte-identical)', () => {
  it('THUMBNAILABLE_KINDS is the 2-tuple [markdown, code]', () => {
    expect([...THUMBNAILABLE_KINDS]).toEqual(['markdown', 'code']);
  });

  it.each(['markdown', 'code'])('accepts %s', (k) => {
    expect(isThumbnailableKind(k)).toBe(true);
  });

  it.each(['image_link', 'video_link', 'pdf_link', 'unknown', ''])(
    'rejects %s (走 MediaPreview / CV-2 v2 既有 path)',
    (k) => {
      expect(isThumbnailableKind(k)).toBe(false);
    },
  );
});

describe('ArtifactThumbnail — img 渲染 (设计 ①)', () => {
  it('renders <img loading="lazy"> 256x256 with thumbnailUrl src for markdown', () => {
    render(
      <ArtifactThumbnail
        kind="markdown"
        title="Roadmap"
        thumbnailUrl="https://cdn.example/thumb.png"
      />,
    );
    const img = container!.querySelector('img.artifact-thumbnail') as HTMLImageElement;
    expect(img).toBeTruthy();
    expect(img.getAttribute('src')).toBe('https://cdn.example/thumb.png');
    expect(img.getAttribute('loading')).toBe('lazy');
    expect(img.getAttribute('alt')).toBe('Roadmap');
    expect(img.getAttribute('data-thumbnail-kind')).toBe('markdown');
    expect(img.getAttribute('width')).toBe('256');
    expect(img.getAttribute('height')).toBe('256');
  });

  it('renders <img> for code kind', () => {
    render(
      <ArtifactThumbnail
        kind="code"
        title="Snippet"
        thumbnailUrl="https://cdn.example/snippet.png"
      />,
    );
    const img = container!.querySelector('img') as HTMLImageElement;
    expect(img.getAttribute('data-thumbnail-kind')).toBe('code');
  });
});

describe('ArtifactThumbnail — fallback div (设计 ① no thumbnailUrl)', () => {
  it('renders fallback div with markdown icon when thumbnailUrl absent', () => {
    render(<ArtifactThumbnail kind="markdown" title="No thumb" />);
    const fb = container!.querySelector('.artifact-thumbnail-fallback') as HTMLDivElement;
    expect(fb).toBeTruthy();
    expect(fb.getAttribute('data-thumbnail-kind')).toBe('markdown');
    expect(fb.getAttribute('aria-label')).toBe('No thumb');
    const icon = fb.querySelector('.artifact-thumbnail-icon');
    expect(icon?.textContent).toBe('📝');
    expect(container!.querySelector('img')).toBeNull();
  });

  it('renders fallback div with code icon when thumbnailUrl absent', () => {
    render(<ArtifactThumbnail kind="code" title="No thumb code" />);
    const icon = container!.querySelector('.artifact-thumbnail-icon');
    expect(icon?.textContent).toBe('💻');
  });
});

describe('ArtifactThumbnail XSS 红线 #1 — https only (设计 ② 反向约束)', () => {
  it.each([
    'http://cdn.example/x.png',
    'javascript:alert(1)',
    'data:image/png;base64,AAA',
    'file:///etc/passwd',
    '//cdn.example/x.png',
    '',
  ])('rejects unsafe scheme %s — falls back to icon, not <img>', (raw) => {
    render(<ArtifactThumbnail kind="markdown" title="Bad" thumbnailUrl={raw} />);
    expect(container!.querySelector('img')).toBeNull();
    expect(container!.querySelector('.artifact-thumbnail-fallback')).toBeTruthy();
  });
});

describe('ArtifactThumbnail kind 闸 — 其它 kind null (设计 ③)', () => {
  it.each(['image_link', 'video_link', 'pdf_link', 'unknown', ''])(
    'returns null for kind=%s (走 MediaPreview)',
    (k) => {
      render(<ArtifactThumbnail kind={k} title="X" thumbnailUrl="https://cdn.example/x.png" />);
      expect(container!.children.length === 0 || container!.firstChild === null).toBe(true);
    },
  );
});
