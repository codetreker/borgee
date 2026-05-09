package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"borgee-server/internal/auth"
	"borgee-server/internal/store"
)

// MessageHandler handles message CRUD endpoints.
//
// MentionDispatcher (DM-2.2, #312) is optional — nil at boot means
// mention parser + offline fallback are off (legacy / test paths).
// When non-nil, handleCreateMessage parses `@<uuid>` tokens, rejects
// cross-channel targets with 400 mention.target_not_in_channel, persists
// message_mentions rows (#361), and fans out mention_pushed frames or
// owner system DM fallback (acceptance §1.1-§2.5).
type MessageHandler struct {
	Store      *store.Store
	Logger     *slog.Logger
	Hub        EventBroadcaster
	Mentions   *MentionDispatcher
}

type EventBroadcaster interface {
	BroadcastEventToChannel(channelID string, eventType string, payload any)
	BroadcastEventToAll(eventType string, payload any)
	BroadcastEventToUser(userID string, eventType string, payload any)
	SignalNewEvents()
}

func (h *MessageHandler) RegisterRoutes(mux *http.ServeMux, authMw func(http.Handler) http.Handler, sendPerm func(http.Handler) http.Handler, readPerm func(http.Handler) http.Handler) {
	// Channel-scoped routes (need auth)
	// AP-0-bis: GET /channels/:id/messages now requires `message.read`
	// capability. New agents get this in default grants; legacy agents get
	// backfilled by migration v=8 (ap_0_bis_message_read.go). Reverse
	// assertion: agent without message.read row → 403 (see messages_perm_test.go).
	mux.Handle("GET /api/v1/channels/{channelId}/messages", authMw(readPerm(http.HandlerFunc(h.handleListMessages))))
	mux.Handle("GET /api/v1/channels/{channelId}/messages/search", authMw(readPerm(http.HandlerFunc(h.handleSearchMessages))))
	mux.Handle("POST /api/v1/channels/{channelId}/messages", authMw(sendPerm(http.HandlerFunc(h.handleCreateMessage))))

	// Message-scoped routes (need auth)
	mux.Handle("PUT /api/v1/messages/{messageId}", authMw(http.HandlerFunc(h.handleUpdateMessage)))
	mux.Handle("DELETE /api/v1/messages/{messageId}", authMw(http.HandlerFunc(h.handleDeleteMessage)))
}

