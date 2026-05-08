# BPP-2 spec brief — 协议抽象语义层 (semantic action dispatch + reverse-channel task lifecycle)

> 战马E (PM 客串) · 2026-04-29 · ≤80 行 spec lock (实施 3 段拆 PR 由战马 落, Phase 4 plugin-protocol 主线起步第一段)
> **蓝图锚**: [`plugin-protocol.md`](../../blueprint/current/plugin-protocol.md) §1.3 ("Plugin 调 Borgee 抽象语义层 C, 不直对 REST" + 7 v1 必须语义动作 + dispatch 层权限收敛 — 协议红线 "不允许 plugin 下穿语义层直调 REST") + §1.5 ("配置热更新按字段分类" — `agent_config_update` server→plugin) + §1.6 ("失联与故障状态" — task_started/task_finished 是 busy/idle source 唯一上行) + §2.1 §2.2 (BPP 接口清单 v1 — 控制面 6 frame + 数据面 3 frame) + §3 ("现状差距 — plugin 通过 WS api_request 直调 REST → 新增高级动作 API + dispatch 层 + 权限收敛")
> **关联**: 已闭 BPP-1 ✅ #304 (envelope CI lint 真落, 9 frame whitelist + reflect schema lock); BPP-1 仅锁 envelope 形, 未拆"动作语义层" — BPP-2 接力把语义动作从 `api_request` 直调 REST 重构为 `semantic_action` 帧路径; AL-1b busy/idle 跟 BPP-2.2 task_started/task_finished 同期 (蓝图 §2.3 字面 source 必须 plugin 上行); AL-2b ConfigUpdated 跟 BPP-2.3 agent_config_update 同期 (BPP-3 SSOT 真接管)
> **章程闸**: Phase 4 起步路径第一段 — plugin-protocol module 接 BPP-1 ✅ 后, AL-1b / AL-2a 等都依赖 BPP-2 抽象语义层 dispatch + reverse-channel 落地

> ⚠️ 锚说明: BPP-1 #304 已锁 9 frame envelope 字面 (control + data plane), 但 plugin 端写动作仍走 `api_request` 直调 REST (蓝图 §3 差距字面), 没有 server-side dispatch 层做权限统一收敛. BPP-2 要落"动作 = 帧" 模型 — plugin 上行 `semantic_action` frame 含 `op` + `args`, server dispatch 层路由到既有 REST handler + 权限统一闸 (复用 AP-0 RequirePermission), 不允许 plugin 下穿走 raw REST endpoint.

## 0. 关键约束 (3 条立场, 蓝图 §1.3 + §1.5 + §1.6 字面)

