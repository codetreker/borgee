package migrations

import "gorm.io/gorm"

// adminActionsPluginActions is migration v=31 — Phase 6 / BPP-8.1.
//
// Blueprint锚: `plugin-protocol.md` §1.6 (失联与故障状态) + §3 plugin
// lifecycle audit. Spec brief: `docs/implementation/modules/bpp-8-spec.md`
// (战马D v0) §0 设计 ① + §1 拆段 BPP-8.1.
//
// What this migration does:
//
//   admin_actions CHECK enum 12-step rebuild — 6 项 → 11 项加 5 条
//   plugin_* 字面 (`plugin_connect / plugin_disconnect / plugin_reconnect
//   / plugin_cold_start / plugin_heartbeat_timeout`).
//
// Reuses ADM-2.1 #484 admin_actions table for plugin lifecycle audit
// instead of a separate plugin_lifecycle_events table — audit forward-only
// pattern shared with ADM-2.1 + AP-2 #525 sweeper + BPP-4 #499 watchdog
// across five milestones (锁链第 5 处).
//
// 反约束 (bpp-8-spec.md §0 设计 ①):
//   - 不裂表: grep 检查 `plugin_lifecycle_events\|plugin_audit_log\|
//     bpp_event_log` 0 hit.
//   - admin_actions enum 加 5 项 (6→11) — 反向 reject spec 外值
//     (TestBPP81_RejectsUnknownAction 守).
//
// v=31 sequencing: AP-2.1 v=30 (#525 merged) → BPP-8.1 **v=31** (本
// migration). 跟 CV-3.1 / CV-2 v2 / AP-2 12-step table-recreate 同模式
// (SQLite 不支持 ALTER CHECK).
//
// v0 stance: forward-only, no Down().
var adminActionsPluginActions = Migration{
	Version: 31,
	Name:    "bpp_8_1_admin_actions_plugin_actions",
	Up: func(tx *gorm.DB) error {
		if exists, err := hasTable(tx, "admin_actions"); err != nil {
			return err
		} else if !exists {
			return nil
		}

		// 12-step CHECK rebuild — SQLite 不支持 ALTER CHECK, 必须 create
		// _new + copy + drop + rename. 跟 ap_2_1 / cv_3_1 同模式.
		if err := tx.Exec(`CREATE TABLE admin_actions_bpp81_new (
  id              TEXT    NOT NULL PRIMARY KEY,
  actor_id        TEXT    NOT NULL,
  target_user_id  TEXT    NOT NULL,
  action          TEXT    NOT NULL CHECK (action IN ('delete_channel','suspend_user','change_role','reset_password','start_impersonation','permission_expired','plugin_connect','plugin_disconnect','plugin_reconnect','plugin_cold_start','plugin_heartbeat_timeout')),
  metadata        TEXT    NOT NULL DEFAULT '',
  created_at      INTEGER NOT NULL
)`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`INSERT INTO admin_actions_bpp81_new
			(id, actor_id, target_user_id, action, metadata, created_at)
			SELECT id, actor_id, target_user_id, action, metadata, created_at
			FROM admin_actions`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`DROP TABLE admin_actions`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`ALTER TABLE admin_actions_bpp81_new RENAME TO admin_actions`).Error; err != nil {
			return err
		}
		// Recreate indexes (DROP TABLE drops them).
		if err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_admin_actions_target_user_id_created_at
			ON admin_actions(target_user_id, created_at DESC)`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_admin_actions_actor_id_created_at
			ON admin_actions(actor_id, created_at DESC)`).Error; err != nil {
			return err
		}
		return nil
	},
}
