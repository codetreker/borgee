// useWebSocket-presence-routing.test.tsx — agent-online routing.
//
// Bug guarded: server emits a single `presence` frame for every WS user
// (humans + agents share /ws). Before the fix, that frame only fed the
// USER_ONLINE / USER_OFFLINE reducer; the agent presence cache read by
// usePresence() stayed empty, so agents rendered "已离线" even while
// their runtime was online.
//
// Fix mirrors the same frame into markPresence() so usePresence(agentId)
// returns 'online' for connected agents. Humans never have a
// usePresence(humanId) consumer (presence-reverse-grep §3.2 allowlist),
// so the mirror is a harmless no-op for them.
import React, { useEffect } from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react';
import { AppProvider } from '../context/AppContext';
import { ToastProvider } from '../components/Toast';
import { useWebSocket } from '../hooks/useWebSocket';
import { __resetPresenceStoreForTest, getPresence } from '../hooks/usePresence';

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
  open(): void {
    this.readyState = MockWebSocket.OPEN;
    this.onopen?.(new Event('open'));
  }
  receive(payload: unknown): void {
    this.onmessage?.({ data: JSON.stringify(payload) } as MessageEvent);
  }
}

const originalWebSocket = globalThis.WebSocket;
let container: HTMLDivElement | null = null;
let root: Root | null = null;

function Harness() {
  // Hook must be mounted somewhere; subscribe() is the only side effect
  // we need to keep alive in this fixture.
  const { subscribe } = useWebSocket();
  useEffect(() => {
    subscribe('ch-1');
  }, [subscribe]);
  return null;
}

function TestApp() {
  return (
    <AppProvider>
      <ToastProvider>
        <Harness />
      </ToastProvider>
    </AppProvider>
  );
}

beforeEach(() => {
  __resetPresenceStoreForTest(() => 1_000_000);
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

function bootWS(): MockWebSocket {
  act(() => {
    root!.render(<TestApp />);
  });
  const ws = MockWebSocket.instances[0]!;
  act(() => {
    ws.open();
  });
  return ws;
}

describe('useWebSocket presence routing — agent online cache fill', () => {
  it('mirrors `presence` frame into the agent presence cache so usePresence reports online', () => {
    const ws = bootWS();
    act(() => {
      ws.receive({ type: 'presence', user_id: 'agent-1', status: 'online' });
    });
    const entry = getPresence('agent-1');
    expect(entry).toBeTruthy();
    expect(entry!.state).toBe('online');
  });

  it('mirrors `presence` offline into the cache too', () => {
    const ws = bootWS();
    act(() => {
      ws.receive({ type: 'presence', user_id: 'agent-1', status: 'online' });
      ws.receive({ type: 'presence', user_id: 'agent-1', status: 'offline' });
    });
    const entry = getPresence('agent-1');
    expect(entry).toBeTruthy();
    expect(entry!.state).toBe('offline');
  });

  it('ignores garbage status values on the `presence` frame (does not write cache)', () => {
    const ws = bootWS();
    act(() => {
      ws.receive({ type: 'presence', user_id: 'agent-2', status: 'maybe' });
    });
    expect(getPresence('agent-2')).toBeUndefined();
  });

  it('`presence.changed` still accepts the AL-3 tri-state (online / offline / error + reason)', () => {
    const ws = bootWS();
    act(() => {
      ws.receive({
        type: 'presence.changed',
        agent_id: 'agent-4',
        status: 'error',
        reason: 'api_key_invalid',
      });
    });
    const entry = getPresence('agent-4');
    expect(entry?.state).toBe('error');
    expect(entry?.reason).toBe('api_key_invalid');
  });

  it('rejects unknown status on `presence.changed` (cache untouched)', () => {
    const ws = bootWS();
    act(() => {
      ws.receive({ type: 'presence.changed', agent_id: 'agent-5', status: 'mystery' });
    });
    expect(getPresence('agent-5')).toBeUndefined();
  });
});
