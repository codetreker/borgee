package migrations

import (
	"gorm.io/gorm"
)

// agentStateLog is migration v=25 — Phase 4 / AL-1 状态四态扩展.
//
// Blueprint锚: `agent-lifecycle.md` §2.3 (4 态: online / busy / idle / error;
// 状态机 + cross-state transition lock + 故障可解释).
// Spec: AL-1 整 milestone (跟 AL-1a #249 三态 stub + AL-1b #453/#457/#462
// 5-state busy/idle 同源 — 此表是 historical audit 轨迹, server reducer
// 真接管 BPP-2.2 #485 task_started/finished frame).
//
// What this migration does:
//   1. CREATE TABLE agent_state_log:
//        - id          INTEGER PK AUTOINCREMENT  (单调 history 序号)
//        - agent_id    TEXT    NOT NULL          (FK users.id role='agent';
//                                                 逻辑 FK 跟 al_1b_1 同模式)
//        - from_state  TEXT    NOT NULL          (前态; '' 表示首次, 跟 AL-1a
//                                                 三态 'online'/'offline'/'error'
//                                                 + AL-1b 'busy'/'idle' 5 字面 union)
//        - to_state    TEXT    NOT NULL          (新态; 同上 5-state union)
//        - reason      TEXT    NOT NULL DEFAULT '' (failed → 复用 AL-1a 6 reason
//                                                 byte-identical: api_key_invalid /
//                                                 quota_exceeded / network_unreachable /
//                                                 runtime_crashed / runtime_timeout /
//                                                 unknown; 非 error 转移留空)
//        - task_id     TEXT    NOT NULL DEFAULT '' (BPP-2.2 task lifecycle 触发的
//                                                 转移记 task_id; presence 触发留空)
//        - ts          INTEGER NOT NULL          (Unix ms, 转移发生时刻)
//   2. CREATE INDEX idx_agent_state_log_agent_id_ts
//      ON agent_state_log(agent_id, ts DESC) — owner GET /api/v1/agents/:id/
//      state-log 热路径.
//
// 反约束 (AL-1 蓝图 §2.3 + AL-1a/AL-1b 设计沿用):
//   - 设计 ① forward-only: log 不可改写, schema 不挂 `updated_at` 列
//     (跟 admin_actions ADM-2.1 设计 ⑤ 同精神 — audit 100% 留痕)
//   - 设计 ② state machine 单源: from_state + to_state CHECK 不挂 schema
//     层 (server-side ValidateTransition 走严格 graph), 让 AL-1a 三态 +
//     AL-1b 5-state 复用同表无需 schema 改 (反向: 不为每态裂表, 跟 AL-1b
//     设计 ① 拆三路径同精神 — 一表一职 audit, 状态语义在 server)
//   - 设计 ③ task-driven 优先: BPP-2.2 task lifecycle (#485) 触发 busy/idle
//     转移时 task_id 必填; presence 触发 (online/offline/error) task_id 空.
//     grep 检查 `agent_state_log.*UPDATE\|DELETE FROM agent_state_log` 在
//     internal/ (除 migration) count==0
//   - 设计 ④ reason 复用 AL-1a 6 字面: error 态转移必带 reason ∈ AL-1a 6 字面
//     (改 = 改 7 处单测锁链: #249 + #305 + #321 + #380 + #454 + #458 + 此表).
//     非 error 转移 reason 留空.
//   - 反向列名: 不挂 `cursor` (跟 al_3_1 / al_4_1 / cv_*_1 / dm_2_1 / cv_4_1 /
//     chn_3_1 / al_2a_1 / adm_2_1 同模式 RT-1 envelope frame 路径不下沉);
//     不挂 `org_id` (派生 users.org_id, 跟 admin_actions 设计 ⑥ 同精神)
//
// v=25 sequencing: ADM-2.1 v=22 / ADM-2.2 v=23 / DL-4 v=24 / **AL-1 v=25** (本 migration);
// v=26 留下个 milestone (CM-5/AP-1/etc).
//
// v0 stance: forward-only, no Down(). 表本身 v0 新增, IF NOT EXISTS 守
// idempotency.
var agentStateLog = Migration{
	Version: 25,
	Name:    "al_1_4_agent_state_log",
	Up: func(tx *gorm.DB) error {
		if err := tx.Exec(`CREATE TABLE IF NOT EXISTS agent_state_log (
  id          INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
  agent_id    TEXT    NOT NULL,
  from_state  TEXT    NOT NULL,
  to_state    TEXT    NOT NULL,
  reason      TEXT    NOT NULL DEFAULT '',
  task_id     TEXT    NOT NULL DEFAULT '',
  ts          INTEGER NOT NULL
)`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_agent_state_log_agent_id_ts
			ON agent_state_log(agent_id, ts DESC)`).Error; err != nil {
			return err
		}
		return nil
	},
}
