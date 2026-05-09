// Package api — bpp_8_lifecycle_list.go: BPP-8.2 GET endpoint for plugin
// lifecycle audit history (owner-only view).
//
// Path: GET /api/v1/agents/{agentId}/lifecycle?limit=N (default 100, max 500).
//
// 设计 (跟 bpp-8-spec.md §0.3 + stance §3 byte-identical):
//   - **owner-only ACL** — agent.OwnerID == current user.ID (跟 AL-2a /
//     BPP-3.2 / AL-1 / AL-5 / DM-4 / CV-4 v2 / BPP-7 owner-only 7 处同模式).
//   - **admin god-mode 不挂** — admin /admin-api/* rail 隔离, this handler
//     mounted on user rail only (ADM-0 §1.3 红线).
//   - 复用 admin_actions 表 — query filter `target_user_id = agent_id AND
//     action LIKE 'plugin_%'` (设计 ① audit forward-only, 跟 ADM-2.1 +
//     AP-2 + BPP-4 + BPP-8 跨五 milestone 同精神 — 锁链第 5 处).
//
// 反约束:
//   - grep 检查 `admin.*plugin.*lifecycle\|admin.*BPP8` 在 admin*.go
//     count==0 (TestBPP83_AdminGodModeNotMounted 守).

package api

import (
	"log/slog"
	"net/http"
	"strconv"

	"borgee-server/internal/store"
)

// PluginListHandler serves GET /api/v1/agents/{agentId}/lifecycle.
type PluginListHandler struct {
	Store  *store.Store
	Logger *slog.Logger
}

// RegisterRoutes wires the GET endpoint on the user rail.
func (h *PluginListHandler) RegisterRoutes(mux *http.ServeMux,
	authMw func(http.Handler) http.Handler) {
	mux.Handle("GET /api/v1/agents/{agentId}/lifecycle",
		authMw(http.HandlerFunc(h.handleList)))
}

// bpp8ClampLifecycleLimit — default 100, max 500, 0/negative/empty → 100.
func bpp8ClampLifecycleLimit(raw string) int {
	const (
		def = 100
		max = 500
	)
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return def
	}
	if n > max {
		return max
	}
	return n
}

// ClampBPP8LifecycleLimitForTest exposes bpp8ClampLifecycleLimit for unit
// tests (test-only export, same pattern as ClampCV4V2LimitForTest).
func ClampBPP8LifecycleLimitForTest(raw string) int { return bpp8ClampLifecycleLimit(raw) }

// handleList returns the most recent plugin_* lifecycle rows for the
// agent. Owner-only ACL (agent.OwnerID == user.ID) — non-owner / unauth /
// non-existent agent paths return 403 / 401 / 404 respectively.
func (h *PluginListHandler) handleList(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}
	agentID := r.PathValue("agentId")
	if agentID == "" {
		writeJSONError(w, http.StatusBadRequest, "Agent ID is required")
		return
	}
	agent, err := h.Store.GetUserByID(agentID)
	if err != nil || agent == nil || agent.Role != "agent" {
		writeJSONError(w, http.StatusNotFound, "Agent not found")
		return
	}
	if agent.OwnerID == nil || *agent.OwnerID != user.ID {
		writeJSONError(w, http.StatusForbidden, "Not the owner of this agent")
		return
	}
	limit := bpp8ClampLifecycleLimit(r.URL.Query().Get("limit"))

	// Query admin_actions filtered by target_user_id == agentID AND action
	// LIKE 'plugin_%'. ListAdminActionsForTargetUser returns ALL actions —
	// we filter post-fetch since the helper doesn't take action-prefix
	// args (and adding one would risk leaking god-mode-style filters).
	all, err := h.Store.ListAdminActionsForTargetUser(agentID, limit*2)
	if err != nil {
		h.Logger.Error("bpp8 list lifecycle", "error", err, "agent_id", agentID)
		writeJSONError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	out := make([]map[string]any, 0, limit)
	for _, row := range all {
		if !isPluginLifecycleAction(row.Action) {
			continue
		}
		out = append(out, map[string]any{
			"id":         row.ID,
			"action":     row.Action,
			"actor_id":   row.ActorID, // 设计 ⑦ — always "system" byte-identical
			"agent_id":   row.TargetUserID,
			"metadata":   row.Metadata,
			"created_at": row.CreatedAt,
		})
		if len(out) >= limit {
			break
		}
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{"events": out})
}

// isPluginLifecycleAction returns true for the 5 plugin_* action literals
// (admin_actions CHECK enum +5 条 byte-identical 跟 migration v=31).
func isPluginLifecycleAction(action string) bool {
	switch action {
	case "plugin_connect",
		"plugin_disconnect",
		"plugin_reconnect",
		"plugin_cold_start",
		"plugin_heartbeat_timeout":
		return true
	}
	return false
}
