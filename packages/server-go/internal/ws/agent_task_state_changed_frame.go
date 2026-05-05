// Package ws — agent_task_state_changed_frame.go: RT-3 ⭐ source-of-truth
// for the `agent_task_state_changed` push frame. Server fans this out to
// channel members when an agent transitions busy↔idle, derived from
// BPP-2.2 task_started / task_finished plugin upstream frames.
//
// Blueprint锚: docs/blueprint/current/realtime.md §1.1 (活物感 / thinking 强制带
// subject) + §0 字面 "v1 realtime 只做'足够让用户感到 AI 在工作'的最小集"
// + agent-lifecycle.md §2.3 (busy/idle source 必须 plugin 上行 frame) +
// plugin-protocol.md §1.6.
//
// Spec brief: docs/implementation/modules/rt-3-spec.md (本 PR 同 commit
// 落) §0 立场 ① + §1 拆段 RT-3.1.
// Stance: docs/qa/rt-3-stance-checklist.md §1 立场 ① 反约束.
//
// Behaviour contract — byte-identical 跟 RT-1.1 ArtifactUpdated /
// CV-2.2 AnchorCommentAdded / DM-2.2 MentionPushed / CV-4.2
// IterationStateChanged / AL-2b AgentConfigUpdate 同模式 (RT-3 是第 6 个
// 共序 frame):
//
//   1. Cursor 走 hub.cursors.NextCursor() 单调发号, 跟 5 上游 frame 共
//      一根 sequence (反约束: 不另起 agent-only 推送通道).
//   2. 字段顺序锁: type / cursor / agent_id / state / subject / reason /
//      changed_at — 7 字段, 跟 BPP-2.2 task_started/finished frame field
//      顺序一致 (subject byte-identical 跟 plugin 上行 source frame).
//   3. JSON tag 跟客户端 ws-frames.ts 字段名严格一致 (BPP-1 #304 envelope
//      CI lint 自动闸位 + RT-3.2 client 接).
//   4. 多端 fanout — BroadcastToChannel 走每个 client subscription, 一
//      user 多 ws session 全收 (跟 P1MultiDeviceWebSocket #197 同源 +
//      Hub.onlineUsers map[userID]map[*Client]bool 数据结构).
//
// 反约束 (rt-3-spec §0 立场 ① + 蓝图 §1.1 ⭐ 关键纪律):
//   - state ∈ 2-enum {'busy', 'idle'}; 中间态 reject (跟 BPP-2.2 outcome
//     enum 同模式 fail-closed).
//   - busy 态 subject 必带非空 (蓝图 §1.1 字面 "BPP progress frame 强制带
//     subject 字段, plugin 必须告诉 Borgee 'agent 在做什么', 否则不展示").
//     反向 grep CI lint guards: empty subject default symbol /
//     fallback-named symbol / hard-coded vague strings — count==0 across
//     this file (excluding _test.go); 字面禁默认值 fallback (跟 BPP-2.2
//     task_lifecycle.go ValidateTaskStarted subject 必带非空 同源).
//   - idle 态 subject 必为空 (反字典污染, 跟 BPP-2.2 cancelled/completed
//     reason 必空 同模式).
//   - reason 仅 idle+failed-derived 时填, ∈ AL-1a 6 字典 byte-identical
//     (复用 internal/agent/state.go::Reason* SSOT).
package ws

// FrameTypeAgentTaskStateChanged is the `type` discriminator emitted on
// the `/ws` envelope; client switch lives in
// packages/client/src/realtime/wsClient.ts (RT-3.2 接).
const FrameTypeAgentTaskStateChanged = "agent_task_state_changed"

// AgentTaskState enum byte-identical 跟蓝图 realtime.md §1.1 + 蓝图
// agent-lifecycle.md §2.3 字面 (busy / idle 二分态).
const (
	AgentTaskStateBusy = "busy"
	AgentTaskStateIdle = "idle"
)

// AgentTaskStateChangedFrame — server → client push fired when an agent
// transitions busy↔idle. Server 派生于 BPP-2.2 task_started/finished 上行
// frame, 不是独立 plugin 上行 source — busy/idle 是 task lifecycle 的算法
//结果不是独立信号 (BPP-2 #485 (a) 派生 设计选择承袭).
//
// Field order is the contract. Do NOT reorder without updating
// packages/client/src/types/ws-frames.ts in the same PR.
type AgentTaskStateChangedFrame struct {
	Type      string `json:"type"`
	Cursor    int64  `json:"cursor"`
	AgentID   string `json:"agent_id"`
	State     string `json:"state"`   // 'busy' | 'idle'
	Subject   string `json:"subject"` // busy 时必带非空; idle 时空 (蓝图 §1.1 ⭐)
	Reason    string `json:"reason"`  // idle + failed-derived 时填 AL-1a 6 字典; 否则空
	ChangedAt int64  `json:"changed_at"` // Unix ms 语义戳; cursor IS the order
}

// PushAgentTaskStateChanged broadcasts AgentTaskStateChangedFrame to every
// channel member of channelID and signals long-poll waiters. Cursor is
// allocated fresh from hub.cursors so the frame slots into the same
// monotonic sequence as ArtifactUpdated / AnchorCommentAdded /
// MentionPushed / IterationStateChanged / AgentConfigUpdate (反约束:
// 不另起 agent-only push channel).
//
// Multi-device fanout: BroadcastToChannel walks every subscribed *Client
// (one user can have N concurrent /ws sessions, all subscribe → all
// receive — Hub.onlineUsers map[userID]map[*Client]bool 数据结构 + P1
// multi-device test #197 已验证).
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
