package migrations

import (
	"gorm.io/gorm"
)

// artifactIterations is migration v=18 — Phase 3 / CV-4.1.
//
// Blueprint出处: `canvas-vision.md` §1.4 ("artifact 自带版本历史: agent 每次
// 修改产生一个版本, 人可以回滚") + §1.5 ("agent 写内容默认允许") + §2 v1
// 做清单 ("agent 可 iterate, 再次写入触发新版本") + §3 差距 ("Agent
// iterate / 版本历史: 无 → 需要新表 + 写入策略").
// Spec brief: `docs/implementation/modules/cv-4-spec.md` (#365 v0,
// merged 9720a66) §0 设计 ① 域隔离 + ② commit 单一来源 + ③ client 算 diff +
// §1 拆段 CV-4.1.
// 原则: `docs/qa/cv-4-stance-checklist.md` (#385, merged 572a5ea).
// Acceptance: `docs/qa/acceptance-templates/cv-4.md` (#384, merged
// 4777bfc) §1.1-§1.5.
// Content lock: `docs/qa/cv-4-content-lock.md` (#380, merged 8c1f30a)
// state 4 态 byte-identical + reason 三处单测锁定 + jsdiff.
//
// What this migration does:
//  1. CREATE TABLE artifact_iterations with these columns:
//     id TEXT PRIMARY KEY (uuid; one row per iterate request); artifact_id TEXT
//     NOT NULL (logical FK to artifacts.id; SQLite FK is off, same as cv_1_1 /
//     cv_2_1 / dm_2_1 / al_3_1 / al_4_1); requested_by TEXT NOT NULL (FK users.id;
//     owner-only, acceptance §2.1 + ADM-0 §1.3 红线 设计 ⑦); intent_text TEXT
//     NOT NULL (user intent and privacy field; admin responses must not return
//     unredacted text, acceptance §2.7 + ADM-0 §1.3); target_agent_id TEXT NOT NULL
//     (FK agents.id = users.id where role='agent'; same single-column agent/human
//     semantics as DM-2.1 target_user_id); state TEXT NOT NULL CHECK ('pending',
//     'running','completed','failed') (#380 locked 4-state byte-identical literals;
//     reject 'starting' / 'busy' / 'unknown'); created_artifact_version_id INTEGER
//     NULL (FK artifact_versions.id; filled only on completed, and remains NULL for
//     pending / running / failed); error_reason TEXT NULL (AL-1a #249 6 reason
//     literals byte-identical: api_key_invalid / quota_exceeded /
//     network_unreachable / runtime_crashed / runtime_timeout / unknown, plus AL-4
//     placeholder fail-closed runtime_not_registered; no schema CHECK enum, server
//     validates as in AL-4.1 #398); created_at INTEGER NOT NULL (Unix ms);
//     completed_at INTEGER NULL (Unix ms; set for completed / failed).
//  2. CREATE INDEX idx_iterations_artifact_id_state
//     ON artifact_iterations(artifact_id, state) — per-artifact pending /
//     running hot path (UI inline + state machine guard).
//  3. CREATE INDEX idx_iterations_target_agent
//     ON artifact_iterations(target_agent_id) — agent 工作队列查
//     (acceptance §1.3 字面双索引).
//
// 反向约束 (cv-4-spec.md §0 + §3 + acceptance §1.5):
//   - 设计 ① 域隔离: 不污染 messages 表加反指列 (mention×artifact×
//     anchor×iterate 四路径独立, 跟 CHN-4 #374/#378 设计 ② 同源). 反向
//     grep 加列模式 count==0 (acceptance §4.2 字面).
//   - 设计 ① v0 immutable append: 不动 artifact_versions schema —
//     反指列不开. grep 检查 加列模式 count==0 (acceptance §4.2 字面).
//   - 设计 ② CV-1 commit 单一来源: 不开 `POST /iterations/:id/commit` 旁路 —
//     commit 走 `?iteration_id=` query atomic UPDATE (CV-4.2 server 层
//     implements it; this schema only carries created_artifact_version_id NULL).
//   - 设计 ③ server 不算 diff: 表无 `diff_blob` / `diff_lines` 列 (jsdiff
//     仅 client 算, acceptance §2.6 + §4.4).
//   - state CHECK 严格 reject 'starting' / 'busy' / 'unknown' 中间态
//     (#380 文案锁定 ③ 4 态 byte-identical, 字面禁字典外值).
//   - 不添加 `cursor` 列 (跟 RT-1 envelope cursor 保持分离 — IterationStateChangedFrame
//     9 字段 cursor 是 frame 路径, 不下沉到 iteration schema. 同
//     al_3_1 / al_4_1 / cv_1_1 / cv_2_1 / dm_2_1 模式).
//   - 不添加 `retry_count` 列 (failed 态 owner 重新触发 = 新 iteration_id,
//     不复用 failed 行 — #380 ⑦ + #365 反向约束 ② 同源, acceptance §3.7).
//
// v0 原则: forward-only, no Down(). 表本身 v0 新增, IF NOT EXISTS 守
// idempotency. 跟 al_3_1 / al_4_1 / cv_2_1 / dm_2_1 / cm_4_0 同模式
// 逻辑 FK.
//
// v=18 sequencing (#365 spec §1 + #379 v2 §2): CV-2.1 v=14 ✅ (#359
// merged) / DM-2.1 v=15 ✅ (#361 merged) / AL-4.1 v=16 ✅ (#398 merged) /
// CV-3.1 v=17 ✅ (#388/#396 merged) / **CV-4.1 v=18** (本 migration).
// registry.go 字面锁定.
var artifactIterations = Migration{
	Version: 18,
	Name:    "cv_4_1_artifact_iterations",
	Up: func(tx *gorm.DB) error {
		if err := tx.Exec(`CREATE TABLE IF NOT EXISTS artifact_iterations (
  id                          TEXT    PRIMARY KEY,
  artifact_id                 TEXT    NOT NULL,
  requested_by                TEXT    NOT NULL,
  intent_text                 TEXT    NOT NULL,
  target_agent_id             TEXT    NOT NULL,
  state                       TEXT    NOT NULL CHECK (state IN ('pending','running','completed','failed')),
  created_artifact_version_id INTEGER,
  error_reason                TEXT,
  created_at                  INTEGER NOT NULL,
  completed_at                INTEGER
)`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_iterations_artifact_id_state
			ON artifact_iterations(artifact_id, state)`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_iterations_target_agent
			ON artifact_iterations(target_agent_id)`).Error; err != nil {
			return err
		}
		return nil
	},
}
