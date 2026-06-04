// RegisterPage.test.tsx
//
// Combines two bf task suites that both pin RegisterPage behavior:
//
//   - `form-labels-a11y` AC-2: every pinned input is associated with a
//     <label htmlFor> element (or wrapped by a <label>). WCAG 1.3.1 + 3.3.2.
//
//   - `submit-button-state`: the submit button's `disabled` prop is wired to
//     the same client-side validation predicate used at click time. Four
//     locked transitions:
//       (1) fresh mount with empty required fields → disabled
//       (2) all required fields filled with valid values → enabled (same render)
//       (3) one field edited to an invalid value → disabled (same render)
//       (4) the offending field fixed back to valid → re-enabled (same render)
//     Pre-fix the submit button only checks `!inviteCode || !email ||
//     !password || !displayName`, so test (3) fails (weak password leaves
//     button enabled).

import React from 'react';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react';
import RegisterPage from './RegisterPage';

const PINNED_INPUTS = ['inviteCode', 'displayName', 'email', 'password'] as const;

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
  root = null;
  vi.restoreAllMocks();
});

function render(node: React.ReactElement) {
  act(() => {
    root!.render(node);
  });
}

function findLabelFor(root: HTMLElement, id: string): HTMLLabelElement | null {
  const explicit = root.querySelector(`label[for="${id}"]`);
  if (explicit) return explicit as HTMLLabelElement;
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

describe('RegisterPage — a11y: every input has an associated <label> (bf form-labels-a11y AC-2)', () => {
  it('renders every pinned input (fail loud if form was conditionally removed)', () => {
    render(<RegisterPage onLogin={() => {}} onBack={() => {}} />);
    const inputs = Array.from(container!.querySelectorAll('input')) as HTMLInputElement[];
    const renderedIds = inputs.map(i => i.id).filter(Boolean);
    for (const name of PINNED_INPUTS) {
      expect(
        renderedIds,
        `RegisterPage must render an <input id="${name}">; got ids=${JSON.stringify(renderedIds)}`,
      ).toContain(name);
    }
  });

  for (const name of PINNED_INPUTS) {
    it(`input "${name}" has an associated <label> (htmlFor or wrapping)`, () => {
      render(<RegisterPage onLogin={() => {}} onBack={() => {}} />);
      const input = container!.querySelector(`#${name}`);
      expect(input, `RegisterPage must render <input id="${name}">`).toBeTruthy();
      const label = findLabelFor(container!, name);
      expect(
        label,
        `RegisterPage input "${name}" must have an associated <label htmlFor="${name}"> (or wrapping <label>)`,
      ).toBeTruthy();
      const text = (label?.textContent ?? '').trim();
      expect(text.length, `Label for "${name}" must have non-empty text`).toBeGreaterThan(0);
    });
  }
});

function getSubmitButton(): HTMLButtonElement {
  const btn = container!.querySelector('button[type="submit"]') as HTMLButtonElement;
  if (!btn) throw new Error('submit button not found');
  return btn;
}

// React's onChange wraps the native `input` event. Setting `.value` then
// dispatching a real `input` event is the canonical way to feed a controlled
// input from a test without a fake event object.
function setInputByPlaceholder(placeholder: string, value: string) {
  const input = container!.querySelector(
    `input[placeholder="${placeholder}"]`,
  ) as HTMLInputElement;
  if (!input) throw new Error(`input[placeholder=${placeholder}] not found`);
  const setter = Object.getOwnPropertyDescriptor(
    window.HTMLInputElement.prototype,
    'value',
  )!.set!;
  act(() => {
    setter.call(input, value);
    input.dispatchEvent(new Event('input', { bubbles: true }));
  });
}

function renderPage() {
  act(() => {
    root!.render(<RegisterPage onLogin={() => {}} onBack={() => {}} />);
  });
}

describe('RegisterPage submit button — bf submit-button-state', () => {
  it('(1) fresh mount with empty required fields → disabled', () => {
    renderPage();
    expect(getSubmitButton().disabled).toBe(true);
  });

  it('(2) all required fields filled with valid values → enabled in same render', () => {
    renderPage();
    setInputByPlaceholder('Invite Code', 'INV-123');
    setInputByPlaceholder('Display Name', 'Alice');
    setInputByPlaceholder('Email', 'alice@example.com');
    setInputByPlaceholder('Password', 'correcthorse'); // 12 bytes, in [8,72]
    expect(getSubmitButton().disabled).toBe(false);
  });

  it('(3) invalid value (weak password) → disabled in same render', () => {
    renderPage();
    setInputByPlaceholder('Invite Code', 'INV-123');
    setInputByPlaceholder('Display Name', 'Alice');
    setInputByPlaceholder('Email', 'alice@example.com');
    setInputByPlaceholder('Password', 'correcthorse');
    expect(getSubmitButton().disabled).toBe(false);
    // Weak password (7 bytes < 8) — validation predicate fails.
    setInputByPlaceholder('Password', 'short77');
    expect(getSubmitButton().disabled).toBe(true);
  });

  it('(4) fix-invalid → re-enables in same render (no extra user action)', () => {
    renderPage();
    setInputByPlaceholder('Invite Code', 'INV-123');
    setInputByPlaceholder('Display Name', 'Alice');
    setInputByPlaceholder('Email', 'not-an-email');
    setInputByPlaceholder('Password', 'correcthorse');
    expect(getSubmitButton().disabled).toBe(true);
    // Fix the email — same render must flip disabled back to false.
    setInputByPlaceholder('Email', 'alice@example.com');
    expect(getSubmitButton().disabled).toBe(false);
  });

  it('(5) re-empty a required field after valid → disabled in same render', () => {
    renderPage();
    setInputByPlaceholder('Invite Code', 'INV-123');
    setInputByPlaceholder('Display Name', 'Alice');
    setInputByPlaceholder('Email', 'alice@example.com');
    setInputByPlaceholder('Password', 'correcthorse');
    expect(getSubmitButton().disabled).toBe(false);
    setInputByPlaceholder('Display Name', '');
    expect(getSubmitButton().disabled).toBe(true);
  });
});
