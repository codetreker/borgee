// Package api — dm_7_edit_history.go: DM-7 GET edit history endpoints.
//
// Blueprint: dm-model.md §3 audit forward-only history. Spec:
// docs/implementation/modules/dm-7-spec.md (战马D v0). schema migration
// v=34 ALTER messages ADD edit_history TEXT NULL (跟 AL-7.1 + 跨七
// milestone ALTER ADD nullable 同模式).
//
// Public surface:
//   - MessageEditHistoryHandler{Store, Logger}
//   - (h *MessageEditHistoryHandler) RegisterUserRoutes(mux, authMw)
//   - (h *MessageEditHistoryHandler) RegisterAdminRoutes(mux, adminMw)
//
// 反约束 (dm-7-spec.md §0):
//   - 设计 ③ owner-only sender — user-rail GET 反向断言 sender ==
//     current user (别 user 403); admin readonly admin-rail GET (admin
//     god-mode 不挂 PATCH/DELETE — ADM-0 §1.3 红线).
//   - 设计 ⑥ AST 锁链延伸第 16 处 forbidden 3 token 0 hit.
package api

import (
	"log/slog"
	"net/http"

	"borgee-server/internal/admin"
	"borgee-server/internal/store"
)

// MessageEditHistoryHandler hosts the user-rail and admin-rail GET endpoints
// for message edit history. user-rail is sender-only; admin-rail is
// readonly only (no PATCH/DELETE — admin god-mode 不挂).
type MessageEditHistoryHandler struct {
	Store  *store.Store
	Logger *slog.Logger
}

// RegisterUserRoutes wires GET /api/v1/channels/{channelId}/messages/
// {messageId}/edit-history behind authMw. user-rail sender-only (设计 ③
// owner-only ACL 锁链第 19 处).
func (h *MessageEditHistoryHandler) RegisterUserRoutes(mux *http.ServeMux, authMw func(http.Handler) http.Handler) {
	mux.Handle("GET /api/v1/channels/{channelId}/messages/{messageId}/edit-history",
		authMw(http.HandlerFunc(h.handleUserGet)))
}

// RegisterAdminRoutes wires GET /admin-api/v1/messages/{messageId}/edit-history
// behind adminMw. admin readonly — no PATCH/DELETE on this path (反向
// grep 守门; admin god-mode ADM-0 §1.3 红线 — admin 看不能改).
func (h *MessageEditHistoryHandler) RegisterAdminRoutes(mux *http.ServeMux, adminMw func(http.Handler) http.Handler) {
	mux.Handle("GET /admin-api/v1/messages/{messageId}/edit-history",
		adminMw(http.HandlerFunc(h.handleAdminGet)))
}

// handleUserGet — GET /api/v1/channels/{channelId}/messages/{messageId}/edit-history.
//
// 设计 ③: sender ≠ current user → 403. 历史空时返 `[]`. HappyPath
// 返 JSON array (server-side store layer pre-normalized).
func (h *MessageEditHistoryHandler) handleUserGet(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}
	messageID := r.PathValue("messageId")
	msg, err := h.Store.GetMessageByID(messageID)
	if err != nil || msg == nil {
		writeJSONError(w, http.StatusNotFound, "Message not found")
		return
	}
	// 设计 ③: sender-only — 反向断言 sender == current user.
	if msg.SenderID != user.ID {
		writeJSONError(w, http.StatusForbidden, "Forbidden")
		return
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{
		"history": parseMessageEditHistory(msg.EditHistory),
	})
}

// handleAdminGet — GET /admin-api/v1/messages/{messageId}/edit-history.
//
// admin readonly. 设计 ③ 反约束: 不挂 PATCH/DELETE (grep 守门).
func (h *MessageEditHistoryHandler) handleAdminGet(w http.ResponseWriter, r *http.Request) {
	a := admin.AdminFromContext(r.Context())
	if a == nil {
		writeJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	messageID := r.PathValue("messageId")
	msg, err := h.Store.GetMessageByID(messageID)
	if err != nil || msg == nil {
		writeJSONError(w, http.StatusNotFound, "Message not found")
		return
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{
		"history": parseMessageEditHistory(msg.EditHistory),
	})
}
