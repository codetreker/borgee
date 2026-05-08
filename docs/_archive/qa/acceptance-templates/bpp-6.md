# Acceptance Template — BPP-6: plugin cold-start handshake + state re-derive

> 蓝图 `plugin-protocol.md` §1.6 (进程死亡 vs 网络重连) + §2.1 control-plane handshake. Spec `bpp-6-spec.md` (战马D v0 5a29eb8) + Stance `bpp-6-stance-checklist.md` (战马D v0). 不需 content-lock — server-only. 拆 PR: 整 milestone 一 PR (`spec/bpp-6` 三段一次合). Owner: 战马D 实施 / 飞马 review / 烈马 验收.

## 验收清单

### §1 BPP-6.1 — cold_start_handshake frame schema (envelope 第 15 frame)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 1.1 ColdStartHandshakeFrame 5 字段 byte-identical `{Type, PluginID, AgentID, RestartAt, RestartReason}` (field 0 = `Type string`) | unit + golden JSON | 战马D / 烈马 | `cold_start_handshake_test.go::TestBPP6_FieldOrder` + BPP-1 #304 reflect lint |
| 1.2 direction lock plugin→server + envelope whitelist 14→15 | unit + reflect | 战马D / 烈马 | `TestBPP6_DirectionLock` + `frame_schemas_test.go::TestBPPEnvelopeFrameWhitelist` count==15 |
| 1.3 字段集与 ReconnectHandshakeFrame 互斥 (cold-start 不含 LastKnownCursor/DisconnectAt/ReconnectAt) — §0.1 立场守 | reflect 反断 | 战马D / 飞马 / 烈马 | `TestBPP6_FrameSet_NoReconnectFields` + 反向 grep `cold_start.*last_known_cursor\|cold_start.*resume\|cold_start.*cursor` count==0 |

### §2 BPP-6.2 — server handler + state re-derive

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 2.1 handler 调 `agent.Tracker.Clear(agentID)` + AL-1 #492 single-gate `AppendAgentStateTransition(any→online, runtime_crashed)` 翻 state-log (3 valid edges: initial/error/offline → online) | unit (mock + 3 sub-case) | 战马D / 烈马 | `cold_start_handler_test.go::TestBPP6_Handler_ClearsAgentTracker` + `_TransitionsToOnline_FromInitial`/`_FromError`/`_FromOffline` |
| 2.2 cross-owner reject + BPP-3 #489 PluginFrameDispatcher 复用 (注册不开新 hub method) | unit + grep | 战马D / 飞马 / 烈马 | `TestBPP6_Handler_CrossOwnerReject` + `_RegistersOnPluginFrameDispatcher` + 反向 grep `hub.*Push.*ColdStart` count==0 |
| 2.3 反向不调 ResolveResume + 不重放历史 — handler 不携 cursor, AST scan 不 reference session_resume.go | AST scan | 战马D / 烈马 | `TestBPP6_Handler_DoesNotInvokeResolveResume` + 反向 grep `cold_start.*replay\|cold_start.*backfill\|cold_start.*history` count==0 |

### §3 BPP-6.3 — restart count derive + e2e + AST 兜底

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 3.1 restart 计数走 state-log COUNT(WHERE to_state='online' AND reason='runtime_crashed') 反向 derive — 不另开 plugin_restart_count 列 | unit (3 cold-start → COUNT == 3) | 战马D / 烈马 | `TestBPP6_RestartCount_DerivedFromStateLog` + 反向 grep `plugin_restart_count\|cold_start_count\|restart_counter` count==0 |
| 3.2 e2e: kill plugin process (SIGKILL) → restart → cold_start_handshake → agent UI 翻 online (无中间 thinking 历史) | E2E (Playwright + 真 4901) | 战马D / 烈马 / 野马 | `packages/e2e/tests/bpp-6-cold-start.spec.ts` |
| 3.3 AST scan + 反向 grep 6 锚 0 hit (字段互斥 + 不重放 + count derive + best-effort + 不加新态 + admin) | AST + CI grep | 飞马 / 烈马 | `TestBPP6_NoColdStartQueueInBPPPackage` (BPP-4+BPP-5 best-effort 锁链延伸第 3 处) |

## 边界

- BPP-1 #304 (envelope reflect lint 自动覆盖 14→15) / BPP-3 #489 (dispatcher 复用) / BPP-4 #499+BPP-5 #503 (best-effort 锁链延伸第 3 处) / AL-1 #492 (single-gate any→online valid edge) / reasons SSOT #496 (复用 `runtime_crashed`, 锁链第 11 处) / BPP-5 #503 反向帧 (字段集互斥) / ADM-0 §1.3 红线

## 退出条件

- §1 (3) + §2 (3) + §3 (3) 全绿 — 一票否决
- AL-1a reason 锁链 BPP-6 = 第 11 处 (BPP-5 第 10 链承袭不漂)
- envelope whitelist 14→15, AST scan `pendingColdStart/coldStartQueue/deadLetterColdStart` 0 hit
- 登记 REG-BPP6-001..006
