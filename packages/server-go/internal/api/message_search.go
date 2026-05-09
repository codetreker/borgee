// Package api — dm_11_search.go: DM-11 cross-DM message search REST.
//
// Blueprint锚: dm-model.md §3 future per-user search index v2 (本 PR
// v0 = LIKE %query% 跨 user DM channels). Spec:
// docs/implementation/modules/dm-11-spec.md (战马E v0).
//
// Public surface:
//   - (h *MessageSearchHandler) RegisterRoutes(mux, authMw)
//
// Endpoints:
//   GET /api/v1/dm/search?q=<query>&limit=<N>
//
// 设计 (跟 spec §0):
//   ① 0 schema 改 — 复用 messages.content + LIKE (跟 messages search
//      #467 既有同模式; FTS5 不走避免跨表 join 复杂度留 v2).
//   ② DM-only scope — store helper SearchDMMessages 强制 channels.type='dm'
//      JOIN, 反 cross-channel leak (跟 DM-10 #597 DM-only path 同精神).
//   ③ channel-member ACL — store helper JOIN channel_members ON cm.user_id
//      = caller (反 cross-user DM leak; 复用 AP-4 #551 + AP-5 #555 设计沿用).
//   ④ 文案 byte-identical — 错码 `dm_search.q_required` / `dm_search.q_too_short`
//      字面锁; query trim + min 2 char + max 200 char (反 DoS).
//   ⑤ admin god-mode 不挂 — grep 检查 `admin.*dm.*search\|/admin-api/.*dm/search`
//      在 admin*.go 0 hit (ADM-0 §1.3 红线).
//
// 反约束:
//   - 不另起 dm_search_index 表 (LIKE %q% 单源 messages.content 列)
//   - 不挂 sort by relevance (留 v2 — order by created_at DESC 单源, 跟
//     既有 SearchMessages #467 同精神)
//   - admin god-mode 不挂 cross-user search (永久, ADM-0 §1.3)
//   - 不返 deleted_at IS NOT NULL 行 (maskDeletedMessages helper 守)

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
// user-rail only; admin god-mode 不挂 (设计 ⑤ ADM-0 §1.3).
func (h *MessageSearchHandler) RegisterRoutes(mux *http.ServeMux, authMw func(http.Handler) http.Handler) {
	mux.Handle("GET /api/v1/dm/search", authMw(http.HandlerFunc(h.handleSearch)))
}

// handleSearch — GET /api/v1/dm/search?q=<query>&limit=<N>.
//
// Validation order:
//  1. Auth (user-rail).
//  2. q query param required + 2..200 char (反 DoS, 反空查询全表扫).
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
