# Acceptance Template — BPP-3: plugin 上行 BPP frame 统一 dispatcher 边界

> Spec: `docs/blueprint/current/plugin-protocol.md` §2.2 (Plugin → Server data plane)
> Implementation: `docs/architecture/plugin-protocol.md` §BPP-3
> (与 AL-2b deferred ack ingress 接管, 见 `bpp-2-spec.md` §3 跨段约束)
> Owner: 战马A 实施 / 烈马 验收

## 验收项

### §1 unified dispatcher boundary (PluginFrameDispatcher)

> 锚: `internal/bpp/plugin_frame_dispatcher.go` + 蓝图 §2.2 fire-and-forget event stream

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 1.1 RPC envelope (`{type, id, data}`) vs BPP envelope (`{type, …payload-direct}`) 拆死 — plugin.go read loop default case 路由 BPP frame, RPC 路径不变 | unit | 战马A / 烈马 | `internal/ws/plugin.go` switch default → `hub.pluginFrameRouterSnapshot().Route(data, …)`; `plugin_frame_dispatcher_test.go::TestPluginFrameDispatcher_Route_Happy` |
| 1.2 Register direction-lock — `DirectionServerToPlugin` frame 注册即 panic (defense-in-depth, 跟 BPP-1 #304 direction 锁同模式) | unit | 战马A / 烈马 | `TestPluginFrameDispatcher_Register_PanicsOnServerToPluginFrame` (FrameTypeBPPAgentConfigUpdate 注册 panic) |
| 1.3 Register envelope-whitelist 守 — whitelist 外 frame type 注册即 panic (envelope.go SSOT) | unit | 战马A / 烈马 | `TestPluginFrameDispatcher_Register_PanicsOnUnknownFrameType` |
| 1.4 Register duplicate panic — 单一 frame type 单一 dispatcher (反约束 broadcast) | unit | 战马A / 烈马 | `TestPluginFrameDispatcher_Register_PanicsOnDuplicate` + `_PanicsOnEmptyType` + `_PanicsOnNilDispatcher` |

### §2 forward-compat routing

> 锚: 蓝图 §2.2 plugin upgrade rolling-rollout 容忍

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 2.1 unknown type → soft-skip + log warn `bpp.plugin_frame_unknown_type`, 返 (false, nil) 不掉链接 | unit | 战马A / 烈马 | `TestPluginFrameDispatcher_Route_UnknownType_SoftSkip` |
| 2.2 malformed JSON → soft-skip + log warn `bpp.plugin_frame_malformed_json`, 返 (false, nil) | unit | 战马A / 烈马 | `TestPluginFrameDispatcher_Route_MalformedJSON_SoftSkip` + `_EmptyPayload_SoftSkip` + `_EmptyType_SoftSkip` |
| 2.3 dispatcher 错误 propagation — 注册 dispatcher 返 err, Route 返 (true, err) 同时 log warn `bpp.plugin_frame_dispatch_failed` | unit | 战马A / 烈马 | `TestPluginFrameDispatcher_Route_DispatcherError` |

### §3 AL-2b ack ingress 真接管 (deferred from #481)

> 锚: AL-2b acceptance #452 §2.5 plugin → server ack 路径

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 3.1 AckFrameAdapter raw → typed AgentConfigAckFrame → AckDispatcher delegate path | unit | 战马A / 烈马 | `TestAckFrameAdapter_DecodesAndDelegates` + `_PanicsOnNilDispatcher` + `_DecodeError` + `TestDispatcher_Integration_RegisterRouteAck` |
| 3.2 AgentConfigAckHandlerImpl 三态 log path — applied (Info) / rejected (Warn + reason) / stale (Warn + reason); nil-logger no-op | unit | 战马A / 烈马 | `internal/api/agent_config_ack_handler_test.go::TestBPP3_HandleAck_Applied/Rejected/Stale/NilLoggerNoOp` |
| 3.3 AgentOwnerResolver 走 store.GetAgent SSOT (跟 anchor #360 owner-only 同源); missing agent → error → bpp.AckDispatcher cross-owner reject | unit | 战马A / 烈马 | `TestBPP3_OwnerResolver_ResolvesOwner` + `_MissingAgent` |

### §4 反约束

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 4.1 hub.go (BPP-3 frame router seam) 不 import internal/bpp — `PluginFrameRouter` interface + `pluginFrameRouterAdapter` bridge 守 (反向: hub.go 文件级直 import 0 — 已有 AL-2b push 文件 al_2b_2_agent_config_push.go 单独 import bpp 是 frame builder, 不走 router seam) | grep | 战马A / 烈马 | `grep -n "internal/bpp" packages/server-go/internal/ws/hub.go packages/server-go/internal/ws/plugin.go` 0 hit |
| 4.2 internal/bpp 不 import internal/api — 通过 `AgentConfigAckHandler` / `OwnerResolver` interface seam | grep | 战马A / 烈马 | `grep -rn "internal/api" packages/server-go/internal/bpp/` 0 hit |
| 4.3 admin god-mode 不入 plugin frame 路由 — admin 不持有 PluginConn, 反向 grep `admin.*pluginFrameRouter\|admin.*PluginFrameDispatcher` 0 hit | grep | 战马A / 烈马 | grep 守 |

## 退出条件

- §1 (4) + §2 (3) + §3 (3) + §4 (3) 全绿
- 15 dispatcher unit + 5 handler unit 全 PASS
- REG-BPP3-001..006 入 `docs/qa/regression-registry.md`
- PROGRESS.md BPP-3 [x]
- AL-2b #481 deferred ack ingress 真接管, plugin → server 三态闭环
