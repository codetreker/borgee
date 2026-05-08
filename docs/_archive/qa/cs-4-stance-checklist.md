# CS-4 立场反查表 (IndexedDB 乐观缓存)

> **状态**: v0 (野马 / 飞马, 2026-04-30)
> **目的**: CS-4 实施 PR 直接吃此表为 acceptance — 战马D 实施 + 烈马 acceptance + 飞马 spec brief 反查立场漂移.
> **关联**: 蓝图 `client-shape.md` §1.4 (本地持久化乐观缓存); RT-1 #290 cursor opaque + DM-3 useDMSync; CS-1/2/3 spec §3 留账 byte-identical.
> **依赖**: 0 server / 0 schema (Wrapper 选项 C 同 CS-1/2/3 模式承袭).

---

## 1. CS-4 立场反查表 (3 立场)

| # | 立场锚 | 一句话立场 | 反约束 (X 是, Y 不是) | v0 / v1 |
|---|--------|----------|----------------------|---------|
| ① | client-shape §1.4 IndexedDB 表 + cursor opaque 数据通路 | **3 store 拆死 byte-identical 跟蓝图表 — `messages` / `last_read_at` / `agent_state`, typing + presence-realtime 必从 server 实时拉不入 IDB** | **是** `lib/cs4-idb.ts` 单源 — 3 store schema (`messages` keyPath=id index channel_id N≤200/channel + `last_read_at` keyPath=channel_id 1 row/channel cursor + `agent_state` keyPath=agent_id TTL 30s); DB version=1, schema 改 = bump version + onupgradeneeded migration; **不是** typing/presence-realtime 入 IDB (蓝图 §1.4 字面 "typing/presence 等真正实时数据 必须从 server 实时拉"; 反向 grep `idb.*put.*typing\|idb.*put.*presence_realtime` 0 hit); **不是** artifact 内容 / DM body / 草稿入 IDB (草稿走 CV-10 localStorage 既有, 字面拆死); **不是** 第 4 store 漂入 (3 store 严锁) | v0/v1 永久锁 — 蓝图 §1.4 字面拆死 (typing/presence 不入是数据通路红线) |
| ② | client-shape §1.4 + RT-1 #290 cursor opaque + 沉默胜于假 loading | **乐观缓存非权威 — server cursor sync 是真相, ≤3s 内不显示 syncing label, IDB miss 时不阻塞 UI** | **是** `useFirstPaintCache` hook — mount 时 IDB.get 返 cached + 同时触发 `?cursor=` server fetch (走 RT-1 既有 lib) + confirm 后 IDB.put 覆盖; cache miss 时不阻塞 UI 直接走 server fetch (跟 sync 串行); offline 时 (`navigator.onLine=false`) skip server fetch 走 cache hit; sync ≥3s 才显示 `同步中…` (跟 RT-1 §1.1 沉默胜于假 loading 字面承袭); **不是** IDB.put 不带 cursor key (反向 grep `idb\.put\(.*messages.*\)` 必带 cursor); **不是** cache hit 阻塞等 server confirm (UX 红线 — 1st paint 必走 cache); **不是** server fetch fail 时清 IDB (跟 CS-2 #595 故障三态联动, failed 时 IDB cache hit 是 graceful fallback) | v0: messages first paint cache; v1: agent_state TTL eviction + IDB cleanup |
| ③ | 0-server-prod 选项 C + 蓝图字面 `离线模式` / `已同步` byte-identical | **0 server prod + 0 schema + RT-1 cursor 复用 byte-identical + 文案 byte-identical 跟蓝图字面 + admin god-mode 不挂** | **是** server diff 0 行 (`git diff origin/main -- packages/server-go/` count==0); 不引入 cs_4 命名 server file (反向 grep `migrations/cs_4\|cs4.*api\|cs4.*server` count==0); 复用 RT-1 #290 cursor lib byte-identical 不改; 文案 byte-identical 跟蓝图 §1.4 (`离线模式` / `已同步` / `同步中…`); **不是** 同义词漂 (`本地缓存` / `离线缓存` / `已加载` 在 user-visible 0 hit); **不是** 走 cs4-newCursor 等并行 helper (反向 grep `cs4.*newCursor\|CS4CursorHelper` 0 hit); **不是** admin god-mode IDB inspect (永久不挂 ADM-0 §1.3 红线; 反向 grep `admin.*idb\|admin.*indexedDB` 0 hit) | v0/v1 永久锁 — 0-server-prod + plain language byte-identical 是 wrapper 立场 |

