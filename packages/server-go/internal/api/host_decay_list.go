// Package api — hb_3_v2_decay_list.go: HB-3 v2.2 GET endpoint for
// heartbeat decay state (owner-only view).
//
// Path: GET /api/v1/agents/{agentId}/heartbeat-decay
//
// 设计 (跟 hb-3-v2-spec.md §0.3 + stance §3 byte-identical):
//   - **owner-only ACL** — agent.OwnerID == user.ID (跟 AL-2a /
//     BPP-3.2 / AL-1 / AL-5 / DM-4 / CV-4 v2 / BPP-7 / BPP-8 owner-only
//     8 处同模式; HB-3 v2 = 第 9 处).
//   - **admin god-mode 不挂** — admin /admin-api/* rail 隔离 (ADM-0
//     §1.3 红线).
//   - response shape — derive decay state from agent_runtimes.last_
//     heartbeat_at via bpp.DeriveDecayState (no schema change).
//
// 反约束:
//   - grep 检查 `admin.*heartbeat.*decay\|admin.*HB3` 在 admin*.go 0 hit.
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

// HostDecayListHandler serves GET /api/v1/agents/{agentId}/heartbeat-decay.
type HostDecayListHandler struct {
	Store  *store.Store
	Logger *slog.Logger
}

// RegisterRoutes wires the GET endpoint on the user rail.
func (h *HostDecayListHandler) RegisterRoutes(mux *http.ServeMux,
	authMw func(http.Handler) http.Handler) {
	mux.Handle("GET /api/v1/agents/{agentId}/heartbeat-decay",
		authMw(http.HandlerFunc(h.handleDecay)))
}

// handleDecay returns {state: "fresh"|"stale"|"dead", agent_id: ...}
// for the agent. Owner-only ACL (agent.OwnerID == user.ID).
func (h *HostDecayListHandler) handleDecay(w http.ResponseWriter, r *http.Request) {
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
