package migrations

import "gorm.io/gorm"

// userPermissionsExpires is migration v=24 — Phase 4 / AP-1.1.
//
// Blueprint reference: `auth-permissions.md` §1.2 (v1 has three scope levels:
// `*`, `channel:<id>`, and `artifact:<id>`; the expires_at column is reserved
// in schema and unused by UI) + §5 gap from current behavior ("expires_at column
// — add the column without breaking schema; do not productize yet"). Spec stance
// inherits docs/blueprint/current/auth-permissions.md §1.2.
//
// What this migration does:
//  1. ALTER TABLE user_permissions ADD COLUMN expires_at INTEGER NULL
//     (Unix ms; NULL = permanent, preserving current ABAC behavior).
//  2. CREATE INDEX idx_user_permissions_expires ON user_permissions(
//     expires_at) WHERE expires_at IS NOT NULL — sparse index scans only rows
//     with an expiry for the future sweeper path; v1 does not consume it.
//
// Constraints (auth-permissions.md §1.2 + design stance "v1 reserves schema,
// UI does not use it"):
//   - Do not add NOT NULL: current rows keep expires_at NULL = permanent, so
//     blueprint §1.1 "ABAC source of truth" behavior stays unchanged.
//   - Do not add a default: NULL is the valid terminal value. A default of 0
//     would look expired to a future sweeper and could revoke permanent rows.
//   - Do not add CHECK (expires_at > granted_at): schema records the field, and
//     future v2+ server paths perform business validation.
//   - Keep INDEX WHERE expires_at IS NOT NULL as a partial index with no current
//     runtime cost; leave the primary key and idx_user_permissions_lookup unchanged.
//
// v=24 sequencing: AL-1b.1 v=21 / ADM-2.1 v=22 / ADM-2.2 v=23 / **AP-1.1
// v=24** (this migration). registry.go pins the literal version.
//
// v0 stance: forward-only, no Down(). ALTER ADD COLUMN 在 SQLite 是
// idempotent-unsafe (rerunning reports duplicate column), so the engine uses
// schema_migrations versioning for idempotency, matching other ALTER migrations
// such as chn_3_1 and cm_3.
var userPermissionsExpires = Migration{
	Version: 24,
	Name:    "ap_1_1_user_permissions_expires",
	Up: func(tx *gorm.DB) error {
		// ALTER ADD COLUMN — SQLite supports this without table rebuild
		// when no constraint is added. NULL default + nullable = no behavior change.
		if err := tx.Exec(`ALTER TABLE user_permissions ADD COLUMN expires_at INTEGER`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_user_permissions_expires
			ON user_permissions(expires_at) WHERE expires_at IS NOT NULL`).Error; err != nil {
			return err
		}
		return nil
	},
}
