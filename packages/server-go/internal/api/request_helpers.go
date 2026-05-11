// Package api — request_helpers.go: REFACTOR-2 helper for the shared
// JSON-decode → 400 response path.
//
// Design: only unify call sites that already use the canonical
// `writeJSONError(w, 400, "Invalid JSON")` response shape. Callers with custom
// reason codes (agent_config / chn_8 / layout / host_grants /
// push_subscriptions / chn_10) keep their inline form because those reason
// codes are part of the public contract and must remain byte-identical
// (non-goal §0 #1).
//
// Caller list for the canonical response shape only:
//   - auth.go (login/register/recover password — "Invalid JSON")
//   - messages.go (post message + reply — "Invalid JSON")
//   - dm_4_message_edit.go (edit body — "Invalid JSON")
//
// Search checks after the refactor:
//   - `decodeJSON(` has exactly one definition in this file.
//   - the canonical response shape has zero remaining inline call sites.

package api

import (
	"encoding/json"
	"net/http"
)

// decodeJSON decodes r.Body into v. On failure it writes the canonical
// 400 "Invalid JSON" response and returns false. Caller MUST early-return
// on false without writing.
//
// This is byte-identical with the previous inline pattern:
//
//	if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
//	    writeJSONError(w, http.StatusBadRequest, "Invalid JSON")
//	    return
//	}
//
// Do not use this helper for callers with custom error codes
// (agent_config.invalid_payload / notification_pref.invalid_value /
// layout.invalid_payload / host_grants.invalid_payload / push.endpoint_invalid /
// chn_10 "invalid JSON body"). Those reason strings are public contract.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid JSON")
		return false
	}
	return true
}
