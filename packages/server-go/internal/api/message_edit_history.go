// Package api — message_edit_history.go: REFACTOR-1 helper-2 SSOT
// edit-history JSON parser shared between DM-7 / CV-15.
//
// 设计 ① + ② (refactor-1-spec.md §0):
//   - byte-identical 行为不变量: NULL/empty → []map[string]any{} (反 nil) +
//     Unmarshal 失败 → []map[string]any{} (跟 dm_7 / cv_15 既有 11 行 byte-
//     identical, REG-DM7 / REG-CV15 不破).
//
// Caller list 锁:
//   - dm_7_edit_history.go (handleUserGet + handleAdminGet 用 history)
//   - cv_15_comment_edit_history.go (handleUserGet + handleAdminGet)
//
// Reverse-grep 锚 (refactor-1-spec.md §2 反约束 #5):
//   - func parseEditHistoryEntries / parseCommentEditHistory 0 hit (合一)
//   - func parseMessageEditHistory ==1 hit (此文件)

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
