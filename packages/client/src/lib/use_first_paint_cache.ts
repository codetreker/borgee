// CS-4 — useFirstPaintCache hook (蓝图 client-shape.md §1.4 cursor sync).
//
// 设计 ② (cs-4-stance-checklist):
//   - mount 时 IDB.get 返 cached + 同时触发 server cursor backfill
//   - cache miss 时不阻塞 UI (cached=null → 直接走 server fetch)
//   - offline 时 (navigator.onLine=false) skip server fetch 走 cache hit
//   - syncing ≥3s 才显示 label (沉默胜于假 loading)
//
// 真 server fetch wiring 由 caller 注入 (cursorBackfillFn) — hook 不绑定
// 具体 RT-1 lib 路径, caller 用 `import { fetchMessages }` 等既有 lib.
// CS-4 仅落 IDB cache + sync state machine.

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
 *                          (走 RT-1 既有 lib; CS-4 不绑定具体 import path)
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
      // 1) IDB.get — 不阻塞 UI; 若 cached present 立即设态
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
        // 3) IDB.put 覆盖 (走 cursor key, 设计 ②)
        try {
          const db = await openCS4DB();
          for (const msg of fresh) {
            await cs4Put(db, STORE_MESSAGES, msg);
          }
        } catch {
          // best-effort; 不阻塞 UI
        }
        // Merge: cached + fresh
        const merged = [...(cachedFromIDB ?? []), ...fresh];
        if (!cancelled) {
          setCached(merged);
          setSyncState('synced');
        }
      } catch {
        // server fail → if had cache, fall back to offline_cache_hit; else cache_miss
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
