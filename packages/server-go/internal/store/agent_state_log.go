// Package store — AL-1 state-machine validator + state log helpers.
//
// Blueprint: docs/blueprint/current/agent-lifecycle.md §2.3 (4 states: online /
// busy / idle / error). This extends the AL-1a #249 three-state stub and the
// AL-1b #453/#457 five-state busy/idle work; this module is the server reducer
// and audit log owner.
//
// State graph (blueprint §2.3 literals + AL-1b design point 2: state-machine
// single source):
//
//	''        ──→ online           (首次 presence track)
//	''        ──→ offline          (首次 presence offline, e.g. seed)
//	online    ──→ busy             (BPP-2.2 task_started frame, #485)
//	online    ──→ idle             (BPP-2.2 reaper 5min stale)
//	online    ──→ error            (AL-1a Reason* set, runtime crash)
//	online    ──→ offline          (presence track offline)
//	busy      ──→ idle             (BPP-2.2 task_finished frame)
//	busy      ──→ error            (runtime crash mid-task)
//	busy      ──→ offline          (presence offline forced)
//	idle      ──→ busy             (BPP-2.2 task_started frame)
//	idle      ──→ error            (runtime crash idle)
//	idle      ──→ offline          (presence offline)
//	error     ──→ online           (AL-1a Clear, runtime recovers)
//	error     ──→ offline          (presence offline does not clear the error)
//	offline   ──→ online           (presence track online recovery)
//
// Invalid transitions:
//   - online ↛ online (no-op transition rejected)
//   - busy ↛ online (must go through idle/error/offline first; busy is
//     task-bound and cannot directly fall back to online without lifecycle state)
//   - idle ↛ online (idle already implies online, so direct online is lossy)
//   - error → busy/idle (must Clear → online first)
//   - offline → busy/idle/error (presence-gated; must online first)
//
// Constraint 1, forward-only: log rows are not rewritten because there is no
// UPDATE/DELETE path; corrections are appended as new rows. This matches
// admin_actions ADM-2.1 constraint 5.
// Constraint 2, state-machine single source: ValidateTransition is the only
// gate. Do not add a `setState(any, any)` bypass; reverse-grep reference
// `agent_state_log.*INSERT.*VALUES` should only hit helper paths.
// Constraint 3, task-driven: busy/idle transitions must carry task_id
// (blueprint §2.3 row 2); presence transitions (online/offline/error) leave
// task_id empty.
// Constraint 4, reason reuse: this module is another alignment point for the
// AL-1a six reason literals (#249 + #305 + #321 + #380 + #454 + #458 + #481 + here).
package store

import (
	"errors"
	"fmt"
	"time"
)

// AgentState is the five-literal state union (AL-1a three-state base plus
// AL-1b busy/idle). '' is the sentinel for "no prior state" on the first
// transition.
type AgentState string

const (
	AgentStateInitial AgentState = ""
	AgentStateOnline  AgentState = "online"
	AgentStateBusy    AgentState = "busy"
	AgentStateIdle    AgentState = "idle"
	AgentStateError   AgentState = "error"
	AgentStateOffline AgentState = "offline"
)

// validTransitions maps from-state → set of allowed to-states. It stays aligned
// with the blueprint §2.3 state graph literals. Invalid transitions return an
// error from ValidateTransition.
var validTransitions = map[AgentState]map[AgentState]bool{
	AgentStateInitial: {
		AgentStateOnline:  true,
		AgentStateOffline: true,
	},
	AgentStateOnline: {
		AgentStateBusy:    true,
		AgentStateIdle:    true,
		AgentStateError:   true,
		AgentStateOffline: true,
	},
	AgentStateBusy: {
		AgentStateIdle:    true,
		AgentStateError:   true,
		AgentStateOffline: true,
	},
	AgentStateIdle: {
		AgentStateBusy:    true,
		AgentStateError:   true,
		AgentStateOffline: true,
	},
	AgentStateError: {
		AgentStateOnline:  true,
		AgentStateOffline: true,
	},
	AgentStateOffline: {
		AgentStateOnline: true,
	},
}

