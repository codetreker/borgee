# Acceptance Template — BPP-7: plugin SDK 真接入

> 蓝图 `plugin-protocol.md` §1+§2+§3 (BPP-1..6 协议全 + plugin SDK 真接入). Spec `bpp-7-spec.md` (战马D v0 12becd1) + Stance `bpp-7-stance-checklist.md` (战马D v0). 不需 content-lock — SDK 内部 (无 DOM/UI), 跟 BPP-3/4/5/6 同模式. 拆 PR: 整 milestone 一 PR (`spec/bpp-7`). Owner: 战马D 实施 / 飞马 review / 烈马 验收. **位置**: SDK 在 `packages/server-go/sdk/bpp/` (同 borgee-server module 内, 共享 envelope 0 drift).

## 验收清单

### §1 BPP-7.1 — SDK Client + Connect + envelope re-export

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 1.1 SDK 包 `sdk/bpp/client.go` Client struct + Connect ws ctor 真发 ConnectFrame 5 字段 byte-identical (Type/PluginID/Token/Version/Capabilities) — 复用 `borgee-server/internal/bpp.ConnectFrame` 不重定义 | unit (golden JSON + reflect) | 战马D / 烈马 | `sdk/bpp/client_test.go::TestBPP7_ConnectFrame_RoundTrip` (encode → decode → reflect 字段集 byte-identical) |
| 1.2 立场 ① frame schema byte-identical 反断 — SDK 不重定义任何 envelope struct, AST scan + reflect 双锁 | grep + reflect | 战马D / 飞马 / 烈马 | `TestBPP7_FrameSchemaByteIdentical` (reflect 对比 SDK 用的 bpp.AllBPPEnvelopes() 15 frame 字段集 + json tag) + `TestBPP7_NoFrameRedefinition` (AST scan sdk/bpp/ 反向 grep `type.*Frame.*struct` 0 hit) |
| 1.3 立场 ②+⑤ ws 库同源 + 不挂 client dispatcher — 反向 grep `gorilla/websocket\|gobwas/ws\|nhooyr.io/websocket\|SDKDispatcher\|ClientFrameDispatcher` 在 sdk/ 0 hit | grep | 飞马 / 烈马 | `TestBPP7_NoForeignWSLib` + `TestBPP7_NoClientDispatcher` (AST + grep 双锁) |
| 1.4 立场 ⑦ admin god-mode 不挂 — 反向 grep `admin.*sdk\|admin.*BPP7` 在 internal/api/admin*.go 0 hit | grep | 飞马 / 烈马 | `TestBPP7_AdminGodModeNotMounted` |

### §2 BPP-7.2 — Reconnect + ColdStart + Heartbeat + GrantRetry

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 2.1 Reconnect 携 last_known_cursor (走 BPP-5 ReconnectHandshakeFrame) — 反向 ColdStart 不携 cursor (走 BPP-6 ColdStartHandshakeFrame, 字段集互斥反断 §0.1 BPP-6 spec 立场承袭) | unit (3 sub-case: Reconnect 字段含 / ColdStart 字段不含 / RestartReason 复用 reasons.RuntimeCrashed 6-dict) | 战马D / 烈马 | `sdk/bpp/reconnect_test.go::TestBPP7_Reconnect_CarriesCursor` + `TestBPP7_ColdStart_NoCursor` + `TestBPP7_ColdStart_ReasonRuntimeCrashed_ByteIdentical` |
| 2.2 Heartbeat ticker 30s byte-identical (跟 BPP-4 watchdog 周期同源) — `HeartbeatInterval = 30 * time.Second` const 复用 | unit + grep | 战马D / 烈马 | `TestBPP7_HeartbeatInterval_30s` + 反向 grep 非 30s 字面 0 hit |
| 2.3 GrantRetry 跟 BPP-3.2 RequestRetryCache 同行为 (3 次/30s 退避) — `MaxPermissionRetries=3` + `RetryBackoff=30s` 字面复用 server const | unit (3 retry 后 stop + nil-safe) | 战马D / 烈马 | `sdk/bpp/grant_retry_test.go::TestBPP7_GrantRetry_StopsAfter3` + `TestBPP7_GrantRetry_BackoffByteIdentical` |
| 2.4 立场 ②+③ best-effort + reason SSOT 反约束 — AST scan forbidden 9 token 0 hit (`pendingSDKReconnect\|sdkRetryQueue\|deadLetterSDK\|runtime_recovered\|sdk_specific_reason\|7th.*reason\|sdkReason\|cv4SDKReason\|sdkCustomReason`) | AST scan | 飞马 / 烈马 | `TestBPP7_NoSDKQueueOrCustomReason` (AST ident scan 锁链延伸 BPP-4+5+6 第 4 处) |
| 2.5 nil-safe ctor — `NewClient(nil ws / nil logger / nil reasons)` panic boot bug | unit | 战马D / 烈马 | `TestBPP7_NilSafeCtor` (3 sub-case panic) |
| 2.6 AL-1a reason 锁链 BPP-7 = 第 12 处 — SDK ColdStart 走 reasons.RuntimeCrashed 字面 byte-identical 跟 server BPP-6 handler 同源 | unit + grep | 战马D / 烈马 | `TestBPP7_ReasonChain_12thLink` (反向 grep `runtime_recovered\|sdk_specific_reason` 0 hit + 字面对比 reasons.RuntimeCrashed) |

### §3 BPP-7.3 — e2e 整链 + AST 兜底

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 3.1 e2e: 真启 server + SDK Client connect → kill ws → reconnect 携 cursor → server 收 ReconnectHandshakeFrame → resume cursor → SDK 收 backfill events; 整链 PASS | E2E (Go test 真启 testserver) | 战马D / 烈马 / 野马 | `sdk/bpp/e2e/round_trip_test.go::TestBPP7_E2E_ConnectReconnectColdStart` |
| 3.2 反向 grep 6 锚 0 hit (frame 重定义 / ws 第三方 / SDK queue / SDK reason / client dispatcher / admin) | CI grep | 飞马 / 烈马 | CI lint 每 BPP-7 PR 必跑 |

## 边界

- BPP-1 #304 envelope reflect lint (`AllBPPEnvelopes/BPPEnvelopeWhitelist` 公共 API 复用) / BPP-3 #489 PluginFrameDispatcher (server-only, SDK 不复制) / BPP-3.2 #498 RequestRetryCache (SDK 同行为, const 复用) / BPP-4 #499 watchdog (30s 周期 byte-identical) / BPP-5 #503 ReconnectHandshakeFrame (字段集字面承袭) / BPP-6 #522 ColdStartHandshakeFrame (字段集互斥反断, reasons.RuntimeCrashed 复用) / AL-1 #492 single-gate (server 端) / REFACTOR-REASONS #496 6-dict (锁链第 12 处) / ADM-0 §1.3 红线

## 退出条件

- §1 (4) + §2 (6) + §3 (2) 全绿 — 一票否决
- AL-1a reason 锁链 BPP-7 = 第 12 处 (BPP-6 第 11 + BPP-5 第 10 + BPP-4 第 9 + AL-2b 第 8 + BPP-2.2 第 7 链承袭不漂)
- AST 锁链延伸第 4 处 (BPP-4 + BPP-5 + BPP-6 + BPP-7 forbidden tokens 全 0 hit)
- 登记 REG-BPP7-001..006
