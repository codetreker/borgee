# CS-4 spec brief — IndexedDB 乐观缓存 (≤80 行)

> 飞马 + 战马D · 2026-04-30 · Phase 4+ Client Shape 第四段 (蓝图 client-shape.md §1.4)
> **蓝图锚**: [`client-shape.md`](../../blueprint/current/client-shape.md) §1.4 (本地持久化乐观缓存 B 路径) + §0 (Web SPA 协作主战场) + `data-layer.md` §4.A.2 (cursor opaque 协议)
> **关联**: RT-1 #290 cursor backfill (CS-4 同步入口) + DM-3 useDMSync hook (既有 client-side cursor 同步) + CS-2 #595 故障三态 (failed 时 cache miss → optimistic fallback) + CS-3 #598 PWA install / Push UI + CS-1 spec §3 留账 (CS-4 = IndexedDB 乐观缓存)
> **命名**: CS-4 = Client Shape 第四段 — IndexedDB first-paint cache (CS-1=三栏 / CS-2=故障 UX / CS-3=PWA install + Push)

> ⚠️ Wrapper milestone — 复用 RT-1 cursor + DM-3 useDMSync 既有, 仅落 client-only
> **IndexedDB store wrapper + first-paint message cache + cursor sync 触发 + 乐观缓存非权威 disclaimer**.
> **0 server prod + 0 schema 改 + 0 新 endpoint** — 跟 CS-1/CS-2/CS-3 / CV-9..14 / DM-5..6 / DM-9 / CHN-11..12 同模式.

## 0. 关键约束 (3 条立场)

1. **IndexedDB store schema byte-identical 跟蓝图 §1.4 表** (3 store 拆死, 不漂第 4): `lib/cs4-idb.ts` 单源 — `messages` (channel-id indexed, 最近 N=200/channel) + `last_read_at` (per-channel cursor) + `agent_state` (presence cache TTL 30s); 反约束: **不允许 typing / presence-realtime 入 IndexedDB** (蓝图 §1.4 字面 "typing/presence 等真正实时数据 必须从 server 实时拉" 字面承袭); **不允许 artifact 内容 / DM body** 入 IndexedDB (草稿走 localStorage CV-10 既有, 不漂); 反向 grep `idb.*put.*typing\|idb.*put.*presence_realtime` count==0; DB version=1, schema 改 = bump version + onupgradeneeded migration (跟 server schema_migrations 同精神).

2. **乐观缓存非权威 disclaimer + cursor sync 触发** (蓝图 §1.4 字面 "缓存非权威, server cursor 增量同步是真相"): `useFirstPaintCache(channelID)` hook 返 `{cachedMessages, syncing, synced}` — mount 时 IndexedDB.get(`messages:${channelID}`) 返 cached + 同时触发 server cursor backfill (走 RT-1 既有 `?cursor=` API), confirm 后 IndexedDB.put 覆盖; 反约束: **不允许 IndexedDB miss 时阻塞 UI** (cached=null → 直接走 server fetch, 跟 sync 串行不阻塞); **不允许 IndexedDB write 不带 cursor** (反向 grep `idb\.put\(.*messages.*\)\s*$` 必带 cursor key, 跟 server cursor opaque 同源); 文案 byte-identical: `离线模式` (offline + cache hit) / `已同步` (online + sync done) / `同步中…` (sync in-flight, ≤3s 内不显示, 跟 RT-1 §1.1 沉默胜于假 loading 字面承袭).

3. **0 server prod + 0 schema + RT-1 cursor 复用 byte-identical** (Wrapper 立场 同 CS-1/CS-2/CS-3): server diff 0 行 (`git diff origin/main -- packages/server-go/` count==0); 不引入 cs_4 命名 server file (反向 grep `migrations/cs_4\|cs4.*api\|cs4.*server` count==0); 复用 RT-1 cursor `?cursor=` URL byte-identical 不改 lib; 反向 grep `cs4.*newCursor\|CS4CursorHelper` count==0 (走 既有 cursor 单源); 文案禁同义词漂 (`本地缓存` / `离线缓存` / `已加载` 在 user-visible 0 hit, 跟蓝图字面 `离线模式` / `已同步` byte-identical).

## 1. 拆段实施 (3 段, 一 milestone 一 PR)

