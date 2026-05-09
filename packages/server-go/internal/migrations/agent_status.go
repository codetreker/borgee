package migrations

import (
	"gorm.io/gorm"
)

// al1b1AgentStatus is migration v=21 — Phase 4 / AL-1b.1.
//
// Blueprint锚: `agent-lifecycle.md` §2.3 (5-state, 2026-04-28 4 人 review #5
// 决议: busy/idle 跟 BPP 同期 Phase 4 — source 必须是 plugin 上行 task_started /
// task_finished frame, 没 BPP 不能 stub). Spec brief: `docs/implementation/
// modules/al-1b-spec.md` (战马C v0). Acceptance: `docs/qa/acceptance-templates/
// al-1b.md` (烈马 #193 v0) §1.*.
//
// What this migration does:
//   1. CREATE TABLE agent_status:
//        - agent_id              TEXT    PRIMARY KEY    (1 row per agent;
//                                                        逻辑 FK agents.id,
//                                                        SQLite FK 默认禁用 —
//                                                        跟 al_3_1 / al_4_1 /
//                                                        cv_2_1 / dm_2_1 同模式)
//        - state                 TEXT    NOT NULL       (CHECK ('busy','idle')
//                                                        — 设计 ③ 文案三态:
//                                                        AL-1b schema 仅 2 态,
//                                                        client UI 合并 AL-1a
//                                                        三态 + AL-3 presence
//                                                        显示 5-state)
//        - last_task_id          TEXT    NULL           (BPP frame 上行的
//                                                        task_id; idle 态时
//                                                        可空 — 5min 无 frame
//                                                        判 idle 时无 task)
//        - last_task_started_at  INTEGER NULL           (Unix ms; busy 态时
//                                                        填, BPP task_started
//                                                        frame 触发更)
//        - last_task_finished_at INTEGER NULL           (Unix ms; idle 态时
//                                                        填, BPP task_finished
//                                                        frame 触发更)
//        - created_at            INTEGER NOT NULL       (Unix ms)
//        - updated_at            INTEGER NOT NULL       (Unix ms; state
//                                                        transition 触发更)
//   2. CREATE INDEX idx_agent_status_state
//        ON agent_status(state) — busy 列表 lookup 热路径 (acceptance §1.5).
//      跟 al_3_1 idx_presence_sessions_user_id / al_4_1 idx_agent_runtimes_
//      agent_id 同模式 — 显式命名让 EXPLAIN QUERY PLAN 可读 + 反查 grep 可断.
//
// 反约束 (al-1b-spec.md §0 + §4 + acceptance §1.* + 设计 ① 拆三路径):
//   - 设计 ① "拆三路径": 表无 `is_online` / `presence` 列 (跟 AL-3
//     presence_sessions 拆死 — agent 在 task in-flight 但 hub 心跳超时
//     是合法态, 不能用一列 is_online 替代两表两路径).
//   - 设计 ① 反 AL-4: 表无 `last_error_reason` / `endpoint_url` /
//     `process_kind` 列 (那是 AL-4 agent_runtimes process-level — busy/idle
//     是 task-level, 拆死).
//   - 设计 ② "BPP 单源": 表无 `source` / `set_by` 列 (反人工伪造 — busy/idle
//     state machine 唯一 source = BPP frame, server 端 state machine 守, 不
//     在 schema 层暴露).
//   - 不挂 cursor 列 (跟 RT-1 envelope cursor 拆死, 同 al_3_1 / al_4_1 /
//     cv_*_1 / dm_2_1 模式).
//   - 不挂 ON DELETE CASCADE (蓝图 §2.3 字面 "保留状态历史"; agent 删后
//     status row 留账 — admin 审计路径).
//
// v0 stance: forward-only, no Down(). 表本身 v0 新增, IF NOT EXISTS 守
// idempotency. 跟 al_3_1 / al_4_1 / cv_2_1 / dm_2_1 / cm_4_0 同模式逻辑 FK.
//
// v=21 sequencing (字面延续, 跟 spec brief §2 byte-identical):
// CV-2.1 v=14 ✅ #359 / DM-2.1 v=15 ✅ #361 / AL-4.1 v=16 ✅ #398 /
// CV-3.1 v=17 ✅ #396 / CV-4.1 v=18 ✅ #405 / CHN-3.1 v=19 ✅ #410 /
// CHN-4.1 v=20 ✅ #411 (占位无 schema 改) / **AL-1b.1 v=21** (本 migration) /
// AL-2a.1 v=22 占号 (zhanma-a Phase 4 平行 — schema 依赖无, 跟 AL-1b 平行).
var al1b1AgentStatus = Migration{
	Version: 21,
	Name:    "al_1b_1_agent_status",
	Up: func(tx *gorm.DB) error {
		if err := tx.Exec(`CREATE TABLE IF NOT EXISTS agent_status (
  agent_id              TEXT    PRIMARY KEY,
  state                 TEXT    NOT NULL CHECK (state IN ('busy','idle')),
  last_task_id          TEXT,
  last_task_started_at  INTEGER,
  last_task_finished_at INTEGER,
  created_at            INTEGER NOT NULL,
  updated_at            INTEGER NOT NULL
)`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_agent_status_state
			ON agent_status(state)`).Error; err != nil {
			return err
		}
		return nil
	},
}
