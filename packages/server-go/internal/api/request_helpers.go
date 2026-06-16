// Package api — request_helpers.go: shared JSON request-body helpers.
//
// decodeJSON is the canonical JSON-decode → 400 "Invalid JSON" path.
// capJSONBody / isBodyTooLarge are the shared 1 MiB body cap (#1108 F7+SK2):
// every JSON r.Body decode site must cap the body so an unbounded
// json.NewDecoder buffer can't be driven to OOM by an unauthenticated
// multi-GB POST. capJSONBody + isBodyTooLarge let the inline custom-reason
// decoders add the 413 branch while preserving their public-contract 400
// strings byte-identically.
//
// Design: only unify call sites that already use the canonical
// `writeJSONError(w, 400, "Invalid JSON")` response shape. Callers with custom
// reason codes (agent_config / chn_8 / layout /
// push_subscriptions / chn_10) keep their inline form because those reason
// codes are part of the public contract and must remain byte-identical
// with their existing API responses.
//
// Caller list for the canonical response shape only:
//   - auth.go (login/register/recover password — "Invalid JSON")
//   - messages.go (post message + reply — "Invalid JSON")
//   - dm_4_message_edit.go (edit body — "Invalid JSON")
//
// Search checks after the refactor:
//   - `decodeJSON(` has exactly one definition in this file.
//   - `writeJSONError(w, http.StatusBadRequest, "Invalid JSON")` has zero
//     remaining inline call sites outside request_helpers.go.

package api

import (
	"encoding/json"
	"errors"
	"net/http"
)

// maxJSONBodyBytes caps every JSON request body decode at 1 MiB, mirroring
// the existing capped helpers (channels.readJSON / server.ReadJSON /
// upload/workspace multipart). Without this cap, an unbounded
// json.NewDecoder(r.Body).Decode buffers attacker-controlled bytes, so an
// unauthenticated multi-GB POST (register/login/admin-login) can exhaust
// memory (DoS — #1108 F7+SK2). The app layer is the only defense (no edge
// proxy / no client_max_body_size in the repo).
const maxJSONBodyBytes = 1 << 20 // 1 MiB

// capJSONBody wraps r.Body with an http.MaxBytesReader(1 MiB) so a too-large
// body fails the subsequent Decode with *http.MaxBytesError instead of being
// buffered without bound. Call this once before json.NewDecoder(r.Body).
func capJSONBody(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBodyBytes)
}

// isBodyTooLarge reports whether a Decode error is the MaxBytesReader limit
// being hit (→ caller responds 413 RequestEntityTooLarge) rather than a plain
// malformed-JSON error (→ caller keeps its existing 400 reason string).
func isBodyTooLarge(err error) bool {
	var mbe *http.MaxBytesError
	return errors.As(err, &mbe)
}

// decodeJSON decodes r.Body into v. The body is capped at 1 MiB; a too-large
// body writes 413 and returns false. On any other decode failure it writes the
// canonical 400 "Invalid JSON" response and returns false. Caller MUST
// early-return on false without writing.
//
// This is byte-identical with the previous inline pattern for the normal
// (≤1 MiB) path:
//
//	if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
//	    writeJSONError(w, http.StatusBadRequest, "Invalid JSON")
//	    return
//	}
//
// Do not use this helper for callers with custom error codes
// (agent_config.invalid_payload / notification_pref.invalid_value /
// layout.invalid_payload / push.endpoint_invalid /
// chn_10 "invalid JSON body"). Those reason strings are public contract.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	capJSONBody(w, r)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		if isBodyTooLarge(err) {
			writeJSONError(w, http.StatusRequestEntityTooLarge, "Request body too large")
			return false
		}
		writeJSONError(w, http.StatusBadRequest, "Invalid JSON")
		return false
	}
	return true
}
