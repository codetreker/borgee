// DM-3.2 — useDMSync hook
//
// 立场 (跟 dm-3-stance-checklist.md §1):
//   ① DM cursor 复用 RT-1.3 既有 sequence
//   ② 多端走 RT-3 fan-out, 不订阅 dm-only frame
//   ④ 复用 useArtifactUpdated / lastSeenCursor 模式 — sessionStorage
//      `dm:<channelID>:cursor` round-trip
//
// 反约束: 不订阅 `borgee:dm-sync` 等 dm-only frame, 不开 dmSubscribe API
// (DM 视角的多端同步走普通 channel push 同 path).
//
// API:
//   const { lastSeenCursor, markSeen } = useDMSync(dmChannelID);
//
// markSeen(cursor) advances monotonically: it only stores cursor values
// greater than lastSeen, matching lastSeenCursor.ts persistLastSeenCursor.

import { useCallback, useEffect, useState } from 'react';

const KEY_PREFIX = 'borgee.dm3.cursor:';

function safeStorage(): Storage | null {
  try {
    if (typeof window === 'undefined' || !window.sessionStorage) return null;
    return window.sessionStorage;
  } catch {
    return null;
  }
}

function storageKey(dmChannelID: string): string {
  return KEY_PREFIX + dmChannelID;
}

/** Load the persisted DM cursor for the given DM channel. */
export function loadDMCursor(dmChannelID: string): number {
  if (!dmChannelID) return 0;
  const s = safeStorage();
  if (!s) return 0;
  const raw = s.getItem(storageKey(dmChannelID));
  if (raw === null) return 0;
  const n = Number(raw);
  if (!Number.isFinite(n) || n < 0) return 0;
  return Math.floor(n);
}

/**
 * Persist the DM cursor IF strictly greater than current. Mirrors
 * persistLastSeenCursor monotonic invariant.
 */
export function persistDMCursor(dmChannelID: string, cursor: number): number {
  if (!dmChannelID || !Number.isFinite(cursor) || cursor <= 0) {
    return loadDMCursor(dmChannelID);
  }
  const current = loadDMCursor(dmChannelID);
  if (cursor <= current) return current;
  const s = safeStorage();
  if (!s) return cursor; // Best effort; in-memory fallback omitted for simplicity.
  s.setItem(storageKey(dmChannelID), String(cursor));
  return cursor;
}

/** Test-only reset hook. Not exported from a barrel. */
export function __resetDMCursorForTests(dmChannelID: string): void {
  const s = safeStorage();
  if (s) s.removeItem(storageKey(dmChannelID));
}

/**
 * useDMSync — React hook for tracking the high-water cursor of a DM
 * channel across tabs / page reloads. Pure client behavior; no WS subscription
 * (立场 ② — DM frames flow through the existing per-channel push path).
 *
 * Returns:
 *   lastSeenCursor — current persisted value (initial render reads sessionStorage)
 *   markSeen(cursor) — advance the persisted value monotonically
 */
export function useDMSync(dmChannelID: string): {
  lastSeenCursor: number;
  markSeen: (cursor: number) => void;
} {
  const [lastSeenCursor, setLastSeenCursor] = useState<number>(() =>
    loadDMCursor(dmChannelID),
  );

  // Reload on dmChannelID change (switching DM panes).
  useEffect(() => {
    setLastSeenCursor(loadDMCursor(dmChannelID));
  }, [dmChannelID]);

  const markSeen = useCallback(
    (cursor: number) => {
      const persisted = persistDMCursor(dmChannelID, cursor);
      setLastSeenCursor(persisted);
    },
    [dmChannelID],
  );

  return { lastSeenCursor, markSeen };
}
