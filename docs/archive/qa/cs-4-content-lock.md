# CS-4 文案锁 — IndexedDB 乐观缓存字面 + DOM byte-identical

> **状态**: v0 (野马, 2026-04-30)
> **目的**: CS-4 实施 PR 落 IDB wrapper + sync UI 前锁字面 — 跟蓝图 §1.4 字面 + RT-1 #290 cursor opaque + 沉默胜于假 loading 同源.
> **关联**: spec `cs-4-spec.md` §0 + RT-1 cursor opaque + 蓝图 client-shape.md §1.4.

---

## 1. SyncState enum + 文案 byte-identical 锁 (4-enum)

| state | 文案 byte-identical | 触发条件 | 反约束 |
|---|------|------|------|
| `offline_cache_hit` | `离线模式` | navigator.onLine=false + IDB cached present | 不准 `本地缓存` / `离线缓存` / `已加载` 漂 |
| `synced` | `已同步` | server fetch confirm + IDB.put done | 不准 `已加载` / `加载完成` 漂 |
| `syncing` | `同步中…` | server fetch in-flight ≥3s | ≤3s 不渲染 (沉默胜于假 loading 字面承袭) |
| `cache_miss` | (return null, 不渲染) | 首次访问 channel + IDB.get → null | 不准 `首次加载…` / `准备中` fallback (走 server fetch 直接, 不阻塞) |

---

## 2. DOM 字面锁 (跟 vitest assertion + e2e selector byte-identical)

| 组件 | DOM 锚 | 反约束 |
|---|------|------|
| `SyncStatusIndicator` | `<span data-cs4-sync-state="{offline_cache_hit\|synced\|syncing\|cache_miss}">{label}</span>` | cache_miss 时 return null (不准 fallback toast); syncing 时 ≤3s return null (沉默胜于假 loading); 不准 spinner 旁路 (走 DOM data-attr 单源) |

---

## 3. IDB 3 store schema 字面锁 (跟蓝图 §1.4 表 byte-identical)

| store | keyPath | index | 用途 | 反约束 |
|---|---|---|---|---|
| `messages` | `id` | `channel_id` | 最近 N=200/channel 消息 first paint | 不入 typing / presence-realtime / artifact body / DM body |
| `last_read_at` | `channel_id` | (none) | per-channel cursor + last_read_at 一行 | 不存 cross-channel aggregate |
| `agent_state` | `agent_id` | (none) | presence cache TTL 30s (CS-2 故障三态联动) | TTL 过期清, 不存 typing/presence-realtime |

**反约束 (跟 stance §1 + spec §2 同源)**:
- ❌ 第 4 store 漂入 (3 store 严锁)
- ❌ typing / presence-realtime 入 IDB (蓝图 §1.4 字面 "typing/presence 必从 server 实时拉")
- ❌ artifact 内容 / DM body / 草稿入 IDB (草稿走 CV-10 localStorage)
- ❌ schema 改不 bump version + onupgradeneeded migration

---

## 4. 反向 grep 锚 (跟 stance §2 + spec §2 同源)

```bash
# ① typing/presence-realtime 不入 IDB
git grep -nE 'idb.*put.*typing|idb.*put.*presence_realtime' packages/client/src/lib/cs4-idb.ts  # 0 hit
# ② artifact 内容 / DM body 不入 IDB
git grep -nE 'idb.*put.*artifact_content|idb.*put.*dm_body' packages/client/src/lib/cs4-idb.ts  # 0 hit
# ③ IDB.put 必带 cursor key (反向断不带 cursor)
git grep -nE 'idb\.put\(.*messages.*\)\s*$' packages/client/src/lib/  # 0 hit
# ④ 不复用 RT-1 之外 cursor helper
git grep -nE 'cs4.*newCursor|CS4CursorHelper' packages/client/src/  # 0 hit
# ⑤ 同义词反向
git grep -nE '本地缓存|离线缓存|已加载' packages/client/src/lib/cs4-sync-state.ts  # 0 hit
# ⑥ admin god-mode 不挂 (ADM-0 §1.3 红线)
git grep -nE 'admin.*idb|admin.*indexedDB' packages/client/src/  # 0 hit
# ⑦ 0 server 改
git diff origin/main -- packages/server-go/ | grep -c '^\+'  # 0
```

---

## 5. 验收挂钩

- CS-4.1 PR: §1 SyncState 4-enum byte-identical + §3 3 store 字面锁 + 单测 `TestCS41_SyncStateLabels_ByteIdentical` + `TestCS41_3StoreCreated`
- CS-4.2 PR: §2 DOM 锚 + §1 syncing ≥3s delay + vitest literal assert
- CS-4.3 entry 闸: §1+§2+§3+§4 全锚 + 跨 milestone byte-identical (RT-1 cursor + DM-3 useDMSync + CV-10 草稿拆死 + CS-2 故障三态联动 + ADM-0 §1.3)

---

## 6. 不在范围

- ❌ background sync (蓝图 §1.1)
- ❌ Service Worker offline page (CS-3 / DL-4 sw.js 既有)
- ❌ 跨设备同步 (server cursor 是真相)
- ❌ admin god-mode IDB inspect
- ❌ IDB cleanup goroutine

---

## 7. 更新日志

| 日期 | 作者 | 变化 |
|------|------|------|
| 2026-04-30 | 野马 | v0 — CS-4 文案锁 (4 段: SyncState 4-enum 字面 byte-identical 跟蓝图 + DOM 锚 + 3 store schema 字面 + 7 反向 grep). 跟蓝图 client-shape.md §1.4 + RT-1 #290 cursor opaque + 沉默胜于假 loading 同源. 同义词漂禁 3 词 + admin god-mode 反向 + typing/presence-realtime 反向 (蓝图字面拆死). |
