package migrations

import "gorm.io/gorm"

// helperAndHostGrantsRailDrop is migration v=54 — drops the helper rail
// (helper_jobs + helper_enrollments) and the host_grants rail (single
// host_grants table) at the tail of the registry.
//
// Forward-only contract: per registry.go / migrations.go top doc, historical
// migration source bodies (v=27 host_grants, v=49 helper_enrollments,
// v=50 helper_credential_rotation, v=51 helper_jobs, v=53
// helper_updates_available) stay in-tree unchanged so any DB whose
// schema_migrations table still records those versions has the matching
// Go body for replay. This new migration drops the three base tables
// in FK-safe order:
//
//  1. helper_jobs — FK enrollment_id → helper_enrollments(id); child first.
//  2. helper_enrollments — parent table; safe to drop after child.
//  3. host_grants — FK-independent; order does not matter for it.
//
// DROP TABLE IF EXISTS — idempotent + safe for environments that never
// ran any of the prior helper migrations (e.g. greenfield deploys after
// this lands). SQLite dialect (server-go 当前唯一 backend) —
// `migrations.go` engine `Exec` 透传 GORM dialector, 不需 dialect 分支.
var helperAndHostGrantsRailDrop = Migration{
	Version: 54,
	Name:    "drop_helper_and_host_grants_rails",
	Up: func(tx *gorm.DB) error {
		if err := tx.Exec(`DROP TABLE IF EXISTS helper_jobs`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`DROP TABLE IF EXISTS helper_enrollments`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`DROP TABLE IF EXISTS host_grants`).Error; err != nil {
			return err
		}
		return nil
	},
}
