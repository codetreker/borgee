// useWebSocket-auth-gate.test.tsx — bf task ws-auth-gate (fix-skill-findings).
//
// Locks the AC-2 contract from .bf/fix-skill-findings/ws-auth-gate/spec.md:
//
//   "AC-2: A vitest covering the gate exists and (a) FAILS on the pre-fix
//   codebase ... (b) PASSES on the post-fix codebase. The test asserts no
//   connect attempt when `isAuthenticated` is false, and a connect on
//   transition to true."
//
// Implementation flexibility (per AC-1, observable contract): the gate can
// live inside `useWebSocket` (parameter) or at the call site (conditional
// render of the hook owner). To keep the test agnostic, we drive the gate
// the way App.tsx does — by passing `isAuthenticated` as a hook argument.
// If a future refactor moves the gate to the call site, this test's
// `Harness` would conditionally mount the consumer instead; same wire-level
// assertion still applies (zero `new WebSocket(...)` calls pre-auth).
//
// Wire-level assertion: the only signal the browser gives the server before
// the auth cookie is set is a `new WebSocket(host + '/ws')` call. We assert
// `MockWebSocket.instances.length === 0` while `isAuthenticated === false`,
// then === 1 after the transition to `true`.

import React from 'react';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react';
import { AppProvider } from '../context/AppContext';
import { ToastProvider } from '../components/Toast';
import { useWebSocket } from '../hooks/useWebSocket';

class MockWebSocket {
  static readonly CONNECTING = 0;
  static readonly OPEN = 1;
  static readonly CLOSING = 2;
  static readonly CLOSED = 3;
  static instances: MockWebSocket[] = [];

  readonly url: string;
  readyState = MockWebSocket.CONNECTING;
  sent: string[] = [];
  onopen: ((event: Event) => void) | null = null;
  onmessage: ((event: MessageEvent) => void) | null = null;
  onclose: ((event: CloseEvent) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;

  constructor(url: string) {
    this.url = url;
    MockWebSocket.instances.push(this);
  }

  send(data: string): void {
    this.sent.push(data);
  }

  close(): void {
    this.readyState = MockWebSocket.CLOSED;
  }

  open(): void {
    this.readyState = MockWebSocket.OPEN;
    this.onopen?.(new Event('open'));
  }
}

const originalWebSocket = globalThis.WebSocket;
let container: HTMLDivElement | null = null;
let root: Root | null = null;

function Harness({ isAuthenticated }: { isAuthenticated: boolean }) {
  useWebSocket(isAuthenticated);
  return null;
}

function TestApp({ isAuthenticated }: { isAuthenticated: boolean }) {
  return (
    <AppProvider>
      <ToastProvider>
        <Harness isAuthenticated={isAuthenticated} />
      </ToastProvider>
    </AppProvider>
  );
}

describe('useWebSocket auth gate (bf task ws-auth-gate)', () => {
  beforeEach(() => {
    MockWebSocket.instances = [];
    Object.defineProperty(globalThis, 'WebSocket', {
      configurable: true,
      value: MockWebSocket,
    });
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
    Object.defineProperty(globalThis, 'WebSocket', {
      configurable: true,
      value: originalWebSocket,
    });
  });

  it('does not open a /ws connection while isAuthenticated is false', () => {
    act(() => {
      root!.render(<TestApp isAuthenticated={false} />);
    });

    expect(MockWebSocket.instances).toHaveLength(0);
  });

  it('opens a /ws connection on transition from false to true', () => {
    act(() => {
      root!.render(<TestApp isAuthenticated={false} />);
    });

    expect(MockWebSocket.instances).toHaveLength(0);

    act(() => {
      root!.render(<TestApp isAuthenticated={true} />);
    });

    expect(MockWebSocket.instances).toHaveLength(1);
    expect(MockWebSocket.instances[0]!.url).toMatch(/\/ws($|\?)/);
  });
});
