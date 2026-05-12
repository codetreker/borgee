// Package api — chn_15_readonly.go: CHN-15 channel readonly toggle REST
// endpoints + IsReadonly predicate.
//
// Blueprint: channel-model.md §3 layout per-user (extension) + §1.4
// owner control. Spec: docs/implementation/modules/chn-15-spec.md.
//
// Behaviour: no schema change. The readonly state uses bit 4 of the
// channel.created_by user's single user_channel_layout.collapsed row. This
// follows the CHN-7 #550 bit 1 mute pattern, but readonly is channel-wide rather
// than per-user: only the creator row controls the channel's global readonly
// state.
//
// Bit map (collapsed INTEGER literal contract):
//   - bit 0 (=1)  = collapsed state (CHN-3 existing)
//   - bit 1 (=2)  = muted state (CHN-7 existing)
//   - bits 2-3    = notification preference (CHN-8 existing)
//   - bit 4 (=16) = readonly channel (CHN-15, channel-wide via creator row)
//
// Constraints (chn-15-spec.md §0):
//   - Design ① no schema change: bit 4 in collapsed; do not add a
//     channels.readonly column or channel_readonly_states table.
//   - Design ② owner-only: PUT/DELETE require channel.CreatedBy == user.ID; no
//     admin route is mounted (ADM-0 §1.3). Owner-only ACL reference site 21.
//   - Design ③ readonly non-creator POST messages → 403 with
//     `channel.readonly_no_send`, byte-identical with content-lock §3.
//   - Design ⑤ IsReadonly + GetChannelReadonly + SetChannelReadonly are the
//     single source for readonly state handling.
package api

import (
	"net/http"
)

// ReadonlyBit is the byte-identical const that flags a readonly channel
// in user_channel_layout.collapsed (bit 4) on the **creator's** row.
//
// Cross-layer invariant: byte-identical with
// packages/client/src/lib/readonly.ts::READONLY_BIT (=16).
const ReadonlyBit = 16

// ChannelErrCodeReadonlyNoSend is the byte-identical error code
// returned to non-creator senders when a channel is readonly. Const so
// both server and client (CHANNEL_READONLY_TOAST map) use the same literal.
// Changing it requires updating this const, the client toast, and content-lock §3.
const ChannelErrCodeReadonlyNoSend = "channel.readonly_no_send"

// IsReadonly reports whether a user_channel_layout.collapsed bitmap
// value represents a readonly channel. Single-source predicate; callers must not
// inline this bit check. Grep checks require `collapsed\s*&\s*16` to match only
// this function in production code.
func IsReadonly(collapsed int64) bool {
	return collapsed&int64(ReadonlyBit) != 0
}

// RegisterCHN15Routes wires PUT + DELETE /api/v1/channels/{channelId}/readonly
// behind authMw. User rail only; no admin route is mounted (design ②,
// ADM-0 §1.3).
func (h *ChannelHandler) RegisterCHN15Routes(mux *http.ServeMux, authMw func(http.Handler) http.Handler) {
	mux.Handle("PUT /api/v1/channels/{channelId}/readonly",
		authMw(http.HandlerFunc(h.handleSetReadonly)))
	mux.Handle("DELETE /api/v1/channels/{channelId}/readonly",
		authMw(http.HandlerFunc(h.handleUnsetReadonly)))
}

// handleSetReadonly — PUT /api/v1/channels/{channelId}/readonly.
//
// Sets bit 4 on channel.CreatedBy's user_channel_layout.collapsed row.
// Design ② owner-only: require channel.CreatedBy == user.ID.
func (h *ChannelHandler) handleSetReadonly(w http.ResponseWriter, r *http.Request) {
	h.handleReadonlyToggle(w, r, true)
}

// handleUnsetReadonly — DELETE /api/v1/channels/{channelId}/readonly.
// Clears bit 4 of creator's collapsed row; idempotent.
func (h *ChannelHandler) handleUnsetReadonly(w http.ResponseWriter, r *http.Request) {
	h.handleReadonlyToggle(w, r, false)
}

func (h *ChannelHandler) handleReadonlyToggle(w http.ResponseWriter, r *http.Request, readonly bool) {
	channelID := r.PathValue("channelId")
	_, ch, ok := requireChannelMember(w, r, h.Store, channelID, ChannelACLOpts{RequireCreator: true})
	if !ok {
		return
	}
	collapsed, err := h.Store.SetMuteBit(ch.CreatedBy, channelID, int64(ReadonlyBit), readonly)
	if err != nil {
		if h.Logger != nil {
			h.Logger.Error("chn15.readonly toggle", "error", err, "readonly", readonly)
		}
		writeJSONError(w, http.StatusInternalServerError, "Failed to update readonly state")
		return
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{
		"channel_id": channelID,
		"collapsed":  collapsed,
		"readonly":   readonly,
	})
}
