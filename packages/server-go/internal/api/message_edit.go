// Package api — dm_4_message_edit.go: DM-4.1 server PATCH endpoint
// for agent message edit 多端同步. Wraps existing message edit (PUT
// /api/v1/messages/{id}) in a DM-scoped path that validates
// channel.kind == "dm" before delegating to the same store helpers.
//
// Blueprint锚: docs/blueprint/channels-dm-collab.md §3 (DM 编辑) +
// RT-3 #488 (多端 fan-out).
// Spec: docs/implementation/modules/dm-4-spec.md §0+§1 DM-4.1.
// Acceptance: docs/qa/acceptance-templates/dm-4.md §1.
//
// 设计 (跟 stance §1+§2+§3+§4 byte-identical):
//   - **DM 编辑同步走 RT-3 既有 fan-out** — PATCH 复用 messages.UpdateMessage
//     + events INSERT kind="message_edited" (BroadcastEventToChannel 触发
//     RT-3 fan-out 多端覆盖). 不另起 channel/frame/sequence — spec §0.1.
//   - **edit 是 cursor 子集** — 复用 events 表既有 sequence; useDMSync (DM-3
//     #508) 客户端订阅 channel events backfill 自动 derive edit 状态.
//     spec §0.2.
//   - **thinking subject 5-pattern 反约束延伸第 3 处** — agent edit 是机械
//     修订, 不暴露 reasoning. grep 检查 5-pattern 在 dm_4*.go 0 hit (RT-3
//     第 1 + DM-3 第 2 + DM-4 第 3). spec §0.3.
//   - **DM-only path** — channel.kind != "dm" reject 403 `dm.edit_only_in_dm`.
//   - **owner-only ACL** — sender == user (跟 PUT /api/v1/messages/{id} 既有
//     ACL 同精神; 跟 AL-2a/BPP-3.2/AL-1/AL-5 owner-only 5 处同模式).
//   - **last-write-wins simplification** — 不挂 edit history audit table,
//     不挂 OT/CRDT (留 v2, spec §2 留账).
//   - **admin god-mode 红线** — admin 不持 user token, 不入 PATCH messages
//     业务 (ADM-0 §1.3).

package api

import (
	"log/slog"
	"net/http"
	"strings"

	"borgee-server/internal/store"
)

// MessageEditHandler is the DM-4.1 PATCH endpoint dispatcher.
// Delegates to MessageHandler.Store + Hub for the actual edit + event
// broadcast (复用 PUT /api/v1/messages/{id} 既有 store 层 helpers, 仅
// 加 DM-only path validation 包裹).
type MessageEditHandler struct {
	Store  *store.Store
	Hub    EventBroadcaster
	Logger *slog.Logger
}

// RegisterRoutes wires PATCH /api/v1/channels/{channelId}/messages/{messageId}.
func (h *MessageEditHandler) RegisterRoutes(mux *http.ServeMux,
	authMw func(http.Handler) http.Handler) {
	mux.Handle("PATCH /api/v1/channels/{channelId}/messages/{messageId}",
		authMw(http.HandlerFunc(h.handleEdit)))
}

