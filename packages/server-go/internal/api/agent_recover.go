// Package api — AL-5 agent error recovery endpoint.
//
// POST /api/v1/agents/:id/recover — owner-only. Reads the most-recent
// agent_state_log row to discover the last error reason, then transitions
// agent state from `error → online` via AL-1 #492 single-gate helper
// `Store.AppendAgentStateTransition`. Reuses the AL-1 5-state graph valid
// edge `error → online`; does not introduce new states or recovery dictionary.
//
// 设计反查 (al-5-spec.md §0):
//
//	② recovery = 单 helper SSOT — 走 AppendAgentStateTransition (AL-1 #492
//	   single-gate); 不另起 recovery 状态机, 不在 5-state 加新态
//	③ recovery reason 不另起字典 — reason ∈ AL-1a 6 字面 (REFACTOR-REASONS
//	   #496 SSOT 同源); 反约束: 不新增 recovery_in_progress / auto_reconnect
//	   等中间态字面
//
// Owner-only ACL: agent.OwnerID == current_user.ID; non-owner → 403.
// Admin god-mode 不挂此路径 (ADM-0 §1.3 红线 — recovery 是业务态变更,
// 不属元数据, 跟 AL-1 #492 state-log endpoint 同精神).
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"borgee-server/internal/datalayer"
	"borgee-server/internal/store"
)

// AgentRecoverHandler hosts AL-5 agent error recovery POST endpoint.
type AgentRecoverHandler struct {
	Store *store.Store
	// DataLayer — DL-1.2 SSOT 4-interface bundle. When non-nil, owner-only
	// ACL agent lookup walks UserRepo.GetByID instead of store.GetUserByID.
	// nil-safe (legacy boot / unit tests fall back to Store).
	DataLayer *datalayer.DataLayer
	Logger    *slog.Logger
}

// RegisterRoutes wires the user-rail endpoint behind authMw.
func (h *AgentRecoverHandler) RegisterRoutes(mux *http.ServeMux, authMw func(http.Handler) http.Handler) {
	mux.Handle("POST /api/v1/agents/{id}/recover", authMw(http.HandlerFunc(h.handleRecover)))
}

// AgentRecoverPayload is the POST body for /agents/:id/recover.
type AgentRecoverPayload struct {
	// RequestID is an optional client-side trace id (idempotency hint, not
	// enforced server-side in v1; future v2 may dedup retries within window).
	RequestID string `json:"request_id,omitempty"`
}

// handleRecover — POST /api/v1/agents/:id/recover.
//
// Flow:
//  1. Auth + path id present.
//  2. Owner-only ACL: agent.OwnerID == user.ID; non-owner 403, non-agent 404.
//  3. Read most-recent state-log row to discover (a) current state must be
//     `error` and (b) the reason to carry forward in the recovery transition.
//  4. AppendAgentStateTransition(agent, error, online, reason, "") via the
//     AL-1 #492 single-gate helper — ValidateTransition守 valid edge.
//  5. Returns 200 with {state: "online", reason}.
//
// Reverse约束:
//   - admin god-mode 不挂此路径 (grep 检查 `admin-api.*recover` count==0,
//     dm-3-stance § 设计 ⑤ 同精神)
//   - 状态机不裂 — 走 AL-1 ValidateTransition 既有 graph
//   - reason 不新增 — 复用 last error transition 的 reason (≤6 字面 byte-identical)
func (h *AgentRecoverHandler) handleRecover(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}
	agentID := r.PathValue("id")
	if agentID == "" {
		writeJSONError(w, http.StatusBadRequest, "agent id required")
		return
	}

	// Optional payload — request_id pass-through; tolerate empty body.
	var payload AgentRecoverPayload
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&payload)
	}
	_ = payload // request_id not used in v1; reserved for future idempotency.

	// Owner-only ACL — load agent, check role + OwnerID.
	// DL-1.2: prefer UserRepo.GetByID (interface seam) when DataLayer wired;
	// fall back to legacy store.GetUserByID (nil-safe, byte-identical).
	var agent *store.User
	var err error
	if h.DataLayer != nil {
		agent, err = h.DataLayer.UserRepo.GetByID(r.Context(), agentID)
	} else {
		agent, err = h.Store.GetUserByID(agentID)
	}
	if err != nil || agent == nil || agent.Role != "agent" {
		writeJSONError(w, http.StatusNotFound, "Agent not found")
		return
	}
	if agent.OwnerID == nil || *agent.OwnerID != user.ID {
		writeJSONError(w, http.StatusForbidden, "Not the owner of this agent")
		return
	}

	// Discover last state-log row — must be currently in `error` state to
	// recover. Reason from that row carries forward into the recovery
	// transition (AL-1a 6-字面 byte-identical, REFACTOR-REASONS #496 SSOT).
	rows, err := h.Store.ListAgentStateLog(agentID, 1)
	if err != nil {
		h.Logger.Error("list state log", "error", err, "agent_id", agentID)
		writeJSONError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	if len(rows) == 0 {
		// No history — agent has never transitioned; cannot be in error state.
		writeJSONError(w, http.StatusConflict, "Agent not in error state")
		return
	}
	last := rows[0]
	if last.ToState != string(store.AgentStateError) {
		writeJSONError(w, http.StatusConflict, "Agent not in error state")
		return
	}
	reason := last.Reason

	// Single-gate transition — AL-1 #492 helper守 ValidateTransition + 5-state graph.
	if _, err := h.Store.AppendAgentStateTransition(
		agentID,
		store.AgentStateError,
		store.AgentStateOnline,
		reason,
		"",
	); err != nil {
		h.Logger.Error("append recovery transition", "error", err, "agent_id", agentID)
		writeJSONError(w, http.StatusInternalServerError, "Recovery transition failed")
		return
	}

	writeJSONResponse(w, http.StatusOK, map[string]any{
		"state":  string(store.AgentStateOnline),
		"reason": reason,
	})
}
