// DiffView.test.tsx — CV-4.3 acceptance §3.5 vitest 锁.
//
// 锚: docs/qa/cv-4-content-lock.md §1 ⑤ + acceptance §3.5 + spec §0 设计 ③.
//
// jsdiff 行级 diff 纯函数测 (computeDiffRows) + DOM 字面验
// (data-diff-line='add|del|context' a11y ARIA + "对比" 文案锁).
import React from 'react';
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { createRoot } from 'react-dom/client';
import { act } from 'react-dom/test-utils';
import DiffView, {
  computeDiffRows,
  parseDiffParam,
  formatDiffParam,
} from '../components/DiffView';

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

describe('computeDiffRows — jsdiff 行级 (设计 ③ client jsdiff)', () => {
  it('identical → all context rows', () => {
    const rows = computeDiffRows('a\nb\nc', 'a\nb\nc');
    expect(rows.every((r) => r.kind === 'context')).toBe(true);
  });

  it('add only → 含 add row', () => {
    const rows = computeDiffRows('a\nb', 'a\nb\nc');
    expect(rows.some((r) => r.kind === 'add' && r.text === 'c')).toBe(true);
  });

  it('del only → 含 del row', () => {
    const rows = computeDiffRows('a\nb\nc', 'a\nb');
    expect(rows.some((r) => r.kind === 'del' && r.text === 'c')).toBe(true);
  });

  it('replace → del + add 两 row', () => {
    const rows = computeDiffRows('a\nb\nc', 'a\nB\nc');
    expect(rows.some((r) => r.kind === 'del' && r.text === 'b')).toBe(true);
    expect(rows.some((r) => r.kind === 'add' && r.text === 'B')).toBe(true);
  });
});

describe('parseDiffParam / formatDiffParam — deep-link byte-identical', () => {
  it('roundtrip vN..vM', () => {
    expect(formatDiffParam(3, 2)).toBe('v3..v2');
    expect(parseDiffParam('v3..v2')).toEqual({ newV: 3, oldV: 2 });
  });

  it.each([null, '', 'v3', 'v3..', '3..2', 'va..vb'])(
    'rejects malformed: %s',
    (raw) => {
      expect(parseDiffParam(raw as string | null)).toBeNull();
    },
  );
});

describe('DiffView DOM byte-identical (acceptance §3.5)', () => {
  it('renders title "v{N} ↔ v{M}" with arrow ↔ byte-identical', () => {
    render(<DiffView newBody="a" oldBody="b" newVersion={3} oldVersion={2} />);
    const title = container!.querySelector('.diff-title');
    expect(title!.textContent).toBe('v3 ↔ v2');
  });

  it('emits data-diff-line="add|del|context" ARIA replacement (a11y 反约束)', () => {
    render(<DiffView newBody="a\nB\nc" oldBody="a\nb\nc" newVersion={2} oldVersion={1} />);
    const adds = container!.querySelectorAll('[data-diff-line="add"]');
    const dels = container!.querySelectorAll('[data-diff-line="del"]');
    expect(adds.length + dels.length).toBeGreaterThan(0);
    // 反向断言 — 每行有 aria-label (色盲反约束).
    for (const el of adds) {
      expect(el.getAttribute('aria-label')).toBe('增行');
    }
    for (const el of dels) {
      expect(el.getAttribute('aria-label')).toBe('删行');
    }
  });

  it('image_link kind → 前后缩略图并排 fallback (jsdiff 不适用)', () => {
    render(
      <DiffView
        newBody="https://example.com/new.png"
        oldBody="https://example.com/old.png"
        newVersion={2}
        oldVersion={1}
        kind="image_link"
      />,
    );
    expect(container!.querySelector('.diff-view-fallback')).toBeTruthy();
    const imgs = container!.querySelectorAll('img.artifact-image');
    expect(imgs).toHaveLength(2);
    for (const img of imgs) {
      expect(img.getAttribute('loading')).toBe('lazy');
    }
  });
});
