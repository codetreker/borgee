// Package api — chn_6_pin.go: CHN-6 channel pin/unpin REST endpoints.
//
// Blueprint: channel-model.md §3 layout per-user. Spec:
// docs/implementation/modules/chn-6-spec.md. No schema change: reuse the
// existing CHN-3.1 #410 user_channel_layout column. Pin state uses the
// position < 0 literal contract plus PinThreshold=0 two-way lock (server +
// client byte-identical).
//
// REFACTOR-1 R1.1: thin wrapper pattern aligns with chn_7_mute.go /
// chn_15_readonly.go. handlePinChannel / handleUnpinChannel remain ≤4-line
// wrappers, while real work goes through handlePinToggle plus the
// requireChannelMember helper-1 single-source preamble shared by the
// chn_6/7/8/15/layout 4-step paths.
//
// Public surface:
//   - (h *ChannelHandler) RegisterCHN6Routes(mux, authMw)
//
// Reverse checks (chn-6-spec.md §0 + refactor-1-spec.md §0):
//   - Design ② owner-only: POST/DELETE /api/v1/channels/{channelId}/pin must go
//     through user-rail authMw; no admin route is mounted. Grep checks require
//     zero `admin.*pin\|/admin-api/.*pin` matches in admin*.go. Owner-only ACL
//     chain site 14.
//   - Design ③ pin state two-way lock: server PinThreshold=0 const and client
//     POSITION_PIN_THRESHOLD=0 remain byte-identical.
//   - Design ⑥ AST check site 11 requires zero matches for the three forbidden
//     tokens.
package api

import (
	"net/http"
	"time"
)

// PinThreshold is the byte-identical const that segregates pinned vs
// non-pinned channels in user_channel_layout.position. Channels with
// `position < PinThreshold` are pinned (server stamps `-(nowMs)` so
// ASC ordering naturally surfaces them at the top of the sidebar).
//
// Cross-layer invariant: byte-identical with
// packages/client/src/lib/pin.ts::POSITION_PIN_THRESHOLD (=0). Changing this
// requires updating both sides. Design ③ + content-lock §4.
const PinThreshold = 0.0

// IsPinned reports whether a user_channel_layout.position represents a
// pinned channel. Single-source predicate; callers must not inline this check.
// Grep checks require `position\s*<\s*0` to match only this function + filter in
// production code.
func IsPinned(position float64) bool {
	return position < PinThreshold
}

// RegisterCHN6Routes wires POST + DELETE /api/v1/channels/{channelId}/pin
// behind authMw. User rail only; no admin route is mounted (ADM-0 §1.3,
// following CHN-3.2 design). Design ②.
func (h *ChannelHandler) RegisterCHN6Routes(mux *http.ServeMux, authMw func(http.Handler) http.Handler) {
	mux.Handle("POST /api/v1/channels/{channelId}/pin",
		authMw(http.HandlerFunc(h.handlePinChannel)))
	mux.Handle("DELETE /api/v1/channels/{channelId}/pin",
		authMw(http.HandlerFunc(h.handleUnpinChannel)))
}

// handlePinChannel — POST /api/v1/channels/{channelId}/pin (thin wrapper).
func (h *ChannelHandler) handlePinChannel(w http.ResponseWriter, r *http.Request) {
	h.handlePinToggle(w, r, true)
}

// handleUnpinChannel — DELETE /api/v1/channels/{channelId}/pin (thin wrapper).
func (h *ChannelHandler) handleUnpinChannel(w http.ResponseWriter, r *http.Request) {
	h.handlePinToggle(w, r, false)
}

// handlePinToggle — single pin/unpin handler, same pattern as chn_7
// handleMuteToggle and chn_15 handleReadonlyToggle (REFACTOR-1 R1.1).
//
// Design ② owner-only: use requireChannelMember helper-1 (RejectDM=true +
// member-only). The DM-gate literal stays byte-identical with CHN-3.2 / CHN-7 /
// CHN-8.
func (h *ChannelHandler) handlePinToggle(w http.ResponseWriter, r *http.Request, pin bool) {
	channelID := r.PathValue("channelId")
	user, _, ok := requireChannelMember(w, r, h.Store, channelID, ChannelACLOpts{RejectDM: true})
	if !ok {
		return
	}
	nowMs := time.Now().UnixMilli()
	if pin {
		// position = -(nowMs): ASC ordering puts the most recent pin first,
		// complementing the CHN-3.3 #415 monotonic fractional-position pattern.
		position := -float64(nowMs)
		if err := h.Store.PinChannelLayout(user.ID, channelID, position, nowMs); err != nil {
			if h.Logger != nil {
				h.Logger.Error("chn6.pin upsert", "error", err)
			}
			writeJSONError(w, http.StatusInternalServerError, "Failed to pin channel")
			return
		}
		writeJSONResponse(w, http.StatusOK, map[string]any{
			"channel_id": channelID,
			"position":   position,
			"pinned":     true,
		})
		return
	}
	// Unpin: position = max(positive)+1.0, complementing the CHN-3.3 #415 client
	// MIN-1.0 fractional-position pattern so the channel returns to the non-pinned section.
	// Idempotent — second call within the same instant returns 200 + position > 0.
	position, err := h.Store.UnpinChannelLayout(user.ID, channelID, nowMs)
	if err != nil {
		if h.Logger != nil {
			h.Logger.Error("chn6.unpin upsert", "error", err)
		}
		writeJSONError(w, http.StatusInternalServerError, "Failed to unpin channel")
		return
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{
		"channel_id": channelID,
		"position":   position,
		"pinned":     false,
	})
}
