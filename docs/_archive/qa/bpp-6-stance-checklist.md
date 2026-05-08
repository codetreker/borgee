# BPP-6 立场反查清单 (战马D v0)

> 战马D · 2026-04-29 · 立场 review checklist (跟 BPP-5 #503 stance + BPP-4 #499 同模式)
> **目的**: BPP-6 三段实施 (BPP-6.1 frame schema / 6.2 server handler / 6.3 closure) PR review 时, 飞马/野马/烈马按此清单逐立场 sign-off, 反向断言代码层守住每条立场.
> **关联**: spec `docs/implementation/modules/bpp-6-spec.md` (战马D v0 5a29eb8) + acceptance `docs/qa/acceptance-templates/bpp-6.md` (战马D v0)
> **不需 content-lock** — server-only (无 client UI 文案锁), 跟 BPP-3 / BPP-4 / BPP-5 同模式 (audit log key + 常量字面已在 spec §0+§3 锁).

## §0 立场总表 (3 立场 + 4 蓝图边界)

| # | 立场 | 蓝图字面 | 反约束 (代码层守门) |
|---|---|---|---|
| ① | cold-start ≠ reconnect — BPP envelope **第 15 frame** `cold_start_handshake` (direction lock plugin→server, 字段集**与 ReconnectHandshakeFrame 互斥** — 不带 cursor / 不 expect resume) | plugin-protocol.md §1.6 + §2.1 connect 路径承袭 (process 死亡 vs 网络重连) | `bppEnvelopeWhitelist` 14→15 扩 + reflect lint 自动覆盖; 反向 grep `cold_start.*last_known_cursor\|cold_start.*resume\|cold_start.*cursor` 0 hit |
| ② | agent state 重新 derive — server 收 cold_start_handshake → ① `agent.Tracker.Clear(agentID)` 清 in-memory + ② AL-1 #492 single-gate `AppendAgentStateTransition(any→online, "")` 翻 state-log + ③ **不重放历史 frame** | plugin-protocol.md §1.5 "runtime 不缓存" + AL-1 #492 single-gate 单门 | 反向 grep `cold_start.*replay\|cold_start.*backfill\|cold_start.*history` 0 hit (跟 BPP-5 反向 — BPP-5 增量 resume / BPP-6 fresh start) |
| ③ | restart count 仅 audit, 不影响 wire path — 复用 `runtime_crashed` reason byte-identical (反映上次 error → 此次复活语义); restart 计数走 state-log COUNT(WHERE to_state='online' AND reason='runtime_crashed') **反向 derive**, 不另开 plugin_restart_count 列 | reasons-spec.md (#496 SSOT) + AL-1 #492 forward-only audit | 反向 grep `plugin_restart_count\|cold_start_count\|restart_counter` 0 hit; reasons.go 6-dict 不动 (改 = 改十一处单测锁) |
| ④ (边界) | BPP-3 #489 PluginFrameDispatcher 复用 — `cold_start_handshake` 注册到现有 dispatcher, 不开新 ws hub method | bpp-3.md §1 unified plugin-upstream BPP frame dispatcher 边界 | `pluginFrameRouter.Register(FrameTypeBPPColdStartHandshake, …)` 单源注册; 反向 grep `hub.*Push.*ColdStart\|new.*plugin.*hub.*method` 0 hit |
| ⑤ (边界) | AL-1a reason 字典锁链 BPP-6 = 第 11 处 (跟 BPP-2.2 #485 第 7 + AL-2b #481 第 8 + BPP-4 #499 第 9 + BPP-5 #503 第 10 链承袭) | reasons-spec.md (#496 SSOT) | `internal/agent/reasons/reasons.go` 6-dict 不动; cold-start 复用既有 `runtime_crashed` 字面 (改 = 改十一处单测锁) |
| ⑥ (边界) | best-effort 立场承袭 (跟 BPP-4 #499 §0.3 + BPP-5 #503 §0.6 同源) — server 端不挂 cold-start retry queue / persistent state / restart rate limit | plugin-protocol.md §1.5 字面承袭 | AST scan 反向断言 `pendingColdStart\|coldStartQueue\|deadLetterColdStart` 0 hit (跟 BPP-4 dead_letter_test.go::TestBPP4_NoRetryQueueInBPPPackage + BPP-5 reconnect_handler_test::TestBPP5_NoReconnectQueueInBPPPackage 锁链延伸第 3 处) |
| ⑦ (边界) | admin god-mode 不入 cold-start 路径 — admin 不持有 PluginConn, 不参与 state reset | admin-model.md ADM-0 §1.3 红线 + REG-INV-002 fail-closed | 反向 grep `admin.*cold_start.*handshake\|admin.*BPP6` 在 `internal/api/admin*.go` 0 hit |

## §1 立场 ① cold-start ≠ reconnect (BPP-6.1 守)

**蓝图字面源**: `plugin-protocol.md` §1.6 "失联与故障状态 — 进程死亡 vs 网络重连" — process 死亡重启 (state 全丢) ≠ socket 断重连 (持 cursor). 字段集互斥反断守门.

**反约束清单**:

- [ ] `bppEnvelopeWhitelist` count 14→15 — `frame_schemas_test.go::TestBPPEnvelopeFrameWhitelist` 锁
- [ ] `ColdStartHandshakeFrame` struct 5 字段 byte-identical: `{Type, PluginID, AgentID, RestartAt, RestartReason}`; field 0 必为 `Type string` (跟 BPP-1 envelope 共序)
- [ ] direction lock = plugin→server (反向 grep `FrameTypeBPPColdStartHandshake.*DirectionServerToPlugin` 0 hit)
- [ ] **字段集与 ReconnectHandshakeFrame 互斥** — cold-start 不含 LastKnownCursor / DisconnectAt / ReconnectAt 字段 (反射断 + golden JSON shape 守)
- [ ] 反向 grep `cold_start.*last_known_cursor\|cold_start.*resume\|cold_start.*cursor` count==0

## §2 立场 ② agent state 重新 derive (BPP-6.2 守)

**蓝图字面源**: `plugin-protocol.md` §1.5 "runtime 不缓存" + AL-1 #492 single-gate AppendAgentStateTransition

**反约束清单**:

- [ ] BPP-6.2 server handler 真调 `agent.Tracker.Clear(agentID)` (清 in-memory state) — 跟 BPP-5 反向同模式 (BPP-5 reconnect 走 cursor resume / BPP-6 cold-start 走 state reset)
- [ ] BPP-6.2 server handler 真调 `Store.AppendAgentStateTransition(agentID, fromState, online, runtime_crashed, "")` — AL-1 #492 single-gate 唯一入口 (state machine 自处理 valid 转: initial→online / error→online / offline→online 全合法)
- [ ] **不重放历史 frame** — 反向 grep `cold_start.*replay\|cold_start.*backfill\|cold_start.*history` count==0 (跟 BPP-5 reconnect_handler_test 反向)
- [ ] **不调 ResolveResume** — cold-start 是 fresh start, 不携 cursor, 反向 grep handler 内不 import session_resume.go

## §3 立场 ③ restart count audit-only (BPP-6.2 守)

**蓝图字面源**: AL-1 #492 forward-only audit + reasons-spec.md 6-dict SSOT (#496)

**反约束清单**:

- [ ] reason 复用 `runtime_crashed` byte-identical — 反向 grep `runtime_restarted\|cold_start_recovered\|7th.*reason` count==0
- [ ] **不另开 plugin_restart_count 列** — 反向 grep `plugin_restart_count\|cold_start_count\|restart_counter` count==0; restart 计数走 state-log COUNT(WHERE to_state='online' AND reason='runtime_crashed') 反向 derive
- [ ] AL-1a reason 锁链 BPP-6 = **第 11 处单测锁** (BPP-2.2 第 7 + AL-2b 第 8 + BPP-4 第 9 + BPP-5 第 10 + BPP-6 第 11); 改 = 改十一处
- [ ] audit log 5 字段 byte-identical (复用 BPP-4 dead-letter / HB-1/2/3 audit 跨五 milestone — log key `bpp.cold_start_handshake_received` + plugin_id + agent_id + restart_at + restart_reason)

## §4 蓝图边界 ④⑤⑥⑦ — 跟 BPP-3 / reasons SSOT / BPP-4+5 best-effort / ADM-0 不漂

**反约束清单**:

- [ ] BPP-3 #489 PluginFrameDispatcher 复用 — 反向 grep `hub.*Push.*ColdStart\|new.*plugin.*hub.*method` 0 hit
- [ ] reasons SSOT (#496) 不动 — `internal/agent/reasons/reasons.go` 6-dict 字面锁 (改 = 改十一处单测)
- [ ] best-effort 立场承袭 BPP-4+BPP-5 — AST scan `pendingColdStart\|coldStartQueue\|deadLetterColdStart` 0 hit (跟 BPP-4+BPP-5 锁链延伸第 3 处)
- [ ] admin god-mode 不入 — `internal/api/admin*.go` 反向 grep `admin.*cold_start.*handshake\|admin.*BPP6` 0 hit (ADM-0 §1.3 红线)
- [ ] 不为 cold-start 另开 transient 中间态 — 反向 grep `StateColdStarting\|state.*= "cold_starting"` 0 hit (single-gate any→online 直翻, 跟 BPP-5 connecting 中间态 deferred 同精神)

## §5 退出条件

- §1 (5) + §2 (4) + §3 (4) + §4 (5) 全 ✅
- 反向 grep 7 项全 0 hit (字段互斥 + 不重放 + count derive + 不开 hub method + best-effort + admin + 不加新态)
- e2e 真测: kill plugin process → restart plugin → 收 cold_start_handshake → agent UI 直接翻 online (无中间 thinking 历史)
- AL-1a reason 字典锁链 BPP-6 = 第 11 处, 跟 BPP-2.2/AL-2b/BPP-4/BPP-5 链承袭不漂
