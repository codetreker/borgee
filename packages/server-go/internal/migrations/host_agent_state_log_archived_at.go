package migrations

import "gorm.io/gorm"

// agentStateLogArchivedAt is migration v=35 — Phase 6 / HB-5.1.
//
// Blueprint锚: `agent-lifecycle.md` §2.3 forward-only state log + AL-7
// #533 archived_at retention 模式延伸. Spec brief: docs/implementation/
// modules/hb-5-spec.md §0 设计 ① + §1 拆段 HB-5.1.
//
// What this migration does (跟 AL-7.1 admin_actions ADD archived_at 同精神):
//
//   1. ALTER TABLE agent_state_log ADD COLUMN archived_at INTEGER NULL
//      (跟 AP-1.1 expires_at + AP-3.1 org_id + AP-2.1 revoked_at + AL-7.1
//      admin_actions ADD archived_at 跨五 milestone 同模式). NULL = active
//      行 (retention sweeper 未 archive); sweeper UPDATE archived_at = now
//      → 软 archive (forward-only 设计沿用 AL-1 + AL-7).
//   2. CREATE INDEX idx_agent_state_log_archived_at ON agent_state_log(
//      archived_at) WHERE archived_at IS NOT NULL — sparse index 仅扫
//      已 archive 行 (跟 AL-7.1 idx_admin_actions_archived_at + AP-2.1
//      revoked_at sparse 同模式).
//
// 反约束 (hb-5-spec.md §0 设计 ①②⑦):
//   - 不挂 NOT NULL — archived_at NULL = active, 跟 AL-1 行为零变.
//   - 不挂 default 值 — NULL 是合法终态.
//   - INDEX WHERE archived_at IS NOT NULL — partial index, 现网零开销.
//   - 不挂 admin_actions CHECK 改 — admin_actions 12-tuple byte-identical
//     跟 AL-7.1 不动 (HB-5 admin override 复用 AL-7 既有 audit retention
//     action const, metadata target='heartbeat' 字面区分; grep 检查
//     heartbeat_retention_override action literal 在 internal/migrations/
//     0 hit, 设计 ② 守).
//   - 不裂表 — grep 检查 `heartbeat_archive_table\|state_log_history\|
//     hb5_archive_log` 0 hit.
//
// v=35 sequencing: AL-7.1 v=33 (#536 待 merge) → HB-5.1 **v=35**. 跟
// AL-8 #538 顺位 (AL-8 是 0 schema query filter, 不占号).
//
// v0 stance: forward-only, no Down().
var agentStateLogArchivedAt = Migration{
	Version: 35,
	Name:    "hb_5_1_agent_state_log_archived_at",
	Up: func(tx *gorm.DB) error {
		if exists, err := hasTable(tx, "agent_state_log"); err != nil {
			return err
		} else if !exists {
			return nil
		}
		// Step 1 — agent_state_log.archived_at column ALTER ADD (sparse,
		// NULL = active 行).
		if err := tx.Exec(`ALTER TABLE agent_state_log ADD COLUMN archived_at INTEGER`).Error; err != nil {
			return err
		}
		// Step 2 — sparse index 跟 AL-7.1 / AP-2.1 / AP-1.1 / AP-3.1 同模式.
		if err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_agent_state_log_archived_at
			ON agent_state_log(archived_at) WHERE archived_at IS NOT NULL`).Error; err != nil {
			return err
		}
		return nil
	},
}
