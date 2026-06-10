// Package api — auth_helpers.go: REFACTOR-2 helper-1 SSOT for the
// 100+ "user == nil → 401" boilerplate sprinkled across handlers.
//
// 设计 ① + ② (refactor-2-spec.md §0):
//   - 行为不变量 byte-identical pre/post refactor: status code 401 +
//     reason "Unauthorized" + writeJSONError signature byte-identical
//     跟既有 100+ inline.
//   - SSOT 一次立, 反平行 helper (反 9th drift).
//
// Caller list 锁 (≥100 callsites across internal/api/*.go):
//   adm_2_2 / agent_invitations / agents / artifacts / artifact_comments /
//   anchors / auth / al_5_recover / al_1_4_state_log / bpp_8_lifecycle_list /
//   channels / channel_helpers (legacy) / chn_10_description /
//   chn_14_description_history / cv_15_comment_edit_history / commands /
//   agent_heartbeat_decay / layout / me_grants / poll / 余 ~30 文件
//
// Reverse-grep 锚 (refactor-2-spec.md §2):
//   - `mustUser(` 单源 == 1 hit (本文件 func 定义)
//   - `if user == nil {` body in handler files ≤5 hits (helper 内 + 极少
//     特殊路径 e.g. channel_helpers.go RequireCreator 变体)

package api

import (
	"net/http"

	"borgee-server/internal/auth"
	"borgee-server/internal/store"
)

// mustUser resolves the request's authenticated user via auth.UserFromContext.
// On nil it writes the canonical 401 "Unauthorized" response and returns
// (nil, false). Caller MUST early-return on false without writing.
//
// Byte-identical 跟既有 100+ inline:
//
//	user := auth.UserFromContext(r.Context())
//	if user == nil {
//	    writeJSONError(w, http.StatusUnauthorized, "Unauthorized")
//	    return
//	}
//
// 反约束: helper 不变 status code (401) / reason ("Unauthorized") /
// response shape (writeJSONError single literal). 任何漂会被 chn_3 /
// auth_test.go / poll_test.go 既有 unit + e2e 抓.
func mustUser(w http.ResponseWriter, r *http.Request) (*store.User, bool) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		writeJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return nil, false
	}
	return user, true
}
