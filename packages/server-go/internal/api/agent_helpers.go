// Package api — agent_helpers.go: REFACTOR-2 helper-6 SSOT for the
// "load agent by path id + 404" boilerplate.
//
// 设计: byte-identical 跟既有 ~10 处 inline:
//
//	id := r.PathValue("id")
//	agent, err := h.Store.GetAgent(id)
//	if err != nil {
//	    writeJSONError(w, http.StatusNotFound, "Agent not found")
//	    return
//	}
//
// Caller list 锁:
//   agents.go (handleGetAgent / handleUpdateAgent / handleDeleteAgent /
//             handleSetAgentEnabled / handleSetAgentRequireMention / handleListAgentInvitations)
//   agent_config.go (handleGetAgentConfig / handleUpdateAgentConfig)
//   runtimes.go (handleStartRuntime)
//   al_1b_2_status.go (handleAgentStatus)
//
// Reverse-grep 锚:
//   - `loadAgentByPath(` 单源 == 1 hit (本文件)
//   - inline `h.Store.GetAgent(id)` callsites 位置 0 hit post-refactor
//
// **不收**: 接 body.AgentID 而不是 path "id" 的位置 (agent_invitations
// handleAcceptInvitation / handleResendInvitation / handleRevokeInvitation /
// agent_config_ack_handler) — 那些不是 path-id pattern.

package api

import (
	"net/http"

	"borgee-server/internal/store"
)

// loadAgentByPath reads r.PathValue("id"), looks up the agent via the
// store, and returns the resolved Agent + its id on success. On error
// it writes the canonical 404 "Agent not found" response and returns
// (nil, "", false). Caller MUST early-return on false without writing.
//
// Note: Store.GetAgent returns *store.User (agents share the users table
// with role='agent' — see queries_phase2b.go).
func loadAgentByPath(w http.ResponseWriter, r *http.Request, s *store.Store) (*store.User, string, bool) {
	id := r.PathValue("id")
	agent, err := s.GetAgent(id)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Agent not found")
		return nil, "", false
	}
	return agent, id, true
}
