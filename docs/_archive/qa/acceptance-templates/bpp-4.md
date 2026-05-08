# Acceptance Template — BPP-4: 失联检测 (heartbeat watchdog) + ack best-effort + dead-letter audit

> 蓝图: `plugin-protocol.md` §1.6 (失联与故障状态 + 故障 UX 区分表 2 类) + §1.5 (runtime 不缓存) + module `plugin-protocol.md` §BPP-4 acceptance ("kill plugin → 30s 内 agent 显示 error" 字面)
> Spec: `docs/implementation/modules/bpp-4-spec.md` (战马A v0, 3 立场 + 3 拆段 + 7 grep 反查 含 5 反约束)
> 文案锁: `docs/qa/bpp-4-content-lock.md` (战马A v0, 30s 阈值 + AL-1a 6-dict 第 9 处单测锁链 + dead-letter log key + 故障 UX 区分表 byte-identical)
> Stance: `docs/qa/bpp-4-stance-checklist.md` (战马A v0, 3 立场 + 4 蓝图边界)
> 拆 PR: **BPP-4 整 milestone 一 PR** (新协议 "一 milestone = 一 worktree = 一 PR" #479): `feat/bpp-4` 三段一次合 — BPP-4.1 watchdog (heartbeat_watchdog.go + hub.go wire) + BPP-4.2 dead-letter audit log (push 失败 log warn, 不入队列) + BPP-4.3 e2e + REG-BPP4-001..006 + acceptance + PROGRESS [x] + closure
> Owner: 战马A (实施) / 飞马 review / 烈马 验收

## 验收清单

### §1 BPP-4.1 — heartbeat watchdog (30s 阈值单源锁)

> 锚: 战马A spec §0.2 + AL-1b #457 5-state PATCH endpoint 真接管复用 + bpp HeartbeatFrame #304 复用

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 1.1 `BPP_HEARTBEAT_TIMEOUT_SECONDS = 30` 单源常量 byte-identical 跟蓝图 BPP-4 acceptance "kill plugin → 30s" 字面; ticker 间隔 = 10s (≤ 阈值/3 防错过窗口) | unit (常量 + grep 锁) | 战马 / 烈马 | `internal/bpp/heartbeat_watchdog.go::BPP_HEARTBEAT_TIMEOUT_SECONDS` 常量 + `bpp/heartbeat_watchdog_test.go::TestWatchdog_ThresholdConstant` (常量 == 30) + 反向 grep `bpp.*heartbeat.*60\|heartbeat.*timeout.*[5-9][0-9]+s` count==0 |
| 1.2 watchdog 触发路径 = 调 AL-1b #457 PATCH /agents/:id/state (state→error, reason=`network_unreachable`); 不下发 cancel/abort frame (立场 ① 蓝图 §1.6 server 不取消任务) | unit (mock PATCH + 反向 grep) | 战马 / 飞马 / 烈马 | `bpp/heartbeat_watchdog_test.go::TestWatchdog_TriggersAL1bPATCH` (mock state PATCH endpoint, 验证 fake clock advance 30s+ → PATCH 命中, body 含 reason=network_unreachable) + 反向 grep `cancel.*task\|abort.*inflight\|server.*kill.*runtime` count==0 |
| 1.3 plugin 重连 → 下次 HeartbeatFrame 入站 → AL-1b 自动 error→online (复用 #457 PATCH endpoint, 不另起 state 路径) | unit (fake clock + 重连 simulate) | 战马 / 烈马 | `bpp/heartbeat_watchdog_test.go::TestWatchdog_ReconnectFlipsBackToOnline` (fake clock advance 35s → error → 模拟 HeartbeatFrame 入站 → state PATCH online) |
| 1.4 反约束 — admin god-mode 不入 watchdog 路径 (admin 不持有 PluginConn) | CI grep | 飞马 / 烈马 | 反向 grep `admin.*heartbeat.*watchdog\|admin.*BPP4` 在 `internal/api/admin*.go` count==0 (CI lint 每 BPP-4.* PR 必跑) |

### §2 BPP-4.2 — dead-letter audit log (best-effort 立场承袭)

> 锚: 战马A spec §0.3 + RT-1.3 #296 cursor replay 兜底 + HB-1/HB-2 audit log schema 三处同源

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 2.1 server→plugin push 失败 (sent=false, plugin offline) → log warn `bpp.frame_dropped_plugin_offline` + audit hint (frame type / agent_id / cursor); **不入队列** | unit (log capture + 反向 grep) | 战马 / 烈马 | `bpp/dead_letter_test.go::TestDeadLetter_LogOnPluginOffline` (slog handler capture, 验证 log key + audit fields) + 反向 grep `pendingAcks\|retryQueue\|deadLetterQueue\|ackTimeout.*resend` count==0 |
| 2.2 audit log schema byte-identical 跟 HB-1 install-butler audit + HB-2 host-bridge IPC audit 三处同源 (`actor / action / target / when / scope` 5 字段) | unit (schema lock + 跨文件 grep) | 战马 / 飞马 / 烈马 | `bpp/dead_letter_test.go::TestDeadLetter_AuditSchemaByteIdentical` (struct field 反射 5 字段名 == HB-1/HB-2) + 跨文件 grep 三处 audit log struct 字段名同源 |
| 2.3 重连后 plugin 走 RT-1.3 cursor replay 主动拉缺失 frame; server 端不主动重发 (反约束 §0.3 best-effort 立场字面承袭) | scenario test (重连 + cursor diff) | 战马 / 烈马 | `bpp/dead_letter_test.go::TestDeadLetter_ReconnectReplaysViaRT13Cursor` (push 5 frame plugin 离线 → 重连 → frame_count == 5 由 cursor replay 拉, 不是 server retry) + 反向 grep `time.*Ticker.*resend\|retry.*frame.*backoff` count==0 |

### §3 BPP-4.3 — e2e + 蓝图行为对照

> 锚: 蓝图 BPP-4 module acceptance "E2E (kill plugin → 30s 内 agent 显示 error)" 字面真测

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 3.1 e2e: kill plugin → ≤30s + 5s 容差 内 agent UI 显示 error 文案 "重连中…" (跟蓝图 §1.6 故障 UX 区分表第 1 行 byte-identical) | E2E (Playwright + 真 4901 ws fixture + clock) | 战马 / 烈马 / 野马 (UI 文案签字) | `packages/e2e/tests/bpp-4-disconnect.spec.ts::test_kill_plugin_30s_error_ui` (真 plugin disconnect → wait ≤35s → DOM 验证 error 文案) |
| 3.2 e2e: 重连 → agent 自动 online (跟 §1.3 同链) | E2E | 战马 / 烈马 | `bpp-4-disconnect.spec.ts::test_reconnect_auto_online` |
| 3.3 e2e: 5 frame push plugin 离线 → 重连 → cursor replay frame_count == 5 (跟 §2.3 同链, RT-1.3 真兜底) | E2E + cursor diff assert | 战马 / 烈马 | `bpp-4-disconnect.spec.ts::test_cursor_replay_after_reconnect` |

### §4 反向 grep / e2e 兜底 (跨 BPP-4 反约束)

> 锚: spec §3 反查 + stance §1+§2+§3+§4 反约束清单

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 4.1 立场 ① 蓝图 §1.6 — 反向 grep `cancel.*task\|abort.*inflight\|server.*kill.*runtime` 在 `internal/bpp/` + `internal/ws/` count==0 (server 不取消任务) | CI grep | 飞马 / 烈马 | CI lint 每 BPP-4 PR 必跑, count==0 守门 |
| 4.2 立场 ② 30s 单源 — 反向 grep `bpp.*heartbeat.*60\|heartbeat.*timeout.*[5-9][0-9]+s\|heartbeatTimeout.*=.*[1-9][0-9]{2,}` count==0 (防隐式调高) | CI grep | 飞马 / 烈马 | CI lint count==0 |
| 4.3 立场 ③ best-effort — 反向 grep `pendingAcks\|retryQueue\|deadLetterQueue\|ackTimeout.*resend\|time.*Ticker.*resend\|retry.*frame.*backoff` count==0 | CI grep | 飞马 / 烈马 | CI lint count==0 |
| 4.4 立场 ⑤ AL-1a 6-dict 不扩 — 反向 grep `BPP4.*reason.*new\|7th.*reason\|reason.*disconnect_unique` count==0 (跟 BPP-2.2 #485 + AL-2b #481 八处单测锁链承袭) | CI grep | 飞马 / 烈马 | CI lint count==0; reason 字典锁链 BPP-4 = 第 9 处 |
| 4.5 立场 ⑦ admin god-mode — 反向 grep `admin.*heartbeat.*watchdog\|admin.*BPP4` 在 `internal/api/admin*.go` count==0 (ADM-0 §1.3 红线) | CI grep | 飞马 / 烈马 | CI lint count==0 |
| 4.6 立场 ⑥ envelope 不动 — `bppEnvelopeWhitelist` 不变 (BPP-4 仅复用 HeartbeatFrame 做触发源, 不开 cancel/abort frame) | CI reflect (BPP-1 #304 lint) | 飞马 / 烈马 | BPP-1 #304 envelope CI lint reflect 自动覆盖, whitelist 数量不变 |

## 边界 (跟其他 milestone 关系)

| Milestone | 关系 | 字面承袭 |
|---|---|---|
| BPP-1 ✅ #304 | envelope CI lint reflect 自动覆盖 — BPP-4 仅复用 HeartbeatFrame 做 watchdog 触发源, 不开新 frame; whitelist 数量不变 | type/cursor 头位锁 byte-identical |
| BPP-2 ✅ #485 | reason 字典第 7 处单测锁; BPP-4 第 9 处链承袭 | AL-1a 6-dict reason byte-identical |
| BPP-3 ✅ #489 | plugin 上行 BPP frame 统一 dispatcher 边界; BPP-4 watchdog 走 hub 现有 plugins map, 不动 dispatcher | PluginFrameDispatcher boundary 不漂 |
| AL-1a ✅ #249 | reason 字典 6-dict source-of-truth; BPP-4 不扩第 7 reason | `internal/agent/state.go::Reason*` byte-identical |
| AL-1b ✅ #457 | 5-state PATCH /agents/:id/state endpoint; BPP-4 watchdog 真接管复用 (不另起 state 路径) | PATCH error/network_unreachable byte-identical |
| AL-2b ✅ #481 | reason 字典第 8 处单测锁; BPP-4 第 9 处链承袭 | 同 BPP-2 链 |
| RT-1.3 ✅ #296 | cursor replay 兜底 — BPP-4 dead-letter 不入队列, 重连后走 RT-1.3 主动拉 | cursor 单调发号同源 |
| HB-1 (spec #491) | audit log schema (`actor/action/target/when/scope` 5 字段); BPP-4 dead-letter audit 三处同源 | audit log struct 字段名 byte-identical |
| HB-2 (spec #491) | 同 HB-1 audit log schema | 三处同源 |
| ADM-0 §1.3 | admin god-mode 不入 watchdog 路径 | 字面立场反断 |

## 退出条件

- §1 watchdog 4 项 + §2 dead-letter 3 项 + §3 e2e 3 项 + §4 反向 grep 6 项**全绿** (一票否决)
- AL-1a reason 字典锁链 BPP-4 = 第 9 处, 跟 BPP-2.2 第 7 处 + AL-2b 第 8 处链承袭不漂 (改 = 改九处单测锁)
- HB-1/HB-2 audit log schema 三处同源不漂 (跟 HB-4 §1.5 release gate 第 4 行守门同源)
- 登记 `docs/qa/regression-registry.md` REG-BPP4-001..010 (4 watchdog + 3 dead-letter + 3 e2e)
- BPP envelope 数量不变 (`bppEnvelopeWhitelist` 不动, BPP-1 #304 reflect lint 自动守)
- 跨 milestone byte-identical 链承袭 (BPP-1 envelope + BPP-2/AL-2b reason + BPP-3 dispatcher + AL-1b state PATCH + RT-1.3 cursor + HB-1/HB-2 audit + ADM-0 §1.3)
