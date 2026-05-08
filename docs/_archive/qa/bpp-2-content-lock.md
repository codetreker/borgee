# BPP-2 文案锁 / 立场反查清单 (野马 + 战马E PM 客串 v0)

> 战马E (PM 客串) · 2026-04-29 · ≤80 行 byte-identical 文案锁 (4 件套并行, 跟 cv-4-content-lock #380 + al-4-content-lock #321 + chn-2-content-lock #354 同模式)
> **蓝图锚**: [`plugin-protocol.md`](../blueprint/current/plugin-protocol.md) §1.3 (语义动作层 7 op 白名单字面) + §1.5 (配置热更新 6 fields 白名单字面) + §1.6 (busy/idle source 上行 frame 唯一) + §2.1+§2.2 (BPP 接口清单 v1)
> **关联**: spec `docs/implementation/modules/bpp-2-spec.md` (战马E v0) + acceptance `docs/qa/acceptance-templates/bpp-2.md` (战马E v0) + 复用 AL-1a #249 6 reason byte-identical (跟 AL-1a #249 + AL-3 #305 + CV-4 #380 + AL-2a #454 + AL-1b #458 + AL-4 #387/#461 **六处单测锁**同源)

## §1 字面锁 (改 = 改两处: 此文档 + spec / acceptance / 实施代码)

### ① 7 op 白名单 byte-identical (蓝图 §1.3 v1 必须列表字面)

```
create_artifact          # 创建产物 (PRD / 代码 / 设计稿等)
update_artifact          # 修改产物内容 (生成新版本)
reply_in_thread          # 在某条消息线程下回复
mention_user             # @ 某个 user/agent
request_agent_join       # 请求邀请其他 agent 进 channel (触发审批)
read_channel_history     # 读 channel 消息历史 (分页)
read_artifact            # 读 workspace 中的 artifact
```

字面 byte-identical 跟蓝图字面同源, **改 = 改三处**: 蓝图 plugin-protocol.md §1.3 + spec bpp-2-spec.md §0 立场 ① + 实施代码 `internal/bpp/semantic_action.go::ValidOps` enum. 反向 grep `op.*propose_artifact_change\|op.*request_owner_review\|op.*request_clarification` count==0 (v2+ 列表蓝图字面禁 v1 进).

### ② 6 fields 白名单 byte-identical (蓝图 §1.4 表字面 — agent 配置 SSOT)

```
name           # agent 显示名
avatar         # 头像
prompt         # system / role prompt
model          # runtime 上报 schema 候选
capabilities   # 能力开关 (哪些 tool 启用)
enabled        # 启用/禁用状态
```

字面 byte-identical 跟蓝图 §1.4 表字面同源 (左列 "归 Borgee 管"), **改 = 改三处**: 蓝图 §1.4 + spec §0 立场 ③ + 实施代码 `internal/bpp/agent_config_update_frame.go::ValidFields` enum. 反向断言: `api_key` / `temperature` / `token_limit` / 限速 / `retry` / `memory` (蓝图 §1.4 右列 "归 Runtime 管") **不入** BPP-2.3 frame.

### ③ task outcome 3 态 enum byte-identical

```
completed      # 任务正常完成 (artifact 落 / message 发 / etc)
failed         # 任务失败, reason 复用 AL-1a 6 项
cancelled      # owner 主动取消 (CV-4 反约束: 失败 owner 重新触发新 task_id, 不 cancelled)
```

3 态严闭 — 反约束: 'partial' / 'paused' / 'pending' 中间态 reject. 跟蓝图 §1.6 失联与故障状态 outcome 字面承袭, **改 = 改三处**: spec §0 立场 ② + acceptance §2.2 + 实施代码 `internal/bpp/task_finished_frame.go::ValidOutcomes` enum.

### ④ AL-1a 6 reason 字面 byte-identical 六处单测锁同源

```
api_key_invalid        # API key 失效 / 配错
quota_exceeded         # 用量超限
network_unreachable    # 网络不通
runtime_crashed        # runtime 崩溃
runtime_timeout        # runtime 超时无响应
unknown                # 兜底未知错误
```

跟 AL-1a #249 (三态机) + AL-3 #305 (presence error 旁路) + CV-4 #380 ③ + AL-2a #454 ④ + AL-1b #458 ⑤ + AL-4 #387/#461 **六处单测锁同源** — 改 = 改六处 (BPP-2.2 task_finished 是第七处, 跟跨 milestone 链同源). `internal/agent/state.go::Reason*` 常量是 source-of-truth.

### ⑤ subject 文案锁 (BPP-2.2 task_started 字段) — 必带非空

- subject 是 plugin 上行声明 "agent 在做什么" 的人类可读字符串 (蓝图 §2.2 字面)
- **空字符串 server 拒收** + log warn `bpp.task_subject_empty` + 不渲染 busy 状态 (反约束: 不 fallback 到 "处理中..." 等默认值)
- subject 不带 raw `task_id` / `agent_id` 等 UUID (隐私 + 可读性)
- 改 = 改两处: spec §0 立场 ② + 实施代码 `internal/bpp/task_started_frame.go` validation

### ⑥ 错误码字面 (BPP-2.* 反约束 reject 时 server 返码)

```
bpp.task_subject_empty             # BPP-2.2 subject 空 reject
bpp.config_field_disallowed        # BPP-2.3 fields 不在白名单 reject
bpp.semantic_op_unknown            # BPP-2.1 op 不在 7 项白名单 reject
bpp.plugin_no_raw_rest             # BPP-2.1 plugin 直调 raw REST 反约束兜底
```

字面 byte-identical 跟 anchor.create_owner_only (#360) / iteration.target_not_in_channel (#409) / dm.workspace_not_supported (#407) 同模式 — error code 是字串唯一定义, **改 = 改两处**: 此文档 + 实施代码 const.

## §2 反向 grep 反约束 (CI 锁, 每 BPP-2.* PR 必跑 0 hit)

### ① 立场 ① plugin 不下穿走 raw REST (协议红线)

```
git grep -nE 'api_request.*method.*POST|raw.*REST.*plugin|plugin.*direct.*REST' packages/server-go/internal/bpp/   # 0 hit
```

### ② 立场 ② busy/idle 唯一 source 是 task lifecycle frame (不写 presence_sessions)

```
git grep -nE 'presence_sessions.*busy|presence.*task_id|busy.*online|online.*busy' packages/server-go/internal/   # 0 hit
```

### ③ 立场 ③ config 单源 server→plugin (plugin 不上行 config)

```
git grep -nE 'plugin.*upload.*config|client.*push.*agent_config|plugin.*POST.*config' packages/server-go/internal/   # 0 hit
```

### ④ subject 必带非空, 反默认值 fallback (BPP-2.2 蓝图 §2.3 文案锁)

```
git grep -nE 'subject.*=.*""|subject.*\.\.\.|fallback.*subject|default.*subject' packages/server-go/internal/bpp/   # 0 hit
```

### ⑤ runtime 调优字段不入 BPP-2.3 frame (蓝图 §1.4 分界字面)

```
git grep -nE 'api_key|temperature.*config|model.*api_key|token_limit|retry_strategy' packages/server-go/internal/bpp/agent_config_update_frame.go   # 0 hit
```

### ⑥ 不裂 BPP envelope namespace (BPP-1 #304 复用)

```
git grep -nE 'type:.*"bpp_v2|type:.*"task_v2|type:.*"new_envelope' packages/server-go/internal/bpp/   # 0 hit
```

### ⑦ v2+ 语义动作不入 v1 (蓝图 §1.3 v2+ 列表字面禁)

```
git grep -nE 'op.*propose_artifact_change|op.*request_owner_review|op.*request_clarification' packages/server-go/internal/bpp/   # 0 hit
```

### ⑧ outcome 3 态严闭 (反中间态)

```
git grep -nE 'outcome.*partial|outcome.*paused|outcome.*pending|outcome.*starting' packages/server-go/internal/bpp/   # 0 hit
```

## §3 立场反查 (从蓝图字面下沉到 spec / acceptance / 实施代码)

| # | 立场 | 蓝图字面源 | 实施代码守门 |
|---|---|---|---|
| ① | 语义动作 = 帧不是 REST 直调 | plugin-protocol.md §1.3 "协议红线 不允许 plugin 下穿语义层直调 REST" | `internal/bpp/dispatcher.go::HandleSemanticAction` 走 AP-0 RequirePermission, plugin 路径无 raw REST 直调 + §2 ① 反向 grep |
| ② | task lifecycle 上行帧是 busy/idle 唯一 source | plugin-protocol.md §1.6 + agent-lifecycle.md §2.3 字面 "busy/idle source 必须 plugin 上行 frame, 不准 stub" | `internal/agent/state.go::SetBusy` 仅由 BPP-2.2 task_started 触发, AL-3 presence_sessions 不写 busy + §2 ② 反向 grep |
| ③ | 配置热更新单源 server→plugin (蓝图 §1.5 字面 "幂等 reload, runtime 不缓存") | plugin-protocol.md §1.5 + §1.4 表字面 (左列 Borgee 管 / 右列 runtime 管) | `internal/bpp/agent_config_update_frame.go::ValidFields` 6 项白名单, runtime 调优字段不入 frame + §2 ⑤ 反向 grep |
| ④ | BPP envelope CI lint 复用 (BPP-1 #304 自动扫) | bpp-1.md §1.1 + plugin-protocol.md §2 字面 "frame schema 字面锁" | `internal/bpp/envelope.go::bppEnvelopeWhitelist` 9→13 扩, reflect schema lock 自动覆盖 + §2 ⑥ 反向 grep |
| ⑤ | reason 字典承袭 AL-1a #249 6 项 (改 = 改六处+1) | concept-model.md §1.6 + agent-lifecycle.md §2.3 故障 UX | `internal/agent/state.go::Reason*` 6 常量是 source-of-truth, BPP-2.2 task_finished + AL-1a #249 + AL-3 #305 + CV-4 #380 + AL-2a #454 + AL-1b #458 + AL-4 #387/#461 同源 |
| ⑥ | 不写跨 runtime / cross-plugin 协作 (蓝图 §4 字面 "跨 runtime 协作场景留 v2 后") | plugin-protocol.md §4 不在本轮范围 | spec §4 + acceptance §4 反约束兜底, BPP-2 仅锁 v1 OpenClaw reference impl |
| ⑦ | 不开 raw REST `api_request` 旁路 (BPP-1 envelope 不裂) | plugin-protocol.md §1.3 "协议红线" | `internal/bpp/envelope.go` 不加 `api_request` 字段 / type, 反向 grep `type.*"api_request"` count==0 |

## §4 跟其他 milestone 的文案 / 立场承袭

| Milestone | 字面承袭 | 改 = 改几处 |
|---|---|---|
| BPP-1 ✅ #304 | envelope CI lint reflect 9→13 frame whitelist 扩 | 一处 (envelope.go bppEnvelopeWhitelist) |
| AL-1a ✅ #249 | 6 reason 字典字面 byte-identical | 改六处+1 (#249 + AL-3 #305 + CV-4 #380 + AL-2a #454 + AL-1b #458 + AL-4 #387/#461 + BPP-2.2) |
| AL-1b 同期 | busy/idle source 真接管 (BPP-2.2 落 frame, AL-1b 状态机驱动) | 同 PR 合, source 不另起 |
| AL-2b 同期 | ConfigUpdated 真接管 (BPP-2.3 落 envelope, AL-2b/BPP-3 接管 SSOT) | 同 PR 合, frame 不另起 |
| AL-3 ✅ #310 | presence 路径拆死 (online session vs busy task) | 反向 grep `presence_sessions.*busy` 0 hit |
| AL-4.1 ✅ #398 | agent_runtimes 路径拆死 (process vs task) | 反向 grep `agent_runtimes.*task_id` 0 hit |
| AL-4.2 ✅ #414 | RequirePermission `agent.runtime.control` 模式承袭 | semantic action dispatch 层走 AP-0 既有 perm 闸 |

## §5 更新日志

| 日期 | 作者 | 变化 |
|------|------|------|
| 2026-04-29 | 战马E (PM 客串) | v0 — Phase 4 plugin-protocol 主线起步 BPP-2 文案锁 4 件套并行 (跟 #380 CV-4 / #321 AL-4 / #354 CHN-2 文案锁同模式); §1 6 处字面锁 (7 op + 6 fields + 3 outcome + 6 reason 三处单测锁 + subject 文案 + 4 错误码) + §2 8 反向 grep 反约束 (plugin 下穿 / busy presence 错位 / config 上行 / subject 默认值 / runtime 调优字段 / envelope namespace 裂 / v2+ 进 v1 / outcome 中间态) + §3 7 立场反查 (蓝图字面 → 实施守门) + §4 跟 BPP-1/AL-1a/AL-1b/AL-2b/AL-3/AL-4 文案/立场承袭表; 4 新 frame (semantic_action / task_started / task_finished / agent_config_update) 加入 BPP-1 #304 envelope CI lint reflect whitelist 自动覆盖. |
| 2026-04-29 | 野马 | v0.x patch — cross-milestone reason count audit (跟 #467 同模式 follow-up): "三处单测锁" → "六处单测锁" + "改三处+1" → "改六处+1" (AL-1a #249 + AL-3 #305 + CV-4 #380 + AL-2a #454 + AL-1b #458 + AL-4 #387/#461, BPP-2.2 task_finished 是第七处). 跟 #339/#393/#387/#461 follow-up patch 同模式, 历史干净 |
