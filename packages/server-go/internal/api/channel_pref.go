// Package api — chn_8_notif_pref.go: CHN-8 channel notification preference
// REST endpoint.
//
// Blueprint: channel-model.md §3 layout per-user. Spec:
// docs/implementation/modules/chn-8-spec.md (战马D v0). 0 schema 改 —
// user_channel_layout.collapsed INTEGER bitmap 扩展:
//   - bit 0 (=1) = 折叠态 (CHN-3 既有)
//   - bit 1 (=2) = 静音态 (CHN-7, in-flight #550)
//   - bits 2-3 (mask 12 = 0b1100) = 通知偏好 3 态 (CHN-8 新增):
//       0 = NotifPrefAll (默认 / 现网行为零变)
//       1 = NotifPrefMention (仅 @mention 触发 push)
//       2 = NotifPrefNone (不发任何 push)
//       3 = reserved/invalid (反 SetNotifPref 入参 reject)
//
// 三向锁: server const + client lib/notif_pref.ts NOTIF_PREF_* + bitmap
// `(collapsed >> NotifPrefShift) & NotifPrefMask` 字面 byte-identical. 改
// 一处 = 改三处. 设计 ① + content-lock §3.
//
// 反约束 (chn-8-spec.md §0):
//   - 设计 ② owner-only — admin god-mode 不挂 PUT/POST.
//     owner-only ACL 锁链第 16 处 (CHN-7 #15 承袭).
//   - 设计 ③ 不 drop messages — CreateMessage / RT-3 fan-out / WS frame
//     全 byte-identical. notif pref 仅影响 DL-4 push notifier.
//   - 设计 ⑥ AST 锁链延伸第 13 处 forbidden 3 token 0 hit.
package api

import (
	"encoding/json"
	"net/http"
)

// NotifPrefShift / NotifPrefMask are byte-identical const that locate the
// 2-bit notification preference field in user_channel_layout.collapsed.
// 三向锁: 跟 packages/client/src/lib/notif_pref.ts 同字面.
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

// NotifPrefStrings maps API string ↔ int const. Single-source 跟
// content-lock §4 mapping table byte-identical.
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
// behind authMw. user-rail only (admin god-mode 不挂 ADM-0 §1.3 红线).
// 设计 ②.
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
// (设计 ① 不互扰).
func (h *ChannelHandler) handleSetNotificationPref(w http.ResponseWriter, r *http.Request) {
	channelID := r.PathValue("channelId")
	user, _, ok := requireChannelMember(w, r, h.Store, channelID, ChannelACLOpts{RejectDM: true})
	if !ok {
		return
	}
	var req chn8NotifPrefRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
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
