package migrations

import "gorm.io/gorm"

// channelsDescriptionEditHistory is migration v=44 — Phase 6 / CHN-14.1.
//
// Blueprint锚: channel-model.md §3 audit forward-only history. Spec brief:
// docs/implementation/modules/chn-14-spec.md §0 设计 ① + §1 拆段 CHN-14.1.
//
// What this migration does (跟 DM-7.1 messages.edit_history #558 +
// AL-7.1 admin_actions ADD archived_at + HB-5.1 agent_state_log ADD
// archived_at + AP-1.1+AP-3.1+AP-2.1 跨七 milestone ALTER ADD COLUMN
// nullable 同模式; CHN-14 是第八处):
//
//   ALTER TABLE channels ADD COLUMN description_edit_history TEXT NULL
//
// description_edit_history is a JSON array of `{old_content, ts, reason}`
// entries appended each time UpdateChannelDescription runs (CHN-10 #561
// owner-only PUT path 单源不漂; CHN-2 #406 既有 PUT /topic member-level
// path 不挂, 仅 CHN-10 owner-only path 包装写入). NULL = no edits / 老
// channel 行 byte-identical 不动 / 现网行为零变.
//
// 反约束 (chn-14-spec.md §0 设计 ①+④):
//   - 不挂 NOT NULL — description_edit_history NULL = 无历史 (跟 DM-7.1
//     edit_history NULL = 无编辑 + AL-7.1 archived_at NULL = active 同精神).
//   - 不挂 default 值 — NULL 是合法终态.
//   - 不另起 channel_description_history 表 — JSON array on channels 列
//     单源 (grep 检查 `channel_description_history\|channel_history_log\|
//     chn14_history` 0 hit, 设计 ① 守).
//
// v=44 sequencing: HB-5.1 v=35 (#540 merged) → CHN-14.1 **v=44** (本
// migration). registry.go 字面锁; 顺位.
//
// v0 stance: forward-only, no Down().
var channelsDescriptionEditHistory = Migration{
	Version: 44,
	Name:    "chn_14_1_channels_description_edit_history",
	Up: func(tx *gorm.DB) error {
		if exists, err := hasTable(tx, "channels"); err != nil {
			return err
		} else if !exists {
			return nil
		}
		// Idempotent guard 跟 DM-7.1 / AL-7.1 / HB-5.1 同模式.
		if has, err := hasColumn(tx, "channels", "description_edit_history"); err != nil {
			return err
		} else if has {
			return nil
		}
		return tx.Exec(`ALTER TABLE channels ADD COLUMN description_edit_history TEXT`).Error
	},
}
