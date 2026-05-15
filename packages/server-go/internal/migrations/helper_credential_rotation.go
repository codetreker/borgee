package migrations

import "gorm.io/gorm"

// helperCredentialRotation is migration v=50 for Helper credential lifecycle
// metadata. It keeps one active credential digest and records rotation metadata
// without adding job execution state.
var helperCredentialRotation = Migration{
	Version: 50,
	Name:    "helper_credential_rotation_metadata",
	Up: func(tx *gorm.DB) error {
		if err := tx.Exec(`ALTER TABLE helper_enrollments ADD COLUMN credential_rotated_at INTEGER`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`ALTER TABLE helper_enrollments ADD COLUMN credential_generation INTEGER NOT NULL DEFAULT 1`).Error; err != nil {
			return err
		}
		return nil
	},
}
