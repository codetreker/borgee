# Acceptance Template — CS-4: IndexedDB 乐观缓存

> Spec: `docs/implementation/modules/cs-4-spec.md` (飞马 + 战马D v0)
> 蓝图: `docs/blueprint/current/client-shape.md` §1.4 (本地持久化乐观缓存 B 路径) + `data-layer.md` §4.A.2 (cursor opaque)
> Stance: `docs/qa/cs-4-stance-checklist.md` (野马 / 飞马 v0)
> 前置: RT-1 #290 cursor opaque ✅ + DM-3 useDMSync ✅ + CS-2 #595 故障三态 (in-flight) + CS-3 #598 PWA (in-flight)
> Owner: 战马D (主战) + 飞马 (spec) + 烈马 (acceptance) + 野马 (文案)

## 验收清单

### 立场 ① — 3 store 拆死 byte-identical (typing/presence-realtime 不入)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 1.1 `lib/cs4-idb.ts` 单源 — 3 store schema (`messages` keyPath=id index channel_id + `last_read_at` keyPath=channel_id + `agent_state` keyPath=agent_id) | unit | 战马D | `cs4-idb.test.ts::TestCS41_DBOpensWithSchema` + `_3StoreCreated` |
| 1.2 cs4Get/Put/Delete typed wrappers — roundtrip + clearStaleEntries cleanup | unit | 战马D | `TestCS41_GetPutRoundtrip` + `TestCS41_ClearStale` |
| 1.3 反向断 typing/presence-realtime 不入 IDB (反向 grep `idb.*put.*typing\|idb.*put.*presence_realtime` 0 hit) | unit | 战马D | `TestCS41_NoTypingPresenceDrift` (filepath.Walk + regex) |
| 1.4 反向断 artifact 内容 / DM body 不入 IDB (草稿走 CV-10 localStorage) | unit | 战马D | `TestCS41_NoArtifactContentOrDMBody` |
| 1.5 DB version=1 + onupgradeneeded migration callback 真触发 | unit | 战马D | `TestCS41_OnUpgradeNeededFires` |

### 立场 ② — 乐观缓存非权威 + cursor sync + ≤3s 不显示 syncing

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 2.1 `useFirstPaintCache(channelID)` hook 返 `{cachedMessages, syncState}` — mount 时 IDB.get 返 cached | vitest | 战马D | `use_first_paint_cache.test.ts::TestCS42_HookReturnsCachedOnMount` |
| 2.2 mount 时同时触发 server `?cursor=` fetch (走 RT-1 既有 lib) + confirm 后 IDB.put 覆盖 | vitest | 战马D | `TestCS42_TriggersServerSyncOnMount` (mock fetch + 验 IDB.put call) |
| 2.3 cache miss 时不阻塞 UI — cached=null → 直接走 server fetch (跟 sync 串行) | vitest | 战马D | `TestCS42_CacheMissNoBlock` |
| 2.4 offline 时 (`navigator.onLine=false`) skip server fetch 走 cache hit | vitest | 战马D | `TestCS42_OfflineSkipsServer` |
| 2.5 sync ≥3s 才显示 `同步中…` (≤3s 沉默胜于假 loading) | vitest | 战马D | `SyncStatusIndicator.test.tsx::TestCS42_SyncingLabelDelayed3s` |
| 2.6 `data-cs4-sync-state` 三态 DOM 字面锁 (`offline_cache_hit` / `synced` / `syncing` / `cache_miss`) | vitest | 战马D | `SyncStatusIndicator.test.tsx::TestCS42_DOMStateAttr` |

### 立场 ③ — 0 server prod + 0 schema + 文案 byte-identical + admin god-mode 不挂

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 3.1 0 server diff (`git diff origin/main -- packages/server-go/` count==0 production lines) | unit | 战马D | `cs4_no_server_diff_test.ts` (filepath.Walk server-go/) |
| 3.2 0 schema 改 (反向 grep `migrations/cs_4\|cs4.*api\|cs4.*server` 在 server-go/internal/ count==0) | unit | 战马D | `TestCS41_NoSchemaChange` |
| 3.3 文案 byte-identical 跟蓝图字面 (`离线模式` / `已同步` / `同步中…`) + 同义词反向 (`本地缓存 / 离线缓存 / 已加载` 0 hit) | unit | 战马D | `cs4-sync-state.test.ts::TestCS41_SyncStateLabels_ByteIdentical` + `_NoSynonymDrift` |
| 3.4 不复用 RT-1 之外 cursor helper (反向 grep `cs4.*newCursor\|CS4CursorHelper` count==0) | unit | 战马D | `TestCS41_NoNewCursorHelper` |
| 3.5 admin god-mode 不挂 (反向 grep `admin.*idb\|admin.*indexedDB` count==0) | unit | 战马D | `TestCS41_NoAdminIDBInspect` |

