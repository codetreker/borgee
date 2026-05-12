// Package api — chn_14_description_history.go: CHN-14 GET description
// edit history endpoints + zero-server-production grep check helper.
//
// Blueprint: channel-model.md §3 audit forward-only history. Spec:
// docs/implementation/modules/chn-14-spec.md. Schema migration v=44 adds
// `channels.description_edit_history TEXT NULL`, following the same nullable
// column pattern as DM-7.1 #558 and AL-7.1; CHN-14 is the eighth such site.
//
// Public surface:
//   - ChannelDescriptionHistoryHandler{Store, Logger}
//   - (h *ChannelDescriptionHistoryHandler) RegisterUserRoutes(mux, authMw)
//   - (h *ChannelDescriptionHistoryHandler) RegisterAdminRoutes(mux, adminMw)
//
// Reverse checks (chn-14-spec.md §0):
//   - Design ③ owner-only: user-rail GET requires caller == channel.CreatedBy
//     and returns 403 for members. Admin rail is readonly GET only; no
//     PATCH/DELETE route is mounted (ADM-0 §1.3).
//   - Design ⑥ AST check site 22 requires zero matches for the three forbidden
//     tokens.
package api

import (
	"log/slog"
	"net/http"

	"borgee-server/internal/admin"
	"borgee-server/internal/store"
)

// ChannelDescriptionHistoryHandler hosts the user-rail and admin-rail GET
// endpoints for channel description edit history. user-rail is owner-only;
// admin-rail is readonly only, with no PATCH/DELETE route mounted.
type ChannelDescriptionHistoryHandler struct {
	Store  *store.Store
	Logger *slog.Logger
}

// RegisterUserRoutes wires GET /api/v1/channels/{channelId}/description/history
// behind authMw. User rail is owner-only (design ②, owner-only ACL chain site 21).
func (h *ChannelDescriptionHistoryHandler) RegisterUserRoutes(mux *http.ServeMux, authMw func(http.Handler) http.Handler) {
	mux.Handle("GET /api/v1/channels/{channelId}/description/history",
		authMw(http.HandlerFunc(h.handleUserGet)))
}

// RegisterAdminRoutes wires GET /admin-api/v1/channels/{channelId}/description/history
// behind adminMw. Admin rail is readonly; no PATCH/DELETE route is mounted on
// this path (ADM-0 §1.3: admin can inspect but not modify).
func (h *ChannelDescriptionHistoryHandler) RegisterAdminRoutes(mux *http.ServeMux, adminMw func(http.Handler) http.Handler) {
	mux.Handle("GET /admin-api/v1/channels/{channelId}/description/history",
		adminMw(http.HandlerFunc(h.handleAdminGet)))
}

// handleUserGet — GET /api/v1/channels/{channelId}/description/history.
//
// Design ②: caller != channel.CreatedBy returns 403. Empty history returns `[]`.
// Happy path returns a JSON array pre-normalized by the store layer.
func (h *ChannelDescriptionHistoryHandler) handleUserGet(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}
	channelID := r.PathValue("channelId")
	ch, err := h.Store.GetChannelByID(channelID)
	if err != nil || ch == nil {
		writeJSONError(w, http.StatusNotFound, "Channel not found")
		return
	}
	// Design ② owner-only: require channel.CreatedBy == user.ID, matching CHN-10
	// #20 + DM-7 #19 owner-only ACL chain site 21.
	if ch.CreatedBy != user.ID {
		writeJSONError(w, http.StatusForbidden, "Only the channel owner can view edit history")
		return
	}
	history, err := h.Store.GetChannelDescriptionHistory(channelID)
	if err != nil {
		if h.Logger != nil {
			h.Logger.Error("chn14.history user", "error", err, "channel_id", channelID)
		}
		writeJSONError(w, http.StatusInternalServerError, "Failed to load history")
		return
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{
		"history": history,
	})
}

// handleAdminGet — GET /admin-api/v1/channels/{channelId}/description/history.
//
// Admin rail is readonly. Design ② requires no PATCH/DELETE route.
func (h *ChannelDescriptionHistoryHandler) handleAdminGet(w http.ResponseWriter, r *http.Request) {
	a := admin.AdminFromContext(r.Context())
	if a == nil {
		writeJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	channelID := r.PathValue("channelId")
	ch, err := h.Store.GetChannelByID(channelID)
	if err != nil || ch == nil {
		writeJSONError(w, http.StatusNotFound, "Channel not found")
		return
	}
	history, err := h.Store.GetChannelDescriptionHistory(channelID)
	if err != nil {
		if h.Logger != nil {
			h.Logger.Error("chn14.history admin", "error", err, "channel_id", channelID)
		}
		writeJSONError(w, http.StatusInternalServerError, "Failed to load history")
		return
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{
		"history": history,
	})
}
