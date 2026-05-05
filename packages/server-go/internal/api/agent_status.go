package api

import (
	"net/http"
	"time"

	agentpkg "borgee-server/internal/agent"
	"borgee-server/internal/store"
)

// AL-1b.2 (#R3 Phase 4) — agent status endpoint.
//
// Blueprint锚: docs/blueprint/current/agent-lifecycle.md §2.3 (5-state, 2026-04-28
// 4 人 review #5 决议: busy/idle 跟 BPP 同期 Phase 4). Spec:
// docs/implementation/modules/al-1b-spec.md (战马C v0) §1 拆段 AL-1b.2.
// Acceptance: docs/qa/acceptance-templates/al-1b.md §2.1 / §2.5.
//
// Endpoint contract:
//
//   GET /api/v1/agents/:id/status — 5-state 合并查询. 返回:
//     {
//       "state": "busy" | "idle" | "online" | "offline" | "error",
//       "reason": "...",                           // error 态时填 (AL-1a 6 reason)
//       "last_task_id": "...",                     // busy/idle 态且有 BPP frame 时填
//       "last_task_started_at": 1700000000000,     // busy 态时填
//       "last_task_finished_at": 1700000000000,    // idle 态时填
//       "state_updated_at": 1700000000000          // 任意态都填 (跟 AL-1a 同模式)
//     }
//
//   PATCH /api/v1/agents/:id/status — **拒绝 405**. 立场 ② BPP 单源 反人工
//     伪造 (acceptance §2.5): admin god-mode 也不允许直接改 busy/idle, 必须
//     走 BPP frame. 返回 `{"error": "AL-1b: status is BPP-driven, no manual
//     override; see al-1b-spec.md §0 立场 ②"}`.
//
// 5-state 合并优先级 (acceptance §2.1):
//
//   error > busy > idle > online > offline
//
//   - error: AL-1a Tracker 持久 error 态 (api_key_invalid / runtime_crashed /
//     network_unreachable / quota_exceeded / runtime_timeout / unknown).
//   - busy:  agent_status.state == 'busy' 且 last_task_started_at <= now-5min
//            被 ReapStaleBusyToIdle 自动 idle, 见 store/agent_status_queries.go.
//   - idle:  agent_status.state == 'idle' (BPP task_finished frame 或 5min reap).
//   - online: AL-1a Tracker 无 error + agent_status 无 row + AL-3 hub presence
//             session active (h.State.ResolveAgentState online 退化).
//   - offline: 上面全无.
//
// 反约束:
//   - 不暴露 PATCH /status (立场 ② BPP 单源, admin 也不能改 — 跟 AL-4.2
//     admin god-mode 反约束同源, ADM-0 ⑦ red-line).
//   - 不返 raw `last_error_reason` 字段 (admin god-mode 不返 reason raw 文本,
//     AL-4.1 schema NoLLMOrPresenceColumns 同源 — 仅返 reason 短码).
//   - 不混 AL-3 presence_sessions row count / AL-4 agent_runtimes status —
//     立场 ① 拆三路径, 5-state 合并仅在 API 层(本 handler), schema 三表独立.

// handleGetAgentStatus implements GET /api/v1/agents/:id/status.
//
// Permission: 任意 authed user 可查任意 agent status (跟 GET /agents/{id}
// 既有 ACL 同源 — agent state 是 channel-scoped 协作场可见性的子集).
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
// rejects with 405 Method Not Allowed. 立场 ② BPP 单源 反人工伪造.
//
// Why 405 not 403: 405 communicates "this resource doesn't accept PATCH in
// any role" (semantic accuracy), not "you lack permission" (which would
// imply some other role could). admin god-mode reject 跟 AL-4.2 同源 — 改
// busy/idle 必须走 BPP frame, 没有任何后门.
func (h *AgentHandler) handleRejectStatusPatch(w http.ResponseWriter, r *http.Request) {
	_, ok := mustUser(w, r)
	if !ok {
		return
	}
	w.Header().Set("Allow", "GET")
	writeJSONError(w, http.StatusMethodNotAllowed,
		"AL-1b: status is BPP-driven, no manual override; see al-1b-spec.md §0 立场 ②")
}

// resolveStatus5State merges AL-1a Tracker (error) + AL-1b agent_status
// (busy/idle) + AL-3 hub presence (online/offline) into a single response
// dict per acceptance §2.1 priority: error > busy > idle > online > offline.
//
// Returned map includes only fields meaningful for the resolved state to
// keep the JSON minimal (跟 sanitizeAgent / withState 同模式 — empty fields
// not emitted).
func (h *AgentHandler) resolveStatus5State(agent *store.User) map[string]any {
	resp := map[string]any{"agent_id": agent.ID}

	// Disabled agents always render offline (跟 withState 同模式).
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

	// Step 3: online/offline fallback (AL-1a Snapshot默认 offline 退化).
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

// IdleThreshold — single source of truth for the 5min "no frame → idle"
// reaper window (acceptance §2.4). 跟 AL-3 60s heartbeat timeout 拆死 (AL-3
// 是 hub session-level, AL-1b 是 task-level — different clock).
const IdleThreshold = 5 * time.Minute