// handleEdit — DM-4.1 PATCH /api/v1/channels/{channelId}/messages/{messageId}.
//
// Validation order:
//  1. Auth (user-rail).
//  2. Path ids present.
//  3. Channel exists + channel.Type == "dm" (else 403 `dm.edit_only_in_dm`).
//  4. Body schema {content} — empty content 400.
//  5. Message exists + not deleted + channel id matches path.
//  6. cross-org 403 (REG-INV-002 fail-closed, 跟 messages.go 既有同模式).
//  7. owner-only ACL: existing.SenderID == user.ID (else 403).
//  8. Store.UpdateMessage(messageID, content).
//  9. CreateEvent kind="message_edited" + Hub.BroadcastEventToChannel —
//     RT-3 fan-out 自动多端覆盖.
//
// Returns 200 with {message} on success.
func (h *MessageEditHandler) handleEdit(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}

	channelID := r.PathValue("channelId")
	messageID := r.PathValue("messageId")
	if channelID == "" || messageID == "" {
		writeJSONError(w, http.StatusBadRequest, "Channel ID and message ID are required")
		return
	}

	// 3. Channel exists + DM-only path.
	ch, err := h.Store.GetChannelByID(channelID)
	if err != nil || ch == nil {
		writeJSONError(w, http.StatusNotFound, "Channel not found")
		return
	}
	if ch.Type != "dm" {
		// 设计 ④ DM-only path — non-DM channel 走既有 PUT
		// /api/v1/messages/{id} 路径, DM-4 不接此 scope.
		writeJSONError(w, http.StatusForbidden, "dm.edit_only_in_dm")
		return
	}

	// 4. Body parse.
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

	// 5. Message lookup + integrity checks.
	existing, err := h.Store.GetMessageByID(messageID)
	if err != nil || existing == nil {
		writeJSONError(w, http.StatusNotFound, "Message not found")
		return
	}
	if existing.ChannelID != channelID {
		writeJSONError(w, http.StatusNotFound, "Message not found")
		return
	}
	if existing.DeletedAt != nil {
		writeJSONError(w, http.StatusBadRequest, "Cannot edit deleted message")
		return
	}

	// 6. cross-org 403 (REG-INV-002 fail-closed, 跟 messages.go 同模式).
	if store.CrossOrg(user.OrgID, existing.OrgID) {
		writeJSONError(w, http.StatusForbidden, "Forbidden")
		return
	}

	// AP-5 设计 ① — channel-member ACL gate (跟 AP-4 + messages.go 同模式).
	// Closes post-removal gap: sender removed from DM channel cannot
	// PATCH-edit prior messages there. byte-identical "Channel not found"
	// 404 fail-closed.
	if !h.Store.IsChannelMember(existing.ChannelID, user.ID) || !h.Store.CanAccessChannel(existing.ChannelID, user.ID) {
		writeJSONError(w, http.StatusNotFound, "Channel not found")
		return
	}

	// 7. owner-only ACL — sender matches caller.
	if existing.SenderID != user.ID {
		// 设计 ⑤ owner-only — 跟 AL-2a/BPP-3.2/AL-1/AL-5 owner-only 5 处
		// 同模式.
		writeJSONError(w, http.StatusForbidden, "dm.edit_non_owner_reject")
		return
	}

	// borgee #1108 F5 (DM-4.1 PATCH rail): editing an image-typed message must
	// keep an http(s) URL or same-origin relative path. Store.UpdateMessage
	// preserves content_type, so an edited image stays an image — without this
	// gate a PATCH could persist javascript:/data:/protocol-relative past the
	// create-rail guard, the same client-render phishing/inert-anchor vector.
	// Same store.IsAllowedImageContentURL allowlist + INVALID_CONTENT 400 as
	// the create rail (handleCreateMessage), the WS rail (ws/client.go), and
	// the PUT edit rail (handleUpdateMessage).
	if existing.ContentType == "image" && !store.IsAllowedImageContentURL(content) {
		writeJSONErrorCode(w, http.StatusBadRequest, "INVALID_CONTENT", "image content must be an http(s) URL or same-origin path")
		return
	}

	// 8. Update message via existing store helper (复用 messages.go 同
	// 路径 — last-write-wins simplification, 设计 ⑥, 不挂 edit history audit).
	msg, err := h.Store.UpdateMessage(messageID, content)
	if err != nil {
		h.Logger.Error("dm_4 update message failed",
			"error", err, "channel_id", channelID, "message_id", messageID)
		writeJSONError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	// 9. Events insert + RT-3 fan-out broadcast — kind="message_edited"
	// byte-identical 跟既有 PUT /api/v1/messages/{id} 路径 (单源 op,
	// useDMSync DM-3 #508 已订阅 channel events backfill, 自动多端 derive).
	h.Store.CreateEvent(&store.Event{
		Kind:      "message_edited",
		ChannelID: existing.ChannelID,
		Payload: mustJSON(map[string]any{
			"id":         messageID,
			"channel_id": existing.ChannelID,
			"sender_id":  user.ID,
			"content":    content,
			"op":         "edit", // DM-4 反查 grep 锚 (events 表 op="edit")
		}),
	})
	if h.Hub != nil {
		h.Hub.BroadcastEventToChannel(existing.ChannelID, "message_edited",
			map[string]any{"message": msg})
	}

	writeJSONResponse(w, http.StatusOK, map[string]any{"message": msg})
}
