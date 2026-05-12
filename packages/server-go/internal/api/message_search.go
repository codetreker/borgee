// Package api — dm_11_search.go: DM-11 cross-DM message search REST.
//
// Blueprint reference: dm-model.md §3 future per-user search index v2. This v0
// uses LIKE %query% across the caller's DM channels. Spec:
// docs/implementation/modules/dm-11-spec.md.
//
// Public surface:
//   - (h *MessageSearchHandler) RegisterRoutes(mux, authMw)
//
// Endpoints:
//   GET /api/v1/dm/search?q=<query>&limit=<N>
//
// Design, matching spec §0:
//   ① No schema change: reuse messages.content + LIKE. This follows the
//      existing message search #467 pattern; FTS5 is deferred to v2 to avoid
//      cross-table join complexity here.
//   ② DM-only scope: store helper SearchDMMessages enforces a
//      channels.type='dm' JOIN to prevent cross-channel leaks, aligned with the
//      DM-10 #597 DM-only path.
//   ③ Channel-member ACL: store helper joins channel_members on cm.user_id =
//      caller to prevent cross-user DM leaks, reusing AP-4 #551 and AP-5 #555
//      design constraints.
//   ④ Byte-identical wording: error codes `dm_search.q_required` and
//      `dm_search.q_too_short` are locked; query handling trims input and
//      enforces min 2 chars / max 200 chars to limit expensive searches.
//   ⑤ Admin path is not mounted: grep checks require zero
//      `admin.*dm.*search\|/admin-api/.*dm/search` matches in admin*.go
//      (ADM-0 §1.3).
//
// Constraints:
//   - Do not add a dm_search_index table; LIKE %q% reads messages.content.
//   - Do not add relevance sorting; v0 orders by created_at DESC, matching the
//     existing SearchMessages #467 approach.
//   - Do not mount cross-user admin search (permanent ADM-0 §1.3 boundary).
//   - Do not return rows with deleted_at IS NOT NULL; maskDeletedMessages covers
//     deleted-message handling.

package api

import (
	"net/http"
	"strconv"
	"strings"

	"borgee-server/internal/store"
)

const (
	dm11MinQueryLen  = 2
	dm11MaxQueryLen  = 200
	dm11DefaultLimit = 30
	dm11MaxLimit     = 50
)

// MessageSearchHandler is the cross-DM search endpoint dispatcher.
type MessageSearchHandler struct {
	Store *store.Store
}

// RegisterRoutes wires GET /api/v1/dm/search behind authMw.
// User rail only; no admin route is mounted (design ⑤, ADM-0 §1.3).
func (h *MessageSearchHandler) RegisterRoutes(mux *http.ServeMux, authMw func(http.Handler) http.Handler) {
	mux.Handle("GET /api/v1/dm/search", authMw(http.HandlerFunc(h.handleSearch)))
}

// handleSearch — GET /api/v1/dm/search?q=<query>&limit=<N>.
//
// Validation order:
//  1. Auth (user-rail).
//  2. q query param required + 2..200 char to limit cost and avoid empty full scans.
//  3. limit clamp default 30 / max 50.
//  4. Store.SearchDMMessages (DM-only + channel-member ACL JOIN).
func (h *MessageSearchHandler) handleSearch(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeJSONErrorCode(w, http.StatusBadRequest, "dm_search.q_required",
			"Search query (q) is required")
		return
	}
	if len(q) < dm11MinQueryLen {
		writeJSONErrorCode(w, http.StatusBadRequest, "dm_search.q_too_short",
			"Search query must be at least 2 characters")
		return
	}
	if len(q) > dm11MaxQueryLen {
		writeJSONErrorCode(w, http.StatusBadRequest, "dm_search.q_too_long",
			"Search query must be at most 200 characters")
		return
	}

	limit := dm11DefaultLimit
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			if n > dm11MaxLimit {
				n = dm11MaxLimit
			}
			limit = n
		}
	}

	msgs, err := h.Store.SearchDMMessages(user.ID, q, limit)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to search DM messages")
		return
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{
		"messages": msgs,
		"count":    len(msgs),
	})
}
