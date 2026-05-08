# BPP-5 spec brief — plugin reconnect handshake + cursor resume 协议化 (≤80 行)

> 战马A · Phase 5 · ≤80 行 · 蓝图 [`plugin-protocol.md`](../../blueprint/current/plugin-protocol.md) §1.6 (失联与故障状态 — 重连恢复) + §2.1 (control-plane `connect` 握手). 模块锚 [`plugin-protocol.md`](plugin-protocol.md) §BPP-5. 依赖 BPP-1 #304 envelope (whitelist 13→14) + BPP-3 #489 PluginFrameDispatcher + BPP-4 #499 watchdog (error→reconnect 反向) + RT-1.3 #296 session.resume cursor replay (复用, 不另起 sequence) + AL-1 5-state (#496 reasons SSOT 单源).

## 0. 关键约束 (3 条立场, 蓝图 §1.6 + RT-1.3 字面)

1. **reconnect handshake = BPP envelope 第 14 frame `reconnect_handshake`** (direction lock plugin→server, BPP-1 #304 reflect lint 自动覆盖). **不另开 channel**, **不另起 frame schema**, **不复用 connect frame** (connect = 首次握手身份+capabilities; reconnect = 已知 plugin 重连恢复, 携带 `last_known_cursor` + `disconnect_at`). **反约束**: 反向 grep `reconnect.*new_channel\|reconnect.*sub_protocol` 0 hit.

2. **cursor resume 复用 RT-1.3 既有 mechanism** — 不另起 sequence, 不另起 dictionary. server 收 `reconnect_handshake.last_known_cursor` → 内部调 `bpp.ResolveResume(EventLister, SessionResumeRequest{Mode: incremental, AfterCursor: last_known_cursor}, …)` → 复用 RT-1.3 §1.3 hardline (incremental 默认, full 仅 explicit). **反约束**: 反向 grep `bpp4.*sequence\|reconnect.*cursor.*= 0\|new.*resume.*dict` 0 hit.

3. **状态翻转用 AL-1 5-state graph 反向链 (error → connecting → online)**, reason 复用 #496 refactor-reasons SSOT 6-dict. **不另起第 7 reason** — 复用 `unknown` 兜底 + connecting state 自身不携带 reason (跟 AL-1 5-state 立场: connecting 是 transient 中间态, reason-less). 反向 grep `runtime_recovered\|reason.*reconnect_success\|7th.*reason` 0 hit. AL-1 reason 锁链 BPP-5 = 第 10 处单测锁 (BPP-2.2 #485 第 7 + AL-2b #481 第 8 + BPP-4 #499 第 9 + BPP-5 第 10).

## 1. 拆段 (一 milestone 一 PR, 整段一次合 — 跟 BPP-2/3/4 协议同源)

| 段 | 文件 | 范围 |
|---|---|---|
| BPP-5.1 frame schema | `internal/bpp/envelope.go` 改 (+ ReconnectHandshakeFrame 6 字段 `{type, plugin_id, agent_id, last_known_cursor, disconnect_at, reconnect_at}` + whitelist 13→14) + `internal/bpp/reconnect_handshake_test.go` 新 (4 unit: 字段顺序锁 / direction lock plugin→server / cursor monotonic 反断 / Frame body 反向 grep 反约束 §0 守) | envelope 第 14 frame, BPP-1 #304 reflect lint 自动扩 13→14 |
| BPP-5.2 server handler + state flip | `internal/bpp/reconnect_handler.go` 新 (PluginFrameDispatcher 注册 `reconnect_handshake` → resume cursor 调 ResolveResume + 通过 AgentErrorSink 真接 agent.Tracker.Clear (跟 BPP-4 SetError 反向同模式)) + 5 unit (last_known_cursor 0 → ResolveResume mode=incremental / AL-1 state 翻 error→online / cross-owner reject / cursor 不重不漏 / handler nil-safe) | BPP-3 dispatcher 复用, 不开新 ws hub method |
| BPP-5.3 e2e + REG-BPP5 + acceptance + PROGRESS [x] + closure | `packages/e2e/tests/bpp-5-reconnect.spec.ts` 新 (kill plugin → ≤30s+5s agent UI error → restart plugin → 收 reconnect_handshake → resume cursor → online + cursor diff 严格递增不重不漏) + REG-BPP5-001..006 + acceptance/bpp-5.md + docs/current sync | RT-1.3 cursor replay 兜底真测 — frame_count 跟 cursor diff 一致 |

## 2. 留账边界

- **cross-plugin session migrate** (留 v2) — plugin A 替换 plugin B 接管同 agent, 反约束: BPP-5 仅同 plugin_id 重连, 跨 plugin_id reject + log warn `bpp.reconnect_cross_plugin_reject`
- **多 plugin instance 同时 reconnect** (留 v2) — plugin SDK 自带 leader election, BPP-5 不接此层
- **plugin SDK 自动 backoff retry** (留 plugin SDK 真接入) — 跟 BPP-4 #499 §0.3 best-effort 立场承袭, server 端不挂 retry queue
- **last_known_cursor 严格单调验证** (留账给 BPP-5 follow-up) — BPP-5.2 仅 trust-but-log 处理 cursor 倒退; v2 加严格 reject

## 3. 反查 grep 锚 (Phase 5 验收 + BPP-5 实施 PR 必跑)

```
git grep -nE 'FrameTypeBPPReconnectHandshake' packages/server-go/internal/bpp/   # ≥ 1 hit (whitelist 字面 + handler 注册)
git grep -nE 'ReconnectHandshakeFrame' packages/server-go/internal/bpp/          # ≥ 1 hit (struct + register)
# 反约束 (5 条 0 hit)
git grep -nE 'reconnect.*new_channel|reconnect.*sub_protocol' packages/server-go/internal/   # 0 hit (单 BPP envelope, 不开新 channel)
git grep -nE 'bpp5.*sequence|reconnect.*cursor.*= 0|new.*resume.*dict' packages/server-go/   # 0 hit (复用 RT-1.3, 不另起 sequence)
git grep -nE 'runtime_recovered|reason.*reconnect_success|7th.*reason' packages/server-go/   # 0 hit (复用 6-dict, 不扩第 7)
git grep -nE 'admin.*reconnect.*handshake|admin.*BPP5' packages/server-go/internal/api/admin*.go   # 0 hit (ADM-0 §1.3 红线)
git grep -nE 'pendingReconnects|reconnectQueue|deadLetterReconnect' packages/server-go/internal/   # 0 hit (跟 BPP-4 §0.3 best-effort 立场承袭, AST scan 锁链延伸)
```

## 4. 不在本轮范围 (反约束 deferred)

- ❌ cross-plugin session migrate (v2, 跟 §2 留账同源)
- ❌ 多 plugin instance leader election (留 plugin SDK 层)
- ❌ server-side retry queue / persistent reconnect state (跟 BPP-4 §0.3 best-effort 立场承袭, AST scan `pendingReconnects|reconnectQueue` 0 hit)
- ❌ AL-1 6-dict 扩第 7 reason (字典分立反约束 — connecting 中间态 reason-less, recovered 复用 unknown 或不挂)
- ❌ admin god-mode 走 reconnect 路径 (ADM-0 §1.3 红线)
- ❌ connect 握手字段 + capabilities 重协商 (留 BPP-1 connect 路径, BPP-5 仅 cursor resume)
