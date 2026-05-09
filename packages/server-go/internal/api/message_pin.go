// Package api — dm_10_pin.go: DM-10 message pin/unpin REST endpoints.
//
// Blueprint锚: dm-model.md §3 future per-user message layout v2 (本 PR
// v0 = per-DM pin, 双方共享 pinned_at 列). Spec:
// docs/implementation/modules/dm-10-spec.md (战马E v0).
//
// Public surface:
//   - (h *MessagePinHandler) RegisterRoutes(mux, authMw)
//
// Endpoints (DM-only, channel.Type == "dm" 守):
//   POST   /api/v1/channels/{channelId}/messages/{messageId}/pin
//   DELETE /api/v1/channels/{channelId}/messages/{messageId}/pin
//   GET    /api/v1/channels/{channelId}/messages/pinned
//
// 设计 (跟 spec §0):
//   ① ALTER TABLE messages ADD pinned_at INTEGER NULL — schema 单源
//      (跟 DM-7.1 edit_history / AL-7.1 archived_at 跨九 milestone
//      ALTER ADD COLUMN nullable 同模式).
//   ② DM-only scope (channel.Type != "dm" → 400 `pin.dm_only_path`,
//      跟 dm_4_message_edit.go::handleEdit DM-only path 同精神).
//   ③ channel-member ACL gate (跟 AP-4 #551 reactions + AP-5 #555
//      messages 同 helper Store.IsChannelMember + Store.CanAccessChannel).
//   ④ POST 立 pinned_at = now() / DELETE 立 pinned_at = NULL / GET list
//      pinned_at IS NOT NULL ORDER BY pinned_at DESC.
//   ⑤ admin god-mode 不挂 — grep 检查 `admin.*pin.*messages` 0 hit
//      (ADM-0 §1.3 红线, 跟 DM-4/CV-7/AP-4/AP-5 owner-only 锁链承袭).
//
// 反约束:
//   - 不另起 pinned_messages 表 (pinned_at on messages 列单源)
//   - 不挂 pinned_by 列 (DM 双方都可 pin, per-DM scope)
//   - 不挂 pinned_reason / pin_note 列 (留 v2)
//   - 不开 admin /admin-api/.*messages.*pin 路径 (god-mode 不挂)

package api

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"borgee-server/internal/auth"
	"borgee-server/internal/store"
	"gorm.io/gorm"
)

// MessagePinHandler is the message pin/unpin REST endpoint dispatcher.
// Wires POST/DELETE/GET via RegisterRoutes (server.go boot).
type MessagePinHandler struct {
	Store  *store.Store
	Logger *slog.Logger
}

// RegisterRoutes wires DM-10 endpoints behind authMw.
// user-rail only; admin god-mode 不挂 (设计 ⑤ ADM-0 §1.3 红线).
func (h *MessagePinHandler) RegisterRoutes(mux *http.ServeMux, authMw func(http.Handler) http.Handler) {
	mux.Handle("POST /api/v1/channels/{channelId}/messages/{messageId}/pin",
		authMw(http.HandlerFunc(h.handlePin)))
	mux.Handle("DELETE /api/v1/channels/{channelId}/messages/{messageId}/pin",
		authMw(http.HandlerFunc(h.handleUnpin)))
	mux.Handle("GET /api/v1/channels/{channelId}/messages/pinned",
		authMw(http.HandlerFunc(h.handleListPinned)))
}

// gateDMScope validates auth + channel-member + DM-only path. Returns
// (channel, message-or-nil, ok). On false, response already written.
//
// Sequence (跟 dm_4_message_edit.go::handleEdit 同模式):
//   1. Auth (user-rail)
//   2. Channel exists + Type == "dm" (else 400 dm_only_path)
//   3. channel-member gate (Store.IsChannelMember + Store.CanAccessChannel,
//      跟 AP-4 #551 + AP-5 #555 同 helper) — fail-closed 404 "Channel not found"
func (h *MessagePinHandler) gateDM(w http.ResponseWriter, r *http.Request) (channelID string, user *store.User, ok bool) {
	user = auth.UserFromContext(r.Context())
	if user == nil {
		writeJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return "", nil, false
	}
	channelID = r.PathValue("channelId")
	if channelID == "" {
		writeJSONError(w, http.StatusBadRequest, "Channel ID required")
		return "", nil, false
	}
	ch, err := h.Store.GetChannelByID(channelID)
	if err != nil || ch == nil {
		writeJSONError(w, http.StatusNotFound, "Channel not found")
		return "", nil, false
	}
	if ch.Type != "dm" {
		// 设计 ② DM-only path — non-DM channel reject.
		writeJSONErrorCode(w, http.StatusBadRequest, "pin.dm_only_path",
			"Pin 仅 DM 路径")
		return "", nil, false
	}
	// 设计 ③ channel-member ACL gate (跟 AP-4 #551 + AP-5 #555 同 helper).
	if !h.Store.IsChannelMember(channelID, user.ID) ||
		!h.Store.CanAccessChannel(channelID, user.ID) {
		writeJSONError(w, http.StatusNotFound, "Channel not found")
		return "", nil, false
	}
	return channelID, user, true
}

