package migrations

import "gorm.io/gorm"

// userPermissionsRevoked is migration v=30 — Phase 5+ / AP-2.1.
//
// Blueprint锚: `auth-permissions.md` §5 ("expires_at 列 — 加列 schema 不破,
// 暂不业务化") — AP-1.1 #493 schema 列已就位, AP-2 接 runtime 业务化 sweeper.
// Spec brief: docs/implementation/modules/ap-2-spec.md (战马C v0, cfa3869)
// §0 设计 ① + §1 拆段 AP-2.1.
//
// What this migration does (two changes in one migration — both required
// for the sweeper round-trip 跟 AP-3 #521 设计 ② cross-org 同精神 schema +
// runtime 同步落地):
//
//   1. ALTER TABLE user_permissions ADD COLUMN revoked_at INTEGER NULL
//      (跟 ap_1_1 #493 expires_at + ap_3_1 #521 org_id ALTER ADD COLUMN
//      NULL 同模式). NULL = active 行 (跟 AP-1 现网行为零变, 任一 NULL
//      走 legacy 路径).
//   2. CREATE INDEX idx_user_permissions_revoked ON user_permissions(
//      revoked_at) WHERE revoked_at IS NOT NULL — sparse index 仅扫
//      已 revoke 行 (跟 ap_1_1 expires_at + ap_3_1 org_id sparse 同模式).
//   3. admin_actions CHECK enum 12-step rebuild — 5 项扩 6 项加
//      'permission_expired' (sweeper revoke 路径写 audit 必用此 action,
//      跟 CV-3.1 / CV-2 v2 12-step table-recreate 同模式; SQLite 不支持
//      ALTER CHECK).
//
// 反约束 (auth-permissions.md §5 + ap-2-spec.md §0 设计 ①②):
//   - 不挂 NOT NULL — revoked_at NULL = active, 跟 AP-1 ABAC 行为零变.
//   - 不挂 default 值 — NULL 是合法终态.
//   - 不挂 FK — 跟 user.org_id / channels.org_id 同精神 (业务校验 server 层).
//   - INDEX WHERE revoked_at IS NOT NULL — partial index, 现网零开销.
//   - admin_actions enum 加 1 项 (5→6) — 反向 reject spec 外值 (TestAP21_
//     AdminActionsRejectsUnknownAction 守).
//
// v=30 sequencing: AP-3.1 v=29 (in flight #521) → CV-2 v2 v=28 (in flight
// #517) → AP-2.1 **v=30** (本 migration). registry.go 字面锁; 谁先 merge
// 谁拿号, 后顺延 (跟 CV-2 v1 spec §2 v=14 三方撞号 sequencing 协议同).
//
// v0 stance: forward-only, no Down(). Idempotent re-run guard via outer
// migration framework's schema_migrations gate.
var userPermissionsRevoked = Migration{
	Version: 30,
	Name:    "ap_2_1_user_permissions_revoked",
	Up: func(tx *gorm.DB) error {
		// Step 1 — user_permissions.revoked_at column (trimmed-schema gate
		// 跟 ap_1_1 / ap_3_1 同模式).
		if exists, err := hasTable(tx, "user_permissions"); err != nil {
			return err
		} else if exists {
			if err := tx.Exec(`ALTER TABLE user_permissions ADD COLUMN revoked_at INTEGER`).Error; err != nil {
				return err
			}
			if err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_user_permissions_revoked
				ON user_permissions(revoked_at) WHERE revoked_at IS NOT NULL`).Error; err != nil {
				return err
			}
		}

		// Step 2 — admin_actions CHECK enum 12-step rebuild (跟 CV-3.1 / CV-2 v2
		// 同模式 — SQLite 不支持 ALTER CHECK, 必须 create _new + copy + swap).
		if exists, err := hasTable(tx, "admin_actions"); err != nil {
			return err
		} else if exists {
			if err := tx.Exec(`CREATE TABLE admin_actions_ap21_new (
  id              TEXT    NOT NULL PRIMARY KEY,
  actor_id        TEXT    NOT NULL,
  target_user_id  TEXT    NOT NULL,
  action          TEXT    NOT NULL CHECK (action IN ('delete_channel','suspend_user','change_role','reset_password','start_impersonation','permission_expired')),
  metadata        TEXT    NOT NULL DEFAULT '',
  created_at      INTEGER NOT NULL
)`).Error; err != nil {
				return err
			}
			if err := tx.Exec(`INSERT INTO admin_actions_ap21_new
				(id, actor_id, target_user_id, action, metadata, created_at)
				SELECT id, actor_id, target_user_id, action, metadata, created_at
				FROM admin_actions`).Error; err != nil {
				return err
			}
			if err := tx.Exec(`DROP TABLE admin_actions`).Error; err != nil {
				return err
			}
			if err := tx.Exec(`ALTER TABLE admin_actions_ap21_new RENAME TO admin_actions`).Error; err != nil {
				return err
			}
			// Recreate indexes (DROP TABLE drops them; CV-3.1 / CV-2 v2 同
			// 模式).
			if err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_admin_actions_target_user_id_created_at
				ON admin_actions(target_user_id, created_at DESC)`).Error; err != nil {
				return err
			}
			if err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_admin_actions_actor_id_created_at
				ON admin_actions(actor_id, created_at DESC)`).Error; err != nil {
				return err
			}
		}
		return nil
	},
}
