# BPP-6 spec brief — plugin cold-start handshake + state re-derive (≤80 行)

> 战马D · Phase 5 · ≤80 行 · 蓝图 [`plugin-protocol.md`](../../blueprint/current/plugin-protocol.md) §1.6 (失联与故障状态 — 进程死亡 vs 网络重连) + §2.1 (control-plane handshake). 模块锚 [`plugin-protocol.md`](plugin-protocol.md) §BPP-6. 依赖 BPP-1 #304 envelope (whitelist 14→15) + BPP-3 #489 PluginFrameDispatcher + BPP-5 #503 reconnect_handshake (含 cursor) — BPP-6 是 BPP-5 反向 (无 cursor 路径) + AL-1 5-state #492 (reasons SSOT 单源) + RT-1.3 #296 cursor (cold-start 不复用, 重新发号).

## 0. 关键约束 (3 条立场, 蓝图 §1.6 + AL-1 字面)

1. **cold-start ≠ reconnect** — plugin 进程死亡重启 (state 全丢, 无 last_known_cursor) 跟 BPP-5 reconnect (process 活着 socket 断, 持有 cursor) 是**不同帧**. cold-start handshake = BPP envelope 第 15 frame `cold_start_handshake` (direction lock plugin→server, BPP-1 #304 reflect lint 自动覆盖). server 收此帧**不 expect resume**, 直接 reset agent state. **反约束**: 反向 grep `cold_start.*last_known_cursor|cold_start.*resume|cold_start.*cursor` 0 hit (跟 reconnect_handshake 字段集互斥).

2. **agent state 重新 derive (跟 BPP-5 复用 agent.Tracker.Clear 同模式)** — server 收 cold_start_handshake → ① 调 `agent.Tracker.Clear(agentID)` 清 in-memory state + ② 通过 AL-1 #492 single-gate `AppendAgentStateTransition(any→online, "")` 翻 state-log (state machine 自处理 valid 转 — initial→online / error→online / offline→online 全合法) + ③ 不重放历史 frame (cold-start 是 fresh start, 跟 BPP-5 增量 resume 反向). **反约束**: 反向 grep `cold_start.*replay|cold_start.*backfill|cold_start.*history` 0 hit (跟 RT-1.3 cursor 不分裂同精神, 不为 cold-start 另开 replay 路径).

3. **restart count tracking 仅 audit, 不影响 wire path** — agent_state_log 已有 reason 字段 (AL-1a 6-dict), cold-start 用既有 `runtime_crashed` reason byte-identical (反映上次 error → 此次 cold-start 是 crash 复活); restart 计数走 state-log COUNT(WHERE to_state='online' AND reason='runtime_crashed') 反向 derive, **不另开 plugin_restart_count 列**. **反约束**: 反向 grep `plugin_restart_count|cold_start_count|restart_counter` 0 hit. AL-1 reason 锁链 BPP-6 = 第 11 处单测锁 (BPP-2.2 #485 第 7 + AL-2b #481 第 8 + BPP-4 #499 第 9 + BPP-5 #503 第 10 + BPP-6 第 11).

## 1. 拆段 (一 milestone 一 PR, 整段一次合 — 跟 BPP-2/3/4/5 协议同源)

| 段 | 文件 | 范围 |
|---|---|---|
| BPP-6.1 frame schema | `internal/bpp/envelope.go` 改 (+ ColdStartHandshakeFrame 5 字段 `{type, plugin_id, agent_id, restart_at, last_error_reason}` + whitelist 14→15) + `internal/bpp/cold_start_handshake_test.go` 新 (4 unit: 字段顺序锁 / direction lock plugin→server / 无 cursor 字段反断 / Frame body 反向 grep 反约束 §0.1 守) | envelope 第 15 frame, BPP-1 #304 reflect lint 自动扩 14→15 |
| BPP-6.2 server handler + state reset | `internal/bpp/cold_start_handler.go` 新 (PluginFrameDispatcher 注册 `cold_start_handshake` → ① agent.Tracker.Clear → ② AL-1 single-gate AppendAgentStateTransition → ③ audit log w/ restart_at) + 6 unit (initial→online / error→online / offline→online / cross-owner reject / nil-safe / 反向不调 ResolveResume) | BPP-3 dispatcher 复用, 不开新 ws hub method, 不重放历史 |
| BPP-6.3 e2e + REG-BPP6 + acceptance + PROGRESS [x] + closure | `packages/e2e/tests/bpp-6-cold-start.spec.ts` 新 (kill plugin process 真死 → restart plugin → cold_start_handshake → agent UI 直接翻 online 不带历史 thinking) + REG-BPP6-001..006 + acceptance/bpp-6.md + docs/current sync | restart 计数从 state-log COUNT 反向 derive 验证 |

## 2. 留账边界

- **cross-plugin restart takeover** (留 v2) — plugin A 死后 plugin B (新 binary) 接管同 agent_id, 反约束: BPP-6 仅同 plugin_id 路径接 cold_start, 跨 plugin_id reject + log warn `bpp.cold_start_cross_plugin_reject`
- **plugin restart 频繁触发 alert** (留 ADM 监控 v2) — server 不挂 rate limit, BPP-6.2 仅 audit log; 阈值告警留 ADM 监控层
- **plugin SDK side restart backoff** (留 plugin SDK) — 跟 BPP-4 #499 §0.3 + BPP-5 #503 §0.2 best-effort 立场承袭
- **state machine 加新态 cold_starting** — 跟 BPP-5 connecting 中间态 deferred 同精神, AL-1 5-state 不为 cold-start 另开 transient 中间态 (any→online single-gate 直翻)

## 3. 反查 grep 锚 (Phase 5 验收 + BPP-6 实施 PR 必跑)

```
git grep -nE 'FrameTypeBPPColdStartHandshake' packages/server-go/internal/bpp/   # ≥ 1 hit (whitelist + handler register)
git grep -nE 'ColdStartHandshakeFrame' packages/server-go/internal/bpp/          # ≥ 1 hit (struct + register)
# 反约束 (6 条 0 hit)
git grep -nE 'cold_start.*last_known_cursor|cold_start.*resume|cold_start.*cursor' packages/server-go/   # 0 hit (与 reconnect 字段互斥, §0.1)
git grep -nE 'cold_start.*replay|cold_start.*backfill|cold_start.*history' packages/server-go/   # 0 hit (不重放历史, §0.2)
git grep -nE 'plugin_restart_count|cold_start_count|restart_counter' packages/server-go/   # 0 hit (count 反向 derive, §0.3)
git grep -nE 'admin.*cold_start.*handshake|admin.*BPP6' packages/server-go/internal/api/admin*.go   # 0 hit (ADM-0 §1.3 红线)
git grep -nE 'pendingColdStart|coldStartQueue|deadLetterColdStart' packages/server-go/internal/   # 0 hit (跟 BPP-4/5 best-effort 立场承袭)
git grep -nE 'StateColdStarting|state.*= "cold_starting"' packages/server-go/internal/   # 0 hit (不为 cold-start 另开 transient 态)
```

## 4. 不在本轮范围 (反约束 deferred)

- ❌ cross-plugin restart takeover (v2, 跟 §2 留账同源)
- ❌ plugin SDK 端 restart backoff / leader election (留 SDK 层)
- ❌ server-side restart rate limit / pendingColdStart queue (跟 BPP-4 §0.3 + BPP-5 §0.2 best-effort 立场承袭)
- ❌ AL-1 6-dict 扩第 7 reason (字典分立反约束 — cold-start 复用 `runtime_crashed` byte-identical, 反映 crash 复活语义)
- ❌ AL-1 5-state graph 加 cold_starting 中间态 (跟 BPP-5 connecting deferred 同精神, single-gate any→online 直翻)
- ❌ admin god-mode 走 cold_start 路径 (ADM-0 §1.3 红线)
- ❌ frame body 携带 cursor / replay / history 字段 (BPP-5 reconnect 路径独占, 字段集互斥反向 grep 守)
