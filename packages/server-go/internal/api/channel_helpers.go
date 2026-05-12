// Package api — channel_helpers.go: REFACTOR-1 helper-1 single-source 4-step
// channel ACL preamble (auth → load channel → DM gate → member/creator).
//
// Designs ① + ② (refactor-1-spec.md §0):
//   - Behavior invariant is byte-identical before and after refactor: status
//     code, error reason code, DM-gate literals (`"DM 不参与个人分组"` /
//     `layout.dm_not_grouped`), and Forbidden / Unauthorized text match the
//     existing 5 sites (chn_6 / chn_7 / chn_8 / chn_15 / layout per-row).
//   - The helper is the single source for the 5 observed option variants:
//     opts.RejectDM + opts.RequireCreator cover chn_15 creator-only and the
//     other 4 RejectDM+IsChannelMember paths.
//
// Tracked callers (grep check for mismatch):
//   - chn_6_pin.go (pin/unpin: RejectDM=true) — this helper owns the DM-gate
//     literal "DM 不参与个人分组" and the `layout.dm_not_grouped` code
//   - chn_7_mute.go (mute toggle: RejectDM=true) — same DM-gate literal/code
//   - chn_8_notif_pref.go (notif pref: RejectDM=true) — same DM-gate literal/code
//   - chn_15_readonly.go (readonly toggle: RequireCreator=true)
//   - layout.go (per-row PUT: RejectDM=true) — same DM-gate literal/code
//
// Reverse-grep references (refactor-1-spec.md §2):
//   - DM-gate literals are owned by this helper, keeping total grep count stable
//     (constraint #2)
//   - Existing chn_6/7/8/15 + layout test literals stay unchanged and pass
//     (constraint #3)

package api

import (
	"net/http"

	"borgee-server/internal/auth"
	"borgee-server/internal/store"
)

// ChannelACLOpts toggles the two real variants observed across the 5
// caller files.
type ChannelACLOpts struct {
	// RejectDM — when true (the chn_6/7/8/layout 4 sites), a channel.Type == "dm"
	// returns 400 with code `layout.dm_not_grouped` + msg `"DM 不参与个人分组"`
	// byte-identical with chn-3 content-lock §1 ④ + REG-CHN3-002 5 sources.
	RejectDM bool
	// RequireCreator — when true (chn_15 only), the membership check is
	// replaced by `ch.CreatedBy == user.ID`. The DM-gate is not engaged
	// (chn_15 readonly toggle does not gate DM, since CreatedBy already
	// implies a non-DM channel by the CHN-15 rule).
	RequireCreator bool
}

// requireChannelMember runs the 4-step preamble (auth → load channel →
// DM gate → member/creator) and returns the resolved user + channel on
// success. On any failure path the helper writes the response (4xx) and
// returns (nil, nil, false) — caller MUST early-return without writing.
//
// Design ① behavior invariant: status, error, DM-gate, and Forbidden literals
// stay byte-identical with the original 5 inline preambles. Grep constraints #2
// and #3 cover this.
func requireChannelMember(
	w http.ResponseWriter,
	r *http.Request,
	s *store.Store,
	channelID string,
	opts ChannelACLOpts,
) (*store.User, *store.Channel, bool) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		writeJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return nil, nil, false
	}
	ch, err := s.GetChannelByID(channelID)
	if err != nil || ch == nil {
		writeJSONError(w, http.StatusNotFound, "Channel not found")
		return nil, nil, false
	}
	// DM gate: byte-identical with chn-3 content-lock §1 ④ and chn-6/7/8 design ②.
	// chn_15 (RequireCreator) does not use this gate because creator implies
	// non-DM under design ②.
	if opts.RejectDM && ch.Type == "dm" {
		writeJSONErrorCode(w, http.StatusBadRequest, "layout.dm_not_grouped",
			"DM 不参与个人分组")
		return nil, nil, false
	}
	if opts.RequireCreator {
		if ch.CreatedBy != user.ID {
			writeJSONError(w, http.StatusForbidden, "Forbidden")
			return nil, nil, false
		}
	} else {
		if !s.IsChannelMember(channelID, user.ID) {
			writeJSONError(w, http.StatusForbidden, "Forbidden")
			return nil, nil, false
		}
	}
	return user, ch, true
}
