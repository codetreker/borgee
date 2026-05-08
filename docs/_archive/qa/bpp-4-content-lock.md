# BPP-4 文案锁 / 立场反查清单 (战马A v0)

> 战马A · 2026-04-29 · ≤80 行 byte-identical 文案锁 (跟 BPP-2 #485 content-lock + AL-2b #481 content-lock 同模式)
> **蓝图锚**: [`plugin-protocol.md`](../blueprint/current/plugin-protocol.md) §1.6 (失联与故障状态 + 故障 UX 区分表 2 类) + module `plugin-protocol.md` §BPP-4 acceptance ("kill plugin → 30s 内 agent 显示 error" 字面)
> **关联**: spec `docs/implementation/modules/bpp-4-spec.md` (战马A v0) + acceptance `docs/qa/acceptance-templates/bpp-4.md` (战马A v0) + stance `docs/qa/bpp-4-stance-checklist.md` (战马A v0) + 复用 AL-1a #249 6 reason byte-identical (跟 AL-1a + AL-3 + CV-4 + AL-2a + AL-1b + AL-4 + BPP-2.2 + AL-2b **八处单测锁链** 同源)

## §1 字面锁 (改 = 改三处: 此文档 + spec / acceptance / 实施代码)

### ① heartbeat watchdog 阈值 30s byte-identical (蓝图 BPP-4 acceptance 字面)

```
BPP_HEARTBEAT_TIMEOUT_SECONDS = 30   // 蓝图 BPP-4 module acceptance "kill plugin → 30s 内 agent 显示 error" 字面单源
BPP_HEARTBEAT_TICKER_INTERVAL = 10s  // ≤ 阈值 / 3 防错过窗口
```

**改 = 改三处**: 此文档 + `bpp-4-spec.md` §0.2 + 实施代码 `internal/bpp/heartbeat_watchdog.go`. 反向 grep `bpp.*heartbeat.*60\|heartbeat.*timeout.*[5-9][0-9]+s\|heartbeatTimeout.*=.*[1-9][0-9]{2,}` count==0 (CI lint 每 BPP-4.* PR 必跑, 防隐式调高).

### ② AL-1a 6-dict reason byte-identical 跟既有八处单测锁链同源 (BPP-4 第 9 处不另起)

```
api_key_invalid          # API key 失效 (AL-1a #249)
quota_exceeded           # 配额超限
network_unreachable      # 网络不通 (BPP-4 watchdog 触发: 长时无 heartbeat = 网络层假定不可达)
runtime_crashed          # runtime 崩溃 (BPP-4 watchdog 触发: 立即 disconnect = 进程死)
runtime_timeout          # runtime 超时
unknown                  # 兜底
```

**字面 byte-identical 跟 AL-1a #249 + AL-3 #305 + CV-4 #380 + AL-2a #454 + AL-1b #458 + AL-4 #387/#461 + BPP-2.2 #485 + AL-2b #481 八处单测锁链同源**, **改 = 改九处单测锁** (BPP-4 是第 9 处). BPP-4 watchdog 触发 PATCH /agents/:id/state, reason 选 `network_unreachable` (跟蓝图 §1.6 故障 UX 区分表第 1 行 "runtime_disconnected → 平台问题 → 重连中…" UI 文案对齐, 平台层判网络失联). 反向 grep `BPP4.*reason.*new\|7th.*reason\|reason.*disconnect_unique` count==0.

### ③ dead-letter log key byte-identical (BPP-4.2 + HB-1 audit schema 复用)

```
bpp.frame_dropped_plugin_offline   # server→plugin push 失败 (sent=false, plugin offline)
                                    # log warn level + audit hint:
                                    #   {actor: "server", action: "frame_drop",
                                    #    target: "<agent_id>", when: <unix_ms>,
                                    #    scope: "<frame_type>:cursor=<cursor>"}
```

**audit log schema byte-identical 跟 HB-1 install-butler audit + HB-2 host-bridge IPC audit 三处同源** (`actor / action / target / when / scope` 五字段), **改 = 改三处单测锁** (跟 HB-4 §1.5 release gate 第 4 行 "审计日志格式锁定 JSON schema" 守门同源). 反向 grep `bpp\.frame_dropped_plugin_offline` count ≥ 1 (CI lint 每 BPP-4 PR 必命中, 防 log key 漂).

### ④ 蓝图 §1.6 故障 UX 区分表 byte-identical (UI 文案锁, BPP-4 不动)

```
runtime_disconnected   平台问题 (plugin 断线、进程崩溃)   "重连中…"
agent_misconfigured    用户问题 (API key 失效、模型超限、tool 配错)   "检查 OpenClaw 设置" + 直达修复入口
```

**字面 byte-identical 跟蓝图 plugin-protocol.md §1.6 故障 UX 区分表同源**. BPP-4 watchdog 触发 server→client 状态翻 error 后, client SPA 按 reason 渲染对应文案 (跟 AL-1b #462 PresenceDot 5-state describeAgentState 已实施同模式; BPP-4 不另起 client 文案). 反向 grep `agent_misconfigured\|runtime_disconnected` 在 client SPA `packages/client/src/` count ≥ 1 (UI 文案锁守).

## §2 反约束 (本 content-lock 守的, byte-identical 跟 spec §0+§3 同源)

1. **不另起 cancel/abort frame** — `bppEnvelopeWhitelist` 不动 (BPP-4 仅复用 HeartbeatFrame 做 watchdog 源)
2. **不挂 retry queue / dead-letter persistent storage** — best-effort 立场, RT-1.3 cursor replay 兜底
3. **不调 admin god-mode 路径走 watchdog** — `internal/api/admin*.go` 反向 grep `admin.*heartbeat.*watchdog\|admin.*BPP4` 0 hit (ADM-0 §1.3 红线)
4. **不直写 presence_sessions 列** — watchdog 走 #457 PATCH /agents/:id/state endpoint (AL-1b 真接管复用), 不绕过

## §3 退出条件 (跟 stance + acceptance + spec 4 件套联签)

- §1 字面锁 4 项 (30s 阈值 / AL-1a 6-dict 第 9 处 / dead-letter log key / 故障 UX 区分表) 全实施代码 byte-identical
- §2 反约束 4 项 CI grep count 全 0 hit
- AL-1a reason 字典锁链 BPP-4 = 第 9 处, 跟 BPP-2.2 第 7 处 + AL-2b 第 8 处链承袭不漂
- HB-1/HB-2 audit log schema 三处同源不漂 (改 = 改三处单测锁)
