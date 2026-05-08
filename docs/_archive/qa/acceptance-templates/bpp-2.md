# Acceptance Template — BPP-2: 协议抽象语义层 (semantic action dispatch + task lifecycle reverse channel + agent_config_update)

> 蓝图: `plugin-protocol.md` §1.3 (Plugin 调 Borgee 抽象语义层 C, 不直对 REST + 协议红线 "不允许 plugin 下穿语义层直调 REST" + 7 v1 必须语义动作字面) + §1.5 (配置热更新单源 server→plugin + 幂等 reload) + §1.6 (失联与故障状态 — task_started/task_finished 是 busy/idle source 唯一上行) + §2.1+§2.2 (BPP 接口清单 v1) + §3 (现状差距 — plugin WS api_request 直调 REST → 新增高级动作 API + dispatch 层 + 权限收敛)
> Spec: `docs/implementation/modules/bpp-2-spec.md` (战马E PM 客串, 3 立场 + 3 拆段 + 11 grep 反查 含 5 反约束)
> 文案锁: 共用 `bpp-2-spec.md` §0 + AL-1a #249 reason 6 项字面承袭 (跟 AL-3 #305 + AL-4 #321 三处单测锁同源)
> 拆 PR: **BPP-2 整 milestone 一 PR** (新协议 "一 milestone = 一 worktree = 一 PR" #479): `feat/bpp-2` PR #485 三段一次合 — BPP-2.1 dispatch 层 + 7 op 白名单 + ActionHandler interface + BPP-2.2 task lifecycle reverse-channel (task_started/task_finished + subject 空 reject + outcome 3 态 + reason AL-1a 6 字典) + BPP-2.3 agent_config_update validation + 6 fields 白名单 + ConfigRevTracker 幂等 reload. AL-1b busy/idle source 真接管 + AL-2b/BPP-3 SSOT 推送触发留 BPP-3 起步时同期合.
> Owner: 战马E (PM 客串 + 实施) / 飞马 review / 烈马 验收
> Status: ✅ 三段四件全闭 (实施 PR #485 land + REG-BPP2-001..017 全 🟢)

## 验收清单

### §1 BPP-2.1 — semantic_action frame envelope + dispatch 层

> 锚: 战马E spec §1 BPP-2.1 + BPP-1 #304 envelope CI lint 复用 + AP-0 RequirePermission 同闸

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 1.1 SemanticActionFrame 8 字段 byte-identical envelope `{type, cursor, action_id, op, agent_id, channel_id, args, started_at}` 跟 BPP-1 envelope 共序 type/cursor 头位 (跟 ArtifactUpdated 7 / AnchorCommentAdded 10 / MentionPushed 8 / IterationStateChanged 9 同模式) | unit + golden JSON | 战马 / 烈马 | `internal/bpp/semantic_action_frame_test.go::TestSemanticActionFrameFieldOrder` (TBD, golden JSON byte-equality 8 字段顺序) + BPP-1 #304 envelope CI lint reflect 自动覆盖加入 `bppEnvelopeWhitelist` 13 frame |
| 1.2 op 白名单 7 项 ('create_artifact'/'update_artifact'/'reply_in_thread'/'mention_user'/'request_agent_join'/'read_channel_history'/'read_artifact') byte-identical 跟蓝图 §1.3 v1 列表字面 | unit table-driven | 战马 / 烈马 | `bpp/semantic_action_test.go::TestSemanticAction_OpWhitelist` (TBD, 7 op 全过 + 'list_users' 等枚举外值 reject + 反向 grep `op.*list_users\|op.*delete_org` count==0) |
| 1.3 dispatch 层 `bpp.HandleSemanticAction(frame)` 路由到既有 REST handler (复用 ArtifactHandler/MessageHandler) — 不开 raw REST 旁路 | unit + grep | 战马 / 飞马 / 烈马 | `bpp/dispatcher_test.go::TestDispatch_RoutesToExistingHandlers` (TBD, mock handler + assert 7 op 路由命中) + `grep -rnE 'api_request.*method.*POST\|raw.*REST.*plugin' packages/server-go/internal/bpp/` count==0 |
| 1.4 权限走 AP-0 RequirePermission middleware (跟 AL-4.2 #414 `agent.runtime.control` 模式同) — 7 op 跟既有 REST endpoint 权限一一对应 | unit | 战马 / 烈马 | `bpp/semantic_action_test.go::TestSemanticAction_PermissionGate` (TBD, 'create_artifact' 走 channel 权限 + 'mention_user' 走 message.send 既有 perm) |
| 1.5 反约束 — plugin 不下穿走 raw REST endpoint (立场 ① 协议红线) | CI grep | 飞马 / 烈马 | `grep -rnE 'api_request.*method.*POST\|raw.*REST.*plugin' packages/server-go/internal/bpp/` count==0 (CI lint 每 BPP-2.* PR 必跑) |

### §2 BPP-2.2 — task lifecycle reverse-channel + AL-1b busy/idle source

> 锚: 战马E spec §1 BPP-2.2 + 蓝图 §1.6 + §2.3 字面 "busy/idle source 必须 plugin 上行 frame, 不准 stub" + AL-1a #249 6 reason 同源

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 2.1 TaskStartedFrame 7 字段 byte-identical `{type, cursor, task_id, agent_id, channel_id, subject, started_at}` 跟 BPP-1 envelope 共序 + subject 必带非空 (空字符串 server 拒收 + log warn `bpp.task_subject_empty`) | unit + golden JSON | 战马 / 野马 (subject 文案) / 烈马 | `bpp/task_started_frame_test.go::TestTaskStartedFrameFieldOrder` (TBD, 7 字段 byte-identical) + `TestTaskStarted_SubjectEmpty_Rejected` (空 subject reject + log warn) |
| 2.2 TaskFinishedFrame 7 字段 byte-identical `{type, cursor, task_id, agent_id, channel_id, outcome, finished_at}` + outcome 3 态 enum ('completed'/'failed'/'cancelled') + failed 时 reason 复用 AL-1a 6 reason byte-identical | unit + table-driven | 战马 / 烈马 | `bpp/task_finished_frame_test.go::TestTaskFinishedFrameFieldOrder` (TBD, 7 字段) + `TestTaskFinished_OutcomeEnum` (3 态全过 + 字典外 reject) + `TestTaskFinished_FailedReasonAL1aSame` (6 reason byte-identical 跟 #249 + AL-3 #305 + AL-4 #321 三处单测锁同源) |
| 2.3 AL-1b busy/idle 状态从 task_started/task_finished 帧驱动 (蓝图 §2.3 字面 source 必须 plugin 上行) — 跟 AL-3 presence 拆死 (busy task-level / online session-level) | unit + clock fixture | 战马 / 飞马 / 烈马 | `bpp/al1b_busy_state_test.go::TestAL1b_TaskStartedSetsBusy` (TBD, clock fixture + assert agent.state='busy') + `TestAL1b_TaskFinishedClearsBusy` + 反向断言 `presence_sessions.*busy\|presence.*task_id` count==0 (跟 AL-4.1 #398 `agent_runtimes.*is_online` count==0 同模式) |
| 2.4 AgentTaskStateChangedFrame 8 字段 byte-identical `{type, cursor, agent_id, channel_id, state, subject, outcome, timestamp}` server→client owner busy/idle UI 推 (cursor 走 hub.cursors 单调发号跟 ArtifactUpdated 共序) | unit + golden JSON | 战马 / 烈马 | `internal/ws/agent_task_state_changed_frame_test.go::TestAgentTaskStateChangedFrameFieldOrder` (TBD, 8 字段 byte-identical) + cursor 共 sequence smoke (跟 anchor_comment_frame_test 同模式) |
| 2.5 反约束 — subject 必带非空 (反默认值不 fallback) + reason 字典字面承袭 AL-1a #249 6 reason 不另起 | CI grep | 飞马 / 烈马 | `grep -nE 'subject.*=.*""\|subject.*\.\.\.\|fallback.*subject' packages/server-go/internal/bpp/` count==0 + `grep -nE '"api_key_invalid"\|"quota_exceeded"\|"network_unreachable"\|"runtime_crashed"\|"runtime_timeout"\|"unknown"' packages/server-go/internal/bpp/task_finished_frame.go` count==6 |

### §3 BPP-2.3 — agent_config_update server→plugin + 反约束 lint

> 锚: 战马E spec §1 BPP-2.3 + 蓝图 §1.5 字面 "幂等 reload, runtime 不缓存" + 跟 AL-2b/BPP-3 同期合

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 3.1 AgentConfigUpdateFrame 6 字段 byte-identical `{type, cursor, agent_id, config_version, fields, updated_at}` 跟 BPP-1 envelope 共序 + config_version 单调 | unit + golden JSON | 战马 / 飞马 / 烈马 | `bpp/agent_config_update_frame_test.go::TestAgentConfigUpdateFrameFieldOrder` (TBD, 6 字段 byte-identical) + BPP-1 #304 envelope CI lint reflect 自动覆盖 |
| 3.2 fields 6 项白名单 ('name'/'avatar'/'prompt'/'model'/'capabilities'/'enabled') byte-identical 跟蓝图 §1.4 表字面 — runtime 调优字段 ('api_key'/'temperature'/'token_limit') reject | unit + grep | 战马 / 烈马 | `bpp/agent_config_update_test.go::TestAgentConfigFieldsWhitelist` (TBD, 6 项全过 + 'api_key'/'temperature' 反向 reject + log warn `bpp.config_field_disallowed`) + `grep -nE 'api_key\|temperature.*config\|model.*api_key' packages/server-go/internal/bpp/agent_config_update_frame.go` count==0 |
| 3.3 server `bpp.PushConfigUpdate(agentID, configVersion, fields)` adapter 跟 hubAnchorAdapter / hubIterationAdapter 同模式 — config 单向 server→plugin | unit + grep | 战马 / 烈马 | `bpp/push_config_test.go::TestPushConfigUpdate` (TBD, mock plugin connection + assert frame 落) + 反向 grep `plugin.*upload.*config\|client.*push.*agent_config` count==0 (立场 ③ config 单向) |
| 3.4 plugin 幂等 reload (同 payload 重复推送无副作用, 蓝图 §1.5 字面) — config_version 反查跨重复推送等值 | unit | 战马 / 烈马 | `bpp/agent_config_update_test.go::TestPushConfigUpdate_Idempotent` (TBD, 同 payload 推 2 次 → plugin 接受但只触发 1 次 reload, runtime 不缓存反向断言) |
| 3.5 反约束 — config 单源 (plugin 不上行 config) + runtime 调优字段不入帧 (立场 ③) | CI grep | 飞马 / 烈马 | `grep -nE 'plugin.*upload.*config\|client.*push.*agent_config' packages/server-go/internal/` count==0 + `grep -nE 'api_key\|temperature' packages/server-go/internal/bpp/agent_config_update_frame.go` count==0 |

### §4 反向 grep / e2e 兜底 (跨 BPP-2.x 反约束横切)

> 锚: 战马E spec §3 11 grep + §4 8 反约束 + BPP-1 #304 envelope CI lint 自动覆盖加入新 4 frame 模式

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 4.1 立场 ① plugin 不下穿走 raw REST — `grep -rnE 'api_request.*method.*POST\|raw.*REST.*plugin' packages/server-go/internal/bpp/` count==0 | CI grep | 飞马 / 烈马 | _(每 BPP-2.* PR 必跑)_ |
| 4.2 立场 ② busy/idle source 唯一是 task lifecycle frame (不写 presence_sessions) — `grep -rnE 'presence_sessions.*busy\|presence.*task_id' packages/server-go/internal/` count==0 | CI grep | 飞马 / 烈马 | _(BPP-2.2 PR + AL-1b 同期合 必跑)_ |
| 4.3 立场 ③ config 单源 server→plugin — `grep -rnE 'plugin.*upload.*config\|client.*push.*agent_config' packages/server-go/internal/` count==0 | CI grep | 飞马 / 烈马 | _(BPP-2.3 PR + AL-2b 同期合 必跑)_ |
| 4.4 subject 必带非空反约束 — `grep -nE 'subject.*=.*""\|fallback.*subject\|subject.*default' packages/server-go/internal/bpp/` count==0 | CI grep | 飞马 / 野马 / 烈马 | _(BPP-2.2 PR 必跑, 跟 AL-1b 状态机文案锁同源)_ |
| 4.5 runtime 调优字段不入 BPP-2.3 frame (立场 ③ Borgee 不带 runtime + 蓝图 §1.4 字面分界) — `grep -nE 'api_key\|temperature' packages/server-go/internal/bpp/agent_config_update_frame.go` count==0 | CI grep | 飞马 / 烈马 | _(BPP-2.3 PR 必跑)_ |
| 4.6 BPP-1 #304 envelope CI lint 反射扫白名单 — 加入 4 新 frame (semantic_action / task_started / task_finished / agent_config_update) 自动覆盖 13 frame whitelist | CI lint | 飞马 / 烈马 | _(每 BPP-2.* PR 必跑, BPP-1 既有 lint reflect 自动扫)_ |
| 4.7 e2e — plugin 上行 semantic_action `create_artifact` → server dispatch 到 ArtifactHandler.handleCreate → REST 路径既有权限 + 行为不变 (反向断言 plugin 路径跟 owner 手动 POST 路径行为 byte-identical) | e2e | 战马 / 烈马 | `packages/e2e/tests/bpp-2-semantic-action.spec.ts` (TBD, 跟 cv-1-3-canvas.spec.ts 真 server-go + plugin mock 同模式) |

## 边界 (跟其他 milestone 关系)

| Milestone | 关系 | 字面承袭 |
|---|---|---|
| BPP-1 ✅ #304 | envelope CI lint 反射扫 + 9 frame whitelist 复用扩 13 | bppEnvelopeWhitelist 加入 4 新 frame, reflect schema lock 自动覆盖 |
| AL-1a ✅ #249 | reason 6 项字典字面承袭 (改 = 改三处单测锁 #249 + AL-3 #305 + AL-4 #321 + BPP-2.2) | task_finished outcome='failed' 时 reason ∈ AL-1a 6 项 byte-identical |
| AL-1b 同期 | busy/idle source 真接管 (蓝图 §2.3 字面 stub 一旦上 v1 拆掉 = 白写) | BPP-2.2 PR + AL-1b 同 PR 合, 不另起 source |
| AL-2b 同期 | ConfigUpdated frame 真接管 (跟 BPP-3 SSOT 同 PR) | BPP-2.3 落 envelope, AL-2b/BPP-3 接管 SSOT 表 + 推送触发 |
| AL-3 ✅ #310 | presence 路径拆死 (busy task-level / online session-level) | 反向 grep `presence_sessions.*busy` count==0 — 一表一职 |
| AL-4.1 ✅ #398 | agent_runtimes 路径拆死 (process-level vs task-level) | 反向 grep `agent_runtimes.*task_id` count==0 — 跟 AL-3 同模式 |
| AL-4.2 ✅ #414 | RequirePermission `agent.runtime.control` 模式承袭 | semantic action dispatch 层走 AP-0 既有 perm 闸 |
| AP-0 ✅ | RequirePermission middleware 复用 (7 op 跟既有 REST 权限一一对应) | 不开新权限名空间 |
| BPP-3 同期 | agent_config SSOT 表 + 推送触发 (BPP-2.3 + AL-2b 同 PR 合) | BPP-2 仅锁 envelope, BPP-3 接管 SSOT |
| CV-4.2 ✅ #409 | IterationStateChanged 9 字段不冲突 (control plane vs data plane 分流) | 共 hub.cursors 单调 sequence |
| RT-1 ✅ | cursor 单调 (hub.cursors) BPP-2 三新 frame 共序 | 不另起 channel cursor |

## 退出条件

- §1 BPP-2.1 5 项 + §2 BPP-2.2 5 项 + §3 BPP-2.3 5 项 + §4 反向 grep 7 项**全绿** (一票否决)
- 4 新 frame (semantic_action / task_started / task_finished / agent_config_update) 加入 `bppEnvelopeWhitelist` (BPP-1 #304 envelope CI lint reflect 自动扫白名单 13 frame 全过)
- 7 v1 op 白名单 (蓝图 §1.3) + 6 fields 白名单 (蓝图 §1.4) + 3 outcome enum + AL-1a 6 reason 字面承袭 共 22 enum 字面 byte-identical 守住
- AL-1b busy/idle source 真接管 (跟 BPP-2.2 同 PR 合, 不留 stub) + AL-2b/BPP-3 同期 (BPP-2.3 PR 单独 land 也可, 留 SSOT 接管账)
- §4 反向 grep 5 反约束 (1 plugin 下穿 / 2 busy presence 错位 / 3 config 上行 / 4 subject 默认值 / 5 runtime 调优字段) 现在已可机器化跑 (跟 AL-4 #318 / CV-4 #384 早期机器化模式同)
- 登记 `docs/qa/regression-registry.md` REG-BPP2-001..017 (5 §1 + 5 §2 + 5 §3 + 7 §4 = 22 行, 待战马 PR 落后开号回填, 跟 #318 AL-4 / #384 CV-4 同模式)

## 5. 更新日志

| 日期 | 作者 | 变化 |
|------|------|------|
| 2026-04-29 | 战马E (PM 客串) | v0 skeleton — Phase 4 plugin-protocol 主线起步第一段 BPP-2 acceptance 4 件套 (spec / acceptance / 文案锁借 spec / stance 借 spec) 起步; §0 3 立场 + §1-§4 验收清单 (15 主项 + 7 反向 grep) + 边界表 (BPP-1/AL-1a/AL-1b/AL-2b/AL-3/AL-4/AP-0/BPP-3/CV-4.2/RT-1 关系); 跟 #318 AL-4 + #293 DM-2 + #384 CV-4 acceptance skeleton 同模式 4 件套并行 — 战马 实施待 spawn / 飞马 review / 野马 subject 文案 / 烈马 验收; 4 新 frame 加入 BPP-1 #304 envelope CI lint reflect whitelist 13 frame 自动覆盖. |
