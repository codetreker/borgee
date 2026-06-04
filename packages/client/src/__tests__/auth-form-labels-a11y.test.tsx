// auth-form-labels-a11y.test.tsx — WCAG 1.3.1 + 3.3.2: every input on
// LoginPage and RegisterPage must have a programmatically associated
// <label htmlFor=…> so screen readers announce the field purpose and
// placeholder text is not the only label (placeholders disappear on input).
// Found by borgee-local-e2e skill first-run.
import React from 'react';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { createRoot, Root } from 'react-dom/client';
import { act } from 'react';
import LoginPage from '../components/LoginPage';
import RegisterPage from '../components/RegisterPage';

let container: HTMLDivElement | null = null;
let root: Root | null = null;

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
});

afterEach(() => {
  if (root) {
    act(() => root!.unmount());
    root = null;
  }
  if (container) {
    document.body.removeChild(container);
    container = null;
  }
  vi.restoreAllMocks();
});

function accessibleName(input: HTMLInputElement): string {
  const ariaLabel = input.getAttribute('aria-label');
  if (ariaLabel && ariaLabel.trim()) return ariaLabel.trim();
  const id = input.getAttribute('id');
  if (id) {
    const label = document.querySelector(`label[for="${id}"]`);
    if (label && label.textContent && label.textContent.trim()) {
      return label.textContent.trim();
    }
  }
  const wrappingLabel = input.closest('label');
  if (wrappingLabel && wrappingLabel.textContent && wrappingLabel.textContent.trim()) {
    return wrappingLabel.textContent.trim();
  }
  return '';
}

describe('LoginPage — WCAG 1.3.1 + 3.3.2 input labeling', () => {
  it('every input has an accessible name via <label htmlFor> (not placeholder-only)', () => {
    root = createRoot(container!);
    act(() => {
      root!.render(<LoginPage onLogin={() => {}} />);
    });
    const inputs = Array.from(container!.querySelectorAll('input')) as HTMLInputElement[];
    expect(inputs.length).toBeGreaterThan(0);
    for (const input of inputs) {
      const name = accessibleName(input);
      expect(name, `input type=${input.type} missing accessible name`).not.toBe('');
    }
  });

  it('exposes Email and Password fields by label association', () => {
    root = createRoot(container!);
    act(() => {
      root!.render(<LoginPage onLogin={() => {}} />);
    });
    const emailLabel = container!.querySelector('label[for="login-email"]');
    const passwordLabel = container!.querySelector('label[for="login-password"]');
    expect(emailLabel?.textContent?.trim()).toBe('Email');
    expect(passwordLabel?.textContent?.trim()).toBe('Password');
    expect(document.getElementById('login-email')?.tagName).toBe('INPUT');
    expect(document.getElementById('login-password')?.tagName).toBe('INPUT');
  });
});

describe('RegisterPage — WCAG 1.3.1 + 3.3.2 input labeling', () => {
  it('every input has an accessible name via <label htmlFor> (not placeholder-only)', () => {
    root = createRoot(container!);
    act(() => {
      root!.render(<RegisterPage onLogin={() => {}} onBack={() => {}} />);
    });
    const inputs = Array.from(container!.querySelectorAll('input')) as HTMLInputElement[];
    expect(inputs.length).toBeGreaterThan(0);
    for (const input of inputs) {
      const name = accessibleName(input);
      expect(name, `input type=${input.type} placeholder=${input.placeholder} missing accessible name`).not.toBe('');
    }
  });

  it('exposes Invite Code, Display Name, Email, Password fields by label association', () => {
    root = createRoot(container!);
    act(() => {
      root!.render(<RegisterPage onLogin={() => {}} onBack={() => {}} />);
    });
    const ids = ['register-invite-code', 'register-display-name', 'register-email', 'register-password'];
    const expectedNames: Record<string, string> = {
      'register-invite-code': 'Invite Code',
      'register-display-name': 'Display Name',
      'register-email': 'Email',
      'register-password': 'Password',
    };
    for (const id of ids) {
      const label = container!.querySelector(`label[for="${id}"]`);
      expect(label, `missing <label for="${id}">`).not.toBeNull();
      expect(label!.textContent?.trim()).toBe(expectedNames[id]);
      expect(document.getElementById(id)?.tagName).toBe('INPUT');
    }
  });
});
