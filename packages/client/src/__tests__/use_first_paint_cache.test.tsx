// CS-4.2 — useFirstPaintCache hook 单测 (cs-4-stance-checklist 设计 ②).
import 'fake-indexeddb/auto';
import React from 'react';
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react-dom/test-utils';
import {
  useFirstPaintCache,
  type CachedMessage,
  type FirstPaintCacheResult,
} from '../lib/use_first_paint_cache';
import { openCS4DB, cs4Put, STORE_MESSAGES } from '../lib/cs4-idb';

let container: HTMLDivElement | null = null;
let root: Root | null = null;

beforeEach(async () => {
  container = document.createElement('div');
  document.body.appendChild(container);
  // Reset fake IDB
  // @ts-expect-error fake-indexeddb internals
  const { default: FDBFactory } = await import('fake-indexeddb/lib/FDBFactory');
  globalThis.indexedDB = new FDBFactory();
  // Reset navigator.onLine
  Object.defineProperty(navigator, 'onLine', { configurable: true, value: true });
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

function HookHarness({
  channelID,
  cursorBackfillFn,
  onResult,
}: {
  channelID: string;
  cursorBackfillFn: (since: string | null) => Promise<CachedMessage[]>;
  onResult: (r: FirstPaintCacheResult) => void;
}) {
  const r = useFirstPaintCache(channelID, cursorBackfillFn);
  React.useEffect(() => {
    onResult(r);
  }, [r, onResult]);
  return null;
}

async function render(node: React.ReactElement) {
  root = createRoot(container!);
  await act(async () => {
    root!.render(node);
  });
}

async function waitMicrotasks() {
  await act(async () => {
    await new Promise<void>((resolve) => setTimeout(resolve, 50));
  });
}

describe('CS-4.2 — useFirstPaintCache (cursor sync)', () => {
  it('TestCS42_CacheMissNoBlock — 无 cached → cache_miss → server fetch confirm → synced', async () => {
    const fresh: CachedMessage[] = [
      { id: 'm1', channel_id: 'c1', body: 'hi', sender_id: 'u1', cursor: 'cur1', ts_ms: 1 },
    ];
    const fetchSpy = vi.fn().mockResolvedValue(fresh);
    let last: FirstPaintCacheResult = { cachedMessages: null, syncState: 'cache_miss' };
    await render(
      <HookHarness
        channelID="c1"
        cursorBackfillFn={fetchSpy}
        onResult={(r) => (last = r)}
      />,
    );
    await waitMicrotasks();
    expect(fetchSpy).toHaveBeenCalled();
    expect(last.syncState).toBe('synced');
    expect(last.cachedMessages).toEqual(fresh);
  });

  it('TestCS42_HookReturnsCachedOnMount — IDB cached present → 立即返 cached', async () => {
    // Seed IDB
    const db = await openCS4DB();
    const seed: CachedMessage = {
      id: 'm1',
      channel_id: 'c1',
      body: 'cached hi',
      sender_id: 'u1',
      cursor: 'cur1',
      ts_ms: 100,
    };
    await cs4Put(db, STORE_MESSAGES, seed);
    db.close();

    const fetchSpy = vi.fn().mockResolvedValue([]);
    let last: FirstPaintCacheResult = { cachedMessages: null, syncState: 'cache_miss' };
    await render(
      <HookHarness
        channelID="c1"
        cursorBackfillFn={fetchSpy}
        onResult={(r) => (last = r)}
      />,
    );
    await waitMicrotasks();
    expect(last.cachedMessages?.length).toBeGreaterThanOrEqual(1);
  });

  it('TestCS42_OfflineSkipsServer — navigator.onLine=false → 不 fetch + offline_cache_hit', async () => {
    Object.defineProperty(navigator, 'onLine', { configurable: true, value: false });
    // Seed IDB
    const db = await openCS4DB();
    await cs4Put(db, STORE_MESSAGES, {
      id: 'm1',
      channel_id: 'c1',
      body: 'offline hi',
      sender_id: 'u1',
      cursor: 'cur1',
      ts_ms: 100,
    });
    db.close();

    const fetchSpy = vi.fn().mockResolvedValue([]);
    let last: FirstPaintCacheResult = { cachedMessages: null, syncState: 'cache_miss' };
    await render(
      <HookHarness
        channelID="c1"
        cursorBackfillFn={fetchSpy}
        onResult={(r) => (last = r)}
      />,
    );
    await waitMicrotasks();
    expect(fetchSpy).not.toHaveBeenCalled();
    expect(last.syncState).toBe('offline_cache_hit');
  });

  it('TestCS42_TriggersServerSyncOnMount — fetch 真 call (走 RT-1 既有 lib 的 caller-supplied fn)', async () => {
    const fetchSpy = vi.fn().mockResolvedValue([]);
    await render(
      <HookHarness
        channelID="c1"
        cursorBackfillFn={fetchSpy}
        onResult={() => {}}
      />,
    );
    await waitMicrotasks();
    expect(fetchSpy).toHaveBeenCalledTimes(1);
    // First arg = sinceCursor (null because no cache)
    expect(fetchSpy.mock.calls[0][0]).toBeNull();
  });
});