| 段 | 文件 | 范围 |
|---|---|---|
| **CS-4.1** IndexedDB wrapper SSOT + 3 store schema | `packages/client/src/lib/cs4-idb.ts` (新, ≤120 行) — `openCS4DB()` 单源 IDBOpenDBRequest + onupgradeneeded 建 3 store (`messages` keyPath=`id` index `channel_id`, `last_read_at` keyPath=`channel_id`, `agent_state` keyPath=`agent_id`) + `cs4Get/Put/Delete` typed wrappers + `clearStaleEntries(maxAgeMs)` cleanup helper; `lib/cs4-sync-state.ts` (新, ≤30 行) — `SyncState` enum (`offline_cache_hit` / `synced` / `syncing` / `cache_miss`) + 文案 byte-identical labels; 8 vitest (TestCS41_DBOpensWithSchema + 3StoreCreated + cs4Get/Put roundtrip + ClearStale + SyncStateLabels_ByteIdentical + NoTypingPresenceDrift) | 战马D |
| **CS-4.2** useFirstPaintCache hook + cursor sync wire | `lib/use_first_paint_cache.ts` (新, ≤80 行) — React hook returning `{cachedMessages, syncState}` + mount 时 IDB.get + 同时 fetch server `?cursor=last_known` (走 RT-1 既有 lib) + confirm 后 IDB.put 覆盖 + offline 时 (navigator.onLine=false) skip server fetch 走 cache hit; `components/SyncStatusIndicator.tsx` (新, ≤50 行) — DOM `data-cs4-sync-state` 文案 byte-identical (`离线模式` / `已同步` / `同步中…` ≥3s 内不渲染); 12 vitest (4 hook + 4 indicator + 4 反向 grep) | 战马D |
| **CS-4.3** closure | REG-CS4-001..006 + acceptance + content-lock + PROGRESS [x] CS-4 + 4 件套 + docs/current sync (`docs/current/client/idb-cache.md` ≤80 行 — 3 store + sync state + 文案 byte-identical) + e2e (`packages/e2e/tests/cs-4-idb-cache.spec.ts` 4 case: cache hit first-paint / sync confirm overwrite / offline mode label / cache miss fallback) | 战马D / 烈马 |

## 2. 反向 grep 锚 (5 反约束, count==0)

```bash
# 1) 0 server 改 (Wrapper 立场 ③)
git diff origin/main -- packages/server-go/ | grep -c '^\+'  # 0
# 2) typing/presence-realtime 不入 IDB (蓝图 §1.4 字面)
git grep -nE 'idb.*put.*typing|idb.*put.*presence_realtime' packages/client/src/lib/cs4-idb.ts  # 0 hit
# 3) artifact 内容 / DM body 不入 IDB (草稿走 CV-10 localStorage)
git grep -nE 'idb.*put.*artifact_content|idb.*put.*dm_body' packages/client/src/lib/cs4-idb.ts  # 0 hit
# 4) 不复用 RT-1 cursor 之外的 helper (走单源)
git grep -nE 'cs4.*newCursor|CS4CursorHelper' packages/client/src/  # 0 hit
# 5) 文案 byte-identical (蓝图 vs 同义词漂禁)
git grep -nE '本地缓存|离线缓存|已加载' packages/client/src/lib/cs4-sync-state.ts  # 0 hit
```

## 3. 不在范围 (留账)

- ❌ background sync (蓝图 §1.1 字面承袭 "完整离线 / background sync 不做")
- ❌ artifact 内容 / DM body / 草稿入 IDB (草稿走 CV-10 localStorage 既有)
- ❌ typing / presence-realtime 入 IDB (蓝图 §1.4 字面承袭, 必从 server 实时拉)
- ❌ Service Worker offline page (留 CS-3 PWA + sw.js DL-4 既有)
- ❌ 跨设备同步 (server cursor 是真相, IDB 只 first paint 加速)
- ❌ admin god-mode IDB inspect (永久不挂, ADM-0 §1.3 红线)
- ❌ IDB cleanup goroutine / scheduled job (用户主动 logout 时清, v1 不做 background sweep)

## 4. 跨 milestone byte-identical 锁

- 复用 RT-1 #290 cursor opaque 协议 byte-identical (CS-4 IDB.put cursor key 跟 server `?cursor=` 同源)
- 复用 DM-3 useDMSync 既有 client cursor 同步模式 (CS-4 useFirstPaintCache 同精神)
- 跟 CS-10 草稿 localStorage 拆死 (CS-4 不入草稿域)
- 跟 CS-2 #595 故障三态联动 (failed 时 IDB cache hit + offline label)
- ADM-0 §1.3 admin god-mode 不挂 (CS-4 仅 client 用户视角)
- 0-server-prod 系列模式承袭 (CV-9..14 / DM-5..6 / DM-9 / CHN-11..12 / CS-1/2/3 / CS-4 第 16 处)

## 5. 验收挂钩

- REG-CS4-001..006 (5 反向 grep + 3 store schema 单测 + sync state vitest + e2e 4 case)
- 既有 RT-1 cursor lib + DM-3 useDMSync 全 PASS (Wrapper 复用 不破)
- vitest cs4-idb.test.ts + cs4-sync-state.test.ts + use_first_paint_cache.test.ts + SyncStatusIndicator.test.tsx (≥20 case 全闭)
