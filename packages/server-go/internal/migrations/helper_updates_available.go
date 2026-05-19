package migrations

import "gorm.io/gorm"

// helperUpdatesAvailable is migration v=53 — adds the #999 update-detection
// snapshot columns to helper_enrollments. The helper POSTs its installed
// versions; the server computes drift vs the current signed manifest and
// stores the latest snapshot here. Both columns are nullable so pre-#999
// rows continue to serialize without backfill (UI shows "never checked"
// when LastUpdateCheckAt is NULL).
//
// Blueprint锚: docs/blueprint/current/host-bridge.md §1.3 "更新策略: 分类,
// 不自动" — 自动更新仍是反模式; this PR records detection state only, apply
// is a user-confirmed follow-up.
var helperUpdatesAvailable = Migration{
	Version: 53,
	Name:    "helper_updates_available_detection",
	Up: func(tx *gorm.DB) error {
		if err := tx.Exec(`ALTER TABLE helper_enrollments ADD COLUMN updates_available_json TEXT`).Error; err != nil {
			return err
		}
		if err := tx.Exec(`ALTER TABLE helper_enrollments ADD COLUMN last_update_check_at INTEGER`).Error; err != nil {
			return err
		}
		return nil
	},
}
