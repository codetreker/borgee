// ReadonlyBadge.test.tsx — CHN-15 acceptance §3.3 + content-lock §2.2.

import React from 'react';
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react';
import ReadonlyBadge from '../components/ReadonlyBadge';

let container: HTMLDivElement | null = null;
let root: Root | null = null;

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
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

function render(node: React.ReactElement) {
  act(() => {
    root!.render(node);
  });
}

describe('ReadonlyBadge', () => {
  it('renders "只读" + data-testid + aria-label when readonly=true', () => {
    render(<ReadonlyBadge readonly={true} />);
    const badge = container!.querySelector('[data-testid="readonly-badge"]');
    expect(badge).toBeTruthy();
    expect(badge?.textContent).toBe('只读');
    expect(badge?.getAttribute('aria-label')).toBe('只读频道');
  });

  it('returns null (no DOM) when readonly=false', () => {
    render(<ReadonlyBadge readonly={false} />);
    const badge = container!.querySelector('[data-testid="readonly-badge"]');
    expect(badge).toBeFalsy();
  });
});
