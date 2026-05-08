# Acceptance Template — CV-4: artifact iterate 完整流 (agent 多版本协作 orchestration)

> 蓝图: `canvas-vision.md` §1.4 ("artifact 自带版本历史: agent 每次修改产生一个版本, 人可以回滚") + §1.5 ("agent 写内容默认允许") + §2 v1 做清单 ("agent 可 iterate, 再次写入触发新版本") + §3 差距 ("Agent iterate / 版本历史: 无 → 需要新表 + 写入策略")
> Spec: `docs/implementation/modules/cv-4-spec.md` (飞马 #365, 3 立场 + 3 拆段 + 10 grep 反查 (5 反约束))
> 文案锁: `docs/qa/cv-4-content-lock.md` (野马 #380, 7 处字面 + state 4 态 byte-identical + 11 行反向 grep)
> 拆 PR (拟): **CV-4.1** schema migration v=18 (`artifact_iterations` 表) + **CV-4.2** server iterate endpoint + state machine + WS push + **CV-4.3** client SPA iterate UI + diff view (jsdiff 行级)
> Owner: 战马A 实施 / 烈马 验收

## 验收清单

### §1 schema (CV-4.1) — artifact_iterations 数据契约

> 锚: 飞马 #365 spec §1 CV-4.1 + CV-1.1 #334 schema 三轴 + AL-1a #249 6 reason 枚举 byte-identical 同源

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 1.1 表 schema: `id` PK + `artifact_id` NOT NULL FK + `requested_by` NOT NULL FK users + `intent_text` TEXT NOT NULL + `target_agent_id` NOT NULL FK agents + `state` TEXT NOT NULL CHECK in ('pending','running','completed','failed') + `created_artifact_version_id` INTEGER NULL FK artifact_versions + `error_reason` TEXT NULL (复用 AL-1a 6 reason 枚举字面) + `created_at` + `completed_at` NULL | migration drift test | 战马A / 烈马 | `internal/migrations/cv_4_1_artifact_iterations_test.go::TestCV41_CreatesArtifactIterationsTable` (TBD, pragma table_info + NOT NULL 全列断言) |
| 1.2 state CHECK 4 态 reject 'unknown' 枚举外值 (`pending`/`running`/`completed`/`failed`) | migration drift test | 战马A / 烈马 | `cv_4_1_artifact_iterations_test.go::TestCV41_RejectsUnknownState` (TBD, INSERT state='unknown' → reject) + `TestCV41_AcceptsAll4States` (4 enum 全过) |
| 1.3 索引双轴 — `idx_iterations_artifact_id_state` (per-artifact pending/running 热路径) + `idx_iterations_target_agent` (agent 工作队列查) | migration drift test | 战马A / 烈马 | `cv_4_1_artifact_iterations_test.go::TestCV41_HasIndexes` (TBD, sqlite_master 双 index 名断言) |
| 1.4 migration v=17 (CV-3.1) → v=18 双向 + idempotent + sequencing 字面延续 14/15/16/17/18 | migration drift test | 战马A / 烈马 | `cv_4_1_artifact_iterations_test.go::TestCV41_Idempotent` (TBD); `grep -n "v=18\|18:" packages/server-go/internal/migrations/registry.go` count==1 |
| 1.5 反约束 — messages 表不加 iteration_id 反指 (立场 ① 域隔离, 跟 CHN-4 #374/#378 立场 ② mention×artifact×anchor×iterate 四路径不污染同源); artifact_versions 表不加 iteration_id 反指 (立场 ① v0 immutable append) | grep | 飞马 / 烈马 | `grep -nE 'ALTER TABLE messages.*ADD.*iteration_id\|messages.*iteration_id' packages/server-go/internal/migrations/` count==0 + `grep -nE 'ALTER TABLE artifact_versions.*ADD\|artifact_versions.*iteration_id' packages/server-go/internal/migrations/` count==0 |

### §2 server API + state machine (CV-4.2) — iterate endpoint + WS push

> 锚: 飞马 #365 spec §1 CV-4.2 + 立场 ② CV-1 commit 单源 + envelope IterationStateChangedFrame 9 字段 byte-identical

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 2.1 `POST /api/v1/artifacts/:id/iterate` (owner-only, body `{intent_text, target_agent_id}`) — 校验 owner perm (非 owner 403) + agent 是 channel member (非 member 400 `iteration.target_not_in_channel`) + INSERT iteration state='pending' + emit BPP-1 control frame | unit + e2e | 战马A / 烈马 | `internal/api/cv_4_2_iterations_test.go::TestCV42_IterateOwnerOnly` (非 owner 403 反断) + `TestCV42_TargetAgentMustBeChannelMember` (TBD) |
| 2.2 立场 ② CV-1 commit 单源 — `POST /artifacts/:id/commits` 加 query `?iteration_id=<uuid>` 命中则 atomic UPDATE iterations.state='completed' + created_artifact_version_id (反约束: 旧无 iteration_id 路径不破); **反约束**: 不开 `POST /iterations/:id/commit` 旁路 endpoint | unit + grep | 战马A / 烈马 | `cv_4_2_iterations_test.go::TestCV42_CommitWithIterationIDAtomicUpdate` (TBD, atomic 事务 完成 state + version_id 同步) + `TestCV42_CommitWithoutIterationID_LegacyPathUnchanged` (反向断言旧路径不破); `grep -rnE 'POST.*\/iterations\/.*\/commit\|iteration_commit_endpoint' packages/server-go/internal/api/` count==0 |
| 2.3 state machine 4 态转移图反断 — 合法转移 (pending→running / pending→failed / running→completed / running→failed); 反 completed→running 等回退 server 拒 | unit | 战马A / 烈马 | `cv_4_2_iterations_test.go::TestCV42_StateMachine_ValidTransitions` (TBD, 4 合法转移 PASS) + `TestCV42_StateMachine_RejectsBackwardTransition` (completed→running / failed→pending 反断 reject); `grep -rnE 'completed.*-> *running\|state.*backward\|UPDATE.*iterations.*state.*pending.*WHERE.*completed' packages/server-go/internal/api/` count==0 |
| 2.4 IterationStateChangedFrame 9 字段 byte-identical envelope `{type, cursor, iteration_id, artifact_id, channel_id, state, error_reason, created_artifact_version_id, completed_at}` 跟 ArtifactUpdated 7 / AnchorCommentAdded 10 / MentionPushed 8 共序 (type/cursor 头位); cursor 走 hub.cursors 单调发号 (RT-1.1 同源, 反约束: 不另起 channel) | unit + grep | 飞马 / 烈马 | `internal/ws/iteration_state_changed_frame_test.go::TestIterationStateChangedFrameFieldOrder` (TBD, JSON byte-equality pin 9 字段顺序); BPP-1 #304 envelope CI lint reflect 自动覆盖 |
| 2.5 AL-4 stub fail-closed (AL-4 runtime 未落时) — state='failed' + error_reason='runtime_not_registered' (跟 AL-1a #249 6 reason 枚举字面 byte-identical 同源, 不另起 reason); AL-4 落地后真路径切 state='completed' | unit | 战马A / 烈马 | `cv_4_2_iterations_test.go::TestCV42_AL4StubFailClosed_RuntimeNotRegistered` (TBD, AL-4 未落 → state='failed' + error_reason byte-identical) + `TestCV42_AL4Live_StateCompleted` (AL-4 落 → state='completed' 双路径切换); `grep -nE 'runtime_not_registered' packages/server-go/internal/api/` count≥1 |
| 2.6 反约束 server 不算 diff (立场 ③ client jsdiff 单源) — 反向 grep `serverDiff|computeDiff.*server|/api/v1/diff` count==0 | grep | 飞马 / 烈马 | `grep -rnE 'serverDiff\|computeDiff.*server\|/api/v1/diff' packages/server-go/internal/api/` count==0 |
| 2.7 反约束 admin god-mode 不返 intent_text raw (ADM-0 §1.3 红线 — intent_text 含 user 输入隐私) — admin endpoint 返元数据白名单不含 intent_text | unit + grep | 飞马 / 烈马 | `cv_4_2_iterations_test.go::TestCV42_AdminGodModeNoIntentTextLeak` (TBD, admin cookie GET → 反向断言 response 字段不含 intent_text); `grep -nE 'admin.*intent_text\|intent_text.*admin' packages/server-go/internal/api/admin*.go` count==0 |

### §3 client SPA (CV-4.3) — iterate UI + diff view

> 锚: 飞马 #365 spec §1 CV-4.3 + 野马 #380 文案锁 7 处字面 byte-identical

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 3.1 iterate 触发按钮 — `<button class="iterate-btn" data-iteration-target-agent-id="" title="请求 agent 迭代">🔄</button>` byte-identical (跟 #380 ① 同源, owner-only DOM omit 跟 CV-1 #347 line 254 showRollbackBtn 同模式 defense-in-depth); 非 owner / DM 视图 / non-markdown artifact 不渲染 | vitest + e2e | 战马A / 烈马 | `__tests__/iterate-btn.test.ts::ownerOnly DOM omit + DM/non-markdown 反向断言` (TBD, 跟 #380 ① byte-identical strict assert) |
| 3.2 intent textarea + agent picker — placeholder `"告诉 agent 你希望它做什么…"` byte-identical (跟 #380 ② 同源, 协作语境锁); agent picker 候选仅 channel member.kind='agent' (反约束: 人/admin 不在候选) | vitest + e2e | 战马A / 烈马 | `__tests__/intent-input.test.ts::placeholder + agent-only candidates` (TBD); `grep -rnE "placeholder=['\"](告诉 agent 你希望它做什么…)['\"]" packages/client/src/components/Iterate*.tsx` count≥1 |
| 3.3 state 4 态文案 byte-identical — DOM `data-iteration-state="{pending\|running\|completed\|failed}"` + 文案 (`"等待 agent 开始…"` / `"agent 正在迭代…"` / `"已生成 v{N}"` / `"失败: {reason_label}"`) byte-identical 跟 #380 ③ 同源; failed reason 走 AL-1a #249 REASON_LABELS byte-identical (改 = 改八处单测锁 #249 + AL-3 #305 + #380) | vitest table-driven | 战马A / 烈马 | `__tests__/iteration-state-labels.test.ts::4 态文案 byte-identical + REASON_LABELS 同源` (TBD, table-driven 反 "Pending/Running/Completed/Failed" 英文 + "处理中/进行中/出错/成功" 同义词) |
| 3.4 completed 自动 navigate 新版本 + kindBadge 二元 — iteration completed → 自动 navigate 到新 artifact_version_id 视图; kindBadge `🤖 {agent_name}` byte-identical 跟 CV-1 #347 line 251 byte-identical (改 = 改五处: #347 + #355 + #314 + #380 + 此); 走 CV-1 既有 fanout 路径不另发 (立场 ② 单源) | vitest + e2e | 战马A / 烈马 | `__tests__/iteration-complete-navigate.test.ts` (TBD, navigate 触发 + kindBadge byte-identical) |
| 3.5 diff view tab + jsdiff 蓝绿配色 — tab 文案 `"对比"` byte-identical (单字, 跟 #380 ⑤ 同源); diff 视图 `"v{N} ↔ v{M}"` 标题; jsdiff 行级配色 `data-diff-line="add\|del\|context"` ARIA label (a11y, 仅靠颜色辨识 visually impaired 不漏); deep-link `?diff=vN..vM` 进对比模式; image_link kind fallback 缩略图并排 (jsdiff 不适用) | vitest + e2e | 战马A / 烈马 | `__tests__/diff-view.test.ts::jsdiff 蓝绿 + ARIA + image_link fallback` (TBD); `grep -nE 'data-diff-line=["'"'"'](add\|del\|context)["'"'"']' packages/client/src/components/Diff*.tsx` count≥1 (a11y 反断, 跟 #380 ⑤ 同源) |
| 3.6 iteration history inline (artifact panel 折叠区) — `data-section="iteration-history"` + active + 最近 5 条 + intent_text 头 40 字截断 (隐私 + UI 噪声防御); 反约束: messages 流不渲染 iteration state 进度 (跟 CHN-4 #374/#378 立场 ② 域隔离同源) | vitest + grep | 战马A / 烈马 | `__tests__/iteration-history-inline.test.ts` (TBD, 折叠区 + 截断 + messages 流反断); `grep -rnE 'messages.*iterate_progress\|messages.*iteration_state\|MessageList.*iteration' packages/client/src/` count==0 |
| 3.7 failed state 反约束 — 仅显示 `"失败: {reason_label}"`, **无重试按钮** (跟 #380 ⑦ + #365 反约束 ② 同源, 失败 = owner 重新触发新 iteration_id 不复用 failed); 反约束自动重试 leak | vitest + grep | 飞马 / 烈马 | `__tests__/iteration-failed-state.test.ts::no retry button + no auto retry` (TBD, 反向断言 DOM 无 "重试" + 无 setTimeout 自动 POST); `grep -rnE "['\"](重试\|Retry\|重新尝试\|再试一次)['\"]" packages/client/src/components/Iterate*.tsx` count==0 + `grep -rnE 'autoRetry.*iteration\|setTimeout.*POST.*iterate.*failed' packages/client/src/` count==0 |

### §4 反向 grep / e2e 兜底 (跨 CV-4.x 反约束)

> 锚: 飞马 #365 spec §3 5 反约束 grep + 野马 #380 §2 11 行反向 grep byte-identical 同源

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 4.1 立场 ② 旁路 endpoint 反断 — `grep -rnE 'POST.*\/iterations\/.*\/commit\|iteration_commit_endpoint' packages/server-go/internal/api/` count==0 (CV-1 commit 单源) | CI grep | 飞马 / 烈马 | _(每 CV-4.2 PR 必跑)_ |
| 4.2 立场 ① 域隔离 — `grep -rnE 'messages.*iteration_id\|ALTER TABLE messages.*iteration\|ALTER TABLE artifact_versions ADD\|artifact_versions.*iteration_id' packages/server-go/internal/migrations/` count==0 | CI grep | 飞马 / 烈马 | _(每 CV-4.* PR 必跑)_ |
| 4.3 state machine 不回退 — `grep -rnE 'completed.*-> *running\|state.*backward\|UPDATE.*iterations.*state.*pending.*WHERE.*completed' packages/server-go/internal/api/` count==0 | CI grep | 飞马 / 烈马 | _(每 CV-4.2 PR 必跑)_ |
| 4.4 立场 ③ server 不算 diff — `grep -rnE 'serverDiff\|computeDiff.*server\|/api/v1/diff' packages/server-go/internal/api/` count==0 | CI grep | 飞马 / 烈马 | _(每 CV-4.2 PR 必跑)_ |
| 4.5 反约束 admin god-mode intent_text 不漏 (ADM-0 §1.3 红线) — `grep -nE 'admin.*intent_text\|intent_text.*admin' packages/server-go/internal/api/admin*.go` count==0 | CI grep | 飞马 / 烈马 | _(每 CV-4.2 PR 必跑)_ |
| 4.6 messages 流 iterate state leak (跟 CHN-4 立场 ② 同源) — `grep -rnE 'messages.*iterate_progress\|messages.*iteration_state\|MessageList.*iteration' packages/client/src/` count==0 | CI grep | 飞马 / 烈马 | _(CV-4.3 PR 必跑)_ |
| 4.7 失败重试 + 自动重试 leak (#380 ⑦ + #365 反约束 ② 同源) — `grep -rnE "['\"](重试\|Retry\|重新尝试)['\"]" packages/client/src/components/Iterate*.tsx` count==0 + `grep -rnE 'autoRetry.*iteration\|setTimeout.*POST.*iterate' packages/client/src/` count==0 | CI grep | 飞马 / 烈马 | _(CV-4.3 PR 必跑)_ |

## 边界 (跟其他 milestone 关系)

| Milestone | 关系 | 字面承袭 |
|---|---|---|
| CV-1 ✅ | commit 端点单源 (`POST /commits?iteration_id=`), rollback owner-only 互不干涉 | CV-1.2 既有 endpoint byte-identical 不破 |
| CV-2 #356/#360 | iteration completed 后可对新 version 加锚 (走既有 CV-2.2), anchor 立场 ② version-pin immutable 承袭 | CV-2 §4 反约束承袭 (anchor 仅 markdown) |
| CV-3 #363/#370 | iterate 适用所有 kind (intent 同适用); diff view code 走 prism + jsdiff, image_link 缩略图并排 | XSS 红线两道闸 不破 |
| CHN-4 #374/#378 | iterate 是 workspace tab 内事; messages 流不渲染 iteration state 进度 (立场 ② 域隔离) | 双 tab 不交叉 |
| RT-1 ✅ | IterationStateChangedFrame 9 字段共序 cursor (跟 ArtifactUpdated 7 / AnchorCommentAdded 10 / MentionPushed 8 同模式) | hub.cursors 单调发号 |
| AL-4 spec #319/#379 | runtime 未落时 stub fail-closed reason='runtime_not_registered' (跟 AL-1a #249 6 reason byte-identical) | error_reason 枚举不另起 |
| AL-1a #249 | failed reason 跟 6 reason 枚举字面 byte-identical 八处单测锁 (#249 + AL-3 #305 + #380 + 本) | REASON_LABELS 同源 |
| ADM-0 §1.3 | admin god-mode 不返 intent_text raw (隐私 user 输入) | 字段白名单 |
| BPP-1 ✅ #304 | envelope CI lint reflect 比对 server-go 端字段顺序自动覆盖 | 字段顺序锁 |

## 退出条件

- §1 schema 5 项 + §2 server 7 项 + §3 client 7 项 + §4 反向 grep 7 项**全绿** (一票否决)
- state machine 4 态转移图反断 (合法 4 + 反向回退 reject) + AL-4 stub 双路径切换 (未落 fail-closed / 落地 completed) 全绿
- failed reason 跟 AL-1a #249 + AL-3 #305 + #380 八处单测锁 byte-identical 守住
- intent_text admin god-mode 反断 (ADM-0 §1.3 红线) + messages 流 iteration state 反断 (CHN-4 #374/#378 立场 ② 同源) 守住
- 登记 `docs/qa/regression-registry.md` REG-CV4-001..026 (5 schema + 7 server + 7 client + 7 反向 grep)
- v=14-18 sequencing 字面延续 (CV-2.1 ✅ / DM-2.1 ✅ / AL-4.1 v=16 / CV-3.1 v=17 / **CV-4.1 v=18**)
- IterationStateChangedFrame 9 字段共序 BPP-1 lint 自动覆盖
- G3.4 demo 截屏 4 张归档 (跟 #380 §3 同源, 撑章程 Phase 3 退出公告)
