package api

import (
	"net/http"
	"time"

	agentpkg "borgee-server/internal/agent"
	"borgee-server/internal/store"
)

// AL-1b.2 (#R3 Phase 4) — agent status endpoint.
//
// Blueprint reference: docs/blueprint/current/agent-lifecycle.md §2.3
// (five states, 2026-04-28 four-person review #5 decision: busy/idle ship
// with BPP in Phase 4). Spec:
// docs/implementation/modules/al-1b-spec.md §1 AL-1b.2 split.
// Acceptance: docs/qa/acceptance-templates/al-1b.md §2.1 / §2.5.
//
// Endpoint contract:
//
//   GET /api/v1/agents/:id/status — query the merged five-state status. Returns:
//     {
//       "state": "busy" | "idle" | "online" | "offline" | "error",
//       "reason": "...",                           // set for error state (AL-1a six reasons)
//       "last_task_id": "...",                     // set for busy/idle when a BPP frame exists
//       "last_task_started_at": 1700000000000,     // set for busy state
//       "last_task_finished_at": 1700000000000,    // set for idle state
//       "state_updated_at": 1700000000000          // set for any state, matching AL-1a
//     }
//
//   PATCH /api/v1/agents/:id/status — always reject with 405. Design ② makes
//     BPP the only write source for status (acceptance §2.5): even admin users
//     cannot directly modify busy/idle; updates must arrive through a BPP
//     frame. Returns `{"error": "AL-1b: status is BPP-driven, no manual
//     override; see al-1b-spec.md §0 设计 ②"}`.
//
// Five-state merge priority (acceptance §2.1):
//
//   error > busy > idle > online > offline
//
//   - error: AL-1a Tracker persisted error state (api_key_invalid / runtime_crashed /
//     network_unreachable / quota_exceeded / runtime_timeout / unknown).
//   - busy: agent_status.state == 'busy'. If last_task_started_at <= now-5min,
//     ReapStaleBusyToIdle moves it to idle; see store/agent_status_queries.go.
//   - idle: agent_status.state == 'idle' (BPP task_finished frame or 5min reap).
//   - online: AL-1a Tracker has no error, agent_status has no row, and AL-3
//     hub presence has an active session (h.State.ResolveAgentState online
//     fallback).
//   - offline: none of the above applies.
//
// Explicit non-goals:
//   - Do not expose PATCH /status. Design ② makes BPP the only write source;
//     admins cannot bypass it. This matches the AL-4.2 admin restriction and
//     ADM-0 ⑦ boundary.
//   - Do not return a raw `last_error_reason` field. Admin responses also
//     return only the short reason code, matching AL-4.1 schema
//     NoLLMOrPresenceColumns.
//   - Do not merge AL-3 presence_sessions row counts or AL-4 agent_runtimes
//     status in storage. Design ① keeps three separate data paths; the
//     five-state merge happens only in this API handler, with the three schema
//     tables remaining independent.

// handleGetAgentStatus implements GET /api/v1/agents/:id/status.
//
// Permission: any authenticated user may read any agent status, matching the
// existing GET /agents/{id} ACL. Agent state is a subset of channel-scoped
// collaboration visibility.
func (h *AgentHandler) handleGetAgentStatus(w http.ResponseWriter, r *http.Request) {
	_, ok := mustUser(w, r)
	if !ok {
		return
	}

	agentID := r.PathValue("id")
	agent, err := h.Store.GetAgent(agentID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Agent not found")
		return
	}

	resp := h.resolveStatus5State(agent)
	writeJSONResponse(w, http.StatusOK, resp)
}

// handleRejectStatusPatch implements PATCH /api/v1/agents/:id/status — always
// rejects with 405 Method Not Allowed. Design ② makes BPP the only write
// source for status.
//
// Why 405 not 403: 405 communicates "this resource doesn't accept PATCH in
// any role" (semantic accuracy), not "you lack permission" (which would
// imply some other role could). The admin rejection matches AL-4.2: busy/idle
// changes must go through a BPP frame, with no direct override path.
func (h *AgentHandler) handleRejectStatusPatch(w http.ResponseWriter, r *http.Request) {
	_, ok := mustUser(w, r)
	if !ok {
		return
	}
	w.Header().Set("Allow", "GET")
	writeJSONError(w, http.StatusMethodNotAllowed,
		"AL-1b: status is BPP-driven, no manual override; see al-1b-spec.md §0 设计 ②")
}

// resolveStatus5State merges AL-1a Tracker (error) + AL-1b agent_status
// (busy/idle) + AL-3 hub presence (online/offline) into a single response
// map per acceptance §2.1 priority: error > busy > idle > online > offline.
//
// Returned map includes only fields meaningful for the resolved state to
// keep the JSON minimal, matching sanitizeAgent / withState behavior: empty
// fields are not emitted.
func (h *AgentHandler) resolveStatus5State(agent *store.User) map[string]any {
	resp := map[string]any{"agent_id": agent.ID}

	// Disabled agents always render offline, matching withState behavior.
	if agent.Disabled {
		resp["state"] = string(agentpkg.StateOffline)
		return resp
	}

	// Step 1: error trumps everything (AL-1a Tracker).
	var al1aSnap agentpkg.Snapshot
	if h.State != nil {
		al1aSnap = h.State.ResolveAgentState(agent.ID)
		if al1aSnap.State == agentpkg.StateError {
			resp["state"] = string(agentpkg.StateError)
			if al1aSnap.Reason != "" {
				resp["reason"] = al1aSnap.Reason
			}
			if al1aSnap.UpdatedAt != 0 {
				resp["state_updated_at"] = al1aSnap.UpdatedAt
			}
			return resp
		}
	}

	// Step 2: busy/idle (AL-1b agent_status row, BPP-driven).
	row, err := h.Store.GetAgentStatus(agent.ID)
	if err == nil && row != nil {
		resp["state"] = row.State
		if row.LastTaskID != nil {
			resp["last_task_id"] = *row.LastTaskID
		}
		if row.LastTaskStartedAt != nil {
			resp["last_task_started_at"] = *row.LastTaskStartedAt
		}
		if row.LastTaskFinishedAt != nil {
			resp["last_task_finished_at"] = *row.LastTaskFinishedAt
		}
		resp["state_updated_at"] = row.UpdatedAt
		return resp
	}
	// err != nil — could be RecordNotFound (no BPP frame yet) or real DB err.
	// Either way, fall through to AL-1a online/offline (acceptance §2.1
	// priority: idle/busy require an explicit row, absent row = online/offline
	// per AL-3 hub).

	// Step 3: online/offline fallback (AL-1a Snapshot defaults to offline).
	if h.State != nil {
		resp["state"] = string(al1aSnap.State)
		if al1aSnap.UpdatedAt != 0 {
			resp["state_updated_at"] = al1aSnap.UpdatedAt
		}
	} else {
		resp["state"] = string(agentpkg.StateOffline)
	}
	return resp
}

// IdleThreshold is the single source of truth for the 5min "no frame → idle"
// reaper window (acceptance §2.4). Keep this separate from the AL-3 60s
// heartbeat timeout: AL-3 is hub session-level, AL-1b is task-level, so they
// use different clocks.
const IdleThreshold = 5 * time.Minute
