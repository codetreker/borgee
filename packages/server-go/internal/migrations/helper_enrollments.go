package migrations

import "gorm.io/gorm"

// helperEnrollments is migration v=49 for the Helper enrollment/status
// foundation. It creates a distinct Helper authority rail, separate from
// remote_nodes, host_grants, and user_permissions.
var helperEnrollments = Migration{
	Version: 49,
	Name:    "helper_enrollment_status_foundation",
	Up: func(tx *gorm.DB) error {
		if err := tx.Exec(`CREATE TABLE IF NOT EXISTS helper_enrollments (
  id                           TEXT    PRIMARY KEY,
  owner_user_id                TEXT    NOT NULL,
  org_id                       TEXT    NOT NULL,
  host_label                   TEXT    NOT NULL,
  helper_device_id             TEXT,
  allowed_categories           TEXT    NOT NULL,
  status                       TEXT    NOT NULL CHECK (status IN ('pending','connected','offline','revoked','uninstalled')),
  last_seen_at                 INTEGER,
  created_at                   INTEGER NOT NULL,
  updated_at                   INTEGER NOT NULL,
  claimed_at                   INTEGER,
  revoked_at                   INTEGER,
  uninstalled_at               INTEGER,
  enrollment_secret_digest     TEXT,
  enrollment_secret_expires_at INTEGER,
  persistent_credential_digest TEXT,
  credential_created_at        INTEGER
)`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_helper_enrollments_owner_org
			ON helper_enrollments(owner_user_id, org_id, status)`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_helper_enrollments_device
			ON helper_enrollments(helper_device_id) WHERE helper_device_id IS NOT NULL`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_helper_enrollments_last_seen
			ON helper_enrollments(last_seen_at)`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`CREATE INDEX IF NOT EXISTS idx_helper_enrollments_secret_expiry
			ON helper_enrollments(enrollment_secret_expires_at) WHERE enrollment_secret_digest IS NOT NULL`).Error; err != nil {
			return err
		}
		return nil
	},
}
