// Package api — chn_8_notif_pref.go: CHN-8 channel notification preference
// REST endpoint.
//
// Blueprint: channel-model.md §3 layout per-user. Spec:
// docs/implementation/modules/chn-8-spec.md. No schema change: extend the
// user_channel_layout.collapsed INTEGER bitmap:
//   - bit 0 (=1) = collapsed state (CHN-3 existing)
//   - bit 1 (=2) = muted state (CHN-7, #550)
//   - bits 2-3 (mask 12 = 0b1100) = three notification preference states:
//     0 = NotifPrefAll (default / current behavior unchanged)
//     1 = NotifPrefMention (only @mention triggers push)
//     2 = NotifPrefNone (no push notifications)
//     3 = reserved/invalid (SetNotifPref rejects this input)
//
// Three-way invariant: server consts, client lib/notif_pref.ts NOTIF_PREF_*, and
// bitmap expression `(collapsed >> NotifPrefShift) & NotifPrefMask` must remain
// byte-identical. Changing one requires updating all three. Design ① +
// content-lock §3.
//
// Constraints (chn-8-spec.md §0):
//   - Design ② owner-only: no admin PUT/POST route is mounted. Owner-only ACL
//     reference site 16 follows CHN-7 #15.
//   - Design ③ notification preferences leave message creation and delivery
//     unchanged: CreateMessage, RT-3 fan-out, and WS frames stay byte-identical.
//     Notification preference only affects DL-4 push notifier behavior.
//   - Design ⑥ AST check site 13 requires zero matches for the three forbidden
//     tokens.
package api

import (
	"encoding/json"
	"net/http"
)

// NotifPrefShift / NotifPrefMask are byte-identical const that locate the
// 2-bit notification preference field in user_channel_layout.collapsed.
// Three-way invariant: match packages/client/src/lib/notif_pref.ts.
const (
	NotifPrefShift = 2
	NotifPrefMask  = 3
)

// NotifPrefAll / NotifPrefMention / NotifPrefNone are byte-identical
// const for the three notification preference states. Stored in
// collapsed bits 2-3.
const (
	NotifPrefAll     = 0
	NotifPrefMention = 1
	NotifPrefNone    = 2
)

// NotifPrefStrings maps API string ↔ int const. Single source for the mapping;
// byte-identical with the content-lock §4 table.
var notifPrefFromString = map[string]int64{
	"all":     NotifPrefAll,
	"mention": NotifPrefMention,
	"none":    NotifPrefNone,
}

// GetNotifPref reports the current notification preference encoded in
// collapsed bits 2-3. Single-source predicate.
func GetNotifPref(collapsed int64) int64 {
	return (collapsed >> NotifPrefShift) & NotifPrefMask
}

// RegisterCHN8Routes wires PUT /api/v1/channels/{channelId}/notification-pref
// behind authMw. User rail only; no admin route is mounted (ADM-0 §1.3).
// Design ②.
func (h *ChannelHandler) RegisterCHN8Routes(mux *http.ServeMux, authMw func(http.Handler) http.Handler) {
	mux.Handle("PUT /api/v1/channels/{channelId}/notification-pref",
		authMw(http.HandlerFunc(h.handleSetNotificationPref)))
}

type chn8NotifPrefRequest struct {
	Pref string `json:"pref"`
}

// handleSetNotificationPref — PUT /api/v1/channels/{channelId}/notification-pref.
//
// Sets bits 2-3 of user_channel_layout.collapsed for (user, channel).
// Other bits (CHN-3 collapsed bit 0, CHN-7 mute bit 1) are preserved
// (design ①: the bits do not interfere with each other).
func (h *ChannelHandler) handleSetNotificationPref(w http.ResponseWriter, r *http.Request) {
	channelID := r.PathValue("channelId")
	user, _, ok := requireChannelMember(w, r, h.Store, channelID, ChannelACLOpts{RejectDM: true})
	if !ok {
		return
	}
	var req chn8NotifPrefRequest
	capJSONBody(w, r)
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		if isBodyTooLarge(err) {
			writeJSONErrorCode(w, http.StatusRequestEntityTooLarge,
				"notification_pref.invalid_value", "request body too large")
			return
		}
		writeJSONErrorCode(w, http.StatusBadRequest,
			"notification_pref.invalid_value", "invalid JSON body")
		return
	}
	prefVal, ok := notifPrefFromString[req.Pref]
	if !ok {
		writeJSONErrorCode(w, http.StatusBadRequest,
			"notification_pref.invalid_value",
			"pref must be one of all|mention|none")
		return
	}
	collapsed, err := h.Store.SetNotifPrefBits(user.ID, channelID,
		int64(NotifPrefShift), int64(NotifPrefMask), prefVal)
	if err != nil {
		if h.Logger != nil {
			h.Logger.Error("chn8.set_notif_pref", "error", err, "pref", req.Pref)
		}
		writeJSONError(w, http.StatusInternalServerError, "Failed to update preference")
		return
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{
		"channel_id": channelID,
		"collapsed":  collapsed,
		"pref":       req.Pref,
	})
}
