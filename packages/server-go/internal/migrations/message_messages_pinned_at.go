package migrations

import "gorm.io/gorm"

// messagesPinnedAt is migration v=45 — Phase 5+ / DM-10.1.
//
// Blueprint锚: dm-model.md §3 (per-user message layout, future v2 split).
// Spec: docs/implementation/modules/dm-10-spec.md §0 设计 ① + §1 拆段.
//
// What this migration does:
//
//   ALTER TABLE messages ADD COLUMN pinned_at INTEGER NULL
//
// pinned_at is Unix ms when an owner pins a DM message (DM channel
// scope only, ch.Type == "dm"); NULL = unpinned (default for old rows
// + new rows, 跟 DM-7.1 edit_history / AL-7.1 archived_at / AP-2.1
// revoked_at 跨八 milestone ALTER ADD COLUMN nullable 同模式).
//
// Pin/unpin endpoints (DM-10.2 server, 复用本列 SSOT):
//   - POST /api/v1/channels/{channelId}/messages/{messageId}/pin
//     → 立 pinned_at = now()
//   - DELETE /api/v1/channels/{channelId}/messages/{messageId}/pin
//     → 立 pinned_at = NULL
//   - GET /api/v1/channels/{channelId}/messages/pinned
//     → list pinned_at IS NOT NULL ORDER BY pinned_at DESC
//
// 反约束 (dm-10-spec.md §0 设计 ①+④):
//   - 不挂 NOT NULL — NULL = unpinned 是合法终态 (跟 DM-7.1 edit_history /
//     AL-7.1 archived_at 同精神).
//   - 不挂 default 值 — NULL 是合法终态, 现网零变.
//   - 不另起 pinned_messages 表 — pinned_at on messages 列单源 (grep 检查
//     `pinned_messages\|message_pin_log\|dm10_pin_table` 0 hit, 设计 ① 守).
//   - 不挂 pinned_by 列 — DM 双方都可 pin (DM-only scope), 设计 ② per-DM
//     pin (反 per-user pin 留 v2 跟 CHN-3.2 user_channel_layout 风格不同源).
//
// v=45 sequencing: chn-15 v=44 (待 ship) → DM-10.1 **v=45** (本 migration).
// registry.go 字面锁; team-lead 占号 reservation 跟 ADM-3 v=43 + chn-14
// v=36 + dm-7 v=34 + al-7 v=33 跨链承袭.
//
// v0 stance: forward-only, no Down().
var messagesPinnedAt = Migration{
	Version: 45,
	Name:    "dm_10_1_messages_pinned_at",
	Up: func(tx *gorm.DB) error {
		if exists, err := hasTable(tx, "messages"); err != nil {
			return err
		} else if !exists {
			return nil
		}
		// Idempotent guard 跟 AL-7.1 / HB-5.1 / DM-7.1 同模式.
		if has, err := hasColumn(tx, "messages", "pinned_at"); err != nil {
			return err
		} else if has {
			return nil
		}
		if err := tx.Exec(`ALTER TABLE messages ADD COLUMN pinned_at INTEGER`).Error; err != nil {
			return err
		}
		// Sparse partial index — pinned_at IS NOT NULL only (ORDER BY
		// pinned_at DESC list 热路径); 跟 AL-7.1 archived_at sparse idx
		// 同模式 (现网零开销).
		//
		// Skip index when channel_id column is absent (some test seed paths
		// create a minimal `messages (id TEXT PRIMARY KEY)` table — the
		// production path always has channel_id from migrations.go legacy
		// DDL). Forward-only stance: index is best-effort, queries still
		// work via WHERE clause without it.
		if has, err := hasColumn(tx, "messages", "channel_id"); err != nil {
			return err
		} else if !has {
			return nil
		}
		return tx.Exec(`CREATE INDEX IF NOT EXISTS idx_messages_pinned_at
			ON messages(channel_id, pinned_at DESC) WHERE pinned_at IS NOT NULL`).Error
	},
}
