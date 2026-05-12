package migrations

import "gorm.io/gorm"

// messagesEditHistory is migration v=34 — Phase 6 / DM-7.1.
//
// Blueprint锚: dm-model.md §3 audit forward-only history. Spec brief:
// docs/implementation/modules/dm-7-spec.md §0 设计 ① + §1 拆段 DM-7.1.
//
// What this migration does (跟 AL-7.1 admin_actions ADD archived_at +
// HB-5.1 agent_state_log ADD archived_at + AP-1.1+AP-3.1+AP-2.1 across seven
// nullable ALTER ADD COLUMN migrations):
//
//	ALTER TABLE messages ADD COLUMN edit_history TEXT NULL
//
// edit_history is a JSON array of `{old_content, ts, reason}` entries
// appended each time UpdateMessage runs (DM-4 #553 既有 PATCH path 单源
// 不漂). NULL = no edits / 老消息行 byte-identical 不动 / 现网行为
// 零变.
//
// 反约束 (dm-7-spec.md §0 设计 ①+④):
//   - 不添加 NOT NULL — edit_history NULL = 无历史 (跟 AL-7.1 archived_at
//     NULL = active 同精神).
//   - 不添加 default 值 — NULL 是合法终态.
//   - 不另起 message_edit_history 表 — JSON array on messages 列单源
//     (grep 检查 `message_edit_history\|message_history_log\|dm7_history`
//     0 hit, 设计 ① 守).
//
// v=34 sequencing: AL-7.1 v=33 (#536 merged) → DM-7.1 **v=34** (本
// migration). registry.go 字面锁; 顺位.
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
		// Idempotent guard 跟 AL-7.1 / HB-5.1 同模式.
		if has, err := hasColumn(tx, "messages", "edit_history"); err != nil {
			return err
		} else if has {
			return nil
		}
		return tx.Exec(`ALTER TABLE messages ADD COLUMN edit_history TEXT`).Error
	},
}
