// Package api — chn_15_readonly.go: CHN-15 channel readonly toggle REST
// endpoints + IsReadonly predicate.
//
// Blueprint: channel-model.md §3 layout per-user (extension) + §1.4
// owner 主权. Spec: docs/implementation/modules/chn-15-spec.md (战马C v0).
//
// Behaviour: 0 schema 改 — readonly 状态走 channel.created_by 的
// user_channel_layout.collapsed bitmap bit 4 单行 SSOT. 跟 CHN-7 #550
// bit 1 mute 同模式但 channel-wide 而非 per-user — 仅 creator 单行决定
// channel 全局 readonly state.
//
// Bit map (collapsed INTEGER 字面约定):
//   - bit 0 (=1)  = 折叠态 (CHN-3 既有)
//   - bit 1 (=2)  = 静音态 (CHN-7 既有)
//   - bits 2-3    = notification preference (CHN-8 既有)
//   - bit 4 (=16) = readonly 频道 (CHN-15 新增, channel-wide 走 creator 单行)
//
// 反约束 (chn-15-spec.md §0):
//   - 设计 ① 0 schema — bit 4 in collapsed; 不另起 channels.readonly /
//     channel_readonly_states 列/表.
//   - 设计 ② owner-only — PUT/DELETE 仅 channel.CreatedBy == user.ID;
//     admin god-mode 不挂 (ADM-0 §1.3 红线). owner-only ACL 锁链第 21 处.
//   - 设计 ③ readonly 时 non-creator POST messages → 403
//     `channel.readonly_no_send` 字面 byte-identical 跟 content-lock §3.
//   - 设计 ⑤ IsReadonly + GetChannelReadonly + SetChannelReadonly 单源.
package api

import (
	"net/http"
)

// ReadonlyBit is the byte-identical const that flags a readonly channel
// in user_channel_layout.collapsed (bit 4) on the **creator's** row.
//
// 双向锁: 跟 packages/client/src/lib/readonly.ts::READONLY_BIT
// byte-identical = 16. 改一处 = 改两处.
const ReadonlyBit = 16

// ChannelErrCodeReadonlyNoSend is the byte-identical error code
// returned to non-creator senders when a channel is readonly. Const so
// both server and client (CHANNEL_READONLY_TOAST map) lock to the same
// literal (改 = 改三处: 此 const + client toast + content-lock §3).
const ChannelErrCodeReadonlyNoSend = "channel.readonly_no_send"

// IsReadonly reports whether a user_channel_layout.collapsed bitmap
// value represents a readonly channel. Single-source predicate; 调用方
// 禁止 inline 重写 (grep 检查 `collapsed\s*&\s*16` 在 production 仅
// 命中此函数).
func IsReadonly(collapsed int64) bool {
	return collapsed&int64(ReadonlyBit) != 0
}

// RegisterCHN15Routes wires PUT + DELETE /api/v1/channels/{channelId}/readonly
// behind authMw. user-rail only (admin god-mode 不挂 ADM-0 §1.3 红线).
// 设计 ②.
func (h *ChannelHandler) RegisterCHN15Routes(mux *http.ServeMux, authMw func(http.Handler) http.Handler) {
	mux.Handle("PUT /api/v1/channels/{channelId}/readonly",
		authMw(http.HandlerFunc(h.handleSetReadonly)))
	mux.Handle("DELETE /api/v1/channels/{channelId}/readonly",
		authMw(http.HandlerFunc(h.handleUnsetReadonly)))
}

// handleSetReadonly — PUT /api/v1/channels/{channelId}/readonly.
//
// Sets bit 4 on channel.CreatedBy's user_channel_layout.collapsed row.
// 设计 ② owner-only — 仅 channel.CreatedBy == user.ID.
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
