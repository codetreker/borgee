// Package api — channel_helpers.go: REFACTOR-1 helper-1 SSOT 4-step
// channel ACL preamble (auth → load channel → DM gate → member/creator).
//
// 设计 ① + ② (refactor-1-spec.md §0):
//   - 行为不变量 byte-identical pre/post refactor: helper 内 status code +
//     error reason code 字面 + DM-gate 字面 (`"DM 不参与个人分组"` /
//     `layout.dm_not_grouped`) + Forbidden / Unauthorized 字面 byte-
//     identical 跟既有 5 处 (chn_6 / chn_7 / chn_8 / chn_15 / layout per-row).
//   - 4 helper SSOT 单源 — opts.RejectDM + opts.RequireCreator 携带 5 处
//     真值变体 (chn_15 creator-only / 其他 4 处 RejectDM+IsChannelMember).
//
// Caller list 锁 (grep 检查 不漂):
//   - chn_6_pin.go (pin/unpin: RejectDM=true) — DM-gate 字面 "DM 不参与个人分组" 由本 helper 承载, 错码 `layout.dm_not_grouped` 同源
//   - chn_7_mute.go (mute toggle: RejectDM=true) — DM-gate 字面 "DM 不参与个人分组" 由本 helper 承载, 错码 `layout.dm_not_grouped` 同源
//   - chn_8_notif_pref.go (notif pref: RejectDM=true) — DM-gate 字面 由本 helper 承载, 错码 `layout.dm_not_grouped` 同源
//   - chn_15_readonly.go (readonly toggle: RequireCreator=true)
//   - layout.go (per-row PUT: RejectDM=true) — DM-gate 字面 由本 helper 承载
//
// Reverse-grep 锚 (refactor-1-spec.md §2):
//   - DM-gate 字面承载在 helper 内, 总 grep count 不变 (反约束 #2)
//   - 既有 chn_6/7/8/15 + layout test 字面不动, 全 PASS (反约束 #3)

package api

import (
	"net/http"

	"borgee-server/internal/auth"
	"borgee-server/internal/store"
)

// ChannelACLOpts toggles the two real variants observed across the 5
// caller files.
type ChannelACLOpts struct {
	// RejectDM — when true (the chn_6/7/8/layout 4 处), a channel.Type == "dm"
	// returns 400 with code `layout.dm_not_grouped` + msg `"DM 不参与个人分组"`
	// byte-identical 跟 chn-3 content-lock §1 ④ + REG-CHN3-002 5 源.
	RejectDM bool
	// RequireCreator — when true (chn_15 only), the membership check is
	// replaced by `ch.CreatedBy == user.ID`. The DM-gate is not engaged
	// (chn_15 readonly toggle does not gate DM, since CreatedBy already
	// implies a non-DM channel by CHN-15 条原则).
	RequireCreator bool
}

// requireChannelMember runs the 4-step preamble (auth → load channel →
// DM gate → member/creator) and returns the resolved user + channel on
// success. On any failure path the helper writes the response (4xx) and
// returns (nil, nil, false) — caller MUST early-return without writing.
//
// 设计 ① 行为不变量: status / error / DM-gate / Forbidden 字面 byte-
// identical 跟原 5 处 inline preamble (反约束 grep #2 + #3 守门).
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
	// DM gate — 跟 chn-3 content-lock §1 ④ / chn-6/7/8 设计 ② 字面 byte-identical.
	// chn_15 (RequireCreator) 不挂此 gate (creator 隐含非 DM, 设计 ②).
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
