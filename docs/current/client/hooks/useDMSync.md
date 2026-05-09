# useDMSync hook (DM-3.2) — implementation note

> DM-3.2 (#508) · Phase 5 候选 · 蓝图 [`concept-model.md`](../../blueprint/current/concept-model.md) §1.3 + DM-2 #361/#372/#388 (mention dispatch) + RT-1.3 #296 (cursor backfill) + RT-3 #488 (多端推 + thinking 反约束).

## 1. 设计

agent-DM 多端 owner cursor 同步 — 复用 RT-1.3 既有 sequence + sessionStorage 持久化 (跟 `lastSeenCursor.ts` 同模式), 不开 dm-only WS subscription. monotonic-only persistence 防 cursor regression.

反约束:
- ① DM cursor 复用 RT-1.3 既有 mechanism (不开 `/api/v1/dm/sync` 旁路 endpoint, dm 走 channel events 同 path)
- ② 多端走 RT-3 fan-out (不开 dm-only WS subscription / frame)
- ③ thinking subject 5-pattern 不出现 system DM body (跟 RT-3 #488 byte-identical 沿用)
- ④ useDMSync 复用 `useArtifactUpdated` / `lastSeenCursor` 模式 — 不裂 hook seam
- ⑤ server 0 行新增 (DM-3.1 反约束 grep test 守门, 复用 RT-1.3 events backfill)

## 2. API surface (`packages/client/src/hooks/useDMSync.ts`)

3 export + 1 internal helper + React hook:

| Export | 签名 | 行为 |
|---|---|---|
| `loadDMCursor(dmChannelID)` | `(string) => number` | 读 sessionStorage `borgee.dm3.cursor:<id>`. 缺失/损坏 → 0. 非 finite / 负数 / 空 channelID 全 fall back 到 0. |
| `persistDMCursor(dmChannelID, cursor)` | `(string, number) => number` | monotonic 推进 — 仅当 `cursor > current` 才写, 返实际持久化值. 空 channelID / 非 finite / ≤0 是 no-op (返 current). |
| `useDMSync(dmChannelID)` | `(string) => { lastSeenCursor: number, markSeen: (n) => void }` | React hook — 初始读 sessionStorage, `dmChannelID` 切换时 reload, `markSeen()` 走 persistDMCursor + setState. |
| `__resetDMCursorForTests(dmChannelID)` | `(string) => void` | test-only reset. 不从 barrel 导出. |

## 3. sessionStorage 协议

- **Key**: `borgee.dm3.cursor:<dmChannelID>` (per-DM 隔离, 多端独立 — 设计 ④ 多 device 同 channel cursor 独立).
- **Value**: 单调递增 int64 (10 进制 ASCII), 跟 RT-1.1 server CursorAllocator 同序.
- **Why sessionStorage**: per-tab, 跨 tab 不共 cursor (跟 lastSeenCursor.ts 同精神 — 反 localStorage 全局共享, 反 IndexedDB 重量级).

## 4. monotonic invariant

`persistDMCursor(id, n)` 协议:
- `n <= current` → no-op, 返 `current`
- `n > current` → 写入, 返 `n`
- `!Number.isFinite(n)` / `n <= 0` / `id===""` → no-op, 返 `current` (或 0)

跟 `persistLastSeenCursor` (RT-1.2) 同精神 — server cursor 只增不减, 客户端 reducer monotonic 守.

## 5. test-only reset

`__resetDMCursorForTests(id)` — 测试 between cases 清 sessionStorage 单 entry. 反向: 不从 barrel 导出, 不走 production import path. 跟 lastSeenCursor.ts `__resetLastSeenCursorForTests` 同模式.

## 6. 反约束

- 不订阅 `borgee:dm-sync` / `dmSubscribe` / dm-only frame (反 grep production 0 hit)
- 不存 secret / token
- 不挂 `dm_id` 字段在 cursor (cursor 是单根 sequence, 跟 RT-1 / AL-2b / CV-* / BPP-3.1 共一根)

## 7. 跨 milestone byte-identical 锁

- cursor 跟 RT-1 #290 + AL-2b #481 + CV-* + BPP-3.1 #494 共 sequence
- hook seam 跟 CV-1.3 #346 useArtifactUpdated 同模式
- sessionStorage round-trip 跟 RT-1.2 #292 lastSeenCursor 同精神 (key prefix 不同, key namespace 隔离)
- thinking 5-pattern 跟 RT-3 #488 byte-identical (改 = 改 5+ 处)

## 8. 测试覆盖 (`packages/client/src/__tests__/useDMSync.test.ts`)

5 vitest case PASS:
- ① cold-start (fresh sessionStorage → 0)
- ② monotonic (smaller cursor 不 regress)
- ③ page-reload (sessionStorage 跨 mount survive)
- ④ corrupt-clamp (NaN / Infinity / -1 / 空 id 全 fallback 0)
- ⑤ multi-device (两 dmID 独立 storage key, 互不干扰)