---

## 2. 黑名单 grep — CS-4 实施 PR merge 后跑, 全部预期 0 命中

```bash
# 立场 ① — typing/presence-realtime 不入 IDB (蓝图字面)
git grep -nE 'idb.*put.*typing|idb.*put.*presence_realtime' packages/client/src/lib/cs4-idb.ts  # 0 hit
# 立场 ① — artifact 内容 / DM body 不入 IDB (草稿走 CV-10)
git grep -nE 'idb.*put.*artifact_content|idb.*put.*dm_body' packages/client/src/lib/cs4-idb.ts  # 0 hit
# 立场 ② — IDB.put 必带 cursor key (反向断不带 cursor)
git grep -nE 'idb\.put\(.*messages.*\)\s*$' packages/client/src/lib/  # 0 hit
# 立场 ③ — 0 server 改
git diff origin/main -- packages/server-go/ | grep -c '^\+'  # 0 production lines
# 立场 ③ — 0 schema 改
git grep -nE 'migrations/cs_4|cs4.*api|cs4.*server' packages/server-go/internal/  # 0 hit
# 立场 ③ — 不复用 RT-1 之外 cursor helper
git grep -nE 'cs4.*newCursor|CS4CursorHelper' packages/client/src/  # 0 hit
# 立场 ③ — 同义词反向
git grep -nE '本地缓存|离线缓存|已加载' packages/client/src/lib/cs4-sync-state.ts  # 0 hit
# 立场 ③ — admin god-mode 不挂
git grep -nE 'admin.*idb|admin.*indexedDB' packages/client/src/  # 0 hit
```

---

## 3. 不在 CS-4 范围 (避免 PR 膨胀, 跟 spec §3 同源)

- ❌ background sync (蓝图 §1.1 字面承袭)
- ❌ artifact 内容 / DM body / 草稿入 IDB (草稿走 CV-10 localStorage 既有)
- ❌ typing / presence-realtime 入 IDB (蓝图 §1.4 字面承袭)
- ❌ Service Worker offline page (留 CS-3 PWA + sw.js DL-4 既有)
- ❌ 跨设备同步 (server cursor 是真相)
- ❌ admin god-mode IDB inspect (永久不挂 ADM-0 §1.3)
- ❌ IDB cleanup goroutine / scheduled job (用户主动 logout 时清, v1 不做 background sweep)

---

## 4. 验收挂钩

- CS-4.1 PR: 立场 ①③ — 3 store schema + cs4Get/Put helper + sync-state 4-enum + 8 vitest (TestCS41_*)
- CS-4.2 PR: 立场 ②③ — useFirstPaintCache hook + SyncStatusIndicator + 12 vitest (TestCS42_*)
- CS-4.3 entry 闸: 立场 ①-③ 全锚 + §2 黑名单 grep 全 0 + 跨 milestone byte-identical (RT-1 #290 cursor + DM-3 useDMSync + CS-2 故障三态联动 + ADM-0 §1.3 红线) + REG-CS4-001..006 全 🟢 + e2e 4 case PASS

---

## 5. 更新日志

| 日期 | 作者 | 变化 |
|------|------|------|
| 2026-04-30 | 野马 / 飞马 | v0, 3 立场 (3 store 拆死 typing/presence 不入 / 乐观缓存非权威 cursor sync ≤3s 不显示 / 0-server-prod 选项 C + 文案 byte-identical) 承袭蓝图 §1.4 IndexedDB 表字面 + RT-1 cursor opaque + 沉默胜于假 loading. 8 行反向 grep (含 admin god-mode 反向第 8 锚) + 7 项不在范围 + 验收挂钩三段对齐. 命名澄清: CS-4 = §1.4 (CS-1=三栏 / CS-2=故障 UX / CS-3=PWA + Push), 跟 CS-1/2/3 spec §3 留账 byte-identical. 0 server / 0 schema wrapper 模式同 CS-1/2/3 / CV-9..14 / DM-5..6 / DM-9. |
