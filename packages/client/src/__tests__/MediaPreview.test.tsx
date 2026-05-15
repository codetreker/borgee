// MediaPreview.test.tsx — CV-2 v2 acceptance vitest 锁 (#cv-2-v2).
//
// 引用: docs/implementation/modules/cv-2-v2-media-preview-spec.md §0 设计
//   ② HTML5 native + ③ kind enum 跟 CV-3 共 schema 单源 + content-lock
//   3 kind 分发 byte-identical.
import React from 'react';
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { createRoot } from 'react-dom/client';
import { act } from 'react';
import MediaPreview, { isPreviewableKind, PREVIEWABLE_KINDS } from '../components/MediaPreview';

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

describe('isPreviewableKind / PREVIEWABLE_KINDS — 设计 ③ (server byte-identical)', () => {
  it('PREVIEWABLE_KINDS is the 3-tuple [image_link, video_link, pdf_link]', () => {
    // Byte-identical 跟 server preview.go::PreviewableKinds.
    expect([...PREVIEWABLE_KINDS]).toEqual(['image_link', 'video_link', 'pdf_link']);
  });

  it.each(['image_link', 'video_link', 'pdf_link'])('accepts %s', (k) => {
    expect(isPreviewableKind(k)).toBe(true);
  });

  it.each(['markdown', 'code', '', 'pdf', 'video', 'kanban', 'mindmap'])(
    'rejects %s (CV-1/CV-3 既有 path 不走 MediaPreview)',
    (k) => {
      expect(isPreviewableKind(k)).toBe(false);
    },
  );
});

describe('MediaPreview image_link 分支 (设计 ①)', () => {
  it('renders <img> with src=preview_url when previewUrl set (thumbnail-first)', () => {
    render(
      <MediaPreview
        kind="image_link"
        body="https://cdn.example/full.png"
        previewUrl="https://cdn.example/thumb.jpg"
        title="Hero"
      />,
    );
    const img = container!.querySelector('img.media-preview-image') as HTMLImageElement;
    expect(img).toBeTruthy();
    expect(img.getAttribute('src')).toBe('https://cdn.example/thumb.jpg');
    expect(img.getAttribute('loading')).toBe('lazy');
    expect(img.getAttribute('alt')).toBe('Hero');
    expect(img.getAttribute('data-media-kind')).toBe('image_link');
  });

  it('falls back to body URL when previewUrl absent', () => {
    render(<MediaPreview kind="image_link" body="https://cdn.example/x.png" title="X" />);
    const img = container!.querySelector('img.media-preview-image') as HTMLImageElement;
    expect(img.getAttribute('src')).toBe('https://cdn.example/x.png');
  });
});

describe('MediaPreview video_link 分支 (设计 ②)', () => {
  it('renders <video controls preload="metadata"> with HTML5 native', () => {
    render(<MediaPreview kind="video_link" body="https://cdn.example/clip.mp4" title="Clip" />);
    const video = container!.querySelector('video.media-preview-video') as HTMLVideoElement;
    expect(video).toBeTruthy();
    expect(video.getAttribute('src')).toBe('https://cdn.example/clip.mp4');
    expect(video.hasAttribute('controls')).toBe(true);
    expect(video.getAttribute('preload')).toBe('metadata');
    expect(video.getAttribute('data-media-kind')).toBe('video_link');
    expect(video.getAttribute('aria-label')).toBe('Clip');
  });

  it('passes preview_url as poster when set', () => {
    render(
      <MediaPreview
        kind="video_link"
        body="https://cdn.example/clip.mp4"
        previewUrl="https://cdn.example/clip-poster.jpg"
        title="Clip"
      />,
    );
    const video = container!.querySelector('video') as HTMLVideoElement;
    expect(video.getAttribute('poster')).toBe('https://cdn.example/clip-poster.jpg');
  });
});

describe('MediaPreview pdf_link 分支 (设计 ②)', () => {
  it('renders <embed type="application/pdf"> 浏览器内嵌', () => {
    render(<MediaPreview kind="pdf_link" body="https://cdn.example/doc.pdf" title="Spec" />);
    const embed = container!.querySelector('embed.media-preview-pdf') as HTMLEmbedElement;
    expect(embed).toBeTruthy();
    expect(embed.getAttribute('src')).toBe('https://cdn.example/doc.pdf');
    expect(embed.getAttribute('type')).toBe('application/pdf');
    expect(embed.getAttribute('data-media-kind')).toBe('pdf_link');
  });
});

describe('MediaPreview XSS 红线 #1 — https only (设计 ② 反约束)', () => {
  it.each([
    'http://cdn.example/x.png',
    'javascript:alert(1)',
    'data:image/png;base64,AAA',
    'file:///etc/passwd',
    '//cdn.example/x.png',
    '',
  ])('rejects unsafe scheme %s — falls back to invalid placeholder', (raw) => {
    render(<MediaPreview kind="image_link" body={raw} title="Bad" />);
    expect(container!.querySelector('img')).toBeNull();
    expect(container!.querySelector('video')).toBeNull();
    expect(container!.querySelector('embed')).toBeNull();
    const fallback = container!.querySelector('.media-preview-invalid');
    expect(fallback).toBeTruthy();
  });

  it('rejects non-https previewUrl — falls back to body URL on image_link', () => {
    render(
      <MediaPreview
        kind="image_link"
        body="https://cdn.example/x.png"
        previewUrl="javascript:alert(1)"
        title="X"
      />,
    );
    const img = container!.querySelector('img') as HTMLImageElement;
    // Unsafe previewUrl ignored; falls back to body.
    expect(img.getAttribute('src')).toBe('https://cdn.example/x.png');
  });
});

describe('MediaPreview kind 闸 — 其它 kind null (设计 ③)', () => {
  it.each(['markdown', 'code', 'unknown', ''])('returns null for kind=%s', (k) => {
    render(<MediaPreview kind={k} body="https://cdn.example/x" title="X" />);
    expect(container!.children.length === 0 || container!.firstChild === null).toBe(true);
  });
});
