package migrations

import "gorm.io/gorm"

// userPermissionsOrg is migration v=29 — Phase 5 / AP-3.1.
//
// Blueprint reference: `auth-permissions.md` §1.2 (v1 has three scope levels)
// + §5 gap from current behavior ("cross-org enforcement — AP-3 later
// milestone"). Spec brief: docs/implementation/modules/ap-3-spec.md (v0,
// d69b617) §0 design point 2 + §1 AP-3.1.
//
// What this migration does:
//  1. ALTER TABLE user_permissions ADD COLUMN org_id TEXT NULL
//     (same nullable ALTER ADD COLUMN pattern as ap_1_1 #493 expires_at).
//     NULL = legacy row, preserving AP-1 current behavior; any NULL keeps the
//     legacy path. Explicit org_id rows carry AP-3 cross-org owner-only enforcement.
//  2. CREATE INDEX idx_user_permissions_org_id ON user_permissions(org_id)
//     WHERE org_id IS NOT NULL — sparse index scans only explicit org_id rows,
//     matching the ap_1_1 expires_at sparse-index pattern with no current cost.
//
// Constraints (auth-permissions.md §5 + ap-3-spec.md §0 design point 2):
//   - Do not add NOT NULL: current rows keep org_id NULL = legacy, preserving
//     AP-1 ABAC behavior.
//   - Do not add a default: NULL is the valid terminal value; 0 / "" are not
//     valid org_id values.
//   - Do not add FK org_id REFERENCES organizations(id): this matches
//     user.org_id, channels.org_id, and messages.org_id (CM-3 #208). Server
//     paths perform business validation; grep reference `user_permissions
//     .*FOREIGN KEY.*organizations` stays at zero hits.
//   - Keep INDEX WHERE org_id IS NOT NULL as a partial index with no current
//     runtime cost; leave the primary key, idx_user_permissions_lookup, and
//     idx_user_permissions_expires unchanged.
//
// v=29 sequencing: AP-1.1 v=24 / AL-1.4 v=25 / DL-4.1 v=26 / HB-3.1 v=27 /
// CV-2 v2 v=28 (in flight #517) / **AP-3.1 v=29** (本 migration). registry.go
// literal. CV-2 v2 / AP-3 were concurrent; merge order determines the version,
// and later work moves forward, matching the CV-2 v1 spec §2 v=14 sequencing
// agreement.
//
// v0 stance: forward-only, no Down(). ALTER ADD COLUMN 在 SQLite
// idempotent-unsafe (rerunning reports duplicate column), so the engine uses
// schema_migrations versioning for idempotency, matching other ALTER migrations
// such as chn_3_1, cm_3, and ap_1_1.
var userPermissionsOrg = Migration{
	Version: 29,
	Name:    "ap_3_1_user_permissions_org",
	Up: func(tx *gorm.DB) error {
		// Trimmed-schema gate, matching ap_1_1 / cv_3_1. Some migration tests
		// register this migration without the upstream user_permissions table.
		exists, err := hasTable(tx, "user_permissions")
		if err != nil {
			return err
		}
		if !exists {
			return nil
		}

		// ALTER ADD COLUMN — SQLite supports this without table rebuild
		// when no constraint is added. NULL default + nullable = no behavior change.
		if err := tx.Exec(`ALTER TABLE user_permissions ADD COLUMN org_id TEXT`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_user_permissions_org_id
			ON user_permissions(org_id) WHERE org_id IS NOT NULL`).Error; err != nil {
			return err
		}
		return nil
	},
}
