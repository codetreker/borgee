package store

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

// AL-1b.2 (#R3 Phase 4) agent_status row helpers.
//
// Blueprint锚: docs/blueprint/current/agent-lifecycle.md §2.3 (5-state, busy/idle
// 跟 BPP 同期 Phase 4 — source 必须 plugin 上行 task_started / task_finished
// frame). Spec: docs/implementation/modules/al-1b-spec.md (战马C v0) §1
// 拆段 AL-1b.2. Migration: al_1b_1_agent_status.go (v=21, AL-1b.1 #453).
//
// Schema contract (跟 al_1b_1 migration 字面 byte-identical):
//   agent_status(agent_id PK, state CHECK busy/idle, last_task_id NULL,
//                last_task_started_at NULL, last_task_finished_at NULL,
//                created_at NOT NULL, updated_at NOT NULL)
//
// 立场 ② BPP 单源 — these helpers are called only from the BPP frame
// dispatcher (BPP-2 待落) + the 5min idle reaper. No public PATCH path
// (GET-only at /api/v1/agents/:id/status, see api/al_1b_2_status.go).

// AgentStatus mirrors the agent_status row. Pointers for last_task_*
// fields preserve nullability across CRUD (跟 al_4_1 / al_3_1 同模式).
type AgentStatus struct {
	AgentID            string  `gorm:"primaryKey;size:36;column:agent_id" json:"agent_id"`
	State              string  `gorm:"not null;size:8;column:state" json:"state"`
	LastTaskID         *string `gorm:"size:64;column:last_task_id" json:"last_task_id,omitempty"`
	LastTaskStartedAt  *int64  `gorm:"column:last_task_started_at" json:"last_task_started_at,omitempty"`
	LastTaskFinishedAt *int64  `gorm:"column:last_task_finished_at" json:"last_task_finished_at,omitempty"`
	CreatedAt          int64   `gorm:"not null;column:created_at" json:"created_at"`
	UpdatedAt          int64   `gorm:"not null;column:updated_at" json:"updated_at"`
}

// TableName pins agent_status — overrides gorm's default pluralization
// (跟 al_4_1 AgentRuntime 同模式).
func (AgentStatus) TableName() string { return "agent_status" }

// GetAgentStatus returns the row for agentID, or (nil, gorm.ErrRecordNotFound)
// if no row exists yet (5-state 合并: 没行 = 没收过 BPP frame, 这种情况
// agent 视为 idle 还是 fallback 到 AL-1a/AL-3 由 server 决定, 不是 store
// 关心).
func (s *Store) GetAgentStatus(agentID string) (*AgentStatus, error) {
	var row AgentStatus
	if err := s.db.Where("agent_id = ?", agentID).First(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}

// IsAgentStatusNotFound — convenience to disambiguate the "no row yet"
// case from real DB errors at call sites (跟 errors.Is 包装 same value
// 同模式).
func IsAgentStatusNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

// SetAgentTaskStarted upserts agent_status to state='busy' with the
// task_id snapshot. Called by the BPP `task_started` frame dispatcher
// (BPP-2 待落) — 立场 ② 单 source. now 注入用于测试.
func (s *Store) SetAgentTaskStarted(agentID, taskID string, now time.Time) error {
	if agentID == "" {
		return errors.New("agent_status: empty agent_id")
	}
	ts := now.UnixMilli()
	taskRef := taskID
	row := AgentStatus{
		AgentID:           agentID,
		State:             "busy",
		LastTaskID:        &taskRef,
		LastTaskStartedAt: &ts,
		CreatedAt:         ts,
		UpdatedAt:         ts,
	}
	// Upsert: ON CONFLICT(agent_id) UPDATE state/last_task_*/updated_at
	// (CreatedAt 仅在初次插入时写, 后续 UPDATE 不动 — 跟 cv_1_1 artifact
	// versions 'first write wins for created_at' 同模式).
	return s.db.Exec(`
		INSERT INTO agent_status
			(agent_id, state, last_task_id, last_task_started_at, created_at, updated_at)
		VALUES (?, 'busy', ?, ?, ?, ?)
		ON CONFLICT(agent_id) DO UPDATE SET
			state = excluded.state,
			last_task_id = excluded.last_task_id,
			last_task_started_at = excluded.last_task_started_at,
			updated_at = excluded.updated_at`,
		row.AgentID, taskRef, ts, ts, ts).Error
}

// SetAgentTaskFinished upserts agent_status to state='idle' with
// last_task_finished_at. Called by the BPP `task_finished` frame
// dispatcher (BPP-2 待落). last_task_id is preserved (idle 态时仍
// 留账上次 task — 跟 acceptance §1.1 last-known semantics 同源).
func (s *Store) SetAgentTaskFinished(agentID, taskID string, now time.Time) error {
	if agentID == "" {
		return errors.New("agent_status: empty agent_id")
	}
	ts := now.UnixMilli()
	taskRef := taskID
	return s.db.Exec(`
		INSERT INTO agent_status
			(agent_id, state, last_task_id, last_task_finished_at, created_at, updated_at)
		VALUES (?, 'idle', ?, ?, ?, ?)
		ON CONFLICT(agent_id) DO UPDATE SET
			state = excluded.state,
			last_task_id = excluded.last_task_id,
			last_task_finished_at = excluded.last_task_finished_at,
			updated_at = excluded.updated_at`,
		agentID, taskRef, ts, ts, ts).Error
}

// ReapStaleBusyToIdle flips agent_status rows from 'busy' to 'idle'
// when the last frame is older than threshold (acceptance §2.4 — 5min
// 无 frame → idle, 单 const `IdleThreshold = 5*time.Minute` server 侧).
// Returns the number of rows reaped. Called by a periodic ticker in
// server.go (跟 AL-3 hub heartbeat reaper 同模式但 task-level vs
// session-level).
func (s *Store) ReapStaleBusyToIdle(now time.Time, threshold time.Duration) (int64, error) {
	cutoff := now.Add(-threshold).UnixMilli()
	res := s.db.Exec(`
		UPDATE agent_status
		   SET state = 'idle', updated_at = ?
		 WHERE state = 'busy'
		   AND COALESCE(last_task_started_at, 0) < ?`,
		now.UnixMilli(), cutoff)
	if res.Error != nil {
		return 0, res.Error
	}
	return res.RowsAffected, nil
}
