// Package api — AL-1.4 agent state log endpoint.
//
// GET /api/v1/agents/:id/state-log — owner-only history of agent state
// transitions (online/busy/idle/error/offline). 跟 ADM-2.2 audit endpoint
// 同模式 (sanitize + scope + 反 inject), 蓝图 §2.3 "故障可解释" 兑现:
// owner 看 agent state 历史轨迹查 病因 + 修复入口.
//
// 设计 ① owner-only: server-side 走 OwnerID check (跟 AL-2a /agents/:id/config
// 同模式); admin god-mode 不挂 (ADM-0 §1.3 god-mode 仅元数据, agent state
// 历史不算 channel content 但 owner 隐私边界, owner 自助看不开 admin path).
package api

import (
	"log/slog"
	"net/http"

	"borgee-server/internal/store"
)

// AgentStateLogHandler hosts AL-1.4 state log GET endpoint.
type AgentStateLogHandler struct {
	Store  *store.Store
	Logger *slog.Logger
}

// RegisterRoutes wires the user-rail endpoint behind authMw.
func (h *AgentStateLogHandler) RegisterRoutes(mux *http.ServeMux, authMw func(http.Handler) http.Handler) {
	mux.Handle("GET /api/v1/agents/{id}/state-log", authMw(http.HandlerFunc(h.handleListStateLog)))
}

// handleListStateLog — GET /api/v1/agents/:id/state-log.
//
// 设计 ① owner-only: agent.OwnerID == current_user.ID; non-owner → 403.
// 设计 ② sanitizer: 反向不返 raw FK (跟 ADM-2 sanitizeAdminAction 同模式 —
// agent_id 已是 path param caller 知道, 不重复返).
func (h *AgentStateLogHandler) handleListStateLog(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}
	agentID := r.PathValue("id")
	if agentID == "" {
		writeJSONError(w, http.StatusBadRequest, "agent id required")
		return
	}

	// Owner-only ACL: load agent and check OwnerID.
	agent, err := h.Store.GetUserByID(agentID)
	if err != nil || agent == nil || agent.Role != "agent" {
		writeJSONError(w, http.StatusNotFound, "Agent not found")
		return
	}
	if agent.OwnerID == nil || *agent.OwnerID != user.ID {
		// Non-owner can't see — 403 with discriminator (跟 AL-2a 同模式).
		writeJSONError(w, http.StatusForbidden, "Not the owner of this agent")
		return
	}

	limit := parseLimit(r, 100, 500)
	rows, err := h.Store.ListAgentStateLog(agentID, limit)
	if err != nil {
		h.Logger.Error("list agent state log", "error", err, "agent_id", agentID)
		writeJSONError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	out := make([]map[string]any, len(rows))
	for i, r := range rows {
		out[i] = map[string]any{
			"id":         r.ID,
			"from_state": r.FromState,
			"to_state":   r.ToState,
			"reason":     r.Reason,
			"task_id":    r.TaskID,
			"ts":         r.TS,
		}
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{"transitions": out})
}
