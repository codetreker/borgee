# AL-1 状态四态扩展 wrapper (state machine + 历史 audit) — 烈马 (QA acceptance) signoff

> **状态**: ✅ **SIGNED** (烈马 acceptance 代签, 2026-04-29, post-#492 18b2e80)
> **范围**: AL-1 wrapper milestone — 状态四态扩展 (agent_state_log v=25 + state machine validator + AppendAgentStateTransition helper + GET /api/v1/agents/:id/state-log owner-only endpoint), 跟蓝图 §2.3 行为不变量 byte-identical 真实施 (validator gate + history audit)
> **关联**: AL-1 #492 (zhanma-d 18b2e80) 整 milestone 一 PR (跟 ADM-2 #484 + BPP-2 #485 + AL-1b #482 + AL-2a #480 同模式); 前置依赖全 merged: AL-1a ✅ #249 (online/offline 三态 + 6 reason) + AL-1b ✅ #453/#457/#462 (busy/idle 5-state) + BPP-2.2 ✅ #485 (task_started/task_finished frame); REG-AL1-001..005 5🟢 + REG-AL1-006..009 4 ⏸️ follow-up; 累计 20+ unit test (6 schema + 9 validator + 5 endpoint + store helper)
> **方法**: 跟 #403 G3.3 + #449 G3.1+G3.2+G3.4 + #459 G4.1 + G4.2 + G4.3 + G4.4 + G4.5 烈马代签机制承袭 — 真单测实施证据 + 立场反查 + acceptance template 闭锁 + 跨 milestone byte-identical 链承袭 + 烈马代签 (AL-1 是工程内部 wrapper, 用户感知层 AL-1b PresenceDot 5-state 已落 #462, 此 PR 不进野马 G4 流, 跟 cm-4 / adm-0 / adm-2 deferred 同模式)

---

## 1. 验收清单 (烈马 acceptance 视角 5 项, 跟 acceptance al-1.md 11 项 ✅ byte-identical)

| # | 验收项 | 立场锚 | 结果 | 实施证据 (PR/SHA + 测试名 byte-identical) |
|---|--------|--------|------|------|
| ① | agent_state_log schema v=25 (id PK AUTOINCREMENT / agent_id / from_state / to_state / reason / task_id / ts) + idx_agent_state_log_agent_id_ts (DESC) + 7 列 NOT NULL + 反向 6 forbidden 列 (updated_at / state / cursor / org_id / owner_id / task_state) — forward-only audit (立场 ① 跟 admin_actions ADM-2.1 立场 ⑤ 同精神) | acceptance §数据契约 + 蓝图 §2.3 + 立场 ① forward-only audit | ✅ pass | `internal/migrations/al_1_4_agent_state_log_test.go` 6 PASS (CreatesAgentStateLogTable + InsertAndAutoIncrement + NoDomainBleed 6 forbidden 列 + HasIndex + AcceptsAL1aReasonValues 6 字面 + Idempotent v=25) — REG-AL1-001 |
| ② | state machine ValidateTransition 5-state graph (initial / online / busy / idle / error / offline) + 15 valid edges byte-identical 跟蓝图 §2.3 + 14+ invalid edges 反向 (same-state / busy↛online / idle↛online / error↛busy/idle / offline↛busy/idle/error / initial↛busy/idle/error 全 reject) | acceptance §状态机验证 + 蓝图 §2.3 立场 ② state machine 单源 | ✅ pass | `agent_state_log_test.go::TestValidateTransition_ValidEdges` (15 valid) + `_RejectsSameState` (5 same-state reject) + `_RejectsInvalidEdges` (10 invalid edge reject) PASS — REG-AL1-002 |
| ③ | error 转移必带 reason ∈ AL-1a 6 字面 byte-identical (`api_key_invalid` / `quota_exceeded` / `network_unreachable` / `runtime_crashed` / `runtime_timeout` / `unknown`); 字典外 / 大小写漂移 / 空 reject — **第 8 处单测锁链** byte-identical (#249 + #305 + #321 + #380 + #454 + #458 + #481/#485 + 此) | acceptance §状态机 立场 ④ + 蓝图 §2.3 故障可解释 + reason 八处单测锁链承袭 | ✅ pass | `TestValidateTransition_ErrorRequiresReason` (6 valid + 7 invalid + empty reject) + `_AllReasons5Pin` (锁 6 字面 + 反向不允许 extra entry) PASS — REG-AL1-003 |
| ④ | AppendAgentStateTransition helper 是 server-side state set 唯一入口 — 走 ValidateTransition gate, 反向: empty agent_id / invalid transition / same state / error 无 reason / error 无效 reason 全 reject; 反向 grep `INSERT INTO agent_state_log` 在非 helper 路径 + `UPDATE agent_state_log` / `DELETE FROM agent_state_log` 在 internal/ (除 migration) 全 count==0 (立场 ① forward-only + 立场 ② state machine 单门) | acceptance §行为不变量 4.1.a+4.1.b + 立场 ① forward-only + 立场 ② state machine 单源 | ✅ pass | `TestAppendAgentStateTransition_HappyPath` (4 transitions + monotonic id) + `_RejectsInvalidViaValidator` (5 reject case) + `TestValidateTransition_RecoveryPath` (full lifecycle: initial→online→error→online→busy 4 transitions) PASS; schema 反向 NoDomainBleed 锁 updated_at 不挂; store helpers 仅 INSERT + SELECT 路径 — REG-AL1-004 |
| ⑤ | GET /api/v1/agents/:id/state-log owner-only (蓝图 §2.3 "故障可解释 owner 看 agent 病历"); non-owner → 403 / non-agent user → 404 / unauthenticated → 401 / agent_not_found → 404; 反向 endpoint response 不返 raw agent_id 字段 (已在 URL path, sanitizer 跟 ADM-2 同模式); ListAgentStateLog store helper DESC ts ordering + scope + limit clamping | acceptance §行为不变量 4.1.c+4.1.d + 蓝图 §2.3 故障可解释 owner-only + ADM-2 sanitizer 同模式 | ✅ pass | `internal/api/al_1_4_state_log_test.go` 5 endpoint PASS (OwnerSeesAgentHistory + 反向 row 不含 agent_id 字段 / NonOwnerRejected 403 / UnauthenticatedReturns401 / AgentNotFound 404 / NonAgentRejected 404) + `ListAgentStateLog` store helper test (DESC ts ordering + scope + limit clamping) PASS — REG-AL1-005 |

**总体**: 5/5 通过 (覆盖 acceptance 11 项 ✅) → ✅ **SIGNED**, AL-1 状态四态扩展 wrapper milestone 通过.

---

## 2. 反向断言 (核心立场守门 byte-identical)

AL-1 三处反向断言全 PASS:

- **state machine 单门 helper**: `AppendAgentStateTransition` 是 server-side state set **唯一入口** — 反向 grep 非 helper 路径 raw `INSERT INTO agent_state_log` count==0 (除 store helper + migration); 跟 ADM-2 admin_actions store helper 同模式 (不暴露 raw INSERT 路径), 跟 BPP-2 ActionHandler interface seam 依赖反转同精神
- **owner-only state-log endpoint**: GET /api/v1/agents/:id/state-log 走 server-side OwnerID check (反向 inject ?target_user_id 不允许 — 跟 ADM-2 GET /api/v1/me/admin-actions 立场 ④ 同模式); admin path 不挂 (ADM-0 §1.3 红线承袭); 反向 endpoint response 不返 raw agent_id 字段 (sanitizer 跟 ADM-2 同模式)
- **6 reason 第 8 处单测锁 byte-identical**: 改 reason = 改八处 (AL-1a #249 + AL-3 #305 + AL-4 #321 + CV-4 #380 + AL-2a #454 + AL-1b #458 + BPP-2/AL-2b #481/#485 + 此 AL-1) — 任一处漂 = lint fail; #748ad89 stance sync 5 篇 post-#481 byte-identical 锁
- **forward-only audit**: schema 不挂 updated_at; store helpers 仅 INSERT + SELECT (反向 grep `UPDATE agent_state_log\|DELETE FROM agent_state_log` 在 internal/ 除 migration count==0); 跟 admin_actions ADM-2.1 立场 ⑤ + CV-1 立场 ③ "agent 默认无删历史权" + AL-1b "状态历史保留" 同精神

---

## 3. 跨 milestone byte-identical 链验 (AL-1 是状态机 wrapper 锚, 多 milestone 锚承袭)

AL-1 兑现/承袭多源 byte-identical:

- **AL-1a #249 三态 + 6 reason 源头承袭**: ValidateTransition 6 reason 字典 byte-identical 跟 #249 agent/state.go Reason* + lib/agent-state.ts REASON_LABELS + AL-3 #305 + AL-4 #321 + ... 八处单测锁链
- **AL-1b #453 schema v=21 + #457 server 5-state GET + #462 client SPA PresenceDot 5-state**: AL-1 wrapper 真把 4 态 (online/busy/idle/error) 串成 state machine + history audit, AL-1b busy/idle 5-state UI 已落用户感知层, AL-1 此 PR 落工程内部 state log + validator gate
- **BPP-2.2 #485 task_started/task_finished frame 前置就位**: BPP-2 dispatcher stub 已落 (REG-AL1B-003 task_started UPSERT + task_finished UPSERT 路径), AL-1 wrapper AppendAgentStateTransition helper 已 ready, dispatcher → audit append wire 留 REG-AL1-006 follow-up (跟 AL-1b SetAgentTaskStarted 同模式)
- **ADM-0 §1.3 admin god-mode 红线承袭**: GET /api/v1/agents/:id/state-log 走 owner-only 路径不挂 admin path (admin 不入业务路径) — 跟 AL-3 #303 ⑦ + AL-4 #379 v2 + AL-2b #471 §2.4 + ADM-2 #484 + BPP-2 #485 同模式
- **forward-only audit 跨 milestone 同精神**: AL-1 agent_state_log + ADM-2.1 admin_actions + ADM-2.2 impersonation_grants (only revoked_at stamp, 不真删) + AL-1b 状态历史保留 — schema 不挂 updated_at, store helpers 仅 INSERT + SELECT
- **sanitizer 跟 ADM-2 同模式**: response 不返 raw agent_id 字段 (已在 URL path), 跟 ADM-2 admin handler audit row metadata 反向不返 raw actor_id / admin_id (REG-ADM2-004) 同模式
- **state machine 单门 helper 跟 ActionHandler / Pusher seam 同精神**: AppendAgentStateTransition 是 server-side state set 唯一 entry (跟 BPP-2 ActionHandler / AL-2a AgentConfigPusher / RT-1 hub.cursors 单调发号 同模式 — 单 entry + interface seam + 依赖反转)

---

## 4. 留账 (AL-1 闭闸不阻, 留 follow-up — 跟蓝图 §2.3 边界 cross-ref)

- ⏸️ **REG-AL1-006 BPP-2.2 dispatcher → audit append wire** — task_started/task_finished frame dispatcher 接收时 server 端自动调 `AppendAgentStateTransition` 写 audit; 当前 #485 dispatcher stub, helper ready, wire 留 follow-up patch (跟 AL-1b SetAgentTaskStarted 同模式 + AL-2a / AL-2b Pusher seam wire follow-up 同模式)
- ⏸️ **REG-AL1-007 PresenceTracker hub lifecycle hook → audit append wire** — AL-1a presence track online/offline 时同样 wire AppendAgentStateTransition (PresenceTracker hub lifecycle hook on Connect/Disconnect/Error), 跟 AL-3 hub lifecycle 同模式 follow-up
- ⏸️ **REG-AL1-008 client SPA agent state 历史轨迹 UI** — 蓝图 §2.3 "直达修复入口" error 状态显示 reason + 修复按钮 (跟 AL-1a AgentStateHelp + AL-1b PresenceDot 5-state 同精神, 用户感知层 v2 留账)
- ⏸️ **REG-AL1-009 e2e 完整状态流** — initial → online → busy → idle → error → online → offline 整链 e2e (依赖 BPP-2.2 dispatcher wire + PresenceTracker wire 真接管, REG-AL1-006/007 落地后翻; 跟 G3.4 野马 5 张截屏 / G4.4 双 agent 截屏野马签同模式 deferred ⏸️)

跟蓝图 §2.3 边界 cross-ref:
- 蓝图 §2.3 "状态四态" — schema 落 6 sentinel 含 initial + offline (5-态 graph + initial sentinel) byte-identical
- 蓝图 §2.3 "cross-state transition lock" — ValidateTransition 14 valid edges + 反向 invalid 全 reject 真实施
- 蓝图 §2.3 "故障可解释" — error 必带 reason 6 字面 + GET state-log endpoint owner-only audit history
- 蓝图 §2.3 "直达修复入口" — error 状态 reason 显示 + 修复按钮 留 REG-AL1-008 follow-up

---

## 5. 解封路径 (Phase 4 退出闸 + AL-1 wrapper 工程内部)

- ✅ **G4.1 ADM-1**: 野马 ✅ #459
- ✅ **G4.2 ADM-2**: 烈马 ✅ #484 6cf5240
- ✅ **G4.3 BPP-2**: 烈马 ✅ G4 batch
- ✅ **G4.4 CM-5**: 烈马 ✅ G4 batch
- ✅ **G4.5 AL-2a + AL-2b + AL-4 联签**: 烈马 ✅ G4 batch
- ✅ **AL-1 状态四态 wrapper**: 烈马 acceptance ✅ 本 signoff (5/5 验收 + REG-AL1-001..005 5🟢 + 20+ unit test PASS + 反向断言全过 + 跨 milestone 八处 reason 单测锁链承袭)
- ⏸️ **REG-AL1-006..009 4 ⏸️ follow-up** — dispatcher wire + presence wire + client UI + e2e (跟 G4.audit 同期收口, 飞马职责)
- ⏸️ **G4.audit** Phase 4 代码债 audit (软 gate 飞马职责) — 含 AL-1 4 ⏸️ follow-up + AL-4.2/4.3 5⚪ + AL-2b ack ingress BPP-3 接管
- ⏸️ **Phase 4 closure announcement** (G4.1-G4.5 全签 ✅ + AL-1 wrapper ✅ + G4.audit 后链入飞马职责)

---

## 6. 更新日志

| 日期 | 作者 | 变化 |
|------|------|------|
| 2026-04-29 | 烈马 | v0 — AL-1 状态四态扩展 wrapper milestone ✅ SIGNED post-#492 18b2e80 (zhanma-d 整 milestone 一 PR). 5/5 验收通过 (agent_state_log schema v=25 + 7 列 + 反向 6 forbidden 列 / state machine ValidateTransition 5-态 graph + 15 valid + 14+ invalid edges 反向 / error 必带 reason 6 字面 第 8 处单测锁链 / AppendAgentStateTransition helper 单门 + 5 reject case + RecoveryPath / GET /api/v1/agents/:id/state-log owner-only + 4 ACL gate + sanitizer 反向). 跟 G4.2 ADM-2 + G4.1-G4.5 烈马代签机制同模式 (AL-1 工程内部 wrapper 不进野马 G4 流, 用户感知层 AL-1b PresenceDot 5-state 已落 #462). REG-AL1-001..005 5🟢 + 20+ unit test PASS (6 schema + 9 validator + 5 endpoint + store helper). 反向断言三处 (state machine 单门 helper + owner-only state-log + 6 reason 第 8 处 byte-identical + forward-only audit) 全过. 跨 milestone byte-identical 链全锚 (AL-1a #249 + AL-1b #453+#457+#462 + BPP-2.2 #485 前置就位 + ADM-0 §1.3 红线 + forward-only 跨 milestone 同精神 + sanitizer 跟 ADM-2 同模式 + state machine 单门 helper 跟 ActionHandler/Pusher seam 同精神). 留账 4 项 ⏸️ deferred (REG-AL1-006..009: BPP-2.2 dispatcher wire + PresenceTracker wire + client UI + e2e), 跟蓝图 §2.3 边界 cross-ref. registry 数学: 220/194/26 → 225/199/26 (+5 行 5🟢 全 active). G4 batch 同 worktree 协调外, AL-1 #492 同 batch 接续不 idle. |
