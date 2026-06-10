// Package api — agent_heartbeat_decay.go: agent heartbeat-decay view —
// owner-only GET endpoint reporting an agent's liveness bucket
// (fresh/stale/dead) derived from its last heartbeat.
//
// NOTE: this is the RETAINED agent-liveness machinery (heartbeat decay
// from agent_runtimes.last_heartbeat_at), NOT the cut host_grants
// high-priv rail. Do not conflate this heartbeat-decay path with the
// dropped host_grants schema (table DROPPED at migration v=54). Sibling
// pure logic lives in bpp/heartbeat_decay.go (DeriveDecayState).
//
// Path: GET /api/v1/agents/{agentId}/heartbeat-decay
//
// 设计:
//   - **owner-only ACL** — agent.OwnerID == user.ID (跟 AL-2a /
//     BPP-3.2 / AL-1 / AL-5 / DM-4 / CV-4 v2 / BPP-7 / BPP-8 owner-only
//     同模式).
//   - **admin god-mode 不挂** — admin /admin-api/* rail 隔离 (ADM-0
//     §1.3 红线).
//   - response shape — derive decay state from agent_runtimes.last_
//     heartbeat_at via bpp.DeriveDecayState (no schema change).
//
// 反约束:
//   - grep 检查 raw `last_heartbeat_at` 不出现在 response (sanitizer
//     反向 — 仅返 derived state, 不漏底层时间戳).

package api

import (
	"log/slog"
	"net/http"
	"time"

	"borgee-server/internal/bpp"
	"borgee-server/internal/store"
)

// AgentHeartbeatDecayHandler serves GET /api/v1/agents/{agentId}/heartbeat-decay.
type AgentHeartbeatDecayHandler struct {
	Store  *store.Store
	Logger *slog.Logger
}

// RegisterRoutes wires the GET endpoint on the user rail.
func (h *AgentHeartbeatDecayHandler) RegisterRoutes(mux *http.ServeMux,
	authMw func(http.Handler) http.Handler) {
	mux.Handle("GET /api/v1/agents/{agentId}/heartbeat-decay",
		authMw(http.HandlerFunc(h.handleDecay)))
}

// handleDecay returns {state: "fresh"|"stale"|"dead", agent_id: ...}
// for the agent. Owner-only ACL (agent.OwnerID == user.ID).
func (h *AgentHeartbeatDecayHandler) handleDecay(w http.ResponseWriter, r *http.Request) {
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

	// Read agent_runtimes.last_heartbeat_at (no schema change; reuse AL-4
	// existing column). If no runtime row exists, treat as never alive
	// → dead (matches DeriveDecayState nil-safe behavior).
	var row struct {
		LastHeartbeatAt int64 `gorm:"column:last_heartbeat_at"`
	}
	_ = h.Store.DB().Raw(`SELECT last_heartbeat_at FROM agent_runtimes WHERE agent_id = ?`, agentID).Scan(&row).Error

	now := time.Now().UnixMilli()
	state := bpp.DeriveDecayState(now, row.LastHeartbeatAt)

	// 反向断言: response 不含 raw last_heartbeat_at 字段; 仅返 derive
	// state + age_ms (delta from now) for client diagnostics.
	ageMs := int64(0)
	if row.LastHeartbeatAt > 0 {
		ageMs = now - row.LastHeartbeatAt
		if ageMs < 0 {
			ageMs = 0
		}
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{
		"agent_id": agentID,
		"state":    string(state),
		"age_ms":   ageMs,
	})
}
