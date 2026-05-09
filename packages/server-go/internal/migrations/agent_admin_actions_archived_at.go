package migrations

import "gorm.io/gorm"

// adminActionsArchivedAt is migration v=33 — Phase 6 / AL-7.1.
//
// Blueprint锚: `admin-model.md` §3 retention + ADM-2.1 #484 forward-only
// audit. Spec brief: docs/implementation/modules/al-7-spec.md §0 设计 ①
// + §1 拆段 AL-7.1.
//
// What this migration does (two changes in one migration — both required
// for retention sweeper round-trip + admin override audit row):
//
//   1. ALTER TABLE admin_actions ADD COLUMN archived_at INTEGER NULL
//      (跟 ap_2_1 revoked_at + ap_1_1 expires_at + ap_3_1 org_id
//      ALTER ADD COLUMN NULL 跨四 milestone 同模式). NULL = active 行
//      (retention sweeper 未 archive); sweeper UPDATE archived_at = now
//      → 软 archive (forward-only 设计沿用 ADM-2.1 + AP-2).
//   2. CREATE INDEX idx_admin_actions_archived_at ON admin_actions(
//      archived_at) WHERE archived_at IS NOT NULL — sparse index 仅扫
//      已 archive 行 (跟 ap_2_1 revoked_at + ap_1_1 expires_at +
//      ap_3_1 org_id sparse 同模式).
//   3. admin_actions CHECK enum 12-step rebuild — 11 项扩 12 项加
//      'audit_retention_override' (admin override endpoint 写 audit
//      必用此 action; 跟 CV-3.1 / CV-2 v2 / AP-2 / BPP-8 12-step 同模式
//      — SQLite 不支持 ALTER CHECK).
//
// 反约束 (al-7-spec.md §0.1+§0.3):
//   - 不挂 NOT NULL — archived_at NULL = active, 跟 ADM-2.1 ABAC 行为
//     零变.
//   - 不挂 default 值 — NULL 是合法终态.
//   - INDEX WHERE archived_at IS NOT NULL — partial index, 现网零开销.
//   - admin_actions enum 加 1 项 (11 → 12) — 反向 reject spec 外值
//     (TestAL71_RejectsUnknownAction 守).
//   - 不裂表 — grep 检查 `audit_archive_table\|audit_history_log\|
//     al7_archive_log` 0 hit (TestAL71_NoSeparateArchiveTable 守).
//
// v=33 sequencing: BPP-8.1 v=31 (#532 merged) → CV-6.1 v=32 (#531 待
// merge) → AL-7.1 **v=33** (本 migration). registry.go 字面锁; 顺位.
//
// v0 stance: forward-only, no Down().
var adminActionsArchivedAt = Migration{
	Version: 33,
	Name:    "al_7_1_admin_actions_archived_at",
	Up: func(tx *gorm.DB) error {
		if exists, err := hasTable(tx, "admin_actions"); err != nil {
			return err
		} else if !exists {
			return nil
		}

		// Step 1 — admin_actions.archived_at column ALTER ADD (sparse, NULL =
		// active 行).
		if err := tx.Exec(`ALTER TABLE admin_actions ADD COLUMN archived_at INTEGER`).Error; err != nil {
			return err
		}

		// Step 2 — sparse index 跟 ap_2_1 / ap_1_1 / ap_3_1 同模式.
		if err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_admin_actions_archived_at
			ON admin_actions(archived_at) WHERE archived_at IS NOT NULL`).Error; err != nil {
			return err
		}

		// Step 3 — admin_actions CHECK enum 12-step rebuild (跟 BPP-8.1 / AP-2.1 /
		// CV-3.1 / CV-2 v2 同模式 — SQLite 不支持 ALTER CHECK, 必须 create
		// _new + copy + drop + rename).
		if err := tx.Exec(`CREATE TABLE admin_actions_al71_new (
  id              TEXT    NOT NULL PRIMARY KEY,
  actor_id        TEXT    NOT NULL,
  target_user_id  TEXT    NOT NULL,
  action          TEXT    NOT NULL CHECK (action IN ('delete_channel','suspend_user','change_role','reset_password','start_impersonation','permission_expired','plugin_connect','plugin_disconnect','plugin_reconnect','plugin_cold_start','plugin_heartbeat_timeout','audit_retention_override')),
  metadata        TEXT    NOT NULL DEFAULT '',
  created_at      INTEGER NOT NULL,
  archived_at     INTEGER
)`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`INSERT INTO admin_actions_al71_new
			(id, actor_id, target_user_id, action, metadata, created_at, archived_at)
			SELECT id, actor_id, target_user_id, action, metadata, created_at, archived_at
			FROM admin_actions`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`DROP TABLE admin_actions`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`ALTER TABLE admin_actions_al71_new RENAME TO admin_actions`).Error; err != nil {
			return err
		}
		// Recreate indexes (DROP TABLE drops them; Step 2 already added
		// archived_at sparse, but rebuild dropped it — recreate together with
		// the legacy ones).
		if err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_admin_actions_target_user_id_created_at
			ON admin_actions(target_user_id, created_at DESC)`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_admin_actions_actor_id_created_at
			ON admin_actions(actor_id, created_at DESC)`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_admin_actions_archived_at
			ON admin_actions(archived_at) WHERE archived_at IS NOT NULL`).Error; err != nil {
			return err
		}
		return nil
	},
}
