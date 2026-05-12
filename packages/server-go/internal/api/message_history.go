// Package api — dm_7_edit_history.go: DM-7 GET edit history endpoints.
//
// Blueprint: dm-model.md §3 audit forward-only history. Spec:
// docs/implementation/modules/dm-7-spec.md. Schema migration v=34 adds
// `messages.edit_history TEXT NULL`, following the same nullable-column pattern
// used by AL-7.1 and related milestones.
//
// Public surface:
//   - MessageEditHistoryHandler{Store, Logger}
//   - (h *MessageEditHistoryHandler) RegisterUserRoutes(mux, authMw)
//   - (h *MessageEditHistoryHandler) RegisterAdminRoutes(mux, adminMw)
//
// Constraints (dm-7-spec.md §0):
//   - Design ③ sender-only access: user-rail GET requires sender == current
//     user; other users receive 403. Admin rail is readonly GET only, with no
//     PATCH/DELETE route mounted (ADM-0 §1.3).
//   - Design ⑥ AST check chain site 16 requires zero matches for the three
//     forbidden tokens.
package api

import (
	"log/slog"
	"net/http"

	"borgee-server/internal/admin"
	"borgee-server/internal/store"
)

// MessageEditHistoryHandler hosts the user-rail and admin-rail GET endpoints
// for message edit history. user-rail is sender-only; admin-rail is
// readonly only with no PATCH/DELETE route.
type MessageEditHistoryHandler struct {
	Store  *store.Store
	Logger *slog.Logger
}

// RegisterUserRoutes wires GET /api/v1/channels/{channelId}/messages/
// {messageId}/edit-history behind authMw. User rail is sender-only (design ③,
// owner-only ACL chain site 19).
func (h *MessageEditHistoryHandler) RegisterUserRoutes(mux *http.ServeMux, authMw func(http.Handler) http.Handler) {
	mux.Handle("GET /api/v1/channels/{channelId}/messages/{messageId}/edit-history",
		authMw(http.HandlerFunc(h.handleUserGet)))
}

// RegisterAdminRoutes wires GET /admin-api/v1/messages/{messageId}/edit-history
// behind adminMw. Admin rail is readonly; no PATCH/DELETE route is mounted on
// this path (ADM-0 §1.3: admin can inspect but not modify).
func (h *MessageEditHistoryHandler) RegisterAdminRoutes(mux *http.ServeMux, adminMw func(http.Handler) http.Handler) {
	mux.Handle("GET /admin-api/v1/messages/{messageId}/edit-history",
		adminMw(http.HandlerFunc(h.handleAdminGet)))
}

// handleUserGet — GET /api/v1/channels/{channelId}/messages/{messageId}/edit-history.
//
// Design ③: sender != current user returns 403. Empty history returns `[]`.
// Happy path returns a JSON array pre-normalized by the store layer.
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
	// Design ③: sender-only, requiring sender == current user.
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
// Admin rail is readonly. Design ③ requires no PATCH/DELETE route.
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
