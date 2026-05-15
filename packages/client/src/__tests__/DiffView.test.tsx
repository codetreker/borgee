// DiffView.test.tsx — CV-4.3 acceptance §3.5 Vitest coverage.
//
// References: docs/qa/cv-4-content-lock.md §1 ⑤ + acceptance §3.5 + spec §0 design ③.
//
// Covers the jsdiff line-level pure function (computeDiffRows) and DOM literals:
// data-diff-line='add|del|context', a11y ARIA labels, and locked "对比" text.
import React from 'react';
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { createRoot } from 'react-dom/client';
import { act } from 'react';
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

describe('computeDiffRows — jsdiff line-level behavior (design ③ client jsdiff)', () => {
  it('identical → all context rows', () => {
    const rows = computeDiffRows('a\nb\nc', 'a\nb\nc');
    expect(rows.every((r) => r.kind === 'context')).toBe(true);
  });

  it('add only → includes an add row', () => {
    const rows = computeDiffRows('a\nb', 'a\nb\nc');
    expect(rows.some((r) => r.kind === 'add' && r.text === 'c')).toBe(true);
  });

  it('del only → includes a del row', () => {
    const rows = computeDiffRows('a\nb\nc', 'a\nb');
    expect(rows.some((r) => r.kind === 'del' && r.text === 'c')).toBe(true);
  });

  it('replace → includes del and add rows', () => {
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

  it('emits data-diff-line="add|del|context" plus ARIA labels', () => {
    render(<DiffView newBody="a\nB\nc" oldBody="a\nb\nc" newVersion={2} oldVersion={1} />);
    const adds = container!.querySelectorAll('[data-diff-line="add"]');
    const dels = container!.querySelectorAll('[data-diff-line="del"]');
    expect(adds.length + dels.length).toBeGreaterThan(0);
    // Each changed row has an aria-label so color is not the only signal.
    for (const el of adds) {
      expect(el.getAttribute('aria-label')).toBe('增行');
    }
    for (const el of dels) {
      expect(el.getAttribute('aria-label')).toBe('删行');
    }
  });

  it('image_link kind → renders side-by-side thumbnails because jsdiff does not apply', () => {
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
