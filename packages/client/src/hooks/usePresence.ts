// AL-3.3 (#R3 Phase 2) — usePresence hook + presence cache.
//
// Design principles:
//   - Single source: server `presence.IsOnline(agent_id)` (AL-3.2 #317 hub
//     lifecycle hook writes presence_sessions; IsOnline OR-query reads it).
//     The client only caches WS `presence.changed` frames and does not
//     reimplement online-state decisions.
//   - 5s throttle: server §2.4 PresenceChange5sCoalesce already throttles the
//     outbound stream; the client adds notification throttling. Cache always
//     stores the latest value, while subscribers (UI) re-render at most once per
//     5s to reduce flap. Substantive cross-window state changes are still
//     notified; duplicate writes for the same state are skipped to save renders.
//   - clock fixture: throttle uses injectable now(), so vitest can advance a
//     fake clock without relying on setTimeout / wall time.
//   - Cache stores only the (state, reason, updatedAt) tuple. It does not store
//     IP, heartbeat time, or connection count, matching the acceptance §2.5
//     frame-field whitelist and preventing runtime internals from leaking into UI
//     state.
import { useEffect, useState } from 'react';
import type { AgentRuntimeReason, AgentRuntimeState } from '../lib/api';

/** Cache row: current known presence state for one agentID. */
export interface PresenceEntry {
  state: AgentRuntimeState;
  reason: AgentRuntimeReason | undefined;
  /** Unix ms — last notification time (5s throttle anchor). */
  updatedAt: number;
}

/** 5-second throttle window, matching server §2.4 PresenceChange5sCoalesce. */
export const PRESENCE_THROTTLE_MS = 5_000;

type Listener = (id: string) => void;

interface PresenceStore {
  /** Latest known value (cache is always written). */
  entries: Map<string, PresenceEntry>;
  /** Last notify time (anchor for throttle window). */
  lastNotified: Map<string, number>;
  /** Pending state inside throttle window; trailing-flush when the window ends. */
  pendingFlush: Map<string, ReturnType<typeof setTimeout>>;
  listeners: Set<Listener>;
  /** Injectable now(); defaults to Date.now and can be overridden in tests. */
  now: () => number;
}

function createStore(now: () => number = () => Date.now()): PresenceStore {
  return {
    entries: new Map(),
    lastNotified: new Map(),
    pendingFlush: new Map(),
    listeners: new Set(),
    now,
  };
}

let defaultStore: PresenceStore = createStore();

/** Test-only: reset and inject a fake clock. Production code should not call this. */
export function __resetPresenceStoreForTest(now?: () => number): PresenceStore {
  defaultStore = createStore(now);
  return defaultStore;
}

function notify(store: PresenceStore, id: string): void {
  store.lastNotified.set(id, store.now());
  for (const l of store.listeners) l(id);
}

/**
 * markPresence — client entry point: WS `presence.changed` frame → cache.
 *
 * Cache always stores the latest value. UI notifications are throttled to 5s:
 *   - Since last notify ≥ 5s → notify immediately.
 *   - Since last notify < 5s → defer notify, but trailing-flush the latest write
 *     at the end of the window (via setTimeout; tests can call flushPendingForTest).
 *   - Duplicate (state, reason) write → skip listener work.
 */
export function markPresence(
  agentID: string,
  state: AgentRuntimeState,
  reason: AgentRuntimeReason | undefined,
  store: PresenceStore = defaultStore,
): void {
  if (!agentID) return;
  const existing = store.entries.get(agentID);
  const stateChanged = !existing || existing.state !== state || existing.reason !== reason;
  const now = store.now();
  // Always write cache (so getPresence reflects latest).
  store.entries.set(agentID, { state, reason, updatedAt: now });
  if (!stateChanged) return;

  const last = store.lastNotified.get(agentID) ?? 0;
  if (now - last >= PRESENCE_THROTTLE_MS) {
    // Outside throttle window: notify immediately and clear any pending flush.
    const pending = store.pendingFlush.get(agentID);
    if (pending) {
      clearTimeout(pending);
      store.pendingFlush.delete(agentID);
    }
    notify(store, agentID);
    return;
  }
  // Inside throttle window: schedule trailing flush. Keep the existing timer;
  // it will flush the latest cached value.
  if (!store.pendingFlush.has(agentID)) {
    const delay = PRESENCE_THROTTLE_MS - (now - last);
    const t = setTimeout(() => {
      store.pendingFlush.delete(agentID);
      notify(store, agentID);
    }, delay);
    store.pendingFlush.set(agentID, t);
  }
}

/** Test-only: flush all pending updates immediately, simulating setTimeout expiry. */
export function flushPendingForTest(store: PresenceStore = defaultStore): void {
  const ids = [...store.pendingFlush.keys()];
  for (const id of ids) {
    const t = store.pendingFlush.get(id);
    if (t) clearTimeout(t);
    store.pendingFlush.delete(id);
    notify(store, id);
  }
}

/** Read one agent's cached presence directly, without side effects. */
export function getPresence(
  agentID: string,
  store: PresenceStore = defaultStore,
): PresenceEntry | undefined {
  return store.entries.get(agentID);
}

/**
 * usePresence — React hook: subscribe to cached presence for one agentID.
 * Returning undefined means cache miss; callers should let PresenceDot render
 * the explicit offline fallback. describeAgentState(undefined, undefined)
 * already implements that fallback per §11.
 */
export function usePresence(agentID: string | undefined): PresenceEntry | undefined {
  const [, setTick] = useState(0);
  useEffect(() => {
    if (!agentID) return;
    const store = defaultStore;
    const listener: Listener = (id) => {
      if (id === agentID) setTick(t => t + 1);
    };
    store.listeners.add(listener);
    return () => {
      store.listeners.delete(listener);
    };
  }, [agentID]);
  return agentID ? defaultStore.entries.get(agentID) : undefined;
}
