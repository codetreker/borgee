// RegisterPage-button-state.test.tsx — submit button reflects validation state.
// Found by borgee-local-e2e first-run: with short password the button stayed
// enabled but click was a no-op. Button must be disabled while form invalid.
import React from 'react';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react';
import RegisterPage from '../components/RegisterPage';

let container: HTMLDivElement | null = null;
let root: Root | null = null;

function getSubmitButton(): HTMLButtonElement {
  const btn = container!.querySelector('button[type="submit"]') as HTMLButtonElement;
  return btn;
}

function setInputValue(input: HTMLInputElement, value: string) {
  const setter = Object.getOwnPropertyDescriptor(
    window.HTMLInputElement.prototype,
    'value',
  )?.set;
  setter?.call(input, value);
  input.dispatchEvent(new Event('input', { bubbles: true }));
}

function inputByPlaceholder(placeholder: string): HTMLInputElement {
  return container!.querySelector(
    `input[placeholder="${placeholder}"]`,
  ) as HTMLInputElement;
}

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

describe('RegisterPage submit button disabled state', () => {
  it('fresh form mounted → submit button disabled (all required fields empty)', () => {
    act(() => {
      root!.render(<RegisterPage onLogin={() => {}} onBack={() => {}} />);
    });
    const btn = getSubmitButton();
    expect(btn.disabled).toBe(true);
  });

  it('all required fields filled with valid values → submit button enabled', () => {
    act(() => {
      root!.render(<RegisterPage onLogin={() => {}} onBack={() => {}} />);
    });
    act(() => {
      setInputValue(inputByPlaceholder('Invite Code'), 'INVITE123');
      setInputValue(inputByPlaceholder('Display Name'), 'Alice');
      setInputValue(inputByPlaceholder('Email'), 'alice@example.com');
      setInputValue(inputByPlaceholder('Password'), 'correctpassword');
    });
    const btn = getSubmitButton();
    expect(btn.disabled).toBe(false);
  });

  it('short password (invalid) → submit button stays disabled even with all fields filled', () => {
    act(() => {
      root!.render(<RegisterPage onLogin={() => {}} onBack={() => {}} />);
    });
    act(() => {
      setInputValue(inputByPlaceholder('Invite Code'), 'INVITE123');
      setInputValue(inputByPlaceholder('Display Name'), 'Alice');
      setInputValue(inputByPlaceholder('Email'), 'alice@example.com');
      setInputValue(inputByPlaceholder('Password'), 'short');
    });
    const btn = getSubmitButton();
    expect(btn.disabled).toBe(true);
    // and the inline password error is visible
    const errEl = container!.querySelector('.login-error');
    expect(errEl?.textContent).toContain('at least 8 characters');
  });

  it('invalid email → submit button stays disabled', () => {
    act(() => {
      root!.render(<RegisterPage onLogin={() => {}} onBack={() => {}} />);
    });
    act(() => {
      setInputValue(inputByPlaceholder('Invite Code'), 'INVITE123');
      setInputValue(inputByPlaceholder('Display Name'), 'Alice');
      setInputValue(inputByPlaceholder('Email'), 'not-an-email');
      setInputValue(inputByPlaceholder('Password'), 'correctpassword');
    });
    expect(getSubmitButton().disabled).toBe(true);
  });

  it('fix invalid → valid transition re-enables button', () => {
    act(() => {
      root!.render(<RegisterPage onLogin={() => {}} onBack={() => {}} />);
    });
    act(() => {
      setInputValue(inputByPlaceholder('Invite Code'), 'INVITE123');
      setInputValue(inputByPlaceholder('Display Name'), 'Alice');
      setInputValue(inputByPlaceholder('Email'), 'alice@example.com');
      setInputValue(inputByPlaceholder('Password'), 'short');
    });
    expect(getSubmitButton().disabled).toBe(true);
    act(() => {
      setInputValue(inputByPlaceholder('Password'), 'correctpassword');
    });
    expect(getSubmitButton().disabled).toBe(false);
  });
});
