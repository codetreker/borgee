# BPP-4 立场反查清单 (战马A v0)

> 战马A · 2026-04-29 · 立场 review checklist (跟 BPP-2 #485 stance + BPP-3 #489 同模式)
> **目的**: BPP-4 三段实施 (BPP-4.1 watchdog / 4.2 dead-letter / 4.3 closure) PR review 时, 飞马/野马/烈马按此清单逐立场 sign-off, 反向断言代码层守住每条立场.
> **关联**: spec `docs/implementation/modules/bpp-4-spec.md` (战马A v0) + acceptance `docs/qa/acceptance-templates/bpp-4.md` (战马A v0) + 文案锁 `docs/qa/bpp-4-content-lock.md` (战马A v0)

## §0 立场总表 (3 立场 + 4 蓝图边界)

| # | 立场 | 蓝图字面 | 反约束 (代码层守门) |
|---|---|---|---|
| ① | Borgee 不取消 in-flight 任务 — server 端 ack timeout 仅触发状态翻转, 不下发 cancel/abort frame | plugin-protocol.md §1.6 字面 "server **不**取消正在执行的任务; plugin 重连后由 runtime 自己决定恢复/放弃" | 反向 grep `cancel.*task\|abort.*inflight\|server.*kill.*runtime` 0 hit (CI lint 每 BPP-4.* PR 必跑) |
| ② | heartbeat 缺失 = 状态降级 (30s 单源阈值锁), 不丢 plugin 连接 | plugin-protocol.md §1.6 "缺心跳按未知" + BPP-4 module acceptance "kill plugin → 30s 内 agent 显示 error" 字面 | `BPP_HEARTBEAT_TIMEOUT_SECONDS = 30` 单源常量, 反向 grep `bpp.*heartbeat.*60\|heartbeat.*timeout.*[5-9][0-9]+s` 0 hit |
| ③ | ack best-effort 不重发 — server→plugin push 不等 ack, plugin 离线 frame 丢弃 + log warn (RT-1.3 cursor replay 兜底) | plugin-protocol.md §1.5 "runtime 不缓存" + RT-1.3 #296 cursor replay | 反向 grep `pendingAcks\|retryQueue\|deadLetterQueue\|ackTimeout.*resend` 0 hit; dead-letter = log warn + audit 不入持久队列 |
| ④ (边界) | AL-1b 5-state 真接管复用 #457 — BPP-4 watchdog 仅触发 PATCH /agents/:id/state, 不另起 state 路径 | agent-lifecycle.md §2.3 + AL-1b #457 PATCH endpoint 字面 | watchdog 调 PATCH error/runtime_disconnected, 不直写 presence_sessions 列 |
| ⑤ (边界) | AL-1a 6-dict reason 不扩 — `runtime_disconnected` 复用既有 6-dict (api_key_invalid/quota_exceeded/network_unreachable/runtime_crashed/runtime_timeout/unknown), 不另起第 7 reason | concept-model.md §1.6 + AL-1a #249 6 reason byte-identical 同源 | `internal/agent/state.go::Reason*` source-of-truth, 改 = 改八处单测锁 (跟 BPP-2.2 #485 + AL-2b #481 链承袭) |
| ⑥ (边界) | BPP envelope CI lint 复用 — 不新增 frame type, 仅复用 HeartbeatFrame (#304 已落) 做 watchdog 触发源 | bpp-1.md §1.1 + plugin-protocol.md §2 | `bppEnvelopeWhitelist` 不动, BPP-4 不开新 frame, 反向 grep `FrameTypeBPP.*Disconnect\|FrameTypeBPP.*CancelTask` 0 hit |
| ⑦ (边界) | admin god-mode 不入 watchdog 路径 — admin 不持有 PluginConn, 不参与 heartbeat 状态翻转 | admin-model.md ADM-0 §1.3 红线 + REG-INV-002 fail-closed | 反向 grep `admin.*heartbeat.*watchdog\|admin.*BPP4` 在 `internal/api/admin*.go` 0 hit |

## §1 立场 ① Borgee 不取消 in-flight 任务 (BPP-4.1 watchdog 守)

**蓝图字面源**: `plugin-protocol.md` §1.6 "Borgee **不**承担任务编排 + server **不**取消正在执行的任务 + plugin 重连后由 runtime 自己决定恢复/放弃"

**反约束清单**:

- [ ] watchdog 触发路径**仅**调用 AL-1b PATCH /agents/:id/state (state→error, reason=runtime_disconnected); 不下发任何 cancel / abort / kill frame
- [ ] `internal/bpp/heartbeat_watchdog.go` 不引用 `cancel` / `abort` / `Kill` 类标识符 (反向 grep AST scan)
- [ ] BPP envelope 不新增 `task_cancel` / `task_abort` / `runtime_kill` frame 类型 (`bppEnvelopeWhitelist` 不动)
- [ ] 反向 grep `cancel.*task\|abort.*inflight\|server.*kill.*runtime` 0 hit (CI lint 每 BPP-4.* PR 必跑, acceptance §4.1)

## §2 立场 ② heartbeat 30s 阈值单源锁 (BPP-4.1 守)

**蓝图字面源**: `plugin-protocol.md` §1.6 "缺心跳按未知" + module `plugin-protocol.md` §BPP-4 acceptance "E2E (kill plugin → 30s 内 agent 显示 error)" 字面

**反约束清单**:

- [ ] `BPP_HEARTBEAT_TIMEOUT_SECONDS` 常量在 `internal/bpp/heartbeat_watchdog.go` 单源定义 = 30 (改 = 改单测 + 反向 grep)
- [ ] watchdog ticker 间隔 ≤ 阈值 / 3 (10s ticker 扫 hub.plugins, 防错过窗口)
- [ ] 反向 grep `bpp.*heartbeat.*60\|heartbeat.*timeout.*[5-9][0-9]+s\|heartbeatTimeout.*=.*[1-9][0-9]{2,}` 0 hit (防隐式调高)
- [ ] e2e: kill plugin → ≤30s + 5s 容差 内 agent UI 显示 error (acceptance §3.1 真测)

## §3 立场 ③ ack best-effort 不重发 (BPP-4.2 dead-letter 守)

**蓝图字面源**: `plugin-protocol.md` §1.5 "runtime 不缓存" + RT-1.3 #296 cursor replay 立场承袭

**反约束清单**:

- [ ] server→plugin push 失败 (sent=false, plugin offline) 路径 = log warn `bpp.frame_dropped_plugin_offline` + audit hint, 不入队列
- [ ] 反向 grep `pendingAcks\|retryQueue\|deadLetterQueue\|ackTimeout.*resend` 0 hit (CI lint 每 BPP-4.* PR 必跑)
- [ ] 反向 grep `time.*Ticker.*resend\|retry.*frame.*backoff` 0 hit (防偷偷下沉 v2 retry 路径)
- [ ] dead-letter audit log schema byte-identical 跟 HB-1 audit log (`actor / action / target / when / scope`, 跨 milestone 字面同源, 跟 HB-4 §1.5 release gate 第 4 行守门同源)
- [ ] 重连后 plugin 走 RT-1.3 cursor replay 主动拉缺失 frame, server 端不主动重发 (acceptance §3.3 真测 — 重连 → frame_count 跟 cursor diff 一致)

## §4 蓝图边界 ④⑤⑥⑦ — 跟 AL-1b / AL-1a / BPP-1 / ADM-0 不漂

**反约束清单**:

- [ ] watchdog 触发 PATCH /agents/:id/state 走既有 #457 endpoint, 不直写 presence_sessions 列 (反向 grep `presence_sessions.*UPDATE.*busy` 0 hit, AL-1b 边界守)
- [ ] reason 字典不扩第 7 — `internal/agent/state.go::Reason*` 6-dict 不动, 改 = 改八处单测锁 (跟 BPP-2.2 reason 第 7 处单测锁链承袭, BPP-4 是第 9 处不另起)
- [ ] BPP envelope CI lint reflect 自动覆盖 — `bppEnvelopeWhitelist` 不动 (BPP-4 仅复用 HeartbeatFrame 做 watchdog 源, 不开新 frame)
- [ ] admin god-mode 不入 watchdog — `internal/api/admin*.go` 反向 grep `admin.*heartbeat.*watchdog\|admin.*BPP4` 0 hit (ADM-0 §1.3 红线)

## §5 退出条件

- §1 (4) + §2 (4) + §3 (5) + §4 (4) 全 ✅
- 反向 grep 7 项 (cancel/abort + heartbeat 60+ + pendingAcks + retry/backoff + admin god-mode + 7th reason + presence_sessions 直写) 全 0 hit
- e2e: kill plugin → 30s+5s 容差 内 agent UI error; 重连 → 自动 online; cursor replay frame_count 一致
- AL-1a reason 字典锁链 BPP-4 = 第 9 处, 八处单测锁链承袭不漂 (跟 BPP-2.2 reason 第 7 处 + AL-2b #481 第 8 处同模式)
