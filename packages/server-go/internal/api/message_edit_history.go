// Package api — message_edit_history.go: REFACTOR-1 helper-2 SSOT
// edit-history JSON parser shared between DM-7 / CV-15.
//
// Designs ① + ② (refactor-1-spec.md §0):
//   - Byte-identical behavior invariant: NULL/empty returns []map[string]any{},
//     not nil; Unmarshal failure also returns []map[string]any{}. This preserves
//     the prior 11-line behavior shared by dm_7 and cv_15 and keeps REG-DM7 /
//     REG-CV15 intact.
//
// Tracked callers:
//   - dm_7_edit_history.go (handleUserGet + handleAdminGet use history)
//   - cv_15_comment_edit_history.go (handleUserGet + handleAdminGet)
//
// Reverse-grep references (refactor-1-spec.md §2 constraint #5):
//   - func parseEditHistoryEntries / parseCommentEditHistory 0 hit (merged)
//   - func parseMessageEditHistory ==1 hit (this file)

package api

import "encoding/json"

// parseMessageEditHistory decodes the stored JSON edit-history array,
// returning an empty slice if NULL/empty or on Unmarshal failure (so the
// client always sees `[]`, not `null`). Single-source for DM-7 + CV-15
// (REFACTOR-1 helper-2 SSOT).
func parseMessageEditHistory(raw *string) []map[string]any {
	if raw == nil || *raw == "" {
		return []map[string]any{}
	}
	var arr []map[string]any
	if err := json.Unmarshal([]byte(*raw), &arr); err != nil {
		return []map[string]any{}
	}
	return arr
}
