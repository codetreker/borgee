// useWebSocket-auth-gate.test.tsx — pre-auth /ws connect must not fire.
//
// Bug guarded (found by borgee-local-e2e skill first-run): on first
// cold SPA load the cookie is not yet set; useWebSocket() opened /ws
// immediately, the server closed it with 401, the reconnect scheduler
// fired again ~50ms later, and the loop stacked 11 console errors
// before signup completed.
//
// Fix: useWebSocket accepts an `enabled` option. App.tsx now passes
// `enabled: authenticated`, so no socket is opened until auth check
// has settled positive. This test asserts both halves of the
// contract:
//   1. enabled=false → zero WebSocket constructors invoked
//   2. enabled flips to true → exactly one WebSocket constructor fires
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
  onopen: ((event: Event) => void) | null = null;
  onmessage: ((event: MessageEvent) => void) | null = null;
  onclose: ((event: CloseEvent) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;

  constructor(url: string) {
    this.url = url;
    MockWebSocket.instances.push(this);
  }
  send(_data: string): void {}
  close(): void {
    this.readyState = MockWebSocket.CLOSED;
  }
}

const originalWebSocket = globalThis.WebSocket;
let container: HTMLDivElement | null = null;
let root: Root | null = null;

function Harness({ enabled }: { enabled: boolean }) {
  useWebSocket({ enabled });
  return null;
}

function TestApp({ enabled }: { enabled: boolean }) {
  return (
    <AppProvider>
      <ToastProvider>
        <Harness enabled={enabled} />
      </ToastProvider>
    </AppProvider>
  );
}

describe('useWebSocket auth gate', () => {
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

  it('does NOT open a WebSocket when enabled is false (pre-auth)', () => {
    act(() => {
      root!.render(<TestApp enabled={false} />);
    });

    expect(MockWebSocket.instances).toHaveLength(0);
  });

  it('opens a WebSocket once enabled flips from false to true (post-auth)', () => {
    act(() => {
      root!.render(<TestApp enabled={false} />);
    });
    expect(MockWebSocket.instances).toHaveLength(0);

    act(() => {
      root!.render(<TestApp enabled={true} />);
    });

    expect(MockWebSocket.instances).toHaveLength(1);
    expect(MockWebSocket.instances[0]!.url).toMatch(/\/ws(\?|$)/);
  });
});
