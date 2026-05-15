// ArtifactCommentBody.test.tsx — CV-11.2 vitest acceptance.
//
// 锚: docs/qa/cv-11-stance-checklist.md §4 + content-lock §1+§2.
// 4 case (cv-11.md §2):
//   1. renders basic markdown (bold/italic/code/list)
//   2. sanitize XSS — script/iframe/onerror= 全删 (3 sub-case)
//   3. mention render — <@uuid> 复用 renderMarkdown mention 路径
//   4. empty body fallback

import React from 'react';
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react';
import ArtifactCommentBody from '../components/ArtifactCommentBody';

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
});

async function render(node: React.ReactElement) {
  root = createRoot(container!);
  await act(async () => {
    root!.render(node);
  });
}

describe('ArtifactCommentBody — CV-11.2 client', () => {
  it('设计 ④ DOM data-cv11-comment-body anchor + renders basic markdown', async () => {
    await render(<ArtifactCommentBody body="**bold** and *italic* with `code`" />);
    const root = container!.querySelector('[data-cv11-comment-body]');
    expect(root).not.toBeNull();
    expect(root!.querySelector('strong')?.textContent).toBe('bold');
    expect(root!.querySelector('em')?.textContent).toBe('italic');
    expect(root!.querySelector('code')?.textContent).toBe('code');
  });

  it('设计 ④ sanitize XSS — <script>alert(1)</script> 全删, 文本保留', async () => {
    await render(<ArtifactCommentBody body="<script>alert(1)</script>hello world" />);
    const scripts = container!.querySelectorAll('script');
    expect(scripts.length).toBe(0);
    // 文本部分应保留 (DOMPurify 删 script 元素但保 text node 视情况而定;
    // 至少 'hello world' 应作为可见文本存在).
    expect(container!.textContent).toContain('hello world');
  });

  it('设计 ④ sanitize — <iframe> 0 + onerror= 0', async () => {
    await render(<ArtifactCommentBody body='<iframe src="//evil"></iframe><img src=x onerror="alert(1)">' />);
    expect(container!.querySelectorAll('iframe').length).toBe(0);
    // img is sanitized but onerror attribute MUST be removed.
    const imgs = container!.querySelectorAll('img');
    imgs.forEach((img) => {
      expect(img.getAttribute('onerror')).toBeNull();
    });
  });

  it('设计 ① mention 复用 renderMarkdown 既有 path — `<@uuid>` 渲染', async () => {
    const uuid = '11111111-2222-3333-4444-555555555555';
    const userMap = new Map([[uuid, 'Alice']]);
    await render(
      <ArtifactCommentBody
        body={`hello <@${uuid}> there`}
        mentionedUserIds={[uuid]}
        userMap={userMap}
      />,
    );
    // DM-2.3 mention pattern renders a span with data-mention-id.
    const mention = container!.querySelector('[data-mention-id]');
    expect(mention).not.toBeNull();
    // 设计 ④ root anchor 仍存在.
    expect(container!.querySelector('[data-cv11-comment-body]')).not.toBeNull();
  });

  it('设计 ④ empty body fallback — 渲染 placeholder + data-empty 锚', async () => {
    await render(<ArtifactCommentBody body="   " />);
    const empty = container!.querySelector('[data-cv11-comment-body][data-empty]');
    expect(empty).not.toBeNull();
  });
});
