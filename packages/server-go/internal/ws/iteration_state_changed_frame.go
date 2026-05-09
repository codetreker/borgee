// Package ws — iteration_state_changed_frame.go: CV-4.2 source-of-truth
// for the `iteration_state_changed` push frame. Iterations broadcast
// state transitions (pending/running/completed/failed) for an
// artifact_iterations row to the artifact's channel members.
//
// Blueprint锚: docs/blueprint/current/canvas-vision.md §1.4 (artifact 自带版本
// 历史: agent 每次修改产生一个版本) + §1.5 (agent 写内容默认允许).
// Spec brief: docs/implementation/modules/cv-4-spec.md §0 设计 ② CV-1
// commit 单源 + §1 拆段 CV-4.2 + 飞马 #365 envelope 9 字段字面.
// Content lock: docs/qa/cv-4-content-lock.md (野马 #380) state 4 态
// byte-identical + reason 三处单测锁.
//
// Behaviour contract — byte-identical 跟 RT-1.1 ArtifactUpdatedFrame /
// CV-2.2 AnchorCommentAddedFrame 同模式 (cursor 第二字段, 走同
// CursorAllocator 单调发号):
//
//   1. Cursor 走 hub.cursors.NextCursor() 单调发号, 跟 ArtifactUpdated /
//      AnchorCommentAdded 共一根 sequence (RT-1 spec §1.1, 反约束: 不另
//      起 channel).
//   2. 字段顺序锁: type/cursor/iteration_id/artifact_id/channel_id/state/
//      error_reason/created_artifact_version_id/completed_at
//      (acceptance §2.4 字面 + spec #365 envelope 9 字段, 跟 ArtifactUpdated
//      7 / AnchorCommentAdded 10 / MentionPushed 8 共序 type/cursor 头位).
//   3. JSON tag 跟客户端 ws-frames.ts 字段名严格一致 (BPP-1 #304 envelope
//      CI lint 自动闸位).
//
// 反约束: error_reason / created_artifact_version_id / completed_at
// 三字段在 pending/running 态时为零值 (string="" / int64=0), 始终序列化 —
// JSON byte-identity 不分支 (跟 AnchorComment resolved_at 模式不同 — 此
// frame 无 *T 指针, 客户端按零值与 state 字段联判, 反约束: 不挂 omitempty).
package ws

// FrameTypeIterationStateChanged is the `type` discriminator emitted on
// the `/ws` envelope; client switch lives in
// packages/client/src/realtime/wsClient.ts (CV-4.3 接).
const FrameTypeIterationStateChanged = "iteration_state_changed"

// IterationState 4 态 byte-identical 跟野马 #380 文案锁 ③ 同源 + 跟
// migration v=18 cv_4_1_artifact_iterations CHECK 字面.
const (
	IterationStatePending   = "pending"
	IterationStateRunning   = "running"
	IterationStateCompleted = "completed"
	IterationStateFailed    = "failed"
)

// IterationStateChangedFrame — server → client push fired on each
// state transition of an artifact_iterations row. 9 字段, 严守 cv-4-spec.md
// 飞马 #365 envelope 字面 + acceptance §2.4 byte-identical.
//
// Field order is the contract. Do NOT reorder without updating
// packages/client/src/types/ws-frames.ts in the same PR.
type IterationStateChangedFrame struct {
	Type                     string `json:"type"`
	Cursor                   int64  `json:"cursor"`
	IterationID              string `json:"iteration_id"`
	ArtifactID               string `json:"artifact_id"`
	ChannelID                string `json:"channel_id"`
	State                    string `json:"state"` // 'pending'|'running'|'completed'|'failed'
	ErrorReason              string `json:"error_reason"`
	CreatedArtifactVersionID int64  `json:"created_artifact_version_id"`
	CompletedAt              int64  `json:"completed_at"` // Unix ms; 0 when not yet completed/failed
}

// PushIterationStateChanged broadcasts IterationStateChangedFrame to every
// member of channelID and signals long-poll waiters. Cursor is allocated
// fresh from hub.cursors so the frame slots into the same monotonic
// sequence as ArtifactUpdated / AnchorCommentAdded (反约束: 不另起
// channel).
//
// Returns (cursor, sent). sent=false only when the hub has no cursor
// allocator (test seam).
func (h *Hub) PushIterationStateChanged(
	iterationID string,
	artifactID string,
	channelID string,
	state string,
	errorReason string,
	createdArtifactVersionID int64,
	completedAt int64,
) (cursor int64, sent bool) {
	if h.cursors == nil {
		return 0, false
	}
	cur := h.cursors.NextCursor()
	frame := IterationStateChangedFrame{
		Type:                     FrameTypeIterationStateChanged,
		Cursor:                   cur,
		IterationID:              iterationID,
		ArtifactID:               artifactID,
		ChannelID:                channelID,
		State:                    state,
		ErrorReason:              errorReason,
		CreatedArtifactVersionID: createdArtifactVersionID,
		CompletedAt:              completedAt,
	}
	if channelID == "" {
		h.BroadcastToAll(frame)
	} else {
		h.BroadcastToChannel(channelID, frame, nil)
	}
	h.SignalNewEvents()
	return cur, true
}
