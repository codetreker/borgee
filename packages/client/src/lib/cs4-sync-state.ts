// CS-4 — SyncState 4-enum + 文案 byte-identical (蓝图 client-shape.md §1.4).
//
// 设计 ② + content-lock §1:
//   - offline_cache_hit → "离线模式" (navigator.onLine=false + cached present)
//   - synced            → "已同步"   (server fetch confirm + IDB.put done)
//   - syncing           → "同步中…"  (≥3s in-flight; ≤3s 沉默)
//   - cache_miss        → null      (return null, 不渲染; 走 server fetch 直接)
//
// 反约束 (cs-4-content-lock §1):
//   - 同义词漂禁: '本地缓存' / '离线缓存' / '已加载' 0 hit
//   - syncing ≤3s 不渲染 (沉默胜于假 loading 字面承袭 RT-1 §1.1)

export type SyncState = 'offline_cache_hit' | 'synced' | 'syncing' | 'cache_miss';

export const SYNC_STATE_LABELS: Record<SyncState, string> = {
  offline_cache_hit: '离线模式',
  synced: '已同步',
  syncing: '同步中…',
  cache_miss: '', // not rendered
};

/** Syncing label 才显示的最小延迟 (ms) — 沉默胜于假 loading. */
export const SYNCING_LABEL_DELAY_MS = 3000;
