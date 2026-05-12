// CS-4 — SyncStatusIndicator (cs-4-content-lock §2 + no unconfirmed loading text).
//
// DOM text/attribute lock:
//   <span data-cs4-sync-state="{offline_cache_hit|synced|syncing|cache_miss}">{label}</span>
//
// Constraints:
//   - cache_miss returns null (no fallback toast)
//   - syncing returns null for ≤3s (avoid unconfirmed loading text; follows RT-1 §1.1)
//   - no spinner path; use the DOM data-attr source above
import React, { useEffect, useState } from 'react';
import { type SyncState, SYNC_STATE_LABELS, SYNCING_LABEL_DELAY_MS } from '../lib/cs4-sync-state';

export interface SyncStatusIndicatorProps {
  state: SyncState;
  /** ms epoch when this state began; if undefined, treat as just now. */
  startedAtMs?: number;
  /** Injectable clock for tests. */
  now?: () => number;
}

export default function SyncStatusIndicator({
  state,
  startedAtMs,
  now = Date.now,
}: SyncStatusIndicatorProps) {
  const [showSyncing, setShowSyncing] = useState<boolean>(() => {
    if (state !== 'syncing') return false;
    if (startedAtMs === undefined) return false;
    return now() - startedAtMs >= SYNCING_LABEL_DELAY_MS;
  });

  useEffect(() => {
    if (state !== 'syncing' || startedAtMs === undefined) {
      setShowSyncing(false);
      return;
    }
    const elapsed = now() - startedAtMs;
    if (elapsed >= SYNCING_LABEL_DELAY_MS) {
      setShowSyncing(true);
      return;
    }
    const timer = setTimeout(() => setShowSyncing(true), SYNCING_LABEL_DELAY_MS - elapsed);
    return () => clearTimeout(timer);
  }, [state, startedAtMs, now]);

  if (state === 'cache_miss') return null;
  if (state === 'syncing' && !showSyncing) return null;

  return (
    <span
      className={`cs4-sync-state cs4-sync-state-${state}`}
      data-cs4-sync-state={state}
    >
      {SYNC_STATE_LABELS[state]}
    </span>
  );
}
