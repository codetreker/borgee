// Package ws — artifact_comment_added_frame.go: CV-5 source-of-truth for
// the `artifact_comment_added` push frame. Sent to every member of the
// virtual `artifact:<artifactId>` channel when a comment is posted on
// an artifact (canvas-vision §0 L24 字面 "Linear issue + comment").
//
// Blueprint锚: docs/blueprint/current/canvas-vision.md L24 + RT-3 #488 hub.cursors
// 共序锚 + DM-2.2 #372 MentionPushedFrame 同模式 (8 字段).
// Spec brief: docs/implementation/modules/cv-5-spec.md §0 设计 ② + §1.
//
// Behaviour contract — byte-identical 跟 RT-1.1 ArtifactUpdatedFrame /
// CV-2.2 AnchorCommentAddedFrame / DM-2.2 MentionPushedFrame:
//
//   1. Cursor 走 hub.cursors.NextCursor() 单调发号, 共一根 sequence
//      (RT-3 #488 cursor 共序锚).
//   2. 字段顺序锁: type/cursor/comment_id/artifact_id/channel_id/
//      sender_id/sender_role/body_preview/created_at (9 字段, body_preview
//      80 rune 截断 — 跟 DM-2.2 隐私 §13 同 cap).
//   3. JSON tag 跟客户端 ws-frames.ts 字段名严格一致.
//
// 反约束 (cv-5-spec.md §0 设计 ②):
//   - frame 仅 fan-out 给 artifact: namespace channel 成员 (BroadcastToChannel),
//     不挂 admin god-mode 抄送 (ADM-0 §1.3 红线).
//   - body_preview 80 rune 截断 (隐私 §13).
package ws

// FrameTypeArtifactCommentAdded is the `type` discriminator emitted on
// the `/ws` envelope; client switch lives in
// packages/client/src/realtime/wsClient.ts (CV-5.2 接).
const FrameTypeArtifactCommentAdded = "artifact_comment_added"

// ArtifactCommentBodyPreviewMaxRunes is the rune-count cap (跟 DM-2.2
// MentionPushed 80 同 cap, 隐私 §13).
const ArtifactCommentBodyPreviewMaxRunes = 80

// ArtifactCommentAddedFrame — server → client push fired after a comment
// lands on an artifact. 9 字段, 严守 cv-5-spec.md §0 设计 ② 字面.
//
// Field order is the contract. Do NOT reorder without updating
// packages/client/src/types/ws-frames.ts in the same PR.
type ArtifactCommentAddedFrame struct {
	Type        string `json:"type"`
	Cursor      int64  `json:"cursor"`
	CommentID   string `json:"comment_id"`
	ArtifactID  string `json:"artifact_id"`
	ChannelID   string `json:"channel_id"`
	SenderID    string `json:"sender_id"`
	SenderRole  string `json:"sender_role"` // 'human' | 'agent'
	BodyPreview string `json:"body_preview"`
	CreatedAt   int64  `json:"created_at"` // Unix ms
}

// PushArtifactCommentAdded broadcasts ArtifactCommentAddedFrame to every
// member of channelID and signals long-poll waiters. Cursor is allocated
// fresh from hub.cursors so the frame slots into the same monotonic
// sequence as RT-1.1 ArtifactUpdated / CV-2.2 AnchorCommentAdded /
// DM-2.2 MentionPushed / RT-3 AgentTaskStateChanged.
//
// Returns (cursor, sent). sent=false only when the hub has no cursor
// allocator (test seam).
func (h *Hub) PushArtifactCommentAdded(
	commentID string,
	artifactID string,
	channelID string,
	senderID string,
	senderRole string,
	bodyPreview string,
	createdAt int64,
) (cursor int64, sent bool) {
	if h.cursors == nil {
		return 0, false
	}
	cur := h.cursors.NextCursor()
	frame := ArtifactCommentAddedFrame{
		Type:        FrameTypeArtifactCommentAdded,
		Cursor:      cur,
		CommentID:   commentID,
		ArtifactID:  artifactID,
		ChannelID:   channelID,
		SenderID:    senderID,
		SenderRole:  senderRole,
		BodyPreview: bodyPreview,
		CreatedAt:   createdAt,
	}
	if channelID == "" {
		h.BroadcastToAll(frame)
	} else {
		h.BroadcastToChannel(channelID, frame, nil)
	}
	h.SignalNewEvents()
	return cur, true
}
