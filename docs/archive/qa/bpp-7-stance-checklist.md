# BPP-7 立场反查清单 (战马D v0)

> 战马D · 2026-04-30 · 立场 review checklist (跟 BPP-6 #522 stance + BPP-5 #503 + BPP-4 #499 同模式)
> **目的**: BPP-7 三段实施 (BPP-7.1 SDK frame stub + connect / 7.2 reconnect/cold-start/heartbeat/retry / 7.3 e2e + closure) PR review 时, 飞马/野马/烈马按此清单逐立场 sign-off, 反向断言代码层守住每条立场.
> **关联**: spec `docs/implementation/modules/bpp-7-spec.md` (战马D v0 12becd1) + acceptance `docs/qa/acceptance-templates/bpp-7.md` (战马D v0)
> **不需 content-lock** — SDK 内部, 无 DOM / 无 UI 文案锁; 跟 BPP-3/4/5/6 同模式.
> **位置调整**: SDK 实施在 `packages/server-go/sdk/bpp/` (同 borgee-server module 内), 共享 `internal/bpp` envelope (避免 go.work 复杂度, 字节一致性零成本); 跟 spec §0.2 立场精神同源 (单 module Go SDK).

## §0 立场总表 (3 立场 + 4 蓝图边界)

| # | 立场 | 蓝图字面 | 反约束 (代码层守门) |
|---|---|---|---|
| ① | SDK frame schema 跟 server **byte-identical** — SDK envelope 直接 import `borgee-server/internal/bpp`, 不重定义 ConnectFrame / ReconnectHandshakeFrame / ColdStartHandshakeFrame; reflect drift 反断 | plugin-protocol.md §2 (frame schema 单源) + BPP-1 #304 reflect lint | SDK 包 `sdk/bpp/` AST scan: `type.*Frame.*struct` 在 production *.go count==0 (除复用 alias type 重命名); `TestBPP7_FrameSchemaByteIdentical` reflect 对比 SDK 引用 vs server.AllBPPEnvelopes 字段集 + json tag |
| ② | SDK 单 Go module 复用 server envelope — SDK 路径 `packages/server-go/sdk/bpp/` 在 borgee-server 同 module 内 (避免 go.work overhead); ws 库跟 server 同源 (`github.com/coder/websocket`, 反向 `gorilla/websocket\|gobwas/ws\|nhooyr.io/websocket` 0 hit在 sdk/) | plugin-protocol.md §3 + best-effort 立场承袭 BPP-4/5/6 | SDK 内部 reconnect/cold-start 走 BPP-5/6 帧路径, 不跑独立持久化 retry queue; AST scan forbidden tokens `pendingSDKReconnect\|sdkRetryQueue\|deadLetterSDK` 0 hit (锁链延伸第 4 处: BPP-4 dead_letter_test + BPP-5 reconnect_handler_test + BPP-6 cold_start_handler_test + BPP-7 sdk_test) |
| ③ | BPP-3.2.3 retry + BPP-4 watchdog + BPP-5/6 reconnect/cold-start SDK side 真实施 — reason 复用 reasons SSOT 6-dict (#496); 不另起 SDK reason 字典 | reasons-spec.md (#496 SSOT) + AL-1 #492 single-gate | SDK 反向 grep `runtime_recovered\|sdk_specific_reason\|7th.*reason` 0 hit; AL-1a reason 锁链 BPP-7 = **第 12 处** (BPP-2.2 第 7 + AL-2b 第 8 + BPP-4 第 9 + BPP-5 第 10 + BPP-6 第 11 + BPP-7 第 12); SDK ColdStart 走 `reasons.RuntimeCrashed` 字面 byte-identical 跟 server BPP-6 handler 同源 |
| ④ (边界) | BPP-1 #304 envelope reflect lint 自动覆盖 — SDK envelope 引用走 server `AllBPPEnvelopes()` + `BPPEnvelopeWhitelist()` 公共 API | bpp-1.md §1 reflect lint 立场 | SDK reflect 测试用 `bpp.AllBPPEnvelopes()` 真获取 15 frame 然后断言 SDK Client.Send 路径覆盖 |
| ⑤ (边界) | BPP-3 #489 PluginFrameDispatcher 复用 — server side 已挂; SDK 仅做 client 端 frame 编/解码 + 发送, 不需 client-side dispatcher | bpp-3.md §1 | SDK 不引入 dispatcher pattern; 反向 grep `SDKDispatcher\|ClientFrameDispatcher` 在 sdk/ 0 hit |
| ⑥ (边界) | BPP-4 best-effort 30s heartbeat — SDK 主动发送, 跟 server watchdog 周期 byte-identical (HeartbeatInterval = 30s) | bpp-4.md §0.3 字面 | SDK 反向 grep `HeartbeatInterval.*[0-9]+\s*\*\s*time` (除 30 字面) 0 hit; const `HeartbeatInterval = 30 * time.Second` byte-identical 跟 server const |
| ⑦ (边界) | admin god-mode 不挂 SDK 路径 — admin-api 不接 SDK 客户端 | admin-model.md ADM-0 §1.3 红线 | 反向 grep `admin.*sdk\|admin.*BPP7` 在 `internal/api/admin*.go` 0 hit |

## §1 立场 ① SDK frame schema byte-identical (BPP-7.1 守)

**蓝图字面源**: plugin-protocol.md §2 frame schema 单源 + BPP-1 #304 reflect lint 自动覆盖

**反约束清单**:

- [ ] SDK 包 `sdk/bpp/` 不重定义任何 envelope struct — 必走 `import "borgee-server/internal/bpp"` 引用
- [ ] AST scan: 反向 grep `type.*Frame.*struct` 在 sdk/bpp/ production *.go count==0
- [ ] `TestBPP7_FrameSchemaByteIdentical` 真测 — reflect 对比 SDK 用的 `bpp.AllBPPEnvelopes()` 15 frame 字段集 + json tag byte-identical
- [ ] connect handshake 真发 ConnectFrame 5 字段 (Type/PluginID/Token/Version/Capabilities) byte-identical 跟 server 接收

## §2 立场 ② SDK Go module + ws 库同源 (BPP-7.1+7.2 守)

**蓝图字面源**: plugin-protocol.md §3 + best-effort 立场承袭 BPP-4/5/6

**反约束清单**:

- [ ] SDK 在 borgee-server 同 module (sdk/bpp/), 复用 server envelope 0 drift
- [ ] ws 库: `github.com/coder/websocket` 跟 server 同源 — 反向 grep `gorilla/websocket\|gobwas/ws\|nhooyr.io/websocket` 在 sdk/ 0 hit
- [ ] AST scan: forbidden tokens `pendingSDKReconnect\|sdkRetryQueue\|deadLetterSDK` 0 hit (best-effort 锁链延伸第 4 处)
- [ ] SDK 不挂 client-side dispatcher (server BPP-3 已挂)

## §3 立场 ③ BPP-3.2.3 retry + BPP-4/5/6 SDK 真实施 (BPP-7.2 守)

**蓝图字面源**: reasons-spec.md (#496 SSOT) + AL-1 #492 single-gate

**反约束清单**:

- [ ] SDK GrantRetry 跟 server `RequestRetryCache` 同行为 — `MaxPermissionRetries=3` + `RetryBackoff=30*time.Second` byte-identical (复用 server const)
- [ ] SDK Heartbeat ticker 30s byte-identical (跟 BPP-4 watchdog 周期同源)
- [ ] SDK Reconnect 携 last_known_cursor (走 BPP-5 帧)
- [ ] SDK ColdStart 不携 cursor (走 BPP-6 帧, RestartReason 走 reasons SSOT 6-dict; 反向 grep `runtime_recovered\|sdk_specific_reason\|7th.*reason` 0 hit)
- [ ] AL-1a reason 锁链 BPP-7 = **第 12 处** (改 = 改十二处)

## §4 蓝图边界 ④⑤⑥⑦ — 跟 BPP-1/3/4/ADM-0 不漂

**反约束清单**:

- [ ] BPP-1 envelope reflect lint 自动覆盖 — 反向断言 SDK 不另定义 frame
- [ ] BPP-3 dispatcher 不复制到 SDK 端 (server-only 边界)
- [ ] BPP-4 30s 周期 byte-identical (HeartbeatInterval const 复用)
- [ ] admin god-mode 不入 SDK 路径 (`internal/api/admin*.go` 反向 grep `admin.*sdk\|admin.*BPP7` 0 hit)

## §5 退出条件

- §1 (4) + §2 (4) + §3 (5) + §4 (4) 全 ✅
- 反向 grep 6 项全 0 hit (frame 重定义 / ws 第三方 / SDK queue / SDK reason / dispatcher / admin)
- e2e 整链: 真启 server + SDK Client connect → 失联 → reconnect cursor resume / cold-start state reset
- AL-1a reason 锁链 BPP-7 = 第 12 处, AST 锁链延伸第 4 处
