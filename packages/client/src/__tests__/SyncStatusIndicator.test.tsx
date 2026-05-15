// CS-4.2 — SyncStatusIndicator 单测 (cs-4-content-lock §2 + 沉默胜于假 loading).
import React from 'react';
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react';
import SyncStatusIndicator from '../components/SyncStatusIndicator';
import { SYNCING_LABEL_DELAY_MS } from '../lib/cs4-sync-state';

let container: HTMLDivElement | null = null;
let root: Root | null = null;

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
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

async function render(node: React.ReactElement) {
  root = createRoot(container!);
  await act(async () => {
    root!.render(node);
  });
}

describe('CS-4.2 — SyncStatusIndicator (DOM + 沉默胜于假 loading)', () => {
  it('TestCS42_OfflineCacheHitLabel — 文案 + DOM byte-identical', async () => {
    await render(<SyncStatusIndicator state="offline_cache_hit" />);
    const el = container!.querySelector('[data-cs4-sync-state="offline_cache_hit"]');
    expect(el).toBeTruthy();
    expect(el!.textContent).toBe('离线模式');
  });

  it('TestCS42_SyncedLabel — 文案 + DOM byte-identical', async () => {
    await render(<SyncStatusIndicator state="synced" />);
    const el = container!.querySelector('[data-cs4-sync-state="synced"]');
    expect(el).toBeTruthy();
    expect(el!.textContent).toBe('已同步');
  });

  it('TestCS42_CacheMissReturnsNull — cache_miss 不渲染', async () => {
    await render(<SyncStatusIndicator state="cache_miss" />);
    expect(container!.querySelector('[data-cs4-sync-state]')).toBeNull();
  });

  it('TestCS42_SyncingLabelDelayed — syncing ≤3s 不渲染 (沉默胜于假 loading)', async () => {
    const startedAtMs = 10000;
    let nowVal = startedAtMs + 100; // 100ms after start
    await render(
      <SyncStatusIndicator state="syncing" startedAtMs={startedAtMs} now={() => nowVal} />,
    );
    expect(container!.querySelector('[data-cs4-sync-state]')).toBeNull();
  });

  it('TestCS42_SyncingLabelShown_AfterThreshold — startedAtMs > 3s ago → 立即显示', async () => {
    const startedAtMs = 10000;
    const nowVal = startedAtMs + SYNCING_LABEL_DELAY_MS + 100; // 3.1s after
    await render(
      <SyncStatusIndicator state="syncing" startedAtMs={startedAtMs} now={() => nowVal} />,
    );
    const el = container!.querySelector('[data-cs4-sync-state="syncing"]');
    expect(el).toBeTruthy();
    expect(el!.textContent).toBe('同步中…');
  });
});
