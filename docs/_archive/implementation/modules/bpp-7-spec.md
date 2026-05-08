# BPP-7 spec brief — plugin SDK 真接入 (≤80 行)

> 战马D · Phase 6 起步 · ≤80 行 · 蓝图 [`plugin-protocol.md`](../../blueprint/current/plugin-protocol.md) §1+§2 (BPP-1..BPP-6 协议全) + §3 (plugin SDK 真接入). 模块锚 [`plugin-protocol.md`](plugin-protocol.md) §BPP-7. 依赖 BPP-1 #304 envelope (whitelist 15 frame) + BPP-3 #489 PluginFrameDispatcher + BPP-3.2 #498 retry cache + BPP-4 #499 watchdog + BPP-5 #503 reconnect + BPP-6 #522 cold-start + AL-1 #492 5-state + REFACTOR-REASONS #496 6-dict.

## 0. 关键约束 (3 条立场, 蓝图 §3 + BPP-1..6 字面承袭)

1. **plugin SDK frame schema 跟 server byte-identical** — SDK 的 envelope struct 跟 `internal/bpp/envelope.go::AllBPPEnvelopes()` 15 frame 字段集 + 顺序 + json tag **byte-identical 反断**. SDK 跟 server 共一根 envelope 定义 (Go module 复用 `internal/bpp/envelope.go` 的 const + struct, 不另定义). **反约束**: SDK 包不允许独立定义 `ConnectFrame` / `ReconnectHandshakeFrame` / `ColdStartHandshakeFrame` 等 — 必 import server 包. 反向 grep `type.*Frame.*struct` 在 `packages/plugin-sdk-go/` count==0 (除 generated test fixture). drift 真断: `TestBPP7_FrameSchemaByteIdentical` 反射对比 SDK 引用 vs server 定义.

2. **SDK 用 Go module (跟 server 同语言, 减少协议序列化测试)** — SDK 路径 `packages/plugin-sdk-go/` 自成 module (go.mod), 但 import `borgee-server/internal/bpp` 共享 envelope (server 暴露 `AllBPPEnvelopes` 公共 API 已就绪). **反约束**: 不引第三方 ws library, 用 `nhooyr.io/websocket` 跟 server 同模块依赖锁; SDK 内部 reconnect/cold-start 走 BPP-5/BPP-6 帧, 不跑独立持久化 retry queue (跟 BPP-4/BPP-5/BPP-6 best-effort 立场承袭, AST scan forbidden tokens 锁链延伸第 4 处 — `pendingSDKReconnect / sdkRetryQueue / deadLetterSDK` 0 hit).