// AL-1a six reasons kept byte-identical with internal/agent/state.go::Reason*.
// Changing them requires updating each aligned test/reference site.
var validReasons = map[string]bool{
	"api_key_invalid":     true,
	"quota_exceeded":      true,
	"network_unreachable": true,
	"runtime_crashed":     true,
	"runtime_timeout":     true,
	"unknown":             true,
}

// ValidateTransition is the single gate for agent state transitions.
// Returns nil if (from, to) is in the valid graph; otherwise descriptive error.
//
// Design point 2, state-machine single source: all server-side state writes must
// go through this function. A SetAgentState* bypass that writes the log without
// this validator is a bug; CI grep `agent_state_log.*INSERT` should only hit
// helper paths.
func ValidateTransition(from, to AgentState, reason string) error {
	// Same-state → reject (lossy/duplicate).
	if from == to {
		return fmt.Errorf("invalid transition: same state %q (no-op rejected, 规则 ②)", from)
	}
	allowed, ok := validTransitions[from]
	if !ok {
		return fmt.Errorf("unknown from_state %q", from)
	}
	if !allowed[to] {
		return fmt.Errorf("invalid transition: %q ↛ %q (蓝图 §2.3 state graph 拒)", from, to)
	}
	// error transition must carry a valid reason (constraint 4, AL-1a six literals).
	if to == AgentStateError {
		if reason == "" {
			return errors.New("transition to 'error' requires reason (蓝图 §2.3 故障可解释)")
		}
		if !validReasons[reason] {
			return fmt.Errorf("invalid reason %q (规则 ④ — AL-1a 6 字面: api_key_invalid|quota_exceeded|network_unreachable|runtime_crashed|runtime_timeout|unknown)", reason)
		}
	}
	// busy/idle transitions should carry task_id (constraint 3, task-driven; this
	// is a soft gate at validator — caller is responsible).
	return nil
}

// AgentStateLogRow is one row of agent_state_log table (AL-1.4 v=25 schema).
type AgentStateLogRow struct {
	ID        int64  `gorm:"column:id;primaryKey;autoIncrement"`
	AgentID   string `gorm:"column:agent_id"`
	FromState string `gorm:"column:from_state"`
	ToState   string `gorm:"column:to_state"`
	Reason    string `gorm:"column:reason"`
	TaskID    string `gorm:"column:task_id"`
	TS        int64  `gorm:"column:ts"`
}

// TableName pins agent_state_log — overrides gorm's pluralization.
func (AgentStateLogRow) TableName() string { return "agent_state_log" }

// AppendAgentStateTransition writes one row to agent_state_log AFTER
// validating the transition. Returns the inserted row id.
//
// Design point 2, single gate: server callers must use this helper, not raw INSERT.
// Design point 3, task-driven: caller passes taskID (empty for presence transitions).
// Design point 4, reason: error transitions require one of the AL-1a six literals.
func (s *Store) AppendAgentStateTransition(agentID string, from, to AgentState, reason, taskID string) (int64, error) {
	if agentID == "" {
		return 0, errors.New("agent_id required")
	}
	if err := ValidateTransition(from, to, reason); err != nil {
		return 0, err
	}
	row := AgentStateLogRow{
		AgentID:   agentID,
		FromState: string(from),
		ToState:   string(to),
		Reason:    reason,
		TaskID:    taskID,
		TS:        time.Now().UnixMilli(),
	}
	if err := s.db.Create(&row).Error; err != nil {
		return 0, err
	}
	return row.ID, nil
}

// ListAgentStateLog returns the most recent transitions for an agent
// (DESC ts). Used by GET /api/v1/agents/:id/state-log (owner-only).
//
// Design point 1, forward-only: read-only path; UPDATE/DELETE is not exposed.
func (s *Store) ListAgentStateLog(agentID string, limit int) ([]AgentStateLogRow, error) {
	if agentID == "" {
		return nil, errors.New("agent_id required")
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	var rows []AgentStateLogRow
	err := s.db.Where("agent_id = ?", agentID).
		Order("ts DESC, id DESC").
		Limit(limit).
		Find(&rows).Error
	return rows, err
}
