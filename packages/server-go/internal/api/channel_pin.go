// Package api — chn_6_pin.go: CHN-6 channel pin/unpin REST endpoints.
//
// Blueprint: channel-model.md §3 layout per-user. Spec:
// docs/implementation/modules/chn-6-spec.md (战马D v0). 0 schema 改 —
// user_channel_layout 列复用 CHN-3.1 #410 既有, pin 状态走 position < 0
// 字面约定 + PinThreshold=0 双向锁 (server + client byte-identical).
//
// REFACTOR-1 R1.1: thin wrapper 模式跟 chn_7_mute.go / chn_15_readonly.go
// 对齐 — handlePinChannel / handleUnpinChannel ≤4 行 thin wrapper, 真活
// 走 handlePinToggle 单 handler + requireChannelMember helper-1 单一来源
// preamble (chn_6/7/8/15/layout 5 处 4-step preamble 单源).
//
// Public surface:
//   - (h *ChannelHandler) RegisterCHN6Routes(mux, authMw)
//
// 反向检查 (chn-6-spec.md §0 + refactor-1-spec.md §0):
//   - 设计 ② owner-only — POST/DELETE /api/v1/channels/{channelId}/pin
//     user-rail authMw 必经; admin god-mode 不挂 (grep 检查
//     `admin.*pin\|/admin-api/.*pin` 在 admin*.go 0 hit) — owner-only ACL
//     对齐链第 14 处.
//   - 设计 ③ pin 状态双源 — server PinThreshold=0 const + client
//     POSITION_PIN_THRESHOLD=0 byte-identical 双向锁.
//   - 设计 ⑥ AST 对齐链延伸第 11 处 forbidden 3 token 0 hit.
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
// 双向锁: 跟 packages/client/src/lib/pin.ts::POSITION_PIN_THRESHOLD
// byte-identical = 0. 改一处 = 改两处. 设计 ③ + content-lock §4.
const PinThreshold = 0.0

// IsPinned reports whether a user_channel_layout.position represents a
// pinned channel. Single-source predicate; 调用方禁止 inline 重写
// (grep 检查 `position\s*<\s*0` 在 production 仅命中此函数 + filter).
func IsPinned(position float64) bool {
	return position < PinThreshold
}

// RegisterCHN6Routes wires POST + DELETE /api/v1/channels/{channelId}/pin
// behind authMw. user-rail only (no admin god-mode 路径; ADM-0 §1.3 红
// 线 + CHN-3.2 设计沿用). 设计 ②.
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

// handlePinToggle — pin/unpin 单 handler, 跟 chn_7 handleMuteToggle /
// chn_15 handleReadonlyToggle 同模式 (REFACTOR-1 R1.1).
//
// 设计 ② owner-only: 走 requireChannelMember helper-1 (RejectDM=true +
// member-only) — DM-gate 字面 byte-identical 跟 CHN-3.2 / CHN-7 / CHN-8
// 同源.
func (h *ChannelHandler) handlePinToggle(w http.ResponseWriter, r *http.Request, pin bool) {
	channelID := r.PathValue("channelId")
	user, _, ok := requireChannelMember(w, r, h.Store, channelID, ChannelACLOpts{RejectDM: true})
	if !ok {
		return
	}
	nowMs := time.Now().UnixMilli()
	if pin {
		// position = -(nowMs) — ASC asc 排序使最近 pin 在最顶 (跟 CHN-3.3
		// #415 单调小数模式互补).
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
	// Unpin: position = max(positive)+1.0 (跟 CHN-3.3 #415 client MIN-1.0
	// 单调小数模式互补) so the channel returns to the non-pinned section.
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