3. **BPP-3.2.3 retry + BPP-4 watchdog + BPP-5/6 reconnect/cold-start SDK side 真实施** — SDK 需跑: ① BPP-3.2.3 capability_grant retry 3次/30s 退避 (跟 server `RequestRetryCache` 同行为) + ② BPP-4 30s heartbeat 主动发 (跟 server watchdog 周期 byte-identical) + ③ BPP-5 reconnect_handshake 携 last_known_cursor (socket 断 process 活) + ④ BPP-6 cold_start_handshake 携 RestartReason (process 重启 cursor 丢). **反约束**: SDK 不另起 reason 字典 — 复用 reasons SSOT 6-dict (#496 字面承袭, AL-1a reason 锁链 BPP-7 = **第 12 处单测锁** BPP-2.2/AL-2b/BPP-4/BPP-5/BPP-6/BPP-7). 反向 grep `runtime_recovered\|sdk_specific_reason\|7th.*reason` in `packages/plugin-sdk-go/` 0 hit.

## 1. 拆段 (一 milestone 一 PR, 整段一次合 — 跟 BPP-2/3/4/5/6 协议同源)

| 段 | 文件 | 范围 |
|---|---|---|
| BPP-7.1 SDK frame stub + connect | `packages/plugin-sdk-go/go.mod` 新 module + `pkg/bpp/client.go` 新 (Client struct + Connect ws ctor + envelope re-export 反射 byte-identical 守) + `pkg/bpp/client_test.go` 4 unit (frame schema byte-identical 反断 vs server / connect handshake 真发 / direction lock 反断 / nil-safe deps panic) | SDK module 立, frame schema 反断, connect happy path |
| BPP-7.2 SDK reconnect + cold-start + heartbeat + retry | `pkg/bpp/reconnect.go` (Reconnect cursor track + 30s heartbeat ticker + ColdStart fresh-start 路径) + `pkg/bpp/grant_retry.go` (BPP-3.2.3 retry cache 3次/30s 退避 client-side) + 6 unit (Reconnect 携 cursor / ColdStart 不携 cursor / Heartbeat 周期 30s / GrantRetry 3次后停 / 反向 forbidden tokens AST scan / nil-safe ctor) | reconnect/cold-start/heartbeat/retry 全实施 SDK side |
| BPP-7.3 SDK e2e + REG-BPP7 + acceptance + PROGRESS [x] + closure | `packages/plugin-sdk-go/e2e/round_trip_test.go` (真启 server + SDK Client connect → reconnect → cold-start → heartbeat 整链验证) + REG-BPP7-001..006 + acceptance/bpp-7.md + docs/current sync (server/bpp/sdk-client.md) | 整链 e2e — connect → 失联 → reconnect cursor resume / cold-start state reset 真兜底 |

## 2. 留账边界

- **JS/TS plugin SDK port** (留 v2) — Phase 6 优先 Go SDK; JS/TS 留 plugin marketplace 起步时
- **plugin SDK auth credential rotation** (留 v2) — connect frame Token 现是固定值, 真 rotation 留 AP-3 cross-org 联动
- **plugin SDK metrics/telemetry** (留 v3) — 当前 SDK 仅暴露 reconnect_count / cold_start_count getter; Prometheus exporter 留 v3
- **plugin SDK structured logger** (留 v2) — 当前 stdlib log; structured logger seam 留 follow-up

## 3. 反查 grep 锚 (Phase 6 验收 + BPP-7 实施 PR 必跑)

```
git grep -nE 'AllBPPEnvelopes|BPPEnvelopeWhitelist' packages/plugin-sdk-go/   # ≥ 1 hit (复用 server envelope, 立场 ①)
git grep -nE 'reasons\.RuntimeCrashed|reasons\.IsValid' packages/plugin-sdk-go/   # ≥ 1 hit (复用 6-dict SSOT, 立场 ③)
# 反约束 (5 条 0 hit)
git grep -nE 'type.*Frame.*struct' packages/plugin-sdk-go/pkg/bpp/   # 0 hit (frame schema 复用, 不重定义, §0.1)
git grep -nE 'pendingSDKReconnect|sdkRetryQueue|deadLetterSDK' packages/plugin-sdk-go/   # 0 hit (best-effort 锁链延伸第 4 处, §0.2)
git grep -nE 'runtime_recovered|sdk_specific_reason|7th.*reason' packages/plugin-sdk-go/   # 0 hit (复用 6-dict, §0.3)
git grep -nE 'gorilla/websocket|gobwas/ws' packages/plugin-sdk-go/   # 0 hit (跟 server nhooyr.io/websocket 同源)
git grep -nE 'admin.*sdk|admin.*BPP7' packages/server-go/internal/api/admin*.go   # 0 hit (ADM-0 §1.3 红线)
```

## 4. 不在本轮范围 (反约束 deferred)

- ❌ JS/TS plugin SDK port (留 v2)
- ❌ plugin SDK auth credential rotation (留 v2 跟 AP-3)
- ❌ plugin SDK metrics exporter (留 v3 Prometheus)
- ❌ plugin SDK structured logger (留 v2)
- ❌ plugin marketplace API (留 Phase 7 完整路径)
- ❌ plugin SDK 独立定义 frame struct (字段 drift 反断, §0.1 立场)
- ❌ admin god-mode 走 SDK 路径 (ADM-0 §1.3 红线)
- ❌ AL-1 6-dict 扩第 7 reason (字典分立反约束 — SDK 复用 reasons SSOT)