// handlePin — POST /api/v1/channels/{channelId}/messages/{messageId}/pin.
// Stamps messages.pinned_at = now() for the (channel, message) pair.
// Idempotent — second call within the same instant overwrites pinned_at
// (last-write-wins, 跟 AL-7 sweeper UPDATE archived_at 同精神).
func (h *MessagePinHandler) handlePin(w http.ResponseWriter, r *http.Request) {
	channelID, _, ok := h.gateDM(w, r)
	if !ok {
		return
	}
	messageID := r.PathValue("messageId")
	if messageID == "" {
		writeJSONError(w, http.StatusBadRequest, "Message ID required")
		return
	}
	msg, err := h.Store.GetMessageByID(messageID)
	if err != nil || msg == nil {
		writeJSONError(w, http.StatusNotFound, "Message not found")
		return
	}
	// Cross-check: message belongs to the path channel (反 cross-channel pin).
	if msg.ChannelID != channelID {
		writeJSONError(w, http.StatusNotFound, "Message not found")
		return
	}
	if msg.DeletedAt != nil {
		writeJSONErrorCode(w, http.StatusBadRequest, "pin.message_deleted",
			"Cannot pin deleted message")
		return
	}
	nowMs := time.Now().UnixMilli()
	if err := h.Store.SetMessagePinnedAt(messageID, &nowMs); err != nil {
		if h.Logger != nil {
			h.Logger.Error("dm10.pin upsert", "error", err, "message_id", messageID)
		}
		writeJSONError(w, http.StatusInternalServerError, "Failed to pin message")
		return
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{
		"message_id": messageID,
		"pinned_at":  nowMs,
		"pinned":     true,
	})
}

// handleUnpin — DELETE /api/v1/channels/{channelId}/messages/{messageId}/pin.
// Stamps messages.pinned_at = NULL. Idempotent — unpinning an unpinned
// message returns 200 + pinned=false (反 fail-closed reject — nothing
// to undo).
func (h *MessagePinHandler) handleUnpin(w http.ResponseWriter, r *http.Request) {
	channelID, _, ok := h.gateDM(w, r)
	if !ok {
		return
	}
	messageID := r.PathValue("messageId")
	if messageID == "" {
		writeJSONError(w, http.StatusBadRequest, "Message ID required")
		return
	}
	msg, err := h.Store.GetMessageByID(messageID)
	if err != nil || msg == nil {
		writeJSONError(w, http.StatusNotFound, "Message not found")
		return
	}
	if msg.ChannelID != channelID {
		writeJSONError(w, http.StatusNotFound, "Message not found")
		return
	}
	if err := h.Store.SetMessagePinnedAt(messageID, nil); err != nil {
		if h.Logger != nil {
			h.Logger.Error("dm10.unpin upsert", "error", err, "message_id", messageID)
		}
		writeJSONError(w, http.StatusInternalServerError, "Failed to unpin message")
		return
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{
		"message_id": messageID,
		"pinned_at":  nil,
		"pinned":     false,
	})
}

// handleListPinned — GET /api/v1/channels/{channelId}/messages/pinned.
// Returns messages with pinned_at IS NOT NULL ORDER BY pinned_at DESC,
// scoped to the path channel. Empty list when no pinned messages
// (反向 fail-closed — gorm.ErrRecordNotFound 走空 list 同 CV-5 同精神).
func (h *MessagePinHandler) handleListPinned(w http.ResponseWriter, r *http.Request) {
	channelID, _, ok := h.gateDM(w, r)
	if !ok {
		return
	}
	msgs, err := h.Store.ListPinnedMessages(channelID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		if h.Logger != nil {
			h.Logger.Error("dm10.list_pinned", "error", err, "channel_id", channelID)
		}
		writeJSONError(w, http.StatusInternalServerError, "Failed to list pinned messages")
		return
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{
		"channel_id": channelID,
		"messages":   msgs,
	})
}
