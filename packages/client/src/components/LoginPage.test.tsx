// LoginPage.test.tsx — bf task `form-labels-a11y` AC-2.
//
// Pins LoginPage's input set by name and asserts each pinned input is
// associated with a <label htmlFor> element (or wrapped by a <label>).
// WCAG 1.3.1 (Info and Relationships) + 3.3.2 (Labels or Instructions).

import React from 'react';
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react';
import LoginPage from '../components/LoginPage';

const PINNED_INPUTS = ['email', 'password'] as const;

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

function findLabelFor(root: HTMLElement, id: string): HTMLLabelElement | null {
  // Explicit association: <label for="id">
  const explicit = root.querySelector(`label[for="${id}"]`);
  if (explicit) return explicit as HTMLLabelElement;
  // Implicit association: input nested inside <label>
  const input = root.querySelector(`#${id}`);
  if (input) {
    let p: HTMLElement | null = input.parentElement;
    while (p) {
      if (p.tagName === 'LABEL') return p as HTMLLabelElement;
      p = p.parentElement;
    }
  }
  return null;
}

describe('LoginPage — a11y: every input has an associated <label> (bf form-labels-a11y AC-2)', () => {
  it('renders every pinned input (fail loud if form was conditionally removed)', () => {
    render(<LoginPage onLogin={() => {}} onRegister={() => {}} />);
    const inputs = Array.from(container!.querySelectorAll('input')) as HTMLInputElement[];
    const renderedIds = inputs.map(i => i.id).filter(Boolean);
    for (const name of PINNED_INPUTS) {
      expect(
        renderedIds,
        `LoginPage must render an <input id="${name}">; got ids=${JSON.stringify(renderedIds)}`,
      ).toContain(name);
    }
  });

  for (const name of PINNED_INPUTS) {
    it(`input "${name}" has an associated <label> (htmlFor or wrapping)`, () => {
      render(<LoginPage onLogin={() => {}} onRegister={() => {}} />);
      const input = container!.querySelector(`#${name}`);
      expect(input, `LoginPage must render <input id="${name}">`).toBeTruthy();
      const label = findLabelFor(container!, name);
      expect(
        label,
        `LoginPage input "${name}" must have an associated <label htmlFor="${name}"> (or wrapping <label>)`,
      ).toBeTruthy();
      // Label must carry non-empty accessible text.
      const text = (label?.textContent ?? '').trim();
      expect(text.length, `Label for "${name}" must have non-empty text`).toBeGreaterThan(0);
    });
  }
});
