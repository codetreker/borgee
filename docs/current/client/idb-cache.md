# CS-4 IndexedDB 乐观缓存 (client)

> 锚: `docs/blueprint/current/client-shape.md` §1.4 (本地持久化乐观缓存 B 路径) + `data-layer.md` §4.A.2 (cursor opaque) + `docs/implementation/modules/cs-4-spec.md` v0
> 落点: 战马D + 飞马 + 烈马 + 野马 (一个 milestone 一个 PR, 0 server prod + 0 schema)

## IDB wrapper SSOT (lib/cs4-idb.ts)

```ts
const DB_NAME = 'borgee-cs4';
const DB_VERSION = 1;

export const STORE_MESSAGES = 'messages';        // keyPath=id, index channel_id
export const STORE_LAST_READ_AT = 'last_read_at'; // keyPath=channel_id
export const STORE_AGENT_STATE = 'agent_state';   // keyPath=agent_id
```

3 store byte-identical 跟蓝图 §1.4 表. **typing / presence-realtime 必从 server 实时拉, 不入 IDB** (蓝图字面拆死). artifact 内容 / DM body / 草稿走 CV-10 localStorage 既有 (拆死).

API: `openCS4DB()` / `cs4Get` / `cs4Put` / `cs4Delete` / `clearStaleEntries(maxAgeMs)`.

DB version=1; schema 改 = bump version + onupgradeneeded migration (跟 server schema_migrations 同精神).

## SyncState 4-enum + 文案 (lib/cs4-sync-state.ts)

```ts
export const SYNC_STATE_LABELS: Record<SyncState, string> = {
  offline_cache_hit: '离线模式',
  synced: '已同步',
  syncing: '同步中…',
  cache_miss: '', // not rendered
};

export const SYNCING_LABEL_DELAY_MS = 3000;
```

byte-identical 跟蓝图 §1.4 字面. **改 = 改两处 + content-lock §1**.

## useFirstPaintCache hook (lib/use_first_paint_cache.ts)

```ts
export function useFirstPaintCache(
  channelID: string,
  cursorBackfillFn: (sinceCursor: string | null) => Promise<CachedMessage[]>,
): { cachedMessages: CachedMessage[] | null; syncState: SyncState };
```

- mount 时 IDB.get 返 cached + 同时触发 caller-supplied `cursorBackfillFn(sinceCursor)` server fetch
- confirm 后 IDB.put 覆盖
- cache miss 时不阻塞 UI (cached=null → 直接走 server fetch, 跟 sync 串行)
- offline 时 (`navigator.onLine=false`) skip server fetch 走 cache hit
- caller 注入 `cursorBackfillFn` 走 RT-1 既有 lib (CS-4 不绑定具体 import path)

## SyncStatusIndicator UI (components/SyncStatusIndicator.tsx)

DOM: `<span data-cs4-sync-state="{4-enum}">{label}</span>`

- cache_miss → `return null`
- syncing ≤3s → `return null` (沉默胜于假 loading 跟 RT-1 §1.1 一致)
- syncing ≥3s → 显示 `同步中…`

## 反约束守门

- typing/presence-realtime 不入 IDB: `idb.*put.*typing|idb.*put.*presence_realtime` 0 hit
- artifact / DM body 不入 IDB: `idb.*put.*artifact_content|idb.*put.*dm_body` 0 hit
- 不复用 RT-1 之外 cursor helper: `cs4.*newCursor|CS4CursorHelper` 0 hit
- 同义词漂禁: `本地缓存|离线缓存|已加载|加载完成|准备中` 0 hit
- admin god-mode 不挂 (ADM-0 §1.3): `admin.*idb|admin.*indexedDB` 0 hit
- 0 server prod: `git diff origin/main -- packages/server-go/` 0 行
- 0 schema 改: `migrations/cs_4|cs4.*api|cs4.*server` 0 hit

## 跨 milestone byte-identical 锁

- RT-1 #290 cursor opaque (CS-4 IDB.put cursor key 跟 server `?cursor=` 同源)
- DM-3 useDMSync 既有 client cursor 同步同模式
- CV-10 草稿 localStorage 拆死 (CS-4 不入草稿域)
- CS-2 #595 故障三态联动 (failed 时 IDB cache hit + offline label graceful fallback)
- ADM-0 §1.3 admin god-mode 不挂

## 不在范围

- background sync (蓝图 §1.1)
- artifact 内容 / DM body / 草稿入 IDB (草稿走 CV-10)
- typing / presence-realtime 入 IDB (蓝图 §1.4)
- Service Worker offline page (留 CS-3 PWA + sw.js DL-4)
- 跨设备同步 (server cursor 是真相)
- admin god-mode IDB inspect (永久不挂)
- IDB cleanup goroutine / scheduled job (留 v1 用户 logout 时清)
