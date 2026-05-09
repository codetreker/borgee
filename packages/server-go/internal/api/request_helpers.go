// Package api — request_helpers.go: REFACTOR-2 helper-2 SSOT for the
// JSON-decode → 400 boilerplate.
//
// 设计: 仅 unify callsites that already use the canonical
// `writeJSONError(w, 400, "Invalid JSON")` shape. Custom-code callers
// (agent_config / chn_8 / layout / host_grants / push_subscriptions /
// chn_10) keep their inline form because their reason codes are part of
// the public contract (反约束 §0 #1 reason code byte-identical).
//
// Caller list 锁 (canonical-shape callers only):
//   - auth.go (login/register/recover password — "Invalid JSON")
//   - messages.go (post message + reply — "Invalid JSON")
//   - dm_4_message_edit.go (edit body — "Invalid JSON")
//
// Reverse-grep 锚:
//   - `decodeJSON(` 单源 == 1 hit (本文件 func 定义)
//   - canonical-shape inline 位置 0 hit post-refactor

package api

import (
	"encoding/json"
	"net/http"
)

// decodeJSON decodes r.Body into v. On failure it writes the canonical
// 400 "Invalid JSON" response and returns false. Caller MUST early-return
// on false without writing.
//
// Byte-identical 跟既有:
//
//	if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
//	    writeJSONError(w, http.StatusBadRequest, "Invalid JSON")
//	    return
//	}
//
// **不收** custom-error-code callers (agent_config.invalid_payload /
// notification_pref.invalid_value / layout.invalid_payload /
// host_grants.invalid_payload / push.endpoint_invalid /
// chn_10 "invalid JSON body") — 那些 reason 字面是 public contract.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid JSON")
		return false
	}
	return true
}
