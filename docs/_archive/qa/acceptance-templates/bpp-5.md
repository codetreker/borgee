# Acceptance Template — BPP-5: plugin reconnect handshake + cursor resume 协议化

> 蓝图: `plugin-protocol.md` §1.6 (失联与故障状态 + 重连恢复) + §2.1 (control-plane connect 路径) + §1.5 (runtime 不缓存 + RT-1.3 cursor replay 兜底)
> Spec: `docs/implementation/modules/bpp-5-spec.md` (战马A v0 df31da7, 3 立场 + 3 拆段 + 5 grep 反查 + 6 反约束)
> Stance: `docs/qa/bpp-5-stance-checklist.md` (战马A v0, 3 立场 + 4 蓝图边界)
> 不需 content-lock — server-only (无 client UI 文案锁), 跟 BPP-3 #489 / BPP-4 #499 同模式
> 拆 PR: **BPP-5 整 milestone 一 PR** (新协议 "一 milestone = 一 worktree = 一 PR" #479): `feat/bpp-5` 三段一次合 — BPP-5.1 frame schema (envelope 13→14 + 6 字段 + direction lock + 4 unit) + BPP-5.2 server handler (BPP-3 dispatcher 复用 + ResolveResume + AL-1 state error→online + 5 unit) + BPP-5.3 e2e + REG-BPP5-001..006 + acceptance + PROGRESS [x] + closure
> Owner: 战马A (实施) / 飞马 review / 烈马 验收

## 验收清单

### §1 BPP-5.1 — reconnect_handshake frame schema (envelope 第 14 frame)

> 锚: 战马A spec §0.1 + §1 BPP-5.1 + BPP-1 #304 envelope CI lint reflect 自动覆盖

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 1.1 ReconnectHandshakeFrame 6 字段 byte-identical envelope `{type, plugin_id, agent_id, last_known_cursor, disconnect_at, reconnect_at}` 跟 BPP-1 envelope 共序 (field 0 = `Type string`) | unit + golden JSON | 战马A / 烈马 | `internal/bpp/reconnect_handshake_test.go::TestBPP5_ReconnectHandshakeFrame_FieldOrder` (golden JSON 6 字段 byte-equality) + BPP-1 #304 envelope CI lint reflect 自动覆盖加入 `bppEnvelopeWhitelist` 13→14 |
| 1.2 direction lock plugin→server (跟 HeartbeatFrame / TaskStartedFrame 同方向) | unit + reflect | 战马A / 烈马 | `TestBPP5_ReconnectHandshake_DirectionLock` (FrameDirection() == DirectionPluginToServer) + `frame_schemas_test.go::TestBPPEnvelopeDirectionLock` (control 6→6 不变, data 7→8) |
| 1.3 envelope whitelist 13→14 + reflect 自动断 | unit | 战马A / 烈马 | `frame_schemas_test.go::TestBPPEnvelopeFrameWhitelist` count 14 (BPP-1 control 6 + data 3 + AL-2b ack +1 + BPP-2.2 task +2 + BPP-3.1 permission_denied +1 + BPP-5 reconnect_handshake +1 = 14) |
| 1.4 反约束 — 不复用 connect frame (connect = 首次身份 + capabilities; reconnect = 携带 last_known_cursor 恢复, 字段集不交) | reflect 反断 | 战马A / 飞马 / 烈马 | `TestBPP5_ConnectFrame_NoReconnectFields` (反射断 ConnectFrame 不含 LastKnownCursor / DisconnectAt 字段) + 反向 grep `reconnect.*new_channel\|reconnect.*sub_protocol` count==0 |

### §2 BPP-5.2 — server handler + cursor resume (RT-1.3 复用)

> 锚: 战马A spec §0.2 + §1 BPP-5.2 + RT-1.3 #296 §1.3 hardline 复用 + BPP-3 #489 PluginFrameDispatcher 复用

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 2.1 BPP-5.2 reconnect handler 真调 `bpp.ResolveResume(EventLister, SessionResumeRequest{Mode: ResumeModeIncremental, AfterCursor: frame.LastKnownCursor}, channelIDs, DefaultResumeLimit)` — 复用 RT-1.3 既有 mechanism, 不另起 sequence | unit (mock EventLister + assert ResolveResume 调用参数) | 战马A / 烈马 | `internal/bpp/reconnect_handler_test.go::TestBPP5_Handler_CallsResolveResumeIncremental` (mock 验证 Mode == ResumeModeIncremental + AfterCursor == frame value) + 反向 grep `bpp5.*sequence\|reconnect.*cursor.*= 0` count==0 |
| 2.2 BPP-3 #489 PluginFrameDispatcher 复用 — handler 注册 `FrameTypeBPPReconnectHandshake` 到现有 dispatcher, 不开新 ws hub method | unit + grep | 战马A / 飞马 / 烈马 | `TestBPP5_Handler_RegistersOnPluginFrameDispatcher` (dispatcher.Register 真调) + 反向 grep `hub.*Push.*Reconnect\|new.*plugin.*hub.*method` count==0 |
| 2.3 AL-1 5-state 反向链 error → online (复用 agent.Tracker.Clear, 反向于 BPP-4 SetError) — agent.Tracker 自动从 error 转 online (因为 hub.GetPlugin(agentID) != nil) | unit (mock agent.Tracker) | 战马A / 烈马 | `TestBPP5_Handler_ClearsAgentError` (mock Tracker.Clear 真调 + state 翻 online) |
| 2.4 cross-owner reject (跟 BPP-3 PluginFrameDispatcher / BPP-4 watchdog ACL 同模式) | unit + grep | 战马A / 烈马 | `TestBPP5_Handler_CrossOwnerReject` (frame.AgentID owner != session OwnerUserID → reject + log warn `bpp.reconnect_cross_owner_reject`) |
| 2.5 cursor 不重不漏 — BPP-5 trust-but-log cursor 倒退 (记 warn `bpp.reconnect_cursor_regression` 但不 reject; 严格 reject 留 v2 §2 留账) | scenario test (cursor 倒退 + frame_count 一致) | 战马A / 烈马 | `TestBPP5_Handler_CursorMonotonic_TrustButLog` (倒退 cursor → log warn + ResolveResume 仍调 + 返 frame 数等于 server 当前 - cursor) |

### §3 BPP-5.3 — e2e + 蓝图行为对照

> 锚: 蓝图 BPP-5 蓝图 §1.6 真测 + RT-1.3 cursor replay 真兜底 + BPP-4 #499 watchdog 真触发 error

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 3.1 e2e: kill plugin → ≤30s+5s 容差 内 agent UI 显示 error/network_unreachable (BPP-4 #499 watchdog 真触发, BPP-5 反向链入口) | E2E (Playwright + 真 4901 ws fixture + clock) | 战马A / 烈马 / 野马 | `packages/e2e/tests/bpp-5-reconnect.spec.ts::test_kill_plugin_then_reconnect` 步骤 1 |
| 3.2 e2e: restart plugin → 收 reconnect_handshake → resume cursor → agent UI 显示 online (AL-1 反向翻 error→online) | E2E | 战马A / 烈马 | 同 spec.ts 步骤 2-4 |
| 3.3 e2e: cursor 不重不漏 — 中间 5 frame push plugin 离线 → 重连 → cursor diff 严格递增 + frame_count == 5 (RT-1.3 cursor replay 真兜底) | E2E + cursor diff assert | 战马A / 烈马 | 同 spec.ts 步骤 5 (拉 GET /api/v1/events?since=last_known_cursor 验 frame_count) |

### §4 反向 grep / e2e 兜底 (跨 BPP-5 反约束)

> 锚: spec §3 反查 + stance §1+§2+§3+§4 反约束清单 + BPP-4 §0.3 best-effort 立场承袭

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 4.1 立场 ① — 反向 grep `reconnect.*new_channel\|reconnect.*sub_protocol` 在 `internal/bpp/` + `internal/ws/` count==0 (单 BPP envelope, 不开 channel) | CI grep | 飞马 / 烈马 | CI lint 每 BPP-5 PR 必跑 |
| 4.2 立场 ② cursor resume 复用 RT-1.3 — 反向 grep `bpp5.*sequence\|reconnect.*cursor.*= 0\|new.*resume.*dict` count==0 | CI grep | 飞马 / 烈马 | CI lint count==0 |
| 4.3 立场 ③ AL-1 6-dict 不扩 — 反向 grep `runtime_recovered\|reason.*reconnect_success\|7th.*reason` count==0; reason 字典锁链 BPP-5 = 第 10 处 (跟 BPP-2.2/AL-2b/BPP-4 链承袭) | CI grep | 飞马 / 烈马 | CI lint count==0 |
| 4.4 立场 ⑥ best-effort 立场承袭 BPP-4 — AST scan `pendingReconnects\|reconnectQueue\|deadLetterReconnect` 在 `internal/bpp/` 非 _test.go 源 count==0 | unit (AST scan, 跟 BPP-4 dead_letter_test 同模式) | 飞马 / 烈马 | `reconnect_handler_test.go::TestBPP5_NoReconnectQueueInBPPPackage` (AST ident scan, forbidden tokens 锁链延伸 BPP-4) |
| 4.5 立场 ⑦ admin god-mode — 反向 grep `admin.*reconnect.*handshake\|admin.*BPP5` 在 `internal/api/admin*.go` count==0 (ADM-0 §1.3 红线) | CI grep | 飞马 / 烈马 | CI lint count==0 |
| 4.6 立场 ④ BPP-3 dispatcher 复用 — 反向 grep `hub.*Push.*Reconnect\|new.*plugin.*hub.*method` count==0 | CI grep | 飞马 / 烈马 | CI lint count==0 |

## 边界 (跟其他 milestone 关系)

| Milestone | 关系 | 字面承袭 |
|---|---|---|
| BPP-1 ✅ #304 | envelope CI lint reflect 自动覆盖 — BPP-5 加第 14 frame, whitelist 13→14 | type/cursor 头位锁 byte-identical |
| BPP-3 ✅ #489 | PluginFrameDispatcher 复用 — BPP-5 handler 注册到现有 dispatcher | dispatcher boundary 不漂 |
| BPP-4 ✅ #499 | best-effort 立场承袭 (server 端不挂 retry queue) + AST scan 锁链延伸 (forbidden tokens 加 reconnect-*) | dead_letter_test::TestBPP4_NoRetryQueueInBPPPackage 锁链延伸 |
| RT-1.3 ✅ #296 | cursor resume 复用 ResolveResume + SessionResumeRequest 三模式 (BPP-5 用 incremental 默认) | DefaultResumeLimit + MaxResumeLimit byte-identical |
| AL-1 ✅ #457 | 5-state graph (error → connecting → online 反向链), agent.Tracker.Clear 真接管 | 5-state PATCH endpoint 立场承袭 |
| reasons SSOT ✅ #496 | 6-dict 不扩 — BPP-5 connecting 中间态 reason-less; AL-1a reason 锁链 BPP-5 = 第 10 处 | reasons.go SSOT 字面锁 |
| ADM-0 §1.3 | admin god-mode 不入 reconnect 路径 | 字面立场反断 |

## 退出条件

- §1 frame schema (4) + §2 server handler (5) + §3 e2e (3) + §4 反向 grep (6) **全绿** (一票否决)
- AL-1a reason 字典锁链 BPP-5 = 第 10 处, 跟 BPP-2.2 第 7 + AL-2b 第 8 + BPP-4 第 9 链承袭不漂 (改 = 改十处单测锁)
- BPP envelope whitelist count 13→14, BPP-1 #304 reflect lint 自动守
- AST scan `pendingReconnects/reconnectQueue/deadLetterReconnect` 0 hit (BPP-4 best-effort 锁链延伸)
- 登记 `docs/qa/regression-registry.md` REG-BPP5-001..009 (4 frame schema + 5 server handler + 3 e2e — 部分项合并)
- 跨 milestone byte-identical 链承袭 (BPP-1 envelope + BPP-3 dispatcher + BPP-4 best-effort + RT-1.3 ResolveResume + AL-1 state graph + reasons SSOT + ADM-0 §1.3)
