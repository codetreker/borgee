// Package api — chn_10_description.go: CHN-10 owner-only PUT /channels/:id
// /description endpoint.
//
// Blueprint: docs/implementation/modules/chn-10-spec.md §0+§1+§2.
//
// Public surface:
//   - ChannelDescriptionHandler{Store, Logger}
//   - (h *ChannelDescriptionHandler) RegisterUserRoutes(mux, authMw)
//   - DescriptionMaxLength (= 500, byte-identical 跟 channels.topic GORM
//     size:500 and client DESCRIPTION_MAX_LENGTH).
//
// Constraints (chn-10-spec.md §0 designs ②③ boundary ⑥):
//   - Owner-only ACL reference site 20, following DM-7 #19 + CHN-9 #14: handler
//     requires channel.CreatedBy == user.ID and rejects member-level writes. The
//     existing CHN-2 #406 PUT /topic path remains byte-identical.
//   - No admin route is mounted: there is no RegisterAdminRoutes, and grep checks
//     require zero PATCH/PUT/POST/DELETE matches for
//     `admin-api/v[0-9]+/.*description` (ADM-0 §1.3).
//   - Existing PUT /topic behavior stays byte-identical: channels.go::handleSetTopic
//     is unchanged; CHN-10 writes the same channels.topic column through
//     store.UpdateChannel as the single source.
//   - AST check site 17: the internal best-effort write path must not add a
//     retry queue or dead-letter async sink; _test.go grep checks cover this.
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"borgee-server/internal/store"
)

// DescriptionMaxLength — server-side 长度上限, byte-identical 跟
// channels.topic GORM size:500 + client DESCRIPTION_MAX_LENGTH 同源.
// Changing this requires updating the server const, GORM size, and client const.
const DescriptionMaxLength = 500

// ChannelDescriptionHandler serves the authenticated PUT endpoint for setting
// the channel description (= channels.topic column). It is owner-only and
// complements the existing member-level PUT /topic CHN-2 #406 path, which
// remains byte-identical.
type ChannelDescriptionHandler struct {
	Store  *store.Store
	Logger *slog.Logger
}

// RegisterUserRoutes registers the authenticated endpoint behind authMw.
// Design ③ is owner-only: channel.CreatedBy must equal user.ID, otherwise
// member-level callers receive 403. It is intentionally not mounted on the
// admin API; there is no RegisterAdminRoutes entry for this handler.
func (h *ChannelDescriptionHandler) RegisterUserRoutes(mux *http.ServeMux, authMw func(http.Handler) http.Handler) {
	mux.Handle("PUT /api/v1/channels/{channelId}/description",
		authMw(http.HandlerFunc(h.handlePut)))
}

type chn10DescriptionRequest struct {
	Description string `json:"description"`
}

// handlePut — PUT /api/v1/channels/{channelId}/description.
//
// owner-only: caller must equal channel.CreatedBy. The length cap is 500
// (DescriptionMaxLength stays byte-identical with the client). Writes use
// store.UpdateChannel as the single source for the same column as PUT /topic.
func (h *ChannelDescriptionHandler) handlePut(w http.ResponseWriter, r *http.Request) {
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
	// Design ② owner-only: creator-only ACL, matching CHN-9 manage_visibility.
	if ch.CreatedBy != user.ID {
		writeJSONError(w, http.StatusForbidden, "Only the channel owner can update description")
		return
	}
	var req chn10DescriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	// Design ③ length cap 500: byte-identical with channels.topic GORM size:500.
	if len(req.Description) > DescriptionMaxLength {
		writeJSONError(w, http.StatusBadRequest,
			"Description must be 500 characters or less")
		return
	}
	if err := h.Store.UpdateChannelDescription(channelID, req.Description); err != nil {
		if h.Logger != nil {
			h.Logger.Error("chn10.update", "error", err)
		}
		writeJSONError(w, http.StatusInternalServerError, "Failed to update description")
		return
	}
	result, _ := h.Store.GetChannelWithCounts(channelID, user.ID)
	writeJSONResponse(w, http.StatusOK, map[string]any{
		"channel": result,
	})
}
