# Acceptance Template — AL-1: 状态四态扩展 (state machine + 历史 audit)

> 蓝图: `docs/blueprint/current/agent-lifecycle.md` §2.3 (4 态: online / busy / idle / error + cross-state transition lock + 故障可解释)
> 依赖: AL-1a ✅ (#249 三态 + 6 reason) + AL-1b ✅ (#453/#457/#462 5-state busy/idle) + BPP-2.2 ✅ (#485 task_started/task_finished frame) — 此 milestone 是 wrapper, 落 state machine validator + historical audit + GET endpoint
> Owner: 战马D 实施 / 烈马 验收
> R3 注: AL-1 是 AL-1a + AL-1b + state log audit 三件 wrapper, 蓝图 §2.3 行为不变量在此真实施 (validator gate + history)

## 拆 PR 顺序 (新协议: 一 milestone 一 PR)

- **AL-1 整 milestone** PR — 一 PR 装 schema (agent_state_log v=25) + state machine validator (5-state graph: online/busy/idle/error/offline + '' sentinel = pre-online 初始态不算 state + 6 reason 复用 AL-1a 字面) + AppendAgentStateTransition helper + GET /api/v1/agents/:id/state-log endpoint + 20 unit tests + closure

> 历史: AL-1a #249 (online/offline + 6 reason) + AL-1b #453/#457/#462 (busy/idle 5-state) 已 merged 各自 PR. AL-1 wrapper 此 PR 真闭 — 跟 ADM-2 整 milestone PR #484 同模式.

---

## 验收清单

### 数据契约 (蓝图 §2.3)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| `agent_state_log` schema (id PK AUTOINCREMENT / agent_id / from_state / to_state / reason / task_id / ts) + idx_agent_state_log_agent_id_ts (DESC) | unit + migration test | 战马D / 烈马 | ✅ — `internal/migrations/al_1_4_agent_state_log_test.go` 6 tests PASS (CreatesAgentStateLogTable / InsertAndAutoIncrement / NoDomainBleed 6 forbidden 列 / HasIndex / AcceptsAL1aReasonValues 6 字面 / Idempotent) |

### 状态机验证 (蓝图 §2.3 — 立场 ② state machine 单源)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 状态图 byte-identical 跟蓝图 §2.3 (4 态 + initial sentinel + 14 valid edges) | unit table-driven `TestValidateTransition_ValidEdges` | 战马D / 烈马 | ✅ — `agent_state_log_test.go::TestValidateTransition_ValidEdges` (15 valid edges PASS) |
| 反向断言: same-state / busy↛online / idle↛online / error↛busy/idle / offline↛busy/idle/error 全 reject | unit `TestValidateTransition_RejectsSameState/_RejectsInvalidEdges` | 战马D / 烈马 | ✅ — 5 same-state reject + 10 invalid edge reject PASS |
| error 转移必带 reason ∈ AL-1a 6 字面 byte-identical (`api_key_invalid`/`quota_exceeded`/`network_unreachable`/`runtime_crashed`/`runtime_timeout`/`unknown`); 字典外 / 大小写漂移 reject | unit `TestValidateTransition_ErrorRequiresReason` + `_AllReasons5Pin` | 战马D / 烈马 | ✅ — 6 valid reason accept + 7 invalid reject + 第 8 处单测锁链 (#249 + #305 + #321 + #380 + #454 + #458 + #481 + 此) |

### 行为不变量 (闸 4 — AL-1.4)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 4.1.a AppendAgentStateTransition 走 ValidateTransition gate (server-side state set 唯一入口) — 反向: empty agent_id reject + invalid transition reject + same state reject + error 无 reason reject | unit `TestAppendAgentStateTransition_HappyPath/_RejectsInvalidViaValidator` | 战马D / 烈马 | ✅ — happy path 4 transitions + 5 reject case PASS |
| 4.1.b 历史轨迹 forward-only (立场 ① 跟 admin_actions ADM-2.1 立场 ⑤ 同精神) — schema 不挂 updated_at, 反向 grep `UPDATE agent_state_log\|DELETE FROM agent_state_log` 在 internal/ (除 migration) count==0 | unit + CI grep | 战马D / 烈马 | ✅ — schema 反向 NoDomainBleed 锁 updated_at 不挂; store helpers 不暴露 update/delete |
| 4.1.c GET /api/v1/agents/:id/state-log owner-only (蓝图 §2.3 "故障可解释" — owner 看自己 agent 病历); non-owner → 403; non-agent user → 404; unauthenticated → 401 | unit + e2e | 战马D / 烈马 | ✅ — `al_1_4_state_log_test.go` 5 endpoint tests PASS (Owner sees / NonOwner 403 / Unauth 401 / NotFound / NonAgent 404) |
| 4.1.d 反向断言 — endpoint response 不返 raw `agent_id` (在 URL path, 反向不重复); 反向不返 owner_id raw (sanitizer 跟 ADM-2 同模式) | unit | 战马D / 烈马 | ✅ — `TestAL14_GetStateLog_OwnerSeesAgentHistory` 反向 row 不含 agent_id 字段 PASS |

### 蓝图行为对照 (闸 2 — 立场反查)

| 验收项 | 实施方式 | Owner | 实施证据 |
|---|---|---|---|
| 立场 ① forward-only audit (蓝图 §2 不变量同精神) — 反向 grep `UPDATE agent_state_log\|DELETE FROM agent_state_log` count==0 | CI grep | 战马D / 烈马 | ✅ — store helpers 仅 INSERT + SELECT 路径; 反向 grep 实跑 (除 migration) count==0 |
| 立场 ② state machine 单源 — `AppendAgentStateTransition` 是 server-side set 唯一入口 (反向 grep raw `INSERT INTO agent_state_log` 在非 helper 路径 count==0) | CI grep + unit | 战马D / 烈马 | ✅ — helper 是单 entry, 测试覆盖 happy + reject; 反向 grep 实跑 |
| 立场 ④ reason 复用 AL-1a 6 字面 byte-identical (改 = 改 8 处单测锁链同源) | unit cross-milestone byte-identical | 战马D / 烈马 | ✅ — `TestValidateTransition_AllReasons5Pin` 锁 6 字面 + 反向不允许 extra entry; 跟 #249 / #305 / #321 / #380 / #454 / #458 / #481 / 此 8 处链 |

### 退出条件

- 上表 11 项: **11 ✅** (全绿)
- AL-1a #249 + AL-1b #453/#457/#462 + BPP-2.2 #485 已 merged 前置满足
- 战马 PR review 同意 + 烈马 acceptance 跑完
- 登记 `docs/qa/regression-registry.md` REG-AL1-001..005 (PR merge 后 24h 内翻 ⚪ → 🟢)
- ⚠️ AL-1 是工程内部 milestone — busy/idle 是平台级状态机, 用户感知层 AL-1b PresenceDot 5-state 已落 (#462), 此 PR 不进野马 G4 签字流, 烈马代签

### Follow-up 留账 (非阻 PR merge)

- BPP-2.2 task_started/task_finished frame 接收时 server 端 dispatcher 自动调 `AppendAgentStateTransition` 写 audit (当前 #485 已 dispatcher stub, 落到 audit 写需 wire — 跟 AL-1b SetAgentTaskStarted 同模式)
- AL-1a presence track online/offline 时同样 wire audit append (PresenceTracker hub lifecycle hook)
- client SPA agent state 历史轨迹 UI (蓝图 §2.3 "直达修复入口" — error 状态显示 reason + 修复按钮)
- e2e 完整状态流 (offline → online → busy → idle → error → offline)

## 更新日志

| 日期 | 作者 | 变化 |
|---|---|---|
| 2026-04-29 | 战马D | v0 — AL-1 wrapper milestone (新协议一 PR 整闭): schema agent_state_log v=25 + state machine validator (5-态 graph + 6 reason 第 8 处单测锁链) + AppendAgentStateTransition helper + GET /api/v1/agents/:id/state-log endpoint owner-only + 20 unit tests; 历史 AL-1a #249 / AL-1b #453+#457+#462 / BPP-2.2 #485 已 merged 前置就位; follow-up 留账 4 项 (dispatcher wire + presence wire + client UI + e2e) |
