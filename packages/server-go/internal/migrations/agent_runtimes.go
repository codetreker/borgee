package migrations

import (
	"gorm.io/gorm"
)

// agentRuntimes is migration v=16 — Phase 4 / AL-4.1.
//
// Blueprint锚: `agent-lifecycle.md` §2.2 (默认 remote-agent + power
// user 直配 plugin 双路径 + v1 务实边界 — only OpenClaw / Mac+Linux /
// 不优化多 runtime 并行) + §4 (remote-agent 安全模型由第 6 轮覆盖);
// `README.md` §1 设计 #7 (Borgee 不带 runtime — 走 plugin 接);
// `concept-model.md` §0 (不调 LLM / 不带 runtime / 不定义角色模板).
// Spec brief: `docs/implementation/modules/al-4-spec.md` (#313 v0
// → #379 v2, merged 962fec7) §0 设计 ①②③ + §1 拆段 AL-4.1.
// Stance: `docs/qa/al-4-stance-checklist.md` (#387, merged 8db1f9c).
// Acceptance: `docs/qa/acceptance-templates/al-4.md` (#318) §1.1-§1.5.
// Content lock: `docs/qa/al-4-content-lock.md` (#321).
//
// What this migration does:
//  1. CREATE TABLE agent_runtimes:
//     - id                 TEXT    PRIMARY KEY      (uuid; 1 row per agent)
//     - agent_id           TEXT    NOT NULL UNIQUE  (FK agents.id; 设计 ①
//     v1 不优化多 runtime
//     并行, 1 runtime per
//     agent. 逻辑 FK,
//     SQLite FK 默认禁用 —
//     跟 cv_1_1 / cv_2_1 /
//     dm_2_1 同模式)
//     - endpoint_url       TEXT    NOT NULL         (plugin WS/HTTP 入口)
//     - process_kind       TEXT    NOT NULL         (CHECK ('openclaw',
//     'hermes') — v1 仅
//     'openclaw' 蓝图 §2.2
//     v1 边界字面, 'hermes'
//     reserved for v2+; 反约束:
//     reject 'unknown' 等
//     枚举外值)
//     - status             TEXT    NOT NULL         (CHECK ('registered',
//     'running','stopped',
//     'error') — process-
//     level 4 态, 设计 ③
//     kept separate from
//     AL-3 session-level)
//     - last_error_reason  TEXT    NULL             (复用 AL-1a #249 6
//     reason 枚举字面 —
//     api_key_invalid /
//     quota_exceeded /
//     network_unreachable /
//     runtime_crashed /
//     runtime_timeout /
//     unknown; 隐私: admin
//     god-mode must not return
//     this field's unredacted text,
//     设计 ⑦
//     ADM-0 红线)
//     - last_heartbeat_at  INTEGER NULL             (Unix ms; process-
//     level heartbeat,
//     设计 ③ 不写
//     presence_sessions —
//     AL-3 hub lifecycle
//     路径)
//     - created_at         INTEGER NOT NULL         (Unix ms)
//     - updated_at         INTEGER NOT NULL         (Unix ms; status
//     transition 触发更)
//  2. CREATE INDEX idx_agent_runtimes_agent_id
//     ON agent_runtimes(agent_id) — lookup hot path (acceptance §1.3).
//     UNIQUE(agent_id) 已自动建 sqlite_autoindex_agent_runtimes_*, 此显式
//     idx 是 acceptance 字面要求 (跟 AL-3.1 idx_presence_sessions_user_id
//     / DM-2.1 idx_message_mentions_target_user_id 同模式 — 显式命名让
//     EXPLAIN QUERY PLAN 可读 + 反查 grep 可断).
//
// 反约束 (al-4-spec.md §0 + §3 + acceptance §1.5):
//   - 设计 ① "Borgee 不带 runtime": 表无 `llm_provider` / `model_name` /
//     `api_key` / `prompt_template` 列 (那是 plugin 内部事, 设计 #7 字面;
//     grep 检查 + 反向 column list 双闸).
//   - 设计 ③ runtime status ≠ presence: 表无 `is_online` 列 (跟 AL-3
//     presence_sessions 保持分离 — runtime 在跑但 WS 断 是合法态, 不能用
//     一列 is_online 替代两表两路径).
//   - 不添加 cursor 列 (跟 RT-1 envelope cursor 保持分离, 同 al_3_1 / cv_1_1 /
//     cv_2_1 / dm_2_1 模式).
//   - 不添加 `pid` / `gpu_id` / `priority` 列 (由第 6 轮 + Phase 5+ 覆盖,
//     蓝图 §4 字面).
//
// v0 stance: forward-only, no Down(). 表本身 v0 新增, IF NOT EXISTS 守
// idempotency. 跟 al_3_1 / cv_2_1 / dm_2_1 / cm_4_0 同模式逻辑 FK.
//
// v=16 sequencing (#379 v2 §2 + #361/#363 兑现): CV-2.1 v=14 ✅ (#359
// merged) / DM-2.1 v=15 ✅ (#361 merged) / **AL-4.1 v=16** (本 migration) /
// CV-3.1 v=17 (#396 after the follow-up spec patch) / CV-4.1 v=18 /
// CHN-3.1 v=19 / CHN-4.1 v=20 占位无 schema 改.
var agentRuntimes = Migration{
	Version: 16,
	Name:    "al_4_1_agent_runtimes",
	Up: func(tx *gorm.DB) error {
		if err := tx.Exec(`CREATE TABLE IF NOT EXISTS agent_runtimes (
  id                 TEXT    PRIMARY KEY,
  agent_id           TEXT    NOT NULL UNIQUE,
  endpoint_url       TEXT    NOT NULL,
  process_kind       TEXT    NOT NULL CHECK (process_kind IN ('openclaw','hermes')),
  status             TEXT    NOT NULL CHECK (status IN ('registered','running','stopped','error')),
  last_error_reason  TEXT,
  last_heartbeat_at  INTEGER,
  created_at         INTEGER NOT NULL,
  updated_at         INTEGER NOT NULL
)`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_agent_runtimes_agent_id
			ON agent_runtimes(agent_id)`).Error; err != nil {
			return err
		}
		return nil
	},
}