### 既有 RT-1 cursor / DM-3 useDMSync / CV-10 localStorage 不破

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 4.1 RT-1 cursor lib 字面 byte-identical 不动 (本 PR 仅 import 不改) | unit | 战马D | git diff `packages/client/src/lib/{cursor,api}.ts` 0 行 |
| 4.2 DM-3 useDMSync 字面 byte-identical 不动 | unit | 战马D | git diff DM-3 既有路径 0 行 |
| 4.3 CV-10 localStorage 草稿路径 不动 (CS-4 不入草稿域) | unit | 战马D | git diff CV-10 useArtifactCommentDraft 0 行 |

### e2e (cs-4-idb-cache.spec.ts 4 case)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 5.1 cache hit first-paint — 第二次访问 channel 时 IDB cached messages 立即渲染 (无 spinner) | playwright | 战马D / 烈马 | `cs-4-idb-cache.spec.ts::TestCS4E2E_CacheHitFirstPaint` |
| 5.2 sync confirm overwrite — server cursor backfill 后 IDB 真覆盖 cached + DOM 更新 | playwright | 战马D / 烈马 | `TestCS4E2E_SyncConfirmOverwrite` |
| 5.3 offline mode label — `navigator.onLine=false` 时 SyncStatusIndicator DOM `data-cs4-sync-state="offline_cache_hit"` + 文案 `离线模式` | playwright | 战马D / 烈马 | `TestCS4E2E_OfflineModeLabel` |
| 5.4 cache miss fallback — 首次访问 channel 时 IDB miss → server fetch 不阻塞 + sync done DOM `data-cs4-sync-state="synced"` | playwright | 战马D / 烈马 | `TestCS4E2E_CacheMissFallback` |

## 不在本轮范围 (spec §3 字面承袭)

- ❌ background sync (蓝图 §1.1)
- ❌ artifact 内容 / DM body / 草稿入 IDB (草稿走 CV-10)
- ❌ typing / presence-realtime 入 IDB (蓝图 §1.4)
- ❌ Service Worker offline page (留 CS-3 PWA + sw.js DL-4)
- ❌ 跨设备同步 (server cursor 是真相)
- ❌ admin god-mode IDB inspect (永久不挂 ADM-0 §1.3)
- ❌ IDB cleanup goroutine / scheduled job (留 v1 用户 logout 时清)

## 退出条件

- 立场 ① 1.1-1.5 (3 store schema + helper + 反向断 typing/presence + 反向断 artifact/DM body + onupgradeneeded) ✅
- 立场 ② 2.1-2.6 (useFirstPaintCache + sync trigger + cache miss noblock + offline skip + 3s syncing delay + DOM state attr) ✅
- 立场 ③ 3.1-3.5 (0 server / 0 schema / 文案 byte-identical / no new cursor helper / admin god-mode 不挂) ✅
- 既有 4.1-4.3 (RT-1 / DM-3 / CV-10 不破) ✅
- e2e 5.1-5.4 全 PASS ✅
- REG-CS4-001..006 = **6 行 🟢**

## 更新日志

- 2026-04-30 — 战马D / 飞马 / 烈马 / 野马 v0: CS-4 4 件套 acceptance template, 跟 spec 3 立场 + stance §2 黑名单 grep + 跨 milestone byte-identical (RT-1 #290 cursor + DM-3 useDMSync + CV-10 localStorage 拆死 + CS-2 故障三态联动 + ADM-0 §1.3 红线) 三段对齐. 0 server prod + 0 schema 改 — wrapper milestone 选项 C 同 CS-1/2/3 / CV-9..14 / DM-5..6 / DM-9 模式承袭.
