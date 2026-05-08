# BPP-2 立场反查清单 (战马E PM 客串 v0)

> 战马E (PM 客串) · 2026-04-29 · 立场 review checklist (跟 #387 al-4-stance-checklist + #385 cv-4-stance-checklist 同模式)
> **目的**: BPP-2 三段实施 (BPP-2.1 / 2.2 / 2.3) PR review 时, 飞马/野马/烈马按此清单逐立场 sign-off, 反向断言代码层守住每条立场.
> **关联**: spec `docs/implementation/modules/bpp-2-spec.md` (战马E v0) + acceptance `docs/qa/acceptance-templates/bpp-2.md` (战马E v0) + 文案锁 `docs/qa/bpp-2-content-lock.md` (战马E v0)

## §0 立场总表 (3 立场 + 4 蓝图边界)

| # | 立场 | 蓝图字面 | 反约束 (代码层守门) |
|---|---|---|---|
| ① | 语义动作 = 帧不是 REST 直调 | plugin-protocol.md §1.3 "协议红线" | `internal/bpp/dispatcher.go::HandleSemanticAction` 走 AP-0 RequirePermission, plugin 路径无 raw REST 直调 |
| ② | task lifecycle 上行帧是 busy/idle 唯一 source | plugin-protocol.md §1.6 + agent-lifecycle.md §2.3 | `agent.state.SetBusy` 仅由 BPP-2.2 task_started 触发, presence_sessions 不写 busy 列 |
| ③ | 配置热更新单源 server→plugin | plugin-protocol.md §1.5 + §1.4 表 | `internal/bpp/agent_config_update_frame.go::ValidFields` 6 项白名单, runtime 调优字段不入 frame |
| ④ (边界) | BPP envelope CI lint 复用 | bpp-1.md §1.1 + plugin-protocol.md §2 | bppEnvelopeWhitelist 9→13 扩, reflect schema lock 自动覆盖 |
| ⑤ (边界) | reason 字典承袭 AL-1a 6 项 | concept-model.md §1.6 | `internal/agent/state.go::Reason*` source-of-truth, 改 = 改六处单测锁 (AL-1a #249 + AL-3 #305 + CV-4 #380 + AL-2a #454 + AL-1b #458 + AL-4 #387/#461) |
| ⑥ (边界) | 不开 raw REST `api_request` 旁路 | plugin-protocol.md §1.3 协议红线 | envelope.go 不加 api_request 字段 / type |
| ⑦ (边界) | 不写跨 runtime / cross-plugin 协作 | plugin-protocol.md §4 | spec §4 + acceptance §4 反约束兜底 |

## §1 立场 ① 语义动作 = 帧不是 REST 直调 (BPP-2.1 守)

**蓝图字面源**: `plugin-protocol.md` §1.3 "Plugin 调 Borgee 抽象语义层 (C), 不直对 REST" + "**关键洞察**: 动作集就是 Borgee 的协作姿态" + "不允许 plugin 下穿语义层直调 REST — 这是协议红线"

**反约束清单**:

- [ ] plugin 上行 frame 类型 ∈ {semantic_action, task_started, task_finished, agent_config_update_ack, heartbeat, error_report} (data plane), **不含** `api_request` / `raw_rest` / `direct_post` 类
- [ ] `internal/bpp/dispatcher.go::HandleSemanticAction(frame)` 是 op→handler 单源路由, 反向断言: 不在 dispatcher 中拼 raw URL 调 `http.Client` (反向 grep `http.Client.*Do\|http.Post.*api/v1` count==0)
- [ ] op ∈ 7 v1 白名单 (create_artifact / update_artifact / reply_in_thread / mention_user / request_agent_join / read_channel_history / read_artifact), 'list_users' / 'delete_org' 等枚举外值 reject + log warn `bpp.semantic_op_unknown`
- [ ] 权限走 AP-0 RequirePermission (跟 AL-4.2 #414 `agent.runtime.control` 模式同), 7 op 跟既有 REST endpoint 权限一一对应
- [ ] 反向 grep `api_request.*method.*POST\|raw.*REST.*plugin` count==0 (CI lint 每 BPP-2.* PR 必跑, acceptance §4.1)

**review 通关条件**: 飞马 sign-off 7 op 白名单字面 + 野马 sign-off `bpp.semantic_op_unknown` 错码字面 + 烈马 sign-off `TestDispatch_RoutesToExistingHandlers` 7 op 路由命中.

## §2 立场 ② task lifecycle 上行帧是 busy/idle 唯一 source (BPP-2.2 守)

**蓝图字面源**: `plugin-protocol.md` §1.6 "工作中状态需要 plugin 主动心跳上报 — 缺心跳按未知" + `agent-lifecycle.md` §2.3 "busy / idle 在 Phase 2 不实现, source 必须是 plugin 上行的 task_started / task_finished frame, 没 BPP 就只能 stub, stub 一旦上 v1 要拆掉 = 白写"

**反约束清单**:

- [ ] `agent.state.SetBusy(agentID, taskID, subject)` 仅由 BPP-2.2 task_started frame 触发, **不**由 timer / heartbeat / REST 路径触发
- [ ] `agent.state.ClearBusy(agentID, taskID, outcome)` 仅由 BPP-2.2 task_finished frame 触发
- [ ] presence_sessions 表 (#310 AL-3.1) 不写 busy 列 / 不写 task_id 列 (跟 AL-3 拆死 — busy task-level / online session-level, 一表一职)
- [ ] subject 必带非空 — 空字符串 server 拒收 + log warn `bpp.task_subject_empty`, **不 fallback 到 "处理中..." 等默认值** (蓝图 §11 文案守 "野马硬条件 不准用模糊文案糊弄")
- [ ] outcome 3 态严闭 ('completed' / 'failed' / 'cancelled'), 'partial' / 'paused' / 'pending' / 'starting' 中间态 reject
- [ ] failed 时 reason ∈ AL-1a 6 项 (api_key_invalid / quota_exceeded / network_unreachable / runtime_crashed / runtime_timeout / unknown), 'unknown_reason' 等不属枚举值 reject
- [ ] AL-1b busy/idle 状态机驱动 (跟 BPP-2.2 同 PR 合, 不留 stub — 蓝图 "stub 一旦上 v1 拆掉 = 白写")
- [ ] AgentTaskStateChangedFrame 8 字段 byte-identical 跟 BPP-1 envelope 共序 cursor 头位 (跟 IterationStateChanged 9 / ArtifactUpdated 7 / AnchorCommentAdded 10 / MentionPushed 8 同模式)
- [ ] 反向 grep `presence_sessions.*busy\|presence.*task_id\|busy.*online\|online.*busy` count==0 (CI lint 每 BPP-2.* + AL-1b PR 必跑, acceptance §4.2)

**review 通关条件**: 飞马 sign-off 8 字段顺序 byte-identical + 野马 sign-off subject 文案锁字面 (空 reject + log warn `bpp.task_subject_empty`) + 烈马 sign-off `TestAL1b_TaskStartedSetsBusy` clock fixture + AL-3 presence 反向断言.

## §3 立场 ③ 配置热更新单源 server→plugin (BPP-2.3 守)

**蓝图字面源**: `plugin-protocol.md` §1.5 "agent_config_update server → plugin 推送, plugin 必须支持幂等 reload, runtime 不缓存 agent 定义 — 每次 inference 前读最新 config" + §1.4 表字面 (Borgee 管: name/avatar/prompt/model/capabilities/enabled / Runtime 管: temperature/token 上限/限速/retry/api_key)

**反约束清单**:

- [ ] AgentConfigUpdateFrame 6 字段 byte-identical (type/cursor/agent_id/config_version/fields/updated_at), config_version 单调递增
- [ ] fields ∈ 6 项白名单 (name / avatar / prompt / model / capabilities / enabled) byte-identical 跟蓝图 §1.4 表字面同源
- [ ] runtime 调优字段 (api_key / temperature / token_limit / retry_strategy / 限速) **不入** BPP-2.3 frame, 反向 grep `api_key\|temperature` count==0 in `internal/bpp/agent_config_update_frame.go`
- [ ] config 单向 server→plugin, plugin 不上行 config (反向断言: 不开 plugin 端 `client.PostConfigUpdate(...)` 路径)
- [ ] plugin 幂等 reload — 同 payload 重复推送无副作用 (config_version 反查跨重复推送等值)
- [ ] runtime 不缓存 agent 定义 (蓝图 §1.5 字面), 每次 inference 前读最新 config — 此为 plugin 端契约, server BPP-2.3 仅推送
- [ ] BPP-3 SSOT 接管前 stub adapter 永远幂等 (跟 hubAnchorAdapter / hubIterationAdapter 同模式)
- [ ] 反向 grep `plugin.*upload.*config\|client.*push.*agent_config` count==0 (CI lint 每 BPP-2.3 + AL-2b PR 必跑, acceptance §4.3)

**review 通关条件**: 飞马 sign-off 6 字段顺序 byte-identical + 飞马 sign-off 6 fields 白名单跟蓝图 §1.4 表字面对齐 + 烈马 sign-off `TestPushConfigUpdate_Idempotent` 反向断言.

## §4 边界立场 ④⑤⑥⑦ (跟其他 milestone 接力)

### ④ BPP envelope CI lint 复用 (BPP-1 #304 自动扫)

- BPP-2 三段四 frame (semantic_action / task_started / task_finished / agent_config_update) 加入 `bppEnvelopeWhitelist` (9→13)
- `frame_schemas_test.go::TestBPPEnvelopeFrameWhitelist` reflect schema lock 自动覆盖 — 加 frame 不加 whitelist = CI 红
- 反约束: 不裂新 namespace `type:.*"bpp_v2"` (反向 grep count==0)

### ⑤ reason 字典承袭 AL-1a #249 6 项 (改 = 改六处单测锁)

- task_finished failed 时 reason ∈ AL-1a 6 项 byte-identical
- 改 reason 字典 = 改六处: AL-1a `agent/state.go::Reason*` (#249) + AL-3 #305 presence error 旁路 + CV-4 #380 ③ + AL-2a #454 ④ + AL-1b #458 ⑤ + AL-4 #387/#461 (BPP-2.2 task_finished 是第七处, 跟跨 milestone 链同源)
- `internal/agent/state.go::Reason*` 是 source-of-truth, 其他文件引用此包不另起字典

### ⑥ 不开 raw REST api_request 旁路 (BPP-1 envelope 不裂)

- BPP-1 #304 envelope 9 frame whitelist 字面承袭, BPP-2 不加 `type: "api_request"` / `type: "raw_rest"` 字段类型
- envelope.go `bppEnvelopeWhitelist` 不加这些 type, 反向 grep `type:.*"api_request"\|type:.*"raw_rest"` count==0

### ⑦ 不写跨 runtime / cross-plugin 协作 (留 v2 后, 蓝图 §4 字面)

- BPP-2 三段实施仅锁 v1 OpenClaw reference impl 路径
- 跨 runtime (OpenClaw + Hermes 混跑) / runtime 协议版本协商 / remote-agent 安装管家 — 反约束兜底, spec §4 + acceptance §4 字面禁

## §5 sign-off 流程 (跟 #387 al-4-stance + #385 cv-4-stance 同模式)

```
BPP-2.1 PR review:
  飞马 (architect):  立场 ①   sign-off (dispatch 层 + 7 op 白名单 + AP-0 perm)
  野马 (UX/文案):     文案锁 § 1 ① + ⑥  sign-off (op 字面 + 错误码字面)
  烈马 (acceptance):  §1 5 项验收  sign-off

BPP-2.2 PR review (同期 AL-1b):
  飞马 (architect):  立场 ② + ⑤   sign-off (task lifecycle source 唯一 + reason 承袭)
  野马 (UX/文案):     文案锁 §1 ③④⑤  sign-off (3 outcome + 6 reason + subject 反默认值)
  烈马 (acceptance):  §2 5 项验收  sign-off

BPP-2.3 PR review (同期 AL-2b/BPP-3):
  飞马 (architect):  立场 ③ + ④   sign-off (config 单源 + envelope CI lint 复用)
  野马 (UX/文案):     文案锁 §1 ②  sign-off (6 fields 白名单字面)
  烈马 (acceptance):  §3 5 项验收  sign-off
```

## §6 更新日志

| 日期 | 作者 | 变化 |
|------|------|------|
| 2026-04-29 | 战马E (PM 客串) | v0 — Phase 4 plugin-protocol 主线起步 BPP-2 stance checklist 4 件套并行 (跟 #387 AL-4 / #385 CV-4 stance checklist 同模式); §0 3 立场 + 4 边界立场总表 + §1-§3 三立场反约束清单 (8 + 9 + 8 = 25 反约束 checkbox) + §4 4 边界立场承袭表 + §5 sign-off 三段流程 (飞马/野马/烈马 三角色 review 各自责任锚); 跟 spec / acceptance / content-lock 三件套字面 byte-identical 同源, 改 = 改四处. |
| 2026-04-29 | 野马 | v0.x patch — cross-milestone reason count audit (跟 #467 同模式 follow-up): "四处单测锁" → "六处单测锁" (AL-1a #249 + AL-3 #305 + CV-4 #380 + AL-2a #454 + AL-1b #458 + AL-4 #387/#461); BPP-2.2 task_finished 是第七处. 跟 #339/#393/#387/#461 follow-up patch 同模式, 历史干净 |
