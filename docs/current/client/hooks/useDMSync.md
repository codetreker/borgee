# useDMSync hook (DM-3.2) — implementation note

> DM-3.2 (#508) · Phase 5 候选 · 蓝图 [`concept-model.md`](../../blueprint/current/concept-model.md) §1.3 + DM-2 #361/#372/#388 (mention dispatch) + RT-1.3 #296 (cursor backfill) + RT-3 #488 (多端推送 + thinking 限制).

## 1. 设计

agent-DM 多端 owner cursor 同步：复用 RT-1.3 已有的 sequence 和 sessionStorage 持久化方式（与 `lastSeenCursor.ts` 同模式），不新增只服务 DM 的 WebSocket subscription。持久化只允许 cursor 增大，防止 cursor 回退。

限制:

- ① DM cursor 复用 RT-1.3 已有机制（不开 `/api/v1/dm/sync` 旁路 endpoint，DM 走 channel events 同一路径）
- ② 多端同步走 RT-3 fan-out（不开 dm-only WebSocket subscription / frame）
- ③ thinking subject 5-pattern 不出现 system DM body（与 RT-3 #488 保持逐字一致）
- ④ useDMSync 复用 `useArtifactUpdated` / `lastSeenCursor` 模式，不拆出新的 hook 边界
- ⑤ server 0 行新增（DM-3.1 限制由 grep test 守住，复用 RT-1.3 events backfill）

## 2. API surface (`packages/client/src/hooks/useDMSync.ts`)

3 export + 1 internal helper + React hook:

| Export                                 | 签名                                                            | 行为                                                                                                               |
| -------------------------------------- | --------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------ |
| `loadDMCursor(dmChannelID)`            | `(string) => number`                                            | 读 sessionStorage `borgee.dm3.cursor:<id>`。缺失或损坏时返回 0；非 finite、负数、空 channelID 也返回 0。           |
| `persistDMCursor(dmChannelID, cursor)` | `(string, number) => number`                                    | 单调推进：仅当 `cursor > current` 才写入，并返回实际持久化值。空 channelID、非 finite、≤0 时不写入，返回 current。 |
| `useDMSync(dmChannelID)`               | `(string) => { lastSeenCursor: number, markSeen: (n) => void }` | React hook：初始读 sessionStorage，`dmChannelID` 切换时重新加载，`markSeen()` 调用 persistDMCursor 后再 setState。 |
| `__resetDMCursorForTests(dmChannelID)` | `(string) => void`                                              | 测试专用 reset。不从 barrel 导出。                                                                                 |

## 3. sessionStorage 协议

- **Key**: `borgee.dm3.cursor:<dmChannelID>` (per-DM 隔离, 多端独立 — 设计 ④ 多 device 同 channel cursor 独立).
- **Value**: 单调递增 int64 (10 进制 ASCII), 跟 RT-1.1 server CursorAllocator 同序.
- **Why sessionStorage**: per-tab，跨 tab 不共享 cursor（与 lastSeenCursor.ts 的取舍一致：避免 localStorage 全局共享，也不引入 IndexedDB）。

## 4. 单调递增约束

`persistDMCursor(id, n)` 协议:

- `n <= current` → 不写入，返回 `current`
- `n > current` → 写入, 返 `n`
- `!Number.isFinite(n)` / `n <= 0` / `id===""` → 不写入，返回 `current`（或 0）

与 `persistLastSeenCursor` (RT-1.2) 的规则一致：server cursor 只增不减，客户端 reducer 也按单调递增处理。

## 5. test-only reset

`__resetDMCursorForTests(id)` 用于在测试 case 之间清理单个 sessionStorage entry。限制：不从 barrel 导出，不走 production import path；与 lastSeenCursor.ts `__resetLastSeenCursorForTests` 同模式。

## 6. 限制

- 不订阅 `borgee:dm-sync` / `dmSubscribe` / dm-only frame（production grep 应为 0 hit）
- 不存 secret / token
- 不挂 `dm_id` 字段在 cursor (cursor 是单根 sequence, 跟 RT-1 / AL-2b / CV-\* / BPP-3.1 共一根)

## 7. 跨 milestone 逐字一致范围

- cursor 跟 RT-1 #290 + AL-2b #481 + CV-\* + BPP-3.1 #494 共 sequence
- hook 边界跟 CV-1.3 #346 useArtifactUpdated 同模式
- sessionStorage round-trip 跟 RT-1.2 #292 lastSeenCursor 同规则（key prefix 不同，key namespace 隔离）
- thinking 5-pattern 跟 RT-3 #488 逐字一致（改动时需要同步 5+ 处）

## 8. 测试覆盖 (`packages/client/src/__tests__/useDMSync.test.ts`)

5 vitest case PASS:

- ① cold-start (fresh sessionStorage → 0)
- ② monotonic (smaller cursor 不回退)
- ③ page-reload (sessionStorage 跨 mount 保留)
- ④ corrupt-clamp (NaN / Infinity / -1 / 空 id 全返回 0)
- ⑤ multi-device (两 dmID 独立 storage key, 互不干扰)
