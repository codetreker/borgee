package migrations

import "gorm.io/gorm"

// bisMessageRead is migration v=8 — AP-0-bis backfill.
//
// Blueprint: docs/blueprint/current/auth-permissions.md §3 (Messaging capability list);
// R3 决议 #1 (2026-04-28): agent default capability set 锁 [message.send,
// message.read]. New agents pick this up via store.GrantDefaultPermissions
// (queries.go); existing agents in the wild predate the row, so this migration
// adds it idempotently.
//
// Version: 8
//   - 6 reserved for ADM-0.3 (users.role enum 收 + admin backfill + session revoke)
//   - 7 reserved for CM-onboarding (Welcome channel infra)
//   - 8 = AP-0-bis (this migration)
//
// What this migration does:
//   - For every row in users where role='agent' AND deleted_at IS NULL, ensure
//     a (user_id, 'message.read', '*') row exists in user_permissions.
//   - INSERT … SELECT WHERE NOT EXISTS — idempotent, no double-grants on rerun.
//   - granted_at = strftime millis (sqlite-portable) at apply time.
//
// v0 stance: forward-only, no Down(). The "delete db and rebuild" contract in
// migrations.go applies (line 15: "There is no Down()"). Acceptance template
// row "migration down 干净回滚, 不残留 message.read" is satisfied vacuously
// since v0 has no down — see PR description for the contract reference.
//
// Idempotency: the WHERE NOT EXISTS guard is the source of truth. Even if this
// migration is replayed against a DB whose schema_migrations was wiped, no
// duplicate (user_id, permission, scope) rows are inserted.
var bisMessageRead = Migration{
	Version: 8,
	Name:    "ap_0_bis_message_read",
	Up: func(tx *gorm.DB) error {
		// SQLite epoch-ms portable: strftime('%s','now') returns seconds; we
		// multiply to ms to match GrantedAt's int64 ms scale used in queries.go.
		const sql = `
INSERT INTO user_permissions (user_id, permission, scope, granted_at)
SELECT u.id, 'message.read', '*', CAST(strftime('%s','now') AS INTEGER) * 1000
  FROM users u
 WHERE u.role = 'agent'
   AND u.deleted_at IS NULL
   AND NOT EXISTS (
     SELECT 1 FROM user_permissions p
      WHERE p.user_id = u.id
        AND p.permission = 'message.read'
        AND p.scope = '*'
   )`
		return tx.Exec(sql).Error
	},
}
