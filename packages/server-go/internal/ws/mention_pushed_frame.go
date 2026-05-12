// Package ws — mention_pushed_frame.go: DM-2.2 source-of-truth for the
// `mention_pushed` push frame. Server fans this out to a mention target
// (user OR agent — rule 6 uses one table with the same meaning) when the target
// is online; offline targets go through the owner system DM fallback path (DM-2.2
// mention_dispatch.go), which does NOT use this frame.
//
// Blueprint reference: docs/blueprint/current/concept-model.md §4 (agent 代表自己 —
// mention 只 ping target, 不抄送 owner) + §13 隐私默认.
// Spec brief: docs/implementation/modules/dm-2-spec.md §0 设计 ② + §1
// 拆段 DM-2.2 (#312, merged) + #362 spec brief 8 字段 envelope.
// Schema (DM-2.1): #361 message_mentions table (merged 2d2ac4e).
//
// Behaviour contract — follows the same wire pattern as RT-1.1 ArtifactUpdated +
// CV-2.2 AnchorCommentAdded:
//
//   1. Cursor uses hub.cursors.NextCursor() and shares one sequence with
//      ArtifactUpdated / AnchorCommentAdded (RT-1 spec §1.1: no mention-only push channel).
//   2. 字段顺序契约 (#362 spec brief): type / cursor / message_id /
//      channel_id / sender_id / mention_target_id / body_preview /
//      created_at — 8 字段, body_preview 80 字符截断 (UTF-8 rune-safe).
//   3. JSON tags must match client ws-frames.ts field names (BPP-1 #304 envelope
//      CI lint + DM-2.3 client wiring).
//
// Constraints (dm-2-spec.md §0 + §3 + #362 §0 decision 3):
//   - 此 frame 仅 push 给 target_id (BroadcastToUser), 不抄送 owner —
//     owner 路径走 mention_dispatch.go 的 enqueueOwnerSystemDM 系统 DM,
//     文案约定 byte-identical (#314 §1 ③) 不复用 body_preview 字符串.
//   - 不存在 mention_target_owner_id 字段 (决定 ③ 蓝图 §4).
//   - body_preview 80 字符截断而非 raw body — 防完整内容借 frame 泄露;
//     完整 body 仍走 new_message event (channel 成员授权路径).
//
// Phase 4 BPP cutover: bpp/frame_schemas.go will type-alias
// MentionPushedFrame, keeping the schema centralized here.
package ws

import "unicode/utf8"

// FrameTypeMentionPushed is the `type` discriminator emitted on the
// `/ws` envelope; client switch lives in
// packages/client/src/realtime/wsClient.ts (DM-2.3 接).
const FrameTypeMentionPushed = "mention_pushed"

// MentionPushedBodyPreviewMaxRunes is the rune-count cap for body_preview.
// 80 characters are fixed by #362 spec §0 design 2; prevent full raw body
// leakage to mention target
// (隐私 §13 — completion goes through new_message event 授权路径).
const MentionPushedBodyPreviewMaxRunes = 80

// MentionPushedFrame — server → client push fired when a message body
// `@<target_user_id>` token resolves to an online target. 8 fields, following
// dm-2-spec.md §0 design 2 + #362 spec brief envelope.
//
// Field order is the contract. Do NOT reorder without updating
// packages/client/src/types/ws-frames.ts in the same PR.
type MentionPushedFrame struct {
	Type             string `json:"type"`
	Cursor           int64  `json:"cursor"`
	MessageID        string `json:"message_id"`
	ChannelID        string `json:"channel_id"`
	SenderID         string `json:"sender_id"`
	MentionTargetID  string `json:"mention_target_id"`
	BodyPreview      string `json:"body_preview"`
	CreatedAt        int64  `json:"created_at"` // Unix ms
}

// TruncateBodyPreview clips body to MentionPushedBodyPreviewMaxRunes runes
// (UTF-8 rune-safe — does not split a multibyte rune mid-byte). Exposed
// (not unexported) so dispatch sites + tests share the exact same cap;
// caller/frame mismatch would re-introduce raw body leakage.
func TruncateBodyPreview(body string) string {
	if utf8.RuneCountInString(body) <= MentionPushedBodyPreviewMaxRunes {
		return body
	}
	out := make([]rune, 0, MentionPushedBodyPreviewMaxRunes)
	for i, r := range body {
		_ = i
		if len(out) >= MentionPushedBodyPreviewMaxRunes {
			break
		}
		out = append(out, r)
	}
	return string(out)
}

// PushMentionPushed delivers MentionPushedFrame to mentionTargetID via
// BroadcastToUser (target-only fanout — do not copy owner; offline
// fallback handled by api.MentionDispatcher, not here). Cursor is
// allocated fresh from hub.cursors so the frame slots into the same
// monotonic sequence as ArtifactUpdated / AnchorCommentAdded.
//
// Returns (cursor, sent). sent=false only when the hub has no cursor
// allocator (test seam), which mirrors PushArtifactUpdated / PushAnchorCommentAdded
// semantics.
//
// Caller is expected to truncate body via TruncateBodyPreview before
// invoking — keeps the cap visible at the dispatch site (api/mention_dispatch.go)
// where the privacy contract lives.
func (h *Hub) PushMentionPushed(
	messageID string,
	channelID string,
	senderID string,
	mentionTargetID string,
	bodyPreview string,
	createdAt int64,
) (cursor int64, sent bool) {
	if h.cursors == nil {
		return 0, false
	}
	cur := h.cursors.NextCursor()
	frame := MentionPushedFrame{
		Type:            FrameTypeMentionPushed,
		Cursor:          cur,
		MessageID:       messageID,
		ChannelID:       channelID,
		SenderID:        senderID,
		MentionTargetID: mentionTargetID,
		BodyPreview:     bodyPreview,
		CreatedAt:       createdAt,
	}
	h.BroadcastToUser(mentionTargetID, frame)
	h.SignalNewEvents()
	return cur, true
}
