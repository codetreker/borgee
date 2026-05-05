package migrations

import "gorm.io/gorm"

// capabilitiesDotNotationBackfill is migration v=48 — CAPABILITY-DOT.
//
// Spec: docs/implementation/modules/capability-dot-spec.md §0.2 立场 ②.
// 蓝图: docs/blueprint/current/auth-permissions.md §1 字面 `<domain>.<verb>` 风格.
//
// What this migration does:
//   - Updates the existing `user_permissions.capability` TEXT column values
//     in-place from snake_case (read_channel/write_channel/...) to
//     dot-notation (channel.read/channel.write/...).
//   - 14 explicit per-token UPDATE statements (反 REPLACE 机械: snake_case
//     `verb_noun` 顺序 vs dot-notation `noun.verb` 顺序对调, 不能机械替换).
//   - Idempotent: each UPDATE only matches rows whose value still equals the
//     legacy literal; re-running this migration is a no-op (反复跑不破).
//
// Field invariants:
//   - 0 column rename / 0 column add / 0 schema shape change
//     (user_permissions.capability TEXT 字段名不动, 仅值改).
//   - 0 endpoint URL / 0 routes.go change.
//
// Cross-layer SSOT:
//   - `internal/auth/capabilities.go` ALL byte-identical 跟此 migration
//     UPDATE 集合 14 行字面 (改 = 改两处).
//   - `packages/client/src/lib/capabilities.ts::CAPABILITY_TOKENS`
//     byte-identical 跟此 migration UPDATE 集合.
//   - `docs/qa/ap-2-content-lock.md §1` byte-identical 跟此 migration.
var capabilitiesDotNotationBackfill = Migration{
	Version: 48,
	Name:    "capabilities_dot_notation_backfill",
	Up: func(tx *gorm.DB) error {
		// Probe: if user_permissions table or `capability` column missing
		// (minimal scaffolds in migration unit tests where seedLegacyTables
		// only creates `(id TEXT PRIMARY KEY)` placeholders), no-op.
		if !hasColumns(tx, "user_permissions", "capability") {
			return nil
		}
		// 14-row per-token literal map (verb_noun → noun.verb 顺序对调).
		mapping := []struct {
			Old string
			New string
		}{
			{"read_channel", "channel.read"},
			{"write_channel", "channel.write"},
			{"delete_channel", "channel.delete"},
			{"read_artifact", "artifact.read"},
			{"write_artifact", "artifact.write"},
			{"commit_artifact", "artifact.commit"},
			{"iterate_artifact", "artifact.iterate"},
			{"rollback_artifact", "artifact.rollback"},
			{"mention_user", "user.mention"},
			{"read_dm", "dm.read"},
			{"send_dm", "dm.send"},
			{"manage_members", "channel.manage_members"},
			{"invite_user", "channel.invite"},
			{"change_role", "channel.change_role"},
		}
		for _, m := range mapping {
			if err := tx.Exec(
				`UPDATE user_permissions SET capability = ? WHERE capability = ?`,
				m.New, m.Old,
			).Error; err != nil {
				return err
			}
		}
		return nil
	},
}
