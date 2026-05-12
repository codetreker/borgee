// Package ws — artifact_comment_added_frame.go: CV-5 source-of-truth for
// the `artifact_comment_added` push frame. Sent to every member of the
// virtual `artifact:<artifactId>` channel when a comment is posted on
// an artifact (canvas-vision §0 L24 wording: "Linear issue + comment").
//
// Blueprint anchor: docs/blueprint/current/canvas-vision.md L24 + RT-3 #488
// hub.cursors shared-sequence anchor + DM-2.2 #372 MentionPushedFrame pattern
// (8 fields). Spec brief: docs/implementation/modules/cv-5-spec.md §0 principle ② + §1.
//
// Behaviour contract — byte-identical with RT-1.1 ArtifactUpdatedFrame /
// CV-2.2 AnchorCommentAddedFrame / DM-2.2 MentionPushedFrame:
//
//   1. Cursor uses hub.cursors.NextCursor() for a monotonic value in the shared
//      sequence (RT-3 #488 cursor shared-sequence anchor).
//   2. Field order lock: type/cursor/comment_id/artifact_id/channel_id/
//      sender_id/sender_role/body_preview/created_at (9 fields; body_preview is
//      truncated to 80 runes, same cap as DM-2.2 privacy §13).
//   3. JSON tags must exactly match client ws-frames.ts field names.
//
// Negative constraints (cv-5-spec.md §0 principle ②):
//   - frame fan-out goes only to artifact namespace channel members
//     (BroadcastToChannel). There is no admin copy path (ADM-0 §1.3
//     prohibition).
//   - body_preview is truncated to 80 runes (privacy §13).
package ws

// FrameTypeArtifactCommentAdded is the `type` discriminator emitted on
// the `/ws` envelope; client switch lives in
// packages/client/src/realtime/wsClient.ts (CV-5.2 接).
const FrameTypeArtifactCommentAdded = "artifact_comment_added"

// ArtifactCommentBodyPreviewMaxRunes is the rune-count cap (跟 DM-2.2
// MentionPushed 80 same cap, privacy §13).
const ArtifactCommentBodyPreviewMaxRunes = 80

// ArtifactCommentAddedFrame — server → client push fired after a comment lands
// on an artifact. 9 fields, following cv-5-spec.md §0 principle ② wording.
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