1. **语义动作 = 帧不是 REST 直调** (蓝图 §1.3 字面 "协议红线 不允许 plugin 下穿语义层直调 REST"): plugin 上行 `semantic_action` frame `{type, cursor, action_id, op, agent_id, channel_id, args, started_at}` 8 字段 byte-identical (跟 BPP-1 envelope 共序 type/cursor 头位); op ∈ 7 v1 白名单 (`create_artifact`/`update_artifact`/`reply_in_thread`/`mention_user`/`request_agent_join`/`read_channel_history`/`read_artifact`); server dispatch 层 `bpp.HandleSemanticAction` 路由到既有 REST handler 接 (复用 ArtifactHandler / MessageHandler 等), **反约束**: 不在 BPP frame envelope 加 raw HTTP method / path / headers 字段 (那是 REST 不是语义层); 不开 plugin 端 WS `api_request` 旁路 (BPP-1 既有 envelope 不裂)
2. **task lifecycle 上行帧是 busy/idle 唯一 source** (蓝图 §1.6 + §2.3 字面 "busy/idle source 必须 plugin 上行 frame, 不准 stub"): plugin 上行 `task_started` frame `{type, cursor, task_id, agent_id, channel_id, subject, started_at}` 7 字段 + `task_finished` frame `{type, cursor, task_id, agent_id, channel_id, outcome, finished_at}` 7 字段 byte-identical (subject 文案锁 — 空字符串 server 拒收 + log warn, 跟蓝图 §2.3 "缺心跳按未知 / runtime 崩溃显示故障" 同源); progress 在 task_started 后周期发, **反约束**: subject 必带且非空 (空则 server 拒收, AL-1b 不渲染), outcome ∈ ('completed','failed','cancelled') 3 态 enum + reason 复用 AL-1a 6 reason byte-identical (改 = 改三处单测锁 #249 + AL-3 #305 + AL-4 #321)
3. **配置热更新单源 server→plugin** (蓝图 §1.5 字面 "agent_config_update server→plugin 推送, plugin 必须支持幂等 reload, runtime 不缓存 agent 定义"): server 推 `agent_config_update` frame `{type, cursor, agent_id, config_version, fields, updated_at}` 6 字段 byte-identical (fields ∈ {name, avatar, prompt, model, capabilities, enabled} 6 项白名单, BPP-3 SSOT 接管时复用); 反约束: plugin 不上行 config (config 是 SSOT 单向 server→plugin, plugin 主动改 config 路径不存在); 不在此帧带 raw `api_key` / `temperature` 等 runtime 调优字段 (蓝图 §1.4 那是 runtime 内部事, 立场 ① "Borgee 不带 runtime")

## 1. 拆段实施 (BPP-2.1 / 2.2 / 2.3, ≤ 3 PR)

| 段 | 范围 | 闭锁 | owner |
|---|---|---|---|
| **BPP-2.1** semantic_action dispatch 层 + envelope schema | `internal/bpp/semantic_action_frame.go` 新 8 字段 frame (type/cursor/action_id/op/agent_id/channel_id/args/started_at); `bpp.HandleSemanticAction(frame)` dispatcher 路由 7 op 到既有 REST handler (复用 ArtifactHandler/MessageHandler); op ∈ 7 v1 白名单 enum (反约束: 'list_users' 等枚举外值 reject + log warn); 权限走 AP-0 RequirePermission (跟 既有 REST 同闸); BPP-1 #304 envelope CI lint reflect 自动覆盖新 frame; **反约束 grep**: `grep -rnE 'api_request.*method.*POST\|raw.*REST.*plugin' packages/server-go/internal/bpp/` count==0 (立场 ① 不下穿) | 待 PR (战马 实施) | 战马 |
| **BPP-2.2** task lifecycle reverse-channel + AL-1b busy/idle source | `internal/bpp/task_started_frame.go` 7 字段 + `task_finished_frame.go` 7 字段 byte-identical (跟 BPP-1 共序); progress 在 task_started 后周期发; subject 文案锁 (空 reject + log warn `bpp.task_subject_empty`); outcome 3 态 enum (`completed`/`failed`/`cancelled`); failed 时 reason 复用 AL-1a 6 reason byte-identical 单测锁同源; AL-1b agent.busy/idle 状态从此帧驱动 (跟 AL-3 presence 拆死 — busy 是 task-level 不是 session-level, 反约束: 不写 presence_sessions); WS push `AgentTaskStateChangedFrame` 8 字段 (type/cursor/agent_id/channel_id/state/subject/outcome/timestamp) 给 owner 端 — busy/idle UI 锁 | 待 PR (战马 + AL-1b 同期) | 战马 / 飞马 / 野马 (subject 文案) |
| **BPP-2.3** agent_config_update server→plugin + 反约束 lint | `internal/bpp/agent_config_update_frame.go` 6 字段 byte-identical (type/cursor/agent_id/config_version/fields/updated_at); fields ∈ 6 项白名单 enum (反约束: api_key/temperature 等 runtime 调优字段 reject); server 端 `bpp.PushConfigUpdate(agentID, configVersion, fields)` adapter (跟 hubAnchorAdapter 同模式); BPP-3 SSOT 接管前 stub 永远幂等 (plugin 重复 reload 同 payload 无副作用); 反向 grep CI lint `grep -nE 'plugin.*upload.*config|client.*push.*agent_config'` count==0 (立场 ③ config 单向 server→plugin); BPP envelope CI lint 反射扫白名单全过 | 待 PR (战马 / BPP-3 同期) | 战马 / 飞马 |

## 2. 与 BPP-1 / AL-1b / AL-2b / BPP-3 留账冲突点

- **BPP-1 envelope CI lint 复用** (核心): BPP-2 三个新 frame (semantic_action / task_started / task_finished / agent_config_update) 加入 `bppEnvelopeWhitelist` (反约束: 不裂新 namespace, 同 9→13 frame 同模式扩); reflect schema lock 自动覆盖
- **AL-1b busy/idle 同期** (强依赖): AL-1b 当前 stub (#249 三态 online/offline/error), busy 状态需 BPP-2.2 task_started 真上行才能落 — AL-1b PR 跟 BPP-2.2 同期合 (蓝图 §2.3 字面 "stub 一旦上 v1 要拆掉 = 白写"); 反约束: AL-1b 不另起 source, busy/idle 永远走 task lifecycle frame
- **AL-2b ConfigUpdated 同期** (BPP-3 路径): BPP-2.3 落 agent_config_update frame envelope 字面, AL-2b 真接管 (config UI + push 触发) 跟 BPP-3 SSOT 同 PR 合 (PROGRESS line 209 字面); 反约束: AL-2b 不再起 frame namespace
- **BPP-3 配置 SSOT** (协调): BPP-2.3 仅锁 frame envelope + stub adapter, 真 SSOT 表 + 推送触发由 BPP-3 接管 (跟 AL-2b 同 PR); 不在 BPP-2 落 SSOT schema migration
- **AP-0 RequirePermission 复用** (核心): semantic_action dispatch 层走既有 RequirePermission middleware (跟 AL-4.2 #414 `agent.runtime.control` 模式同), 不开新权限名空间; 7 op 白名单跟既有 REST endpoint 权限一一对应
- **AL-3 presence 拆死** (反约束): busy 是 task-level (BPP-2.2 反信号), online 是 session-level (AL-3 #310 presence_sessions); 反向 grep `presence_sessions.*busy|presence.*task_id` count==0 (跟 AL-4.1 #398 `agent_runtimes.*is_online` count==0 同模式 — 一表一职)
- **CV-4.2 IterationStateChanged 同源** (非冲突): CV-4.2 #409 已落 IterationStateChangedFrame 9 字段, BPP-2 task_started/task_finished 是 plugin 上行 (data plane), CV-4.2 是 server 下推 (control plane), 两 frame 共 hub.cursors 单调 sequence 不冲突

## 3. 反查 grep 锚 (Phase 4 BPP-2 验收)

```
git grep -nE 'SemanticActionFrame|type.*semantic_action'      packages/server-go/internal/bpp/         # ≥ 1 hit (BPP-2.1)
git grep -nE 'TaskStartedFrame|TaskFinishedFrame|task_started|task_finished' packages/server-go/internal/bpp/  # ≥ 2 hit (BPP-2.2)
git grep -nE 'AgentConfigUpdateFrame|agent_config_update'     packages/server-go/internal/bpp/         # ≥ 1 hit (BPP-2.3)
git grep -nE 'HandleSemanticAction|bpp.*Dispatch.*op'         packages/server-go/internal/bpp/         # ≥ 1 hit (BPP-2.1 dispatcher)
git grep -nE 'subject.*empty|task_subject_empty'              packages/server-go/internal/bpp/         # ≥ 1 hit (BPP-2.2 subject 反约束)
# 反约束 (5 条 0 hit)
git grep -nE 'api_request.*method.*POST|raw.*REST.*plugin'    packages/server-go/internal/bpp/         # 0 hit (立场 ① plugin 不下穿 REST)
git grep -nE 'plugin.*upload.*config|client.*push.*agent_config' packages/server-go/internal/      # 0 hit (立场 ③ config 单向)
git grep -nE 'presence_sessions.*busy|presence.*task_id'      packages/server-go/internal/         # 0 hit (busy task-level vs online session-level 拆死)
git grep -nE 'subject.*=.*""|subject.*\.\.\.|fallback.*subject' packages/server-go/internal/bpp/    # 0 hit (subject 必带非空, 反默认值)
git grep -nE 'api_key|temperature.*config|model.*api_key'     packages/server-go/internal/bpp/agent_config_update_frame.go   # 0 hit (立场 ③ runtime 调优字段不入)
```

任一 0 hit (除反约束行) → CI fail.

## 4. 不在本轮范围 (反约束)

- ❌ BPP 协议版本协商机制 (留 Phase 5+, 蓝图 §4 字面)
- ❌ remote-agent setup OpenClaw 安装管家 (留第 6 轮 host-bridge module, 蓝图 §4 字面)
- ❌ 跨 runtime 协作场景 (OpenClaw + Hermes 混跑, 留 v2 后, 蓝图 §4 字面)
- ❌ semantic action v2+ propose_artifact_change / request_owner_review / request_clarification 3 项 (留 v2+, 蓝图 §1.3 v2+ 列表字面)
- ❌ progress frame body 算法详细 (subject 必带非空 + 频率 ≤ 1/s 蓝图 §2.2 字面, 具体 progress 内容 schema 留 BPP-2 后续 patch)
- ❌ agent_config UI 形态 (留第 11 轮 client SPA, 蓝图 §4 字面)
- ❌ 权限 dispatch 层具体实现 (复用 AP-0 RequirePermission, 蓝图 §4 第 8 轮 auth 留账)
- ❌ memory_ref 内容存储 (memory 内容在 runtime, 蓝图 §1.4 字面 "v1 不让 Borgee 变向量库")

## 5. Test plan (实施 PR 各自带, 此 spec 不带)

- BPP-2.1: SemanticActionFrame 8 字段 byte-identical (golden JSON) + dispatch 层 7 op 白名单 hit + 'list_users' 枚举外值 reject + 权限走 AP-0 RequirePermission 复用 + plugin 不下穿 REST 反向断言 (TestBPP21NoRawRESTBypass)
- BPP-2.2: TaskStartedFrame + TaskFinishedFrame 7 字段 byte-identical 各 + subject 空 reject (TestBPP22SubjectEmptyRejected) + outcome 3 态 enum 全过 + outcome 字典外 reject + AgentTaskStateChangedFrame 8 字段 (server→client owner busy/idle UI 推) + AL-1b busy/idle 状态机驱动 (clock fixture 跟 AL-3 同模式)
- BPP-2.3: AgentConfigUpdateFrame 6 字段 byte-identical + fields 6 项白名单全过 + 'api_key' / 'temperature' 反向 reject (TestBPP23ConfigFieldsWhitelist) + 幂等 reload (同 payload 重复推送无副作用) + plugin 不上行 config 反向断言 (TestBPP23ConfigSingleSource)
