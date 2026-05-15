import React, { useEffect } from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
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

  closeFromServer(code = 1006): void {
    this.readyState = MockWebSocket.CLOSED;
    this.onclose?.({ code, reason: '' } as CloseEvent);
  }

  open(): void {
    this.readyState = MockWebSocket.OPEN;
    this.onopen?.(new Event('open'));
  }
}

const originalWebSocket = globalThis.WebSocket;
let container: HTMLDivElement | null = null;
let root: Root | null = null;

function Harness({ channelId, renderTick }: { channelId: string; renderTick: number }) {
  const { subscribe } = useWebSocket();

  useEffect(() => {
    subscribe(channelId);
  }, [channelId, renderTick, subscribe]);

  return null;
}

function TestApp({ channelId, renderTick }: { channelId: string; renderTick: number }) {
  return (
    <AppProvider>
      <ToastProvider>
        <Harness channelId={channelId} renderTick={renderTick} />
      </ToastProvider>
    </AppProvider>
  );
}

function subscribeFrames(ws: MockWebSocket): Array<{ type: string; channel_id: string }> {
  return ws.sent
    .map(raw => JSON.parse(raw) as { type: string; channel_id: string })
    .filter(frame => frame.type === 'subscribe');
}

describe('useWebSocket subscribe', () => {
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

  it('does not resend subscribe frame when the same channel is subscribed again', () => {
    act(() => {
      root!.render(<TestApp channelId="ch-1" renderTick={1} />);
    });

    const ws = MockWebSocket.instances[0]!;
    act(() => {
      ws.open();
    });

    expect(subscribeFrames(ws)).toEqual([{ type: 'subscribe', channel_id: 'ch-1' }]);

    act(() => {
      root!.render(<TestApp channelId="ch-1" renderTick={2} />);
    });

    expect(subscribeFrames(ws)).toEqual([{ type: 'subscribe', channel_id: 'ch-1' }]);
  });

  it('re-subscribes tracked channels when a new websocket connection opens after reconnect', () => {
    vi.useFakeTimers();

    act(() => {
      root!.render(<TestApp channelId="ch-1" renderTick={1} />);
    });

    const firstWs = MockWebSocket.instances[0]!;
    act(() => {
      firstWs.open();
    });
    expect(subscribeFrames(firstWs)).toEqual([{ type: 'subscribe', channel_id: 'ch-1' }]);

    act(() => {
      firstWs.closeFromServer();
      vi.advanceTimersByTime(1_000);
    });

    const reconnectWs = MockWebSocket.instances[1]!;
    act(() => {
      reconnectWs.open();
    });

    expect(subscribeFrames(reconnectWs)).toEqual([{ type: 'subscribe', channel_id: 'ch-1' }]);

    vi.useRealTimers();
  });
});
