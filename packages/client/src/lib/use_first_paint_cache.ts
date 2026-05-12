// CS-4 — useFirstPaintCache hook (蓝图 client-shape.md §1.4 cursor sync).
//
// Design ② (cs-4-stance-checklist):
//   - on mount, IDB.get returns cached data and also starts server cursor backfill
//   - cache miss does not block UI (cached=null → go directly to server fetch)
//   - offline mode (navigator.onLine=false) skips server fetch and uses cache hit
//   - show the syncing label only after >=3s (silence is better than fake loading)
//
// Real server fetch wiring is caller-supplied through cursorBackfillFn. This
// hook does not bind to a specific RT-1 lib path; callers use existing libs such
// as `import { fetchMessages }`. CS-4 only owns the IDB cache and sync state machine.

import { useEffect, useState, useRef } from 'react';
import { openCS4DB, cs4Get, cs4Put, STORE_MESSAGES } from './cs4-idb';
import { type SyncState, SYNCING_LABEL_DELAY_MS } from './cs4-sync-state';

/** Cached message envelope stored in IDB. */
export interface CachedMessage {
  id: string;
  channel_id: string;
  body: string;
  sender_id: string;
  cursor: string;
  ts_ms: number;
}

export interface FirstPaintCacheResult {
  cachedMessages: CachedMessage[] | null;
  syncState: SyncState;
}

/**
 * useFirstPaintCache — IDB first-paint + cursor sync orchestration.
 *
 * @param channelID - which channel to load
 * @param cursorBackfillFn - caller-supplied fn: (sinceCursor) → Promise<CachedMessage[]>
 *                          (uses existing RT-1 libs; CS-4 does not bind a specific import path)
 * @param now - injectable clock for tests
 */
export function useFirstPaintCache(
  channelID: string,
  cursorBackfillFn: (sinceCursor: string | null) => Promise<CachedMessage[]>,
  now: () => number = Date.now,
): FirstPaintCacheResult {
  const [cached, setCached] = useState<CachedMessage[] | null>(null);
  const [syncState, setSyncState] = useState<SyncState>('cache_miss');
  const startedAtRef = useRef<number>(0);

  useEffect(() => {
    let cancelled = false;
    startedAtRef.current = now();

    (async () => {
      // 1) IDB.get does not block UI; set state immediately when cached data exists.
      let cachedFromIDB: CachedMessage[] | null = null;
      try {
        const db = await openCS4DB();
        const tx = db.transaction(STORE_MESSAGES, 'readonly');
        const idx = tx.objectStore(STORE_MESSAGES).index('channel_id');
        const req = idx.getAll(channelID);
        cachedFromIDB = await new Promise<CachedMessage[]>((resolve) => {
          req.onsuccess = () => resolve((req.result as CachedMessage[]) ?? []);
          req.onerror = () => resolve([]);
        });
      } catch {
        cachedFromIDB = null;
      }
      if (cancelled) return;

      const isOnline = typeof navigator !== 'undefined' ? navigator.onLine : true;
      if (cachedFromIDB && cachedFromIDB.length > 0) {
        setCached(cachedFromIDB);
        if (!isOnline) {
          setSyncState('offline_cache_hit');
          return; // offline → skip server fetch
        }
        setSyncState('syncing');
      } else {
        setCached(null);
        if (!isOnline) {
          setSyncState('offline_cache_hit'); // graceful, even if empty
          return;
        }
        setSyncState('syncing');
      }

      // 2) Server cursor backfill (caller-supplied fn)
      const sinceCursor =
        cachedFromIDB && cachedFromIDB.length > 0
          ? cachedFromIDB[cachedFromIDB.length - 1].cursor
          : null;
      try {
        const fresh = await cursorBackfillFn(sinceCursor);
        if (cancelled) return;
        // 3) IDB.put overwrite using the cursor key from design ②.
        try {
          const db = await openCS4DB();
          for (const msg of fresh) {
            await cs4Put(db, STORE_MESSAGES, msg);
          }
        } catch {
          // best-effort; do not block UI
        }
        // Merge: cached + fresh
        const merged = [...(cachedFromIDB ?? []), ...fresh];
        if (!cancelled) {
          setCached(merged);
          setSyncState('synced');
        }
      } catch {
        // Server failure: use offline_cache_hit when cache exists; otherwise cache_miss.
        if (!cancelled) {
          if (cachedFromIDB && cachedFromIDB.length > 0) {
            setSyncState('offline_cache_hit');
          } else {
            setSyncState('cache_miss');
          }
        }
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [channelID, cursorBackfillFn, now]);

  return { cachedMessages: cached, syncState };
}

// Read for tests + components — reuse via `import { cs4Get } from './cs4-idb'`
export { cs4Get, openCS4DB, SYNCING_LABEL_DELAY_MS };