// GET /api/v1/channels/:channelId/messages?before=&after=&limit=
func (h *MessageHandler) handleListMessages(w http.ResponseWriter, r *http.Request) {
	channelID := r.PathValue("channelId")
	if channelID == "" {
		writeJSONError(w, http.StatusBadRequest, "Channel ID is required")
		return
	}

	user := auth.UserFromContext(r.Context())

	// Validate channel exists
	ch, err := h.Store.GetChannelByID(channelID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Channel not found")
		return
	}

	// Private channel access check
	if ch.Visibility == "private" {
		if user == nil || !h.Store.CanAccessChannel(channelID, user.ID) {
			writeJSONError(w, http.StatusNotFound, "Channel not found")
			return
		}
	}

	// Parse query params
	var before, after *int64
	if v := r.URL.Query().Get("before"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			before = &n
		}
	}
	if v := r.URL.Query().Get("after"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			after = &n
		}
	}

	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			if n > 200 {
				n = 200
			}
			limit = n
		}
	}

	msgs, hasMore, err := h.Store.ListChannelMessages(channelID, before, after, limit)
	if err != nil {
		h.Logger.Error("failed to list messages", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	type messageWithReactions struct {
		store.MessageWithSender
		Reactions []store.AggregatedReaction `json:"reactions"`
	}
	// TODO: N+1 query — each message triggers a separate DB query for reactions.
	// Optimize with batch query: SELECT ... WHERE message_id IN (...) grouped by message_id.
	out := make([]messageWithReactions, len(msgs))
	for i, msg := range msgs {
		reactions, err := h.Store.GetReactionsByMessage(msg.ID)
		if err != nil {
			h.Logger.Error("failed to get message reactions", "error", err)
			writeJSONError(w, http.StatusInternalServerError, "Internal server error")
			return
		}
		if reactions == nil {
			reactions = []store.AggregatedReaction{}
		}
		out[i] = messageWithReactions{MessageWithSender: msg, Reactions: reactions}
	}

	writeJSONResponse(w, http.StatusOK, map[string]any{
		"messages": out,
		"has_more": hasMore,
	})
}

// GET /api/v1/channels/:channelId/messages/search?q=&limit=
func (h *MessageHandler) handleSearchMessages(w http.ResponseWriter, r *http.Request) {
	channelID := r.PathValue("channelId")
	if channelID == "" {
		writeJSONError(w, http.StatusBadRequest, "Channel ID is required")
		return
	}

	user := auth.UserFromContext(r.Context())

	// Validate channel exists
	ch, err := h.Store.GetChannelByID(channelID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Channel not found")
		return
	}

	if ch.Visibility == "private" {
		if user == nil || !h.Store.CanAccessChannel(channelID, user.ID) {
			writeJSONError(w, http.StatusNotFound, "Channel not found")
			return
		}
	}

	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		writeJSONError(w, http.StatusBadRequest, "Search query (q) is required")
		return
	}

	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			if n > 50 {
				n = 50
			}
			limit = n
		}
	}

	msgs, err := h.Store.SearchMessages(channelID, q, limit)
	if err != nil {
		h.Logger.Error("failed to search messages", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	writeJSONResponse(w, http.StatusOK, map[string]any{"messages": msgs})
}

// POST /api/v1/channels/:channelId/messages
func (h *MessageHandler) handleCreateMessage(w http.ResponseWriter, r *http.Request) {
	channelID := r.PathValue("channelId")
	if channelID == "" {
		writeJSONError(w, http.StatusBadRequest, "Channel ID is required")
		return
	}

	user, ok := mustUser(w, r)
	if !ok {
		return
	}

	var body struct {
		Content     string   `json:"content"`
		ContentType string   `json:"content_type"`
		ReplyToID   *string  `json:"reply_to_id"`
		Mentions    []string `json:"mentions"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}

	content := strings.TrimSpace(body.Content)
	if content == "" {
		writeJSONError(w, http.StatusBadRequest, "Message content is required")
		return
	}

	ct := body.ContentType
	if ct == "" {
		ct = "text"
	}
	if ct != "text" && ct != "image" && ct != "command" && ct != "artifact_comment" {
		writeJSONError(w, http.StatusBadRequest, "content_type must be 'text', 'image', 'command', or 'artifact_comment'")
		return
	}

	// Validate channel exists
	ch, err := h.Store.GetChannelByID(channelID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Channel not found")
		return
	}

	// Private channel access check
	if ch.Visibility == "private" && !h.Store.CanAccessChannel(channelID, user.ID) {
		writeJSONError(w, http.StatusNotFound, "Channel not found")
		return
	}

	// Must be a member
	if !h.Store.IsChannelMember(channelID, user.ID) {
		writeJSONError(w, http.StatusForbidden, "Not a member of this channel")
		return
	}

	// CHN-15.2 readonly gate — readonly=true 时 non-creator 发消息 → 403
	// `channel.readonly_no_send` 字面 byte-identical 跟 content-lock §3.
	// creator 自己仍可发 (反向断言不误伤). admin god-mode 不入此路径
	// (ADM-0 §1.3 红线; admin 无 user.ID 走 user-rail authMw).
	if user.ID != ch.CreatedBy {
		readonly, _ := h.Store.GetChannelReadonly(channelID)
		if readonly {
			writeJSONError(w, http.StatusForbidden, ChannelErrCodeReadonlyNoSend)
			return
		}
	}

	// DM-2.2 mention parser + cross-channel guard.
	// 设计 ① parse `@<uuid>` token 拆死 raw UUID; 设计 ② cross-channel
	// reject 400 (mention 仅在 channel 内, 跟 RT-1/CHN-1 留账边界对齐).
	// Dispatch (online push / offline owner DM fallback) 在 message
	// 落库后执行 — 失败仅 log 不阻断 message 创建 (best-effort fanout,
	// 反约束 (§2.4): fallback 走 owner 后台不污染发送方).
	var parsedMentionTargets []string
	if h.Mentions != nil {
		targets, offender, mErr := h.Mentions.MentionTargetsFromBody(channelID, content)
		if mErr != nil {
			// ErrMentionTargetNotInChannel → 400 with offender hint.
			writeJSONError(w, http.StatusBadRequest, "mention.target_not_in_channel: "+offender)
			return
		}
		parsedMentionTargets = targets
	}

	// CV-8: artifact comment thread reply validators (1-level depth + agent
	// thinking subject 5-pattern 第 6 处链 RT-3 + BPP-2.2 + AL-1b + CV-5 +
	// CV-7 + CV-8 byte-identical). Only fires on artifact_comment with
	// reply_to_id; non-comment paths unchanged.
	if ct == "artifact_comment" && body.ReplyToID != nil && *body.ReplyToID != "" {
		parent, perr := h.Store.GetMessageByID(*body.ReplyToID)
		if perr != nil {
			writeJSONErrorCode(w, http.StatusBadRequest, "comment.reply_target_invalid", "reply target not found")
			return
		}
		if parent.ContentType != "artifact_comment" {
			writeJSONErrorCode(w, http.StatusBadRequest, "comment.reply_target_invalid", "reply target must be an artifact comment")
			return
		}
		if parent.ReplyToID != nil && *parent.ReplyToID != "" {
			writeJSONErrorCode(w, http.StatusBadRequest, "comment.thread_depth_exceeded", "thread depth limited to 1 level")
			return
		}
		// Agent senders must pass the 5-pattern thinking-subject guard
		// byte-identical to CV-5 / CV-7 (errcode 同字符串).
		if user.Role == "agent" && violatesThinkingSubjectCV8(content) {
			writeJSONErrorCode(w, http.StatusBadRequest, "comment.thinking_subject_required",
				"agent comment must carry a concrete subject (thinking-only body rejected)")
			return
		}
	}

	msg, err := h.Store.CreateMessageFull(channelID, user.ID, content, ct, body.ReplyToID, body.Mentions)
	if err != nil {
		h.Logger.Error("failed to create message", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	// #687 Layer 1 — sender 发完消息顺手 mark-read 自己 channel 的 last_read_at,
	// 反 "自己发的消息对自己显未读" 的 UX bug. best-effort: 失败仅 log 不阻断
	// 主流程 (反约束: unread 是 UX, 不能让 5xx 把发消息挡住; Layer 2 SQL
	// `m.sender_id != ?` 兜底 + Layer 3 client reducer 拦 own message).
	// sender 一定是 channel member (上面 IsChannelMember gate 过), MarkChannelRead
	// 在 channel_members.last_read_at = now 上做等值更新.
	if mrErr := h.Store.MarkChannelRead(channelID, user.ID); mrErr != nil {
		h.Logger.Error("failed to update sender last_read_at after message create",
			"error", mrErr, "channel_id", channelID, "user_id", user.ID)
	}

	// DM-2.2 persist + dispatch — non-blocking on errors (log + continue).
	// PersistMentions writes #361 message_mentions rows; Dispatch fans
	// online targets via PushMentionPushed and offline agents via owner
	// system DM (#314 §1 ③ byte-identical body).
	if h.Mentions != nil && len(parsedMentionTargets) > 0 {
		if pErr := h.Mentions.PersistMentions(msg.ID, parsedMentionTargets); pErr != nil {
			h.Logger.Error("failed to persist mentions", "error", pErr, "message_id", msg.ID)
		}
		if dErr := h.Mentions.Dispatch(msg.ID, channelID, ch.Name, user.ID, content, parsedMentionTargets, msg.CreatedAt); dErr != nil {
			h.Logger.Error("mention dispatch partial failure", "error", dErr, "message_id", msg.ID)
		}
	}

	h.Store.CreateEvent(&store.Event{
		Kind:      "new_message",
		ChannelID: channelID,
		Payload:   mustJSON(map[string]any{"message": msg}),
	})
	if h.Hub != nil {
		h.Hub.BroadcastEventToChannel(channelID, "new_message", map[string]any{"message": msg})
	}

	writeJSONResponse(w, http.StatusCreated, map[string]any{"message": msg})
}

// PUT /api/v1/messages/:messageId
func (h *MessageHandler) handleUpdateMessage(w http.ResponseWriter, r *http.Request) {
	messageID := r.PathValue("messageId")
	if messageID == "" {
		writeJSONError(w, http.StatusBadRequest, "Message ID is required")
		return
	}

	user, ok := mustUser(w, r)
	if !ok {
		return
	}

	var body struct {
		Content string `json:"content"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}

	content := strings.TrimSpace(body.Content)
	if content == "" {
		writeJSONError(w, http.StatusBadRequest, "Content is required")
		return
	}

	existing, err := h.Store.GetMessageByID(messageID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Message not found")
		return
	}

	if existing.DeletedAt != nil {
		writeJSONError(w, http.StatusBadRequest, "Cannot edit deleted message")
		return
	}

	// CM-3.2: cross-org 403 (#200 §3 row 1).
	// MUST run BEFORE the AP-5 channel-member gate — cross-org contract
	// (cm-3-resource-ownership-checklist.md §3) explicitly returns 403 for
	// foreign-org callers (TestCrossOrgRead403 lock).
	if store.CrossOrg(user.OrgID, existing.OrgID) {
		writeJSONError(w, http.StatusForbidden, "Forbidden")
		return
	}

	// AP-5 设计 ① — channel-member ACL gate (跟 AP-4 reactions 同模式).
	// Closes the post-removal gap: a same-org user removed from the
	// channel must not be able to edit messages they previously sent
	// there. byte-identical "Channel not found" 404 fail-closed.
	if !h.Store.IsChannelMember(existing.ChannelID, user.ID) || !h.Store.CanAccessChannel(existing.ChannelID, user.ID) {
		writeJSONError(w, http.StatusNotFound, "Channel not found")
		return
	}

	if existing.SenderID != user.ID {
		writeJSONError(w, http.StatusForbidden, "Can only edit your own messages")
		return
	}

	// CV-7 设计 ③: agent edit on artifact-comment-typed message must
	// re-pass the 5-pattern thinking-subject guard. byte-identical 跟
	// CV-5 #530 artifact_comments.go::violatesThinkingSubject — 5-pattern
	// 第 5 处链 (RT-3 + BPP-2.2 + AL-1b + CV-5 + CV-7). 5-pattern 改 =
	// 改 5 处 byte-identical.
	if existing.ContentType == "artifact_comment" {
		sender, _ := h.Store.GetUserByID(existing.SenderID)
		if sender != nil && sender.Role == "agent" && violatesThinkingSubjectCV7(content) {
			writeJSONErrorCode(w, http.StatusBadRequest, "comment.thinking_subject_required",
				"agent comment must carry a concrete subject (thinking-only body rejected)")
			return
		}
	}

	msg, err := h.Store.UpdateMessage(messageID, content)
	if err != nil {
		h.Logger.Error("failed to update message", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	// Write edit event
	h.Store.CreateEvent(&store.Event{
		Kind:      "message_edited",
		ChannelID: existing.ChannelID,
		Payload:   mustJSON(map[string]any{"id": messageID, "channel_id": existing.ChannelID, "sender_id": user.ID, "content": content, "system_message": "用户 " + user.DisplayName + " 编辑了消息"}),
	})
	if h.Hub != nil {
		h.Hub.BroadcastEventToChannel(existing.ChannelID, "message_edited", map[string]any{"message": msg})
	}

	writeJSONResponse(w, http.StatusOK, map[string]any{"message": msg})
}

// DELETE /api/v1/messages/:messageId
func (h *MessageHandler) handleDeleteMessage(w http.ResponseWriter, r *http.Request) {
	messageID := r.PathValue("messageId")
	if messageID == "" {
		writeJSONError(w, http.StatusBadRequest, "Message ID is required")
		return
	}

	user, ok := mustUser(w, r)
	if !ok {
		return
	}

	existing, err := h.Store.GetMessageByID(messageID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Message not found")
		return
	}

	// Already deleted — idempotent
	if existing.DeletedAt != nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// CM-3.2: cross-org 403. MUST run BEFORE the AP-5 channel-member gate
	// — cross-org contract returns 403 for foreign-org callers
	// (TestCrossOrgRead403 lock).
	if store.CrossOrg(user.OrgID, existing.OrgID) {
		writeJSONError(w, http.StatusForbidden, "Forbidden")
		return
	}

	// AP-5 设计 ① — channel-member ACL gate (跟 AP-4 + handleUpdateMessage 同模式).
	// Closes the post-removal gap on DELETE.
	if !h.Store.IsChannelMember(existing.ChannelID, user.ID) || !h.Store.CanAccessChannel(existing.ChannelID, user.ID) {
		writeJSONError(w, http.StatusNotFound, "Channel not found")
		return
	}

	// ADM-0.3: no role short-circuit. Sender-only delete on the user-rail;
	// admin-rail message delete uses /admin-api/v1/messages.
	if existing.SenderID != user.ID {
		writeJSONError(w, http.StatusForbidden, "Permission denied")
		return
	}

	deletedAt, err := h.Store.SoftDeleteMessage(messageID)
	if err != nil {
		h.Logger.Error("failed to delete message", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	// Write delete event
	h.Store.CreateEvent(&store.Event{
		Kind:      "message_deleted",
		ChannelID: existing.ChannelID,
		Payload:   mustJSON(map[string]any{"message_id": messageID, "channel_id": existing.ChannelID, "deleted_at": deletedAt, "sender_id": user.ID, "system_message": "用户 " + user.DisplayName + " 删除了一条消息"}),
	})
	if h.Hub != nil {
		h.Hub.BroadcastEventToChannel(existing.ChannelID, "message_deleted", map[string]any{"message_id": messageID, "channel_id": existing.ChannelID, "deleted_at": deletedAt})
	}

	w.WriteHeader(http.StatusNoContent)
}

// mustJSON marshals v to JSON string, returning "{}" on error.
func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}
