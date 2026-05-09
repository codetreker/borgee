// Package api — chn_7_mute.go: CHN-7 channel mute/unmute REST endpoints.
//
// Blueprint: channel-model.md §3 layout per-user. Spec:
// docs/implementation/modules/chn-7-spec.md (战马D v0). 0 schema 改 —
// user_channel_layout 列复用 CHN-3.1 #410 既有, mute 状态走 collapsed
// INTEGER bitmap 字面约定:
//   - bit 0 (=1) = 折叠态 (CHN-3 既有)
//   - bit 1 (=2) = 静音态 (CHN-7 新增)
// MuteBit=2 const 双向锁跟 client lib/mute.ts::MUTE_BIT byte-identical.
//
// 反约束 (chn-7-spec.md §0):
//   - 设计 ① 0 schema — collapsed bitmap, 不另起 muted/muted_until 列.
//   - 设计 ② owner-only — POST/DELETE per-user; admin god-mode 不挂.
//     owner-only ACL 锁链第 15 处 (CHN-6 #14 承袭).
//   - 设计 ③ mute 不 drop messages — CreateMessage/RT-3 fan-out/WS frame
//     全 byte-identical 不动. mute 仅 DL-4 push notifier skip (设计 ③).
//   - 设计 ⑥ AST 锁链延伸第 12 处 forbidden 3 token 0 hit.
package api

import (
	"net/http"
)

// MuteBit is the byte-identical const that flags a muted channel in
// user_channel_layout.collapsed (bit 1). bit 0 is reserved for the
// existing CHN-3 collapsed state, so legacy clients writing
// collapsed=0/1 keep their behavior (bit 1 defaults to 0 = unmuted).
//
// 双向锁: 跟 packages/client/src/lib/mute.ts::MUTE_BIT byte-identical
// = 2. 改一处 = 改两处. 设计 ③ + content-lock §4 (跟 CHN-6 PinThreshold
// 双向锁模式承袭).
const MuteBit = 2

// IsMuted reports whether a user_channel_layout.collapsed bitmap value
// represents a muted channel. Single-source predicate; 调用方禁止 inline
// 重写 (grep 检查 `collapsed\s*&\s*2` 在 production 仅命中此函数).
func IsMuted(collapsed int64) bool {
	return collapsed&int64(MuteBit) != 0
}

// RegisterCHN7Routes wires POST + DELETE /api/v1/channels/{channelId}/mute
// behind authMw. user-rail only (admin god-mode 不挂 ADM-0 §1.3 红线 +
// CHN-3.2 设计沿用). 设计 ②.
func (h *ChannelHandler) RegisterCHN7Routes(mux *http.ServeMux, authMw func(http.Handler) http.Handler) {
	mux.Handle("POST /api/v1/channels/{channelId}/mute",
		authMw(http.HandlerFunc(h.handleMuteChannel)))
	mux.Handle("DELETE /api/v1/channels/{channelId}/mute",
		authMw(http.HandlerFunc(h.handleUnmuteChannel)))
}

// handleMuteChannel — POST /api/v1/channels/{channelId}/mute.
//
// Sets bit 1 of user_channel_layout.collapsed for (user, channel)
// preserving bit 0 (CHN-3 collapsed state). 设计 ② owner-only (cm.user_id
// 走 IsChannelMember + DM reject byte-identical 跟 CHN-3.2 / CHN-6 同源).
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
