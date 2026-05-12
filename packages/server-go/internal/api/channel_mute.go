// Package api — chn_7_mute.go: CHN-7 channel mute/unmute REST endpoints.
//
// Blueprint: channel-model.md §3 layout per-user. Spec:
// docs/implementation/modules/chn-7-spec.md. No schema change:
// user_channel_layout reuses the existing CHN-3.1 #410 column, and mute state
// is encoded in the collapsed INTEGER bitmap:
//   - bit 0 (=1) = collapsed state (CHN-3 existing)
//   - bit 1 (=2) = muted state (CHN-7)
// MuteBit=2 const 双向锁跟 client lib/mute.ts::MUTE_BIT byte-identical.
//
// Constraints (chn-7-spec.md §0):
//   - Design ① no schema change: use the collapsed bitmap; do not add
//     muted/muted_until columns.
//   - Design ② owner-only: POST/DELETE are per-user; no admin route is mounted.
//     Owner-only ACL reference site 15, following CHN-6 #14.
//   - Design ③ muting leaves message creation and delivery unchanged:
//     CreateMessage, RT-3 fan-out, and WS frames remain byte-identical. Mute only
//     affects DL-4 push notifier skips.
//   - Design ⑥ AST check site 12 requires zero matches for the three forbidden
//     tokens.
package api

import (
	"net/http"
)

// MuteBit is the byte-identical const that flags a muted channel in
// user_channel_layout.collapsed (bit 1). bit 0 is reserved for the
// existing CHN-3 collapsed state, so legacy clients writing
// collapsed=0/1 keep their behavior (bit 1 defaults to 0 = unmuted).
//
// Cross-layer invariant: byte-identical with
// packages/client/src/lib/mute.ts::MUTE_BIT (=2). Changing this requires
// updating both sides. Design ③ + content-lock §4 follows the CHN-6
// PinThreshold pattern.
const MuteBit = 2

// IsMuted reports whether a user_channel_layout.collapsed bitmap value
// represents a muted channel. Single-source predicate; callers must not inline
// this bit check. Grep checks require `collapsed\s*&\s*2` to match only this
// function in production code.
func IsMuted(collapsed int64) bool {
	return collapsed&int64(MuteBit) != 0
}

// RegisterCHN7Routes wires POST + DELETE /api/v1/channels/{channelId}/mute
// behind authMw. User rail only; no admin route is mounted (ADM-0 §1.3,
// following CHN-3.2 design). Design ②.
func (h *ChannelHandler) RegisterCHN7Routes(mux *http.ServeMux, authMw func(http.Handler) http.Handler) {
	mux.Handle("POST /api/v1/channels/{channelId}/mute",
		authMw(http.HandlerFunc(h.handleMuteChannel)))
	mux.Handle("DELETE /api/v1/channels/{channelId}/mute",
		authMw(http.HandlerFunc(h.handleUnmuteChannel)))
}

// handleMuteChannel — POST /api/v1/channels/{channelId}/mute.
//
// Sets bit 1 of user_channel_layout.collapsed for (user, channel)
// preserving bit 0 (CHN-3 collapsed state). Design ② owner-only uses
// IsChannelMember, and the DM reject path stays byte-identical with CHN-3.2 /
// CHN-6.
func (h *ChannelHandler) handleMuteChannel(w http.ResponseWriter, r *http.Request) {
	h.handleMuteToggle(w, r, true)
}

// handleUnmuteChannel — DELETE /api/v1/channels/{channelId}/mute.
//
// Clears bit 1 of user_channel_layout.collapsed; idempotent.
func (h *ChannelHandler) handleUnmuteChannel(w http.ResponseWriter, r *http.Request) {
	h.handleMuteToggle(w, r, false)
}

func (h *ChannelHandler) handleMuteToggle(w http.ResponseWriter, r *http.Request, muted bool) {
	channelID := r.PathValue("channelId")
	user, _, ok := requireChannelMember(w, r, h.Store, channelID, ChannelACLOpts{RejectDM: true})
	if !ok {
		return
	}
	collapsed, err := h.Store.SetMuteBit(user.ID, channelID, int64(MuteBit), muted)
	if err != nil {
		if h.Logger != nil {
			h.Logger.Error("chn7.mute toggle", "error", err, "muted", muted)
		}
		writeJSONError(w, http.StatusInternalServerError, "Failed to update mute state")
		return
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{
		"channel_id": channelID,
		"collapsed":  collapsed,
		"muted":      muted,
	})
}
