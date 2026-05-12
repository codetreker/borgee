// Package ws — anchor_comment_frame.go: CV-2.2 (#NNN) source-of-truth
// for the `anchor_comment_added` push frame. Anchors review-comments
// posted on artifact_versions to the artifact's channel members.
//
// Blueprint reference: docs/blueprint/current/canvas-vision.md §1.6 (锚点对话 = owner
// review agent 产物的工具). Spec brief: docs/implementation/modules/cv-2-spec.md
// §0 points 1 + 3 + §1 CV-2.2 breakdown + spec v2 envelope (10 fields, field name
// `author_kind` 不复用 CV-1 `committer_kind`).
//
// Behaviour contract — follows the same wire pattern as RT-1.1 ArtifactUpdatedFrame
// (cursor is the second field and uses the same CursorAllocator monotonic sequence):
//
//   1. Cursor uses hub.cursors.NextCursor() and shares one sequence with ArtifactUpdated
//      (RT-1 spec §1.1: no separate channel).
//   2. Field order contract: type/cursor/anchor_id/comment_id/artifact_id/
//      artifact_version_id/channel_id/author_id/author_kind/created_at
//      (cv-2-spec.md §0 立场 ③ + 飞马 v2 changelog 字面 — 第 9 字段
//      `author_kind` 不是 `kind`, 跟 anchor_comments.author_kind 列名
//      一致, anchor 是评论作者非 commit 提交者).
//   3. JSON tags must match client ws-frames.ts field names (BPP-1 #304 envelope
//      CI lint).
//
// Phase 4 BPP cutover: bpp/frame_schemas.go will type-alias
// AnchorCommentAddedFrame, keeping the schema centralized here.
package ws

// FrameTypeAnchorCommentAdded is the `type` discriminator emitted on
// the `/ws` envelope; client switch lives in
// packages/client/src/realtime/wsClient.ts (CV-2.3 接).
const FrameTypeAnchorCommentAdded = "anchor_comment_added"

// AnchorCommentAddedFrame — server → client push fired after a comment
// lands on an active anchor thread. 10 fields, following cv-2-spec.md §0 point 3
// + v2 changelog text.
//
// Field order is the contract. Do NOT reorder without updating
// packages/client/src/types/ws-frames.ts in the same PR.
type AnchorCommentAddedFrame struct {
	Type              string `json:"type"`
	Cursor            int64  `json:"cursor"`
	AnchorID          string `json:"anchor_id"`
	CommentID         int64  `json:"comment_id"`
	ArtifactID        string `json:"artifact_id"`
	ArtifactVersionID int64  `json:"artifact_version_id"`
	ChannelID         string `json:"channel_id"`
	AuthorID          string `json:"author_id"`
	AuthorKind        string `json:"author_kind"` // 'human' | 'agent' (注: 不是 committer_kind)
	CreatedAt         int64  `json:"created_at"`  // Unix ms
}

// PushAnchorCommentAdded broadcasts AnchorCommentAddedFrame to every
// member of channelID and signals long-poll waiters. Cursor is allocated
// fresh from hub.cursors so the frame slots into the same monotonic
// sequence as ArtifactUpdated (no separate channel).
//
// Returns (cursor, sent). sent=false only when the hub has no cursor
// allocator (test seam), which mirrors PushArtifactUpdated semantics.
func (h *Hub) PushAnchorCommentAdded(
	anchorID string,
	commentID int64,
	artifactID string,
	artifactVersionID int64,
	channelID string,
	authorID string,
	authorKind string,
	createdAt int64,
) (cursor int64, sent bool) {
	if h.cursors == nil {
		return 0, false
	}
	cur := h.cursors.NextCursor()
	frame := AnchorCommentAddedFrame{
		Type:              FrameTypeAnchorCommentAdded,
		Cursor:            cur,
		AnchorID:          anchorID,
		CommentID:         commentID,
		ArtifactID:        artifactID,
		ArtifactVersionID: artifactVersionID,
		ChannelID:         channelID,
		AuthorID:          authorID,
		AuthorKind:        authorKind,
		CreatedAt:         createdAt,
	}
	if channelID == "" {
		h.BroadcastToAll(frame)
	} else {
		h.BroadcastToChannel(channelID, frame, nil)
	}
	h.SignalNewEvents()
	return cur, true
}
