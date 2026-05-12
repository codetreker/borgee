# CS-4 IndexedDB optimistic cache (client)

> Source: `docs/blueprint/current/client-shape.md` §1.4 (local persistent optimistic-cache path B) + `data-layer.md` §4.A.2 (cursor opaque) + `docs/implementation/modules/cs-4-spec.md` v0
> Scope: only covers the CS-4 client IndexedDB optimistic cache; does not modify server production code or schema.

## IDB Wrapper Definition (lib/cs4-idb.ts)

```ts
const DB_NAME = 'borgee-cs4';
const DB_VERSION = 1;

export const STORE_MESSAGES = 'messages';        // keyPath=id, index channel_id
export const STORE_LAST_READ_AT = 'last_read_at'; // keyPath=channel_id
export const STORE_AGENT_STATE = 'agent_state';   // keyPath=agent_id
```

The three stores stay aligned with the blueprint §1.4 table.

| Data domain | Storage strategy | Constraint source |
|---|---|---|
| messages / last_read_at / agent_state | Write to CS-4 IndexedDB stores | Blueprint §1.4 table |
| typing / presence-realtime | Must be fetched from the server in real time; do not store in IDB | Blueprint requires separation from cached domains |
| artifact content / DM body / drafts | Continue using the existing CV-10 localStorage path | Explicitly separate from the CS-4 cache domain |

API: `openCS4DB()` / `cs4Get` / `cs4Put` / `cs4Delete` / `clearStaleEntries(maxAgeMs)`.

DB version=1. Schema changes must bump the version and add the migration in `onupgradeneeded`; governance matches server schema_migrations.

## SyncState 4-Enum + Copy (lib/cs4-sync-state.ts)

```ts
export const SYNC_STATE_LABELS: Record<SyncState, string> = {
  offline_cache_hit: '离线模式',
  synced: '已同步',
  syncing: '同步中…',
  cache_miss: '', // not rendered
};

export const SYNCING_LABEL_DELAY_MS = 3000;
```

These labels must match the blueprint §1.4 literals. **Changing them requires updating two places + content-lock §1**.

## useFirstPaintCache hook (lib/use_first_paint_cache.ts)

```ts
export function useFirstPaintCache(
  channelID: string,
  cursorBackfillFn: (sinceCursor: string | null) => Promise<CachedMessage[]>,
): { cachedMessages: CachedMessage[] | null; syncState: SyncState };
```

| Scenario | Behavior |
|---|---|
| mount | `IDB.get` returns cached data and also triggers the caller-provided `cursorBackfillFn(sinceCursor)` server fetch |
| server confirm | `IDB.put` overwrites the cache |
| cache miss | Does not block UI; after `cached=null`, go directly to server fetch and serialize it with sync |
| offline (`navigator.onLine=false`) | Skip server fetch and use the cache hit |
| cursor backfill | Caller passes `cursorBackfillFn`; reuse the existing RT-1 library, and CS-4 does not specify a concrete import path |

## SyncStatusIndicator UI (components/SyncStatusIndicator.tsx)

DOM: `<span data-cs4-sync-state="{4-enum}">{label}</span>`

| syncState | UI behavior |
|---|---|
| `cache_miss` | `return null` |
| `syncing` ≤3s | `return null`; do not show loading within 3 seconds, avoiding unconfirmed progress display and matching RT-1 §1.1 |
| `syncing` ≥3s | Show `同步中…` |

## Prohibited Behavior / QA Checks

| Constraint | Check |
|---|---|
| typing/presence-realtime must not enter IDB | `idb.*put.*typing|idb.*put.*presence_realtime` has no matches |
| artifact / DM body must not enter IDB | `idb.*put.*artifact_content|idb.*put.*dm_body` has no matches |
| Do not reuse a cursor helper outside RT-1 | `cs4.*newCursor|CS4CursorHelper` has no matches |
| Do not introduce unlocked cache-state copy | `本地缓存|离线缓存|已加载|加载完成|准备中` has no matches |
| Do not provide an admin IDB inspection entry point (ADM-0 §1.3) | `admin.*idb|admin.*indexedDB` has no matches |
| Do not modify server production code | `git diff origin/main -- packages/server-go/` has 0 lines |
| Do not modify schema | `migrations/cs_4|cs4.*api|cs4.*server` has no matches |

## Cross-Module Consistency Requirements

| Source | Lock point |
|---|---|
| RT-1 #290 cursor opaque | CS-4 `IDB.put` cursor key shares the same rule as server `?cursor=` |
| DM-3 useDMSync | Existing client cursor sync pattern |
| CV-10 draft localStorage | Explicitly separate; CS-4 does not enter the draft domain |
| CS-2 #595 failure three-state interaction | When failed, IDB cache hit + offline label degrades smoothly |
| ADM-0 §1.3 | Do not provide an admin IDB inspection entry point |

## Out of Scope

- background sync (blueprint §1.1)
- artifact content / DM body / drafts in IDB (drafts use CV-10)
- typing / presence-realtime in IDB (blueprint §1.4)
- Service Worker offline page (covered by CS-3 PWA + sw.js DL-4)
- cross-device sync (server cursor is authoritative)
- admin IDB inspection entry point (admin / privileged admin routes must not expose or mount this entry point)
- IDB cleanup goroutine / scheduled job (handled by the v1 user logout flow)
