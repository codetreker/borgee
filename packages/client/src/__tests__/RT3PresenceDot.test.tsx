// RT-3 ⭐ presence dot tests (vitest, react-dom/client pattern).
//
// 设计沿用 (rt-3-spec.md §0 + content-lock §1+§2):
//   - 4 态 UI 渲染 exact-match (online / offline / away / thinking)
//   - DOM data-attr source of truth (data-rt3-presence-dot/last-seen/cursor-user)
//   - 字面 exact-match (`在线` / `离线` / `刚刚活跃` / `最近活跃 N 分钟前`)
//   - thinking subject reverse constraint — 空 subject thinking → drop (防"假 loading" 漂)
import React from 'react';
import { describe, it, expect, beforeEach, afterEach } from 'vitest';
import { createRoot, type Root } from 'react-dom/client';
import { act } from 'react';
import { RT3PresenceDot } from '../components/RT3PresenceDot';
import {
  __resetRT3PresenceStoreForTest,
  markRT3Presence,
  getRT3Presence,
  RT3_AWAY_THRESHOLD_MS,
} from '../hooks/useRT3Presence';

let container: HTMLDivElement | null = null;
let root: Root | null = null;

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
  __resetRT3PresenceStoreForTest(() => 1_700_000_000_000);
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

function dot(): Element {
  const el = container!.querySelector('[data-rt3-presence-dot]');
  if (!el) throw new Error('RT3PresenceDot 未找到 data-rt3-presence-dot');
  return el;
}

describe('RT-3 ⭐ presence dot — content-lock §1+§2 exact-match', () => {
  it('§1.1 online state renders `在线` tooltip + data-rt3-presence-dot=online', async () => {
    markRT3Presence('user-A', 'online', undefined);
    await render(<RT3PresenceDot userID="user-A" now={() => 1_700_000_000_000} />);
    expect(dot().getAttribute('data-rt3-presence-dot')).toBe('online');
    expect(dot().getAttribute('title')).toBe('在线');
    expect(dot().getAttribute('data-rt3-cursor-user')).toBe('user-A');
  });

  it('§1.2 offline state renders `离线` tooltip + data-rt3-presence-dot=offline', async () => {
    markRT3Presence('user-B', 'offline', undefined);
    await render(<RT3PresenceDot userID="user-B" now={() => 1_700_000_000_000} />);
    expect(dot().getAttribute('data-rt3-presence-dot')).toBe('offline');
    expect(dot().getAttribute('title')).toBe('离线');
  });

  it('§1.3 unknown user (cache miss) renders `离线` (兜底)', async () => {
    await render(<RT3PresenceDot userID="user-ghost" now={() => 1_700_000_000_000} />);
    expect(dot().getAttribute('data-rt3-presence-dot')).toBe('offline');
    expect(dot().getAttribute('title')).toBe('离线');
  });

  it('§1.4 away (last-seen <1min) renders `刚刚活跃` + recently-active', async () => {
    const t0 = 1_700_000_000_000;
    __resetRT3PresenceStoreForTest(() => t0);
    markRT3Presence('user-C', 'away', undefined);
    await render(<RT3PresenceDot userID="user-C" now={() => t0 + 30_000} />);
    expect(dot().getAttribute('data-rt3-presence-dot')).toBe('recently-active');
    expect(dot().getAttribute('title')).toBe('刚刚活跃');
  });

  it('§1.5 away (last-seen 5min) renders `最近活跃 5 分钟前`', async () => {
    const t0 = 1_700_000_000_000;
    __resetRT3PresenceStoreForTest(() => t0);
    markRT3Presence('user-D', 'away', undefined);
    await render(<RT3PresenceDot userID="user-D" now={() => t0 + 5 * 60_000} />);
    expect(dot().getAttribute('data-rt3-presence-dot')).toBe('recently-active');
    expect(dot().getAttribute('title')).toBe('最近活跃 5 分钟前');
  });

  it('§2.1 thinking 态 subject 必带非空 — 空 subject drop (反"假 loading" 漂)', () => {
    markRT3Presence('user-E', 'thinking', '');
    expect(getRT3Presence('user-E')).toBeUndefined();
    markRT3Presence('user-E', 'thinking', '   '); // whitespace-only
    expect(getRT3Presence('user-E')).toBeUndefined();
  });

  it('§2.2 thinking 态 subject 非空 通过 — recently-active UI', async () => {
    markRT3Presence('user-F', 'thinking', 'writing section 3');
    const entry = getRT3Presence('user-F');
    expect(entry?.state).toBe('thinking');
    expect(entry?.subject).toBe('writing section 3');
    await render(<RT3PresenceDot userID="user-F" now={() => 1_700_000_000_000} />);
    expect(dot().getAttribute('data-rt3-presence-dot')).toBe('recently-active');
  });

  it('§3 multi-device — 同 userID 多次写以最新值为准', () => {
    markRT3Presence('user-H', 'online', undefined);
    markRT3Presence('user-H', 'thinking', 'tool: bash');
    const entry = getRT3Presence('user-H');
    expect(entry?.state).toBe('thinking');
    expect(entry?.subject).toBe('tool: bash');
  });

  it('§4 RT3_AWAY_THRESHOLD_MS const = 5min exact-match', () => {
    expect(RT3_AWAY_THRESHOLD_MS).toBe(300_000);
  });
});
