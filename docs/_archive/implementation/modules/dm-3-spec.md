# DM-3 — agent-DM 多端同步 spec brief (战马D v0)

> 战马D · 2026-04-29 · ≤80 行 · Phase 5 候选 (DM-2 续, RT-3 复用)
> 关联: DM-2 #361/#372/#388 (mention dispatch + agent task notify) ✅ / RT-3 #488 (多端推 + thinking subject 反约束) ✅ / RT-1.3 #296 (cursor backfill) ✅ / BPP-5 #503 (server reconnect cursor, 同精神 client 侧) 🔄
> Owner: TBD 主战 (战马D 起 spec, 实施战马 接力)

---

## 0. 立场 (3 项)

### ① DM cursor 复用 RT-1.3 既有 mechanism (不另起序列)
- 当前: RT-1.3 #296 server `events_backfill` endpoint + client `last-seen-cursor` round-trip 已锁 (REG-RT1-006..010 全 🟢)
- 多端 owner DM 同步走同 cursor — agent-DM 是普通 channel 的子集 (DM channel.type='dm'), `since` cursor 共序锁 (跟 RT-1 + AL-2b ack + CV-* + BPP-3.1 共一根 sequence 同精神)
- 反约束: 不开 `/api/v1/dm/sync` 旁路 endpoint (反 grep ≥1 守, dm 走 channel events 同 path)

### ② 多端同步走 RT-3 多端推 (不另起 channel)
- RT-3 #488 已实现 owner 多端 fan-out (presence sessions 多端 active, frame 推到所有 active conn)
- DM-3 不开 dm-only WS subscription — owner DM 行为 = 普通 channel 行为, RT-3 push 路径 byte-identical
- 反约束: 不开 dm_session_changed / dm_multi_device_sync 专属 frame (反 envelope whitelist 加 hit count==0)

### ③ thinking subject 反约束 (RT-3 已锁, DM-3 延伸)
- RT-3 #488 已锁 5-pattern: `processing` / `responding` / `thinking` / `analyzing` / `planning` 在 system DM 文案不出现 (反"假 loading"立场)
- DM-3 延伸: agent-DM 多端同步推送时 system DM 加固字面锁 — 跨设备显示同一 cursor, 不显 "agent thinking..." 等模糊态 (跟 §11 沉默胜于假 loading 同源)
- 反约束: DM-3.1 server push frame body 反向 grep `thinking|processing|analyzing|planning|responding` count==0 (跟 RT-3 #488 同源)

---

## 1. 拆 ≤3 段

### DM-3.1 — server cursor sync endpoint (复用 RT-1.3)
- 不新增 endpoint — `GET /api/v1/events/backfill?since=<cursor>` 既有路径 dm channel events 一起返
- agent-DM scope: GET /api/v1/channels/{id}/messages?since=N 走 RT-1.3 cursor (channel.type='dm' 同 path)
- 反约束: 不挂 `dm_id` 字段在 cursor (cursor 是单根 sequence, 跟 RT-1 同源)
- 单测: TestDM31_BackfillIncludesDMChannel + TestDM31_CursorMonotonicAcrossDM
- 预计 0 行 server 代码新增 (反约束 grep + 既有 path 验证)

### DM-3.2 — client SPA 多端 cursor sync hook
- `useDMSync(dmChannelID)` hook — 复用 `useArtifactUpdated` / `useLastSeenCursor` 既有 path
- sessionStorage `dm:<channel-id>:cursor` round-trip (跟 CV-1.3 `last-seen-cursor.test.ts` 同模式 5 vitest)
- 反约束: 不订阅 dm-only frame (反 grep `borgee:dm-sync` 0 hit)
- vitest: useDMSync.test.ts (5 case: cold-start 0-call / monotonic / page-reload / corrupt-clamp / multi-device sync)
- 文案锁: 多端同步 toast 静默 (无 "已同步到其他设备" 模糊文案, 立场 ③ 沉默胜过)

### DM-3.3 — e2e + closure
- e2e `dm-3-multi-device-sync.spec.ts`: 双 tab 同 owner 同 agent-DM, tab A 发消息, tab B ≤3s 收 (跟 RT-1.2 ≤3s 硬条件同源)
- 反约束 e2e: tab B 不显 "agent thinking..." 等 thinking subject 文案 (RT-3 锚)
- closure: REG-DM3-001..005 + 烈马 acceptance signoff

---

## 2. 不在范围 (留账)

- agent-DM e2ee (出范围) / DM 跨 org (AP-3 prerequisite) / dm-channel 列表多端排序状态同步 (CHN-3 layout) / offline DM 队列 (DL-4 已盖)

---

## 3. 跨 milestone byte-identical 锁

- DM-3.1 cursor 跟 RT-1 #290 + AL-2b #481 + CV-* + BPP-3.1 #494 共一根 sequence (BPP-1 #304 envelope reflect 覆盖)
- DM-3.2 useDMSync 跟 CV-1.3 #346 useArtifactUpdated 同模式 hook seam
- DM-3.3 ≤3s 硬条件跟 RT-1.2 #292 / G3.1 烈马 signoff 同源
- thinking subject 5-pattern 跟 RT-3 #488 byte-identical (改 = 改 5+ 处)

---

## 4. 验收挂钩 (REG-DM3-001..005 占号)

| ID | 锚 | Test |
|---|---|---|
| 001 | DM cursor 复用 RT-1.3, 不开旁路 | server unit + 反向 grep |
| 002 | useDMSync hook 复用既有 path | vitest 5 case |
| 003 | 多端 ≤3s 收, RT-3 fan-out path | e2e dual-tab |
| 004 | thinking subject 反约束 (5 pattern 0 hit) | 反向 grep + e2e |
| 005 | sessionStorage cursor round-trip | vitest cold-start/monotonic |
