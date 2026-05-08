# BPP-5 立场反查清单 (战马A v0)

> 战马A · 2026-04-29 · 立场 review checklist (跟 BPP-4 #499 stance + BPP-2 #485 同模式)
> **目的**: BPP-5 三段实施 (BPP-5.1 frame schema / 5.2 server handler / 5.3 closure) PR review 时, 飞马/野马/烈马按此清单逐立场 sign-off, 反向断言代码层守住每条立场.
> **关联**: spec `docs/implementation/modules/bpp-5-spec.md` (战马A v0 df31da7) + acceptance `docs/qa/acceptance-templates/bpp-5.md` (战马A v0)
> **不需 content-lock** — server-only (无 client UI 文案锁), 跟 BPP-3 / BPP-4 同模式 (audit log key + 常量字面已在 spec §0+§3 锁).

## §0 立场总表 (3 立场 + 4 蓝图边界)

| # | 立场 | 蓝图字面 | 反约束 (代码层守门) |
|---|---|---|---|
| ① | reconnect handshake = BPP envelope **第 14 frame** `reconnect_handshake` (direction lock plugin→server, 不另开 channel, 不复用 connect frame) | plugin-protocol.md §1.6 + §2.1 connect 路径承袭 | `bppEnvelopeWhitelist` 13→14 扩 + reflect lint 自动覆盖; 反向 grep `reconnect.*new_channel\|reconnect.*sub_protocol` 0 hit |
| ② | cursor resume **复用 RT-1.3 既有 mechanism** — 不另起 sequence 字典, 不另起 resume mode | plugin-protocol.md §1.5 "runtime 不缓存" + RT-1.3 #296 hardline | server handler 调 `bpp.ResolveResume(EventLister, SessionResumeRequest{Mode: incremental, AfterCursor: last_known_cursor}, …)`; 反向 grep `bpp5.*sequence\|reconnect.*cursor.*= 0\|new.*resume.*dict` 0 hit |
| ③ | 状态翻转用 AL-1 5-state graph 反向链 (error → connecting → online), reason 复用 #496 reasons SSOT 6-dict (BPP-5 = **第 10 处单测锁链**) | plugin-protocol.md §1.6 故障 UX 区分 + AL-1 #457 5-state | connecting state 自身 reason-less (transient 中间态); 反向 grep `runtime_recovered\|reason.*reconnect_success\|7th.*reason` 0 hit |
| ④ (边界) | BPP-3 #489 PluginFrameDispatcher 复用 — `reconnect_handshake` 注册到现有 dispatcher, 不开新 ws hub method | bpp-3.md §1 unified plugin-upstream BPP frame dispatcher 边界 | `pluginFrameRouter.Register(FrameTypeBPPReconnectHandshake, …)` 单源注册; 反向 grep `hub.*Push.*Reconnect\|new.*plugin.*hub.*method` 0 hit |
| ⑤ (边界) | AL-1a reason 字典锁链 BPP-5 = 第 10 处 (跟 BPP-2.2 #485 第 7 + AL-2b #481 第 8 + BPP-4 #499 第 9 链承袭) | reasons-spec.md (#496 SSOT) | `internal/agent/reasons/reasons.go` 6-dict 不动; 改 = 改十处单测锁 |
| ⑥ (边界) | best-effort 立场承袭 (跟 BPP-4 #499 §0.3 同源) — server 端不挂 reconnect retry queue / persistent state | plugin-protocol.md §1.5 字面承袭 | AST scan 反向断言 `pendingReconnects\|reconnectQueue\|deadLetterReconnect` 0 hit (跟 BPP-4 dead_letter_test.go::TestBPP4_NoRetryQueueInBPPPackage 锁链延伸) |
| ⑦ (边界) | admin god-mode 不入 reconnect 路径 — admin 不持有 PluginConn, 不参与 cursor resume | admin-model.md ADM-0 §1.3 红线 + REG-INV-002 fail-closed | 反向 grep `admin.*reconnect.*handshake\|admin.*BPP5` 在 `internal/api/admin*.go` 0 hit |

## §1 立场 ① reconnect_handshake = BPP envelope 第 14 frame (BPP-5.1 守)

**蓝图字面源**: `plugin-protocol.md` §1.6 "plugin 重连后由 runtime 自己决定恢复/放弃" + §2.1 control-plane connect 握手 (reconnect = 已有身份 + 状态恢复; **不**重做身份认证 + capabilities 协商, 那是 connect 范围)

**反约束清单**:

- [ ] `bppEnvelopeWhitelist` count 13→14 — `frame_schemas_test.go::TestBPPEnvelopeFrameWhitelist` 锁
- [ ] `ReconnectHandshakeFrame` struct 6 字段 byte-identical: `{type, plugin_id, agent_id, last_known_cursor, disconnect_at, reconnect_at}`; field 0 必为 `Type string` (跟 BPP-1 envelope 共序)
- [ ] direction lock = plugin→server (反向 grep `FrameTypeBPPReconnectHandshake.*DirectionServerToPlugin` 0 hit)
- [ ] **不复用 connect frame** — connect 是首次身份+capabilities, reconnect 携带 last_known_cursor 恢复, 两者字段集不交; 反向断言 ConnectFrame 不含 `last_known_cursor`/`disconnect_at` 字段
- [ ] 反向 grep `reconnect.*new_channel\|reconnect.*sub_protocol` count==0

## §2 立场 ② cursor resume 复用 RT-1.3 (BPP-5.2 守)

**蓝图字面源**: `plugin-protocol.md` §1.5 "runtime 不缓存" + RT-1.3 #296 §1.3 hardline (incremental 默认, full 仅 explicit)

**反约束清单**:

- [ ] BPP-5.2 server handler 真调 `bpp.ResolveResume(eventLister, SessionResumeRequest{Mode: ResumeModeIncremental, AfterCursor: frame.LastKnownCursor}, channelIDs, DefaultResumeLimit)` — 不绕过, 不复制
- [ ] 不挂 plugin-only sequence — RT-1/CV-2/DM-2/CV-4/AL-2b/RT-3/BPP-3.1 共一根 sequence (反约束 跟 RT-1 立场承袭)
- [ ] 反向 grep `bpp5.*sequence\|reconnect.*cursor.*= 0\|new.*resume.*dict` count==0
- [ ] cursor 单调验证: BPP-5 trust-but-log (cursor 倒退记 warn `bpp.reconnect_cursor_regression` 但不 reject); 严格 reject 留 v2 (§2 留账)

## §3 立场 ③ AL-1 5-state error → connecting → online (BPP-5.2 守)

**蓝图字面源**: `plugin-protocol.md` §1.6 + AL-1 #457 5-state PATCH endpoint (error 反向回 online)

**反约束清单**:

- [ ] BPP-5.2 server handler 收 reconnect_handshake 后 → 调 AgentErrorSink interface (跟 BPP-4 SetError 反向: `Clear(agentID)` 既有 method, agent.Tracker 自动从 error → online 因为 hub.GetPlugin(agentID) != nil)
- [ ] connecting 中间态 reason-less — 跟 AL-1 5-state graph 立场承袭 (transient state 不携带 reason)
- [ ] **不另起第 7 reason** — `runtime_recovered` 字面 0 hit (反向 grep); 复用 unknown 兜底或不挂
- [ ] AL-1a reason 锁链 BPP-5 = **第 10 处单测锁** (BPP-2.2 #485 第 7 + AL-2b #481 第 8 + BPP-4 #499 第 9 + BPP-5 第 10); 改 = 改十处

## §4 蓝图边界 ④⑤⑥⑦ — 跟 BPP-3 / reasons SSOT / BPP-4 best-effort / ADM-0 不漂

**反约束清单**:

- [ ] BPP-3 #489 PluginFrameDispatcher 复用 — 反向 grep `hub.*Push.*Reconnect\|new.*plugin.*hub.*method` 0 hit
- [ ] reasons SSOT (#496) 不动 — `internal/agent/reasons/reasons.go` 6-dict 字面锁 (改 = 改十处单测)
- [ ] best-effort 立场承袭 BPP-4 — AST scan `pendingReconnects\|reconnectQueue\|deadLetterReconnect` 0 hit (跟 BPP-4 dead_letter_test.go::TestBPP4_NoRetryQueueInBPPPackage 锁链延伸; 本 BPP-5 加 reconnect-* 类 identifier 到 forbidden list)
- [ ] admin god-mode 不入 — `internal/api/admin*.go` 反向 grep `admin.*reconnect.*handshake\|admin.*BPP5` 0 hit (ADM-0 §1.3 红线)

## §5 退出条件

- §1 (5) + §2 (4) + §3 (4) + §4 (4) 全 ✅
- 反向 grep 7 项全 0 hit (cancel/abort 复用 BPP-4 grep; new_channel + new sequence + 7th reason + new hub method + best-effort + admin)
- e2e: kill plugin → 30s+5s error → restart plugin → reconnect_handshake → resume cursor → online + cursor 不重不漏
- AL-1a reason 字典锁链 BPP-5 = 第 10 处, 跟 BPP-2.2 第 7 + AL-2b 第 8 + BPP-4 第 9 链承袭不漂
