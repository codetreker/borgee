# BPP-4 spec brief — 失联检测 + ack timeout + retry policy + dead-letter (≤80 行)

> 战马A · Phase 4 · ≤80 行 · 蓝图 [`plugin-protocol.md`](../../blueprint/current/plugin-protocol.md) §1.6 失联与故障状态 + §2.2 data plane heartbeat. 模块锚 [`plugin-protocol.md`](plugin-protocol.md) §BPP-4. 依赖 BPP-1 envelope (#304) + BPP-2 ack lifecycle (#485) + BPP-3 plugin frame dispatcher (#489) + AL-1a runtime 6-dict (#249) + AL-3 SessionsTracker (#310).

## 0. 关键约束 (3 条立场, 蓝图 §1.6 字面)

1. **Borgee 不取消 in-flight 任务** (蓝图 §1.6 字面 "server **不**取消正在执行的任务; plugin 重连后由 runtime 自己决定恢复/放弃"). server 端 ack timeout **仅触发状态翻转**, 不下发 cancel/abort frame. **反约束**: 反向 grep `cancel.*task\|abort.*inflight\|server.*kill.*runtime` 0 hit.

2. **heartbeat 缺失 = 状态降级, 不丢 plugin 连接** (蓝图 §1.6 "缺心跳按未知"). server 端 heartbeat watchdog 阈值 **30s** (跟蓝图 BPP-4 acceptance "kill plugin → 30s 内 agent 显示 error" byte-identical), 超时 → agent 状态走 AL-1b 5-state 机器 → `error` + reason=`runtime_disconnected` (跟 AL-1a 6-dict byte-identical, **不另起 reason**). **反约束**: 反向 grep `bpp.*heartbeat.*60\|heartbeat.*timeout.*[5-9][0-9]+s` 0 hit (单源 30s 阈值锁).

3. **ack 是 best-effort 不重发** (跟 BPP-2 #485 ack lifecycle 立场承袭 + BPP-3 #489 dispatcher 软跳 unknown frame 同模式). server → plugin push frame (RT-1/CV-2/DM-2/CV-4/AL-2b 共序) **不**等 ack, plugin 离线 frame 丢弃 (反约束 不入队列, 蓝图 §1.5 "runtime 不缓存"). plugin 重连后走 cursor replay (RT-1.3 #296) 兜底, **server 端不挂 retry queue**. **反约束**: 反向 grep `pendingAcks\|retryQueue\|deadLetterQueue\|ackTimeout.*resend` 0 hit. **dead-letter = log warn + audit, 不入持久队列** (best-effort, single source RT-1 cursor replay).

## 1. 拆段 (一 milestone 一 PR, 整段一次合 — 跟 BPP-2/BPP-3 协议同源)

| 段 | 文件 | 范围 |
|---|---|---|
| BPP-4.1 watchdog | `internal/bpp/heartbeat_watchdog.go` (新) + `internal/ws/hub.go` 启动 wire (改) | 30s ticker 扫 hub.plugins, lastHeartbeatAt > 30s → 触发 agent state 翻 error reason=`runtime_disconnected` (跟 AL-1b 5-state PATCH 同源, 调 PATCH /agents/:id/state `error/runtime_disconnected`); 重连 → 下次 HeartbeatFrame 入站 → AL-1b 自动 error→online; 单测 fake clock 5 case |
| BPP-4.2 audit + dead-letter log | `internal/bpp/dead_letter.go` (新) + `internal/ws/al_2b_2_agent_config_push.go` (改, 失败 log path) | server → plugin push 失败 (sent=false, plugin offline) → log warn `bpp.frame_dropped_plugin_offline` + audit hint (frame type / agent_id / cursor); **不入队列**; 跟蓝图 §1.5 "runtime 不缓存" + RT-1.3 cursor replay 兜底立场承袭; 反向 grep `pendingAcks\|retryQueue` 0 hit 单测锁 |
| BPP-4.3 e2e + REG-BPP4 + acceptance + PROGRESS [x] + closure | `packages/e2e/tests/bpp-4-disconnect.spec.ts` (新) + REG-BPP4-001..006 + acceptance/bpp-4.md + PROGRESS [x] | 4 cases: kill plugin → 30s agent error UI / 重连 → 自动 online / push 离线 frame drop log / cursor replay 重发 frame_count==N |

## 2. 留账边界

- **BPP-4 不动 cancel/abort 路径** — 蓝图 §1.6 字面 "server 不取消任务"; 反向 grep CI 守.
- **BPP-4 不接 v2 retry queue** — 留 v2 阶段 (反约束 §0.3 best-effort 立场字面).
- **AL-1b state machine 真接管复用** (#457 5-state PATCH endpoint) — BPP-4 watchdog 仅触发 PATCH, 不另起 state 路径; 防 double-source.
- **AL-1a 6-dict reason 不扩** — `runtime_disconnected` 已在 6-dict (api_key_invalid/quota_exceeded/network_unreachable/runtime_crashed/runtime_timeout/unknown 第 5 处接近, 字面是 `network_unreachable` + UI 文案 "重连中…"); BPP-4 不另起 7th reason (跟 AL-2b/BPP-2.2 同模式).

## 3. 反查 grep 锚 (Phase 4 验收 + BPP-4 实施 PR 必跑)

```
git grep -nE 'BPP_HEARTBEAT_TIMEOUT_SECONDS *= *30\|heartbeatTimeout.*30' packages/server-go/internal/   # ≥ 1 hit (单源阈值锁)
git grep -nE 'bpp\.frame_dropped_plugin_offline' packages/server-go/internal/                           # ≥ 1 hit (dead-letter log key)
# 反约束 (5 条 0 hit)
git grep -nE 'cancel.*task|abort.*inflight|server.*kill.*runtime' packages/server-go/internal/         # 0 hit (蓝图 §1.6 server 不取消任务)
git grep -nE 'bpp.*heartbeat.*60|heartbeat.*timeout.*[5-9][0-9]+s' packages/server-go/internal/         # 0 hit (30s 单源)
git grep -nE 'pendingAcks|retryQueue|deadLetterQueue|ackTimeout.*resend' packages/server-go/internal/   # 0 hit (best-effort, RT-1 cursor replay 兜底)
git grep -nE 'admin.*heartbeat.*watchdog|admin.*BPP4' packages/server-go/internal/api/admin*.go         # 0 hit (ADM-0 §1.3 红线)
git grep -nE 'BPP4.*reason.*runtime_disconnected.*new\|7th.*reason' packages/server-go/internal/        # 0 hit (AL-1a 6-dict 不扩第 7)
```

## 4. 不在本轮范围 (反约束 deferred)

- ❌ in-flight task cancel / abort (蓝图 §1.6 字面立场, v2 也不做)
- ❌ server-side retry queue / dead-letter persistent storage (best-effort 立场, RT-1.3 cursor replay 兜底)
- ❌ heartbeat 阈值动态调 (30s 字面单源锁, 改 = 改单测 + 反向 grep)
- ❌ AL-1a 6-dict 扩第 7 reason (字典分立反约束 — 跟 HB-1/HB-2 reason 字典分立同模式)
- ❌ admin god-mode 走 BPP-4 watchdog 路径 (ADM-0 §1.3 红线)
