// RT-3 presence: multi-device delivery and four states (blueprint §1.4).
//
// Policy inherited from rt-3-spec.md §0:
//   - 4-state enum must match the server
//     `internal/datalayer/presence.go` PresenceState const
//     (online / away / offline / thinking).
//   - Display text must match content-lock §1+§2 (`在线` / `离线` /
//     `刚刚活跃` / `最近活跃 ${N} 分钟前` + DOM data attrs
//     `data-rt3-presence-dot|last-seen|cursor-user`).
//   - Keep false-loading indicator wording (content-lock §3) and
//     thought-process 5-pattern checks aligned (content-lock §4; RT-3 extends that guard).
//   - Do not duplicate the existing AL-3 usePresence hook. That hook is for
//     agent presence cache; RT-3 is for human multi-device presence.
//
// Tests: __tests__/RT3PresenceDot.test.tsx + presence-reverse-grep.test.ts (extended).
import { useEffect, useState } from 'react';

/** RT-3 four-state enum; must match the server PresenceState const. */
export type RT3PresenceState = 'online' | 'away' | 'offline' | 'thinking';

/** RT-3 presence cache entry. */
export interface RT3PresenceEntry {
  state: RT3PresenceState;
  /** Subject field; thinking state requires a non-empty value (blueprint §1.1). */
  subject?: string;
  /** Unix ms — last activity time used to derive the away threshold. */
  lastSeenAt: number;
}

/** Away threshold: 5 minutes without activity changes online to away. */
export const RT3_AWAY_THRESHOLD_MS = 5 * 60 * 1000;

type Listener = (userID: string) => void;

interface RT3PresenceStore {
  entries: Map<string, RT3PresenceEntry>;
  listeners: Set<Listener>;
  now: () => number;
}

function createStore(now: () => number = () => Date.now()): RT3PresenceStore {
  return { entries: new Map(), listeners: new Set(), now };
}

let defaultStore: RT3PresenceStore = createStore();

/** Test-only: reset the store and inject a fake clock. Production code should not call this. */
export function __resetRT3PresenceStoreForTest(now?: () => number): RT3PresenceStore {
  defaultStore = createStore(now);
  return defaultStore;
}

function notify(store: RT3PresenceStore, userID: string): void {
  for (const l of store.listeners) l(userID);
}

/**
 * markRT3Presence — client entry point: WS multi-device fanout frame → cache.
 * Thinking state requires a non-empty subject, preventing false-loading
 * indicators. Empty subject follows the `thinking.subject_required`
 * server reject path (RT-3 rule ②).
 */
export function markRT3Presence(
  userID: string,
  state: RT3PresenceState,
  subject: string | undefined,
  store: RT3PresenceStore = defaultStore,
): void {
  if (!userID) return;
  // Thinking state requires a non-empty subject. Empty subject is dropped here;
  // the server also rejects it through the server-side ValidateTaskStarted validator.
  if (state === 'thinking' && (!subject || subject.trim() === '')) {
    return;
  }
  const now = store.now();
  store.entries.set(userID, { state, subject, lastSeenAt: now });
  notify(store, userID);
}

/** Read cached presence directly, without side effects. */
export function getRT3Presence(
  userID: string,
  store: RT3PresenceStore = defaultStore,
): RT3PresenceEntry | undefined {
  return store.entries.get(userID);
}

/**
 * useRT3Presence — React hook: subscribe to cached presence for one userID and derive
 * away (last-seen ≥ 5min derives online → away).
 */
export function useRT3Presence(userID: string | undefined): RT3PresenceEntry | undefined {
  const [, setTick] = useState(0);
  useEffect(() => {
    if (!userID) return;
    const store = defaultStore;
    const listener: Listener = (id) => {
      if (id === userID) setTick(t => t + 1);
    };
    store.listeners.add(listener);
    return () => {
      store.listeners.delete(listener);
    };
  }, [userID]);
  if (!userID) return undefined;
  const entry = defaultStore.entries.get(userID);
  if (!entry) return undefined;
  // Derive away from last-seen: online changes to away after ≥ 5min without activity.
  if (entry.state === 'online' && defaultStore.now() - entry.lastSeenAt >= RT3_AWAY_THRESHOLD_MS) {
    return { ...entry, state: 'away' };
  }
  return entry;
}
