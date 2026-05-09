package api

import (
	"log/slog"
	"net/http"

	"borgee-server/internal/store"
)

type ReactionHandler struct {
	Store  *store.Store
	Logger *slog.Logger
	Hub    EventBroadcaster
}

func (h *ReactionHandler) RegisterRoutes(mux *http.ServeMux, authMw func(http.Handler) http.Handler) {
	wrap := func(f http.HandlerFunc) http.Handler { return authMw(f) }

	mux.Handle("PUT /api/v1/messages/{messageId}/reactions", wrap(h.handleAddReaction))
	mux.Handle("DELETE /api/v1/messages/{messageId}/reactions", wrap(h.handleRemoveReaction))
	mux.Handle("GET /api/v1/messages/{messageId}/reactions", wrap(h.handleGetReactions))
}

// AP-4 设计 ①+③ — channel-member ACL gate (CV-7 #535 既存 gap 闭合).
// Reuses Store.IsChannelMember + Store.CanAccessChannel (跟 messages.go
// 既有 ACL 同源). Returns true iff caller may operate on reactions of
// `messageID`. On false, caller MUST emit 404 "Channel not found"
// byte-identical to messages.go::handleCreateMessage line 230-232 (channel
// hidden from non-member, fail-closed).
func (h *ReactionHandler) canAccessMessage(user *store.User, messageID string) (*store.Message, bool) {
	msg, err := h.Store.GetMessageByID(messageID)
	if err != nil {
		return nil, false
	}
	if !h.Store.IsChannelMember(msg.ChannelID, user.ID) || !h.Store.CanAccessChannel(msg.ChannelID, user.ID) {
		return nil, false
	}
	return msg, true
}

func (h *ReactionHandler) handleAddReaction(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}

	messageID := r.PathValue("messageId")
	msg, ok := h.canAccessMessage(user, messageID)
	if !ok {
		writeJSONError(w, http.StatusNotFound, "Channel not found")
		return
	}

	var body struct {
		Emoji string `json:"emoji"`
	}
	if err := readJSON(r, &body); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if body.Emoji == "" {
		writeJSONError(w, http.StatusBadRequest, "emoji is required")
		return
	}

	h.Store.AddReaction(messageID, user.ID, body.Emoji)

	reactions, err := h.Store.GetReactionsByMessage(messageID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to get reactions")
		return
	}

	if h.Hub != nil {
		h.Store.CreateEvent(&store.Event{
			Kind:      "reaction_update",
			ChannelID: msg.ChannelID,
			Payload:   mustJSON(map[string]any{"message_id": messageID, "reactions": reactions}),
		})
		h.Hub.BroadcastEventToChannel(msg.ChannelID, "reaction_update", map[string]any{"message_id": messageID, "reactions": reactions})
	}

	writeJSONResponse(w, http.StatusOK, map[string]any{"ok": true, "reactions": reactions})
}

func (h *ReactionHandler) handleRemoveReaction(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}

	messageID := r.PathValue("messageId")
	msg, ok := h.canAccessMessage(user, messageID)
	if !ok {
		writeJSONError(w, http.StatusNotFound, "Channel not found")
		return
	}

	var body struct {
		Emoji string `json:"emoji"`
	}
	if err := readJSON(r, &body); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if body.Emoji == "" {
		writeJSONError(w, http.StatusBadRequest, "emoji is required")
		return
	}

	h.Store.RemoveReaction(messageID, user.ID, body.Emoji)

	reactions, err := h.Store.GetReactionsByMessage(messageID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to get reactions")
		return
	}

	if h.Hub != nil {
		h.Store.CreateEvent(&store.Event{
			Kind:      "reaction_update",
			ChannelID: msg.ChannelID,
			Payload:   mustJSON(map[string]any{"message_id": messageID, "reactions": reactions}),
		})
		h.Hub.BroadcastEventToChannel(msg.ChannelID, "reaction_update", map[string]any{"message_id": messageID, "reactions": reactions})
	}

	writeJSONResponse(w, http.StatusOK, map[string]any{"ok": true, "reactions": reactions})
}

func (h *ReactionHandler) handleGetReactions(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}

	messageID := r.PathValue("messageId")
	if _, ok := h.canAccessMessage(user, messageID); !ok {
		writeJSONError(w, http.StatusNotFound, "Channel not found")
		return
	}

	reactions, err := h.Store.GetReactionsByMessage(messageID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to get reactions")
		return
	}

	writeJSONResponse(w, http.StatusOK, map[string]any{"reactions": reactions})
}
