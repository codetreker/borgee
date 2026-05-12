// Package ws — agent_task_state_changed_frame.go: RT-3 source of truth
// for the `agent_task_state_changed` push frame. Server fans this out to
// channel members when an agent transitions busy↔idle, derived from
// BPP-2.2 task_started / task_finished plugin upstream frames.
//
// Blueprint reference: docs/blueprint/current/realtime.md §1.1 (thinking requires
// subject) + §0 text "v1 realtime 只做'足够让用户感到 AI 在工作'的最小集"
// + agent-lifecycle.md §2.3 (busy/idle source 必须 plugin 上行 frame) +
// plugin-protocol.md §1.6.
//
// Spec brief: docs/implementation/modules/rt-3-spec.md (本 PR 同 commit
// landed with this PR) §0 point 1 + §1 RT-3.1 breakdown.
// Checklist: docs/qa/rt-3-stance-checklist.md §1 point 1 negative constraints.
//
// Behaviour contract — follows the same wire pattern as RT-1.1 ArtifactUpdated /
// CV-2.2 AnchorCommentAdded / DM-2.2 MentionPushed / CV-4.2
// IterationStateChanged / AL-2b AgentConfigUpdate (RT-3 is the 6th shared-sequence frame):
//
//  1. Cursor uses hub.cursors.NextCursor() and shares the same monotonic
//     sequence as the five upstream frames (no separate agent-only push channel).
//  2. Field order contract: type / cursor / agent_id / state / subject / reason /
//     changed_at. The 7-field order matches BPP-2.2 task_started/finished;
//     subject matches the plugin upstream source frame.
//  3. JSON tags must match client ws-frames.ts field names (BPP-1 #304 envelope
//     CI lint + RT-3.2 client wiring).
//  4. Multi-device fanout — BroadcastToChannel walks every client subscription, so one
//     user with multiple ws sessions receives all frames (same rule as P1MultiDeviceWebSocket #197 +
//     Hub.onlineUsers map[userID]map[*Client]bool structure).
//
// Negative constraints (rt-3-spec §0 point 1 + blueprint §1.1):
//   - state ∈ 2-enum {'busy', 'idle'}; 中间态 reject (跟 BPP-2.2 outcome
//     enum 同模式 fail-closed).
//   - busy 态 subject 必带非空 (蓝图 §1.1 字面 "BPP progress frame 强制带
//     subject 字段, plugin 必须告诉 Borgee 'agent 在做什么', 否则不展示").
//     反向 grep CI lint guards: empty subject default symbol /
//     fallback-named symbol / hard-coded vague strings — count==0 across
//     this file (excluding _test.go); default subject fallbacks are forbidden,
//     matching BPP-2.2 task_lifecycle.go ValidateTaskStarted.
//   - idle 态 subject 必为空 (prevents stale subject text, matching BPP-2.2
//     cancelled/completed reason-empty behavior).
//   - reason is only set for idle frames derived from failed tasks, using the
//     same AL-1a six-value dictionary strings
//     (复用 internal/agent/state.go::Reason* SSOT).
package ws

// FrameTypeAgentTaskStateChanged is the `type` discriminator emitted on
// the `/ws` envelope; client switch lives in
// packages/client/src/realtime/wsClient.ts (RT-3.2 接).
const FrameTypeAgentTaskStateChanged = "agent_task_state_changed"

// AgentTaskState enum matches blueprint realtime.md §1.1 and
// agent-lifecycle.md §2.3 (busy / idle).
const (
	AgentTaskStateBusy = "busy"
	AgentTaskStateIdle = "idle"
)

// AgentTaskStateChangedFrame — server → client push fired when an agent
// transitions busy↔idle. Server derives this from BPP-2.2 task_started/finished
// upstream frames; busy/idle is computed from task lifecycle rather than sent
// as an independent plugin signal (BPP-2 #485 (a) design choice).
//
// Field order is the contract. Do NOT reorder without updating
// packages/client/src/types/ws-frames.ts in the same PR.
type AgentTaskStateChangedFrame struct {
	Type      string `json:"type"`
	Cursor    int64  `json:"cursor"`
	AgentID   string `json:"agent_id"`
	State     string `json:"state"`      // 'busy' | 'idle'
	Subject   string `json:"subject"`    // non-empty for busy; empty for idle (blueprint §1.1 ⭐)
	Reason    string `json:"reason"`     // set only for idle + failed-derived; otherwise empty
	ChangedAt int64  `json:"changed_at"` // Unix ms 语义戳; cursor IS the order
}

// PushAgentTaskStateChanged broadcasts AgentTaskStateChangedFrame to every
// channel member of channelID and signals long-poll waiters. Cursor is
// allocated fresh from hub.cursors so the frame slots into the same
// monotonic sequence as ArtifactUpdated / AnchorCommentAdded /
// MentionPushed / IterationStateChanged / AgentConfigUpdate (no separate
// agent-only push channel).
//
// Multi-device fanout: BroadcastToChannel walks every subscribed *Client
// (one user can have N concurrent /ws sessions, all subscribe → all
// receive — backed by Hub.onlineUsers map[userID]map[*Client]bool and verified
// by P1 multi-device test #197).
//
// Returns (cursor, sent). sent=false only when the hub has no cursor
// allocator (test seam).
func (h *Hub) PushAgentTaskStateChanged(
	agentID string,
	channelID string,
	state string,
	subject string,
	reason string,
	changedAt int64,
) (cursor int64, sent bool) {
	if h.cursors == nil {
		return 0, false
	}
	cur := h.cursors.NextCursor()
	frame := AgentTaskStateChangedFrame{
		Type:      FrameTypeAgentTaskStateChanged,
		Cursor:    cur,
		AgentID:   agentID,
		State:     state,
		Subject:   subject,
		Reason:    reason,
		ChangedAt: changedAt,
	}
	if channelID == "" {
		h.BroadcastToAll(frame)
	} else {
		h.BroadcastToChannel(channelID, frame, nil)
	}
	h.SignalNewEvents()
	return cur, true
}
