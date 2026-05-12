package migrations

import "gorm.io/gorm"

// messagesEditHistory is migration v=34 — Phase 6 / DM-7.1.
//
// Blueprint reference: dm-model.md §3 audit forward-only history. Spec brief:
// docs/implementation/modules/dm-7-spec.md §0 design point 1 + §1 DM-7.1.
//
// What this migration does (same nullable ALTER ADD COLUMN pattern as AL-7.1
// admin_actions ADD archived_at,
// HB-5.1 agent_state_log ADD archived_at + AP-1.1+AP-3.1+AP-2.1 across seven
// nullable ALTER ADD COLUMN migrations):
//
//	ALTER TABLE messages ADD COLUMN edit_history TEXT NULL
//
// edit_history is a JSON array of `{old_content, ts, reason}` entries
// appended each time UpdateMessage runs; the existing DM-4 #553 PATCH path is
// the single source. NULL = no edits; legacy message rows stay byte-identical;
// current behavior is unchanged.
//
// Constraints (dm-7-spec.md §0 design points 1 and 4):
//   - Do not add NOT NULL: edit_history NULL = no history, matching AL-7.1
//     archived_at NULL = active.
//   - Do not add a default: NULL is the valid terminal value.
//   - Do not create a message_edit_history table: the JSON array on messages is
//     the single source. Grep reference
//     `message_edit_history\|message_history_log\|dm7_history` stays at zero hits.
//
// v=34 sequencing: AL-7.1 v=33 (#536 merged) → DM-7.1 **v=34** (本
// migration). registry.go pins the literal version and ordering.
//
// v0 stance: forward-only, no Down().
var messagesEditHistory = Migration{
	Version: 34,
	Name:    "dm_7_1_messages_edit_history",
	Up: func(tx *gorm.DB) error {
		if exists, err := hasTable(tx, "messages"); err != nil {
			return err
		} else if !exists {
			return nil
		}
		// Idempotent guard matching AL-7.1 / HB-5.1.
		if has, err := hasColumn(tx, "messages", "edit_history"); err != nil {
			return err
		} else if has {
			return nil
		}
		return tx.Exec(`ALTER TABLE messages ADD COLUMN edit_history TEXT`).Error
	},
}
