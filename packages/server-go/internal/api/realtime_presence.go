// Package api — rt_4_presence.go: RT-4 channel presence indicator
// (member-only GET — channel.members ∩ presence.IsOnline).
//
// Blueprint: docs/implementation/modules/rt-4-spec.md §0+§1+§2.
//
// Public surface:
//   - RealtimePresenceHandler{Store, Tracker, Logger}
//   - (h *RealtimePresenceHandler) RegisterUserRoutes(mux, authMw)
//
// 反约束 (rt-4-spec.md §0 设计 ②③ 边界 ⑥):
//   - 0 schema 改 — 复用 AL-3.1 #277 presence_sessions + idx_presence_sessions_user_id.
//   - 0 新 WS frame — synchronous GET 即时取交集 (presence-change push 留 v3).
//   - member-only ACL — IsChannelMember 反向断 (非成员 403).
//   - admin god-mode 不挂 — 无 RegisterAdminRoutes (ADM-0 §1.3 红线).
//   - 既有 RT-2 typing path byte-identical 不变 — RT-4 不改 ws/client.go::handleTyping.
//   - AST 锁链延伸 — 不引入 retry queue / dead-letter sink (grep 守门 _test.go).
package api

import (
	"log/slog"
	"net/http"
	"time"

	"borgee-server/internal/presence"
	"borgee-server/internal/store"
)

// RealtimePresenceHandler — handler for GET /api/v1/channels/{id}/presence.
// Hosts the member-only synchronous read: channel.members ∩ IsOnline.
type RealtimePresenceHandler struct {
	Store   *store.Store
	Tracker presence.PresenceTracker
	Logger  *slog.Logger
}

// RegisterUserRoutes wires user-rail GET behind authMw. 设计 ③: member-
// only — IsChannelMember 反向断. admin god-mode 不挂 (no RegisterAdminRoutes).
func (h *RealtimePresenceHandler) RegisterUserRoutes(mux *http.ServeMux, authMw func(http.Handler) http.Handler) {
	mux.Handle("GET /api/v1/channels/{channelId}/presence",
		authMw(http.HandlerFunc(h.handleGet)))
}

// PresenceSnapshot — response shape (single-source, no separate types pkg).
type PresenceSnapshot struct {
	OnlineUserIDs []string `json:"online_user_ids"`
	CountedAt     int64    `json:"counted_at"`
}

// handleGet — GET /api/v1/channels/{channelId}/presence.
//
// Returns the subset of channel members whose IsOnline predicate holds,
// computed synchronously at request time (no caching / no background job).
func (h *RealtimePresenceHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}
	channelID := r.PathValue("channelId")
	if !h.Store.IsChannelMember(channelID, user.ID) {
		writeJSONError(w, http.StatusForbidden, "Must be a channel member")
		return
	}
	members, err := h.Store.ListChannelMembers(channelID)
	if err != nil {
		if h.Logger != nil {
			h.Logger.Error("rt4.list_members", "error", err)
		}
		writeJSONError(w, http.StatusInternalServerError, "Failed to list members")
		return
	}
	online := make([]string, 0, len(members))
	if h.Tracker != nil {
		for _, m := range members {
			if h.Tracker.IsOnline(m.UserID) {
				online = append(online, m.UserID)
			}
		}
	}
	writeJSONResponse(w, http.StatusOK, PresenceSnapshot{
		OnlineUserIDs: online,
		CountedAt:     time.Now().UnixMilli(),
	})
}
