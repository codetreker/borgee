// useWebSocket-rt3-presence-wiring.test.tsx — #971 RT-3 human presence feed.
//
// Bug guarded: RT3PresenceDot (mounted on ChannelMembersModal human rows)
// reads the RT-3 presence store via useRT3Presence, but markRT3Presence had
// ZERO production callers — so the dot was permanently offline even when a
// human user's WS was connected.
//
// Fix: the `presence` frame (server broadcasts {type:'presence', user_id,
// status} for every WS user on connect/disconnect, server-go
// internal/ws/client.go) now also feeds markRT3Presence(userId, status). This
// test asserts that online/offline frames write the RT-3 store, and garbage
// status values do not.
import React, { useEffect } from 'react';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react';
import { AppProvider } from '../context/AppContext';
import { ToastProvider } from '../components/Toast';
import { useWebSocket } from '../hooks/useWebSocket';
import { __resetPresenceStoreForTest } from '../hooks/usePresence';
import { __resetRT3PresenceStoreForTest, getRT3Presence } from '../hooks/useRT3Presence';

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
  __resetRT3PresenceStoreForTest(() => 1_700_000_000_000);
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

describe('useWebSocket → markRT3Presence wiring (#971)', () => {
  it('writes the RT-3 store online for a human `presence` online frame', () => {
    const ws = bootWS();
    act(() => {
      ws.receive({ type: 'presence', user_id: 'human-1', status: 'online' });
    });
    const entry = getRT3Presence('human-1');
    expect(entry).toBeTruthy();
    expect(entry!.state).toBe('online');
  });

  it('writes the RT-3 store offline for a `presence` offline frame', () => {
    const ws = bootWS();
    act(() => {
      ws.receive({ type: 'presence', user_id: 'human-1', status: 'online' });
      ws.receive({ type: 'presence', user_id: 'human-1', status: 'offline' });
    });
    const entry = getRT3Presence('human-1');
    expect(entry).toBeTruthy();
    expect(entry!.state).toBe('offline');
  });

  it('does not write the RT-3 store for a garbage status value', () => {
    const ws = bootWS();
    act(() => {
      ws.receive({ type: 'presence', user_id: 'human-2', status: 'maybe' });
    });
    expect(getRT3Presence('human-2')).toBeUndefined();
  });

  it('does not touch the RT-3 store on the agent-only `presence.changed` frame', () => {
    const ws = bootWS();
    act(() => {
      ws.receive({
        type: 'presence.changed',
        agent_id: 'agent-9',
        status: 'error',
        reason: 'api_key_invalid',
      });
    });
    // presence.changed is the AL-3 agent runtime frame — RT-3 store untouched.
    expect(getRT3Presence('agent-9')).toBeUndefined();
  });
});
