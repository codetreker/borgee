// Package ws — iteration_state_changed_frame.go: CV-4.2 source-of-truth
// for the `iteration_state_changed` push frame. Iterations broadcast
// state transitions (pending/running/completed/failed) for an
// artifact_iterations row to the artifact's channel members.
//
// Blueprint reference: docs/blueprint/current/canvas-vision.md §1.4 (artifact 自带版本
// 历史: agent 每次修改产生一个版本) + §1.5 (agent 写内容默认允许).
// Spec brief: docs/implementation/modules/cv-4-spec.md §0 point 2 CV-1
// commit single source + §1 CV-4.2 breakdown + #365 envelope 9-field text.
// Content lock: docs/qa/cv-4-content-lock.md (#380) state 4-state enum
// + reason coverage in three unit tests.
//
// Behaviour contract — follows the same wire pattern as RT-1.1 ArtifactUpdatedFrame /
// CV-2.2 AnchorCommentAddedFrame (cursor is the second field and uses the same
// CursorAllocator 单调发号):
//
//   1. Cursor uses hub.cursors.NextCursor() and shares one sequence with
//      ArtifactUpdated / AnchorCommentAdded (RT-1 spec §1.1: no separate channel).
//   2. Field order contract: type/cursor/iteration_id/artifact_id/channel_id/state/
//      error_reason/created_artifact_version_id/completed_at
//      (acceptance §2.4 text + spec #365 envelope 9 fields, sharing the
//      type/cursor prefix with ArtifactUpdated 7 / AnchorCommentAdded 10 / MentionPushed 8).
//   3. JSON tags must match client ws-frames.ts field names (BPP-1 #304 envelope
//      CI lint).
//
// Negative constraint: error_reason / created_artifact_version_id / completed_at
// 三字段在 pending/running 态时为零值 (string="" / int64=0), 始终序列化 —
// JSON byte-identity does not branch (unlike AnchorComment resolved_at — this
// frame has no *T pointer; client interprets zero values with state, so do not add omitempty).
package ws

// FrameTypeIterationStateChanged is the `type` discriminator emitted on
// the `/ws` envelope; client switch lives in
// packages/client/src/realtime/wsClient.ts (CV-4.3 接).
const FrameTypeIterationStateChanged = "iteration_state_changed"

// IterationState 4-state enum matches #380 wording point 3 and
// migration v=18 cv_4_1_artifact_iterations CHECK text.
const (
	IterationStatePending   = "pending"
	IterationStateRunning   = "running"
	IterationStateCompleted = "completed"
	IterationStateFailed    = "failed"
)

// IterationStateChangedFrame — server → client push fired on each
// state transition of an artifact_iterations row. 9 fields, following cv-4-spec.md
// #365 envelope text + acceptance §2.4.
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
// sequence as ArtifactUpdated / AnchorCommentAdded (no separate channel).
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
