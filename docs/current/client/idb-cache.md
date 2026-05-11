# CS-4 IndexedDB 乐观缓存 (client)

> 出处: `docs/blueprint/current/client-shape.md` §1.4 (本地持久化乐观缓存 B 路径) + `data-layer.md` §4.A.2 (cursor opaque) + `docs/implementation/modules/cs-4-spec.md` v0
> 范围: 仅覆盖 CS-4 客户端 IndexedDB 乐观缓存；不修改 server production code 或 schema。

## IDB wrapper 定义 (lib/cs4-idb.ts)

```ts
const DB_NAME = 'borgee-cs4';
const DB_VERSION = 1;

export const STORE_MESSAGES = 'messages';        // keyPath=id, index channel_id
export const STORE_LAST_READ_AT = 'last_read_at'; // keyPath=channel_id
export const STORE_AGENT_STATE = 'agent_state';   // keyPath=agent_id
```

3 个 store 与蓝图 §1.4 表保持一致。

| 数据域 | 存储策略 | 约束来源 |
|---|---|---|
| messages / last_read_at / agent_state | 写入 CS-4 IndexedDB stores | 蓝图 §1.4 表 |
| typing / presence-realtime | 必须从 server 实时拉取，不入 IDB | 蓝图明确要求与缓存域区分 |
| artifact 内容 / DM body / 草稿 | 继续走 CV-10 localStorage 既有路径 | 与 CS-4 缓存域明确区分 |

API: `openCS4DB()` / `cs4Get` / `cs4Put` / `cs4Delete` / `clearStaleEntries(maxAgeMs)`.

DB version=1。schema 改动必须 bump version，并在 `onupgradeneeded` 中补 migration；治理方式与 server schema_migrations 同精神。

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

这些 label 必须与蓝图 §1.4 字面一致。**改 = 改两处 + content-lock §1**。

## useFirstPaintCache hook (lib/use_first_paint_cache.ts)

```ts
export function useFirstPaintCache(
  channelID: string,
  cursorBackfillFn: (sinceCursor: string | null) => Promise<CachedMessage[]>,
): { cachedMessages: CachedMessage[] | null; syncState: SyncState };
```

| 场景 | 行为 |
|---|---|
| mount | `IDB.get` 返回 cached，同时触发调用方提供的 `cursorBackfillFn(sinceCursor)` server fetch |
| server confirm | `IDB.put` 覆盖缓存 |
| cache miss | 不阻塞 UI；`cached=null` 后直接走 server fetch，并与 sync 串行 |
| offline (`navigator.onLine=false`) | skip server fetch，走 cache hit |
| cursor 补齐 | 调用方传入 `cursorBackfillFn`，复用 RT-1 既有 lib；CS-4 不指定具体 import path |

## SyncStatusIndicator UI (components/SyncStatusIndicator.tsx)

DOM: `<span data-cs4-sync-state="{4-enum}">{label}</span>`

| syncState | UI 行为 |
|---|---|
| `cache_miss` | `return null` |
| `syncing` ≤3s | `return null`；3 秒内不显示 loading，避免展示未确认进度，与 RT-1 §1.1 一致 |
| `syncing` ≥3s | 显示 `同步中…` |

## 禁止行为 / QA 检查

| 约束 | 检查 |
|---|---|
| typing/presence-realtime 不入 IDB | `idb.*put.*typing|idb.*put.*presence_realtime` 无匹配 |
| artifact / DM body 不入 IDB | `idb.*put.*artifact_content|idb.*put.*dm_body` 无匹配 |
| 不复用 RT-1 之外 cursor helper | `cs4.*newCursor|CS4CursorHelper` 无匹配 |
| 禁止引入未锁定的缓存状态文案 | `本地缓存|离线缓存|已加载|加载完成|准备中` 无匹配 |
| 不提供管理端 IDB 查看入口 (ADM-0 §1.3) | `admin.*idb|admin.*indexedDB` 无匹配 |
| 不修改 server production code | `git diff origin/main -- packages/server-go/` 0 行 |
| 不修改 schema | `migrations/cs_4|cs4.*api|cs4.*server` 无匹配 |

## 跨模块一致性要求

| 来源 | 锁定点 |
|---|---|
| RT-1 #290 cursor opaque | CS-4 `IDB.put` cursor key 跟 server `?cursor=` 同源 |
| DM-3 useDMSync | 既有 client cursor 同步同模式 |
| CV-10 草稿 localStorage | 明确区分；CS-4 不入草稿域 |
| CS-2 #595 故障三态联动 | failed 时 IDB cache hit + offline label graceful fallback |
| ADM-0 §1.3 | 不提供管理端 IDB 查看入口 |

## 不在范围

- background sync (蓝图 §1.1)
- artifact 内容 / DM body / 草稿入 IDB (草稿走 CV-10)
- typing / presence-realtime 入 IDB (蓝图 §1.4)
- Service Worker offline page (由 CS-3 PWA + sw.js DL-4 覆盖)
- 跨设备同步 (server cursor 为权威来源)
- 管理端 IDB inspect 入口 (不纳入本范围)
- IDB cleanup goroutine / scheduled job (由 v1 用户 logout 流程清理)
