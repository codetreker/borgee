package migrations

import "gorm.io/gorm"

// channelMemberRequireMentionPolicy is migration v=52 for per-channel agent
// attention policy. It is idempotent because Store.Migrate creates the legacy
// baseline before running the forward-only registry in tests and dev boot.
var channelMemberRequireMentionPolicy = Migration{
	Version: 52,
	Name:    "channel_member_require_mention_policy",
	Up: func(tx *gorm.DB) error {
		exists, err := channelMemberPolicyColumnExists(tx)
		if err != nil {
			return err
		}
		if exists {
			return nil
		}
		return tx.Exec(`ALTER TABLE channel_members ADD COLUMN require_mention_policy TEXT NOT NULL DEFAULT 'inherit' CHECK (require_mention_policy IN ('inherit','on','off'))`).Error
	},
}

func channelMemberPolicyColumnExists(tx *gorm.DB) (bool, error) {
	rows, err := tx.Raw(`PRAGMA table_info(channel_members)`).Rows()
	if err != nil {
		return false, err
	}
	defer rows.Close()
	sawColumn := false
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull int
		var dflt any
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dflt, &pk); err != nil {
			return false, err
		}
		sawColumn = true
		if name == "require_mention_policy" {
			return true, nil
		}
	}
	if !sawColumn {
		return true, rows.Err()
	}
	return false, rows.Err()
}
