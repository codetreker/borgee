// Package api — chn_10_description.go: CHN-10 owner-only PUT /channels/:id
// /description endpoint.
//
// Blueprint: docs/implementation/modules/chn-10-spec.md §0+§1+§2.
//
// Public surface:
//   - ChannelDescriptionHandler{Store, Logger}
//   - (h *ChannelDescriptionHandler) RegisterUserRoutes(mux, authMw)
//   - DescriptionMaxLength (= 500, byte-identical 跟 channels.topic GORM
//     size:500 同源, 双向锁守门 跟 client DESCRIPTION_MAX_LENGTH).
//
// 反约束 (chn-10-spec.md §0 设计 ②③ 边界 ⑥):
//   - owner-only ACL 锁链第 20 处 (DM-7 #19 + CHN-9 #14 承袭) — handler
//     走 channel.CreatedBy == user.ID 反向断言, 反 member-level (跟 既有
//     PUT /topic CHN-2 #406 互补 byte-identical 不动).
//   - admin god-mode 不挂 — RegisterAdminRoutes 不存在; grep 检查
//     `admin-api/v[0-9]+/.*description` PATCH/PUT/POST/DELETE 0 hit (ADM-0
//     §1.3 红线).
//   - 既有 PUT /topic byte-identical 不变 — channels.go::handleSetTopic
//     不动; CHN-10 写入相同 channels.topic 列 (store.UpdateChannel 单源).
//   - AST 锁链延伸第 17 处 — internal best-effort write path 不引入 retry
//     queue / dead-letter 异步 sink (grep 守门 _test.go).
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"borgee-server/internal/store"
)

// DescriptionMaxLength — server-side 长度上限, byte-identical 跟
// channels.topic GORM size:500 + client DESCRIPTION_MAX_LENGTH 同源.
// 改一处 = 改三处 (server const + GORM size + client const) 反向锁守门.
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
	// 设计 ② owner-only — creator-only ACL (CHN-9 manage_visibility 同精神).
	if ch.CreatedBy != user.ID {
		writeJSONError(w, http.StatusForbidden, "Only the channel owner can update description")
		return
	}
	var req chn10DescriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	// 设计 ③ length cap 500 — channels.topic GORM size:500 byte-identical.
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
