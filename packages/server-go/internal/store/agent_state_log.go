// Package store — AL-1 状态机 validator + state log helpers.
//
// Blueprint: docs/blueprint/current/agent-lifecycle.md §2.3 (4 态: online / busy /
// idle / error). 跟 AL-1a #249 三态 stub + AL-1b #453/#457 5-state busy/idle
// 同源 — 此模块是 server reducer + audit log 真接管.
//
// State graph (蓝图 §2.3 字面 + AL-1b 设计 ② state machine 单一来源):
//
//   ''        ──→ online           (首次 presence track)
//   ''        ──→ offline          (首次 presence offline, e.g. seed)
//   online    ──→ busy             (BPP-2.2 task_started frame, #485)
//   online    ──→ idle             (BPP-2.2 reaper 5min stale)
//   online    ──→ error            (AL-1a Reason* set, runtime crash)
//   online    ──→ offline          (presence track offline)
//   busy      ──→ idle             (BPP-2.2 task_finished frame)
//   busy      ──→ error            (runtime crash mid-task)
//   busy      ──→ offline          (presence offline forced)
//   idle      ──→ busy             (BPP-2.2 task_started frame)
//   idle      ──→ error            (runtime crash idle)
//   idle      ──→ offline          (presence offline)
//   error     ──→ online           (AL-1a Clear, runtime recovers)
//   error     ──→ offline          (presence offline 错误清不掉)
//   offline   ──→ online           (presence track online recovery)
//
// 禁止 (invalid):
//   - online ↛ online (no-op transition rejected)
//   - busy ↛ online (must go through idle/error/offline first; busy 是
//     task-bound, 不能直接掉回 online without state lifecycle)
//   - idle ↛ online (idle 已含 online 属性, 直接 online 是 lossy)
//   - error → busy/idle (must Clear → online first)
//   - offline → busy/idle/error (presence-gated; must online first)
//
// 约束 ① forward-only — log 不可改写 (UPDATE/DELETE 路径不存在); 错误录入
// 只能再写新行. 跟 admin_actions ADM-2.1 约束 ⑤ 同精神.
// 约束 ② state machine 单一来源 — ValidateTransition 是唯一 gate, 不开
// `setState(any, any)` 旁路 (反向 grep `agent_state_log.*INSERT.*VALUES`
// 在非 helper 路径 count==0).
// 约束 ③ task-driven — busy/idle 转移必带 task_id (蓝图 §2.3 row 2 字面);
// presence 转移 (online/offline/error) task_id 留空.
// 约束 ④ reason 复用 AL-1a 6 字面 — 此模块是第 8 处单测对齐位 (#249 + #305 +
// #321 + #380 + #454 + #458 + #481 + 此).
package store

import (
	"errors"
	"fmt"
	"time"
)

// AgentState is the 5-字面 state union (AL-1a 三态 + AL-1b busy/idle).
// '' is sentinel for "no prior state" (首次 transition).
type AgentState string

const (
	AgentStateInitial AgentState = ""
	AgentStateOnline  AgentState = "online"
	AgentStateBusy    AgentState = "busy"
	AgentStateIdle    AgentState = "idle"
	AgentStateError   AgentState = "error"
	AgentStateOffline AgentState = "offline"
)

// validTransitions maps from-state → set of allowed to-states. 跟蓝图 §2.3
// state graph 字面对齐. Invalid transitions return error from ValidateTransition.
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

// AL-1a 6 reason byte-identical (改 = 改 8 处单测对齐位同源, 跟
// internal/agent/state.go::Reason* 字面). 此模块是第 8 处.
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
// 设计 ② state machine 单一来源 — 所有 server-side state set must go through
// this function (反向: 调 SetAgentState* 旁路写 log 不通过此 validator
// 是 bug, CI grep `agent_state_log.*INSERT` 应仅在 helper 路径).
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
	// error transition must carry valid reason (规则 ④ AL-1a 6 字面).
	if to == AgentStateError {
		if reason == "" {
			return errors.New("transition to 'error' requires reason (蓝图 §2.3 故障可解释)")
		}
		if !validReasons[reason] {
			return fmt.Errorf("invalid reason %q (规则 ④ — AL-1a 6 字面: api_key_invalid|quota_exceeded|network_unreachable|runtime_crashed|runtime_timeout|unknown)", reason)
		}
	}
	// busy/idle transitions should carry task_id (约定 ③ task-driven; this
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
// 设计 ② single gate — server callers must use this helper, not raw INSERT.
// 设计 ③ task-driven — caller passes taskID (empty for presence transitions).
// 设计 ④ reason — error transitions require reason ∈ AL-1a 6 字面.
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
// 设计 ① forward-only — read-only path; UPDATE/DELETE not exposed.
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
