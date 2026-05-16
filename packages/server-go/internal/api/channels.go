package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"borgee-server/internal/idgen"

	"borgee-server/internal/auth"
	"borgee-server/internal/config"
	"borgee-server/internal/store"
)

func readJSON(r *http.Request, dst any) error {
	const maxBytes = 1 << 20
	r.Body = http.MaxBytesReader(nil, r.Body, maxBytes)
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dst); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			return fmt.Errorf("request body too large")
		}
		return fmt.Errorf("invalid JSON: %w", err)
	}
	return nil
}

type ChannelHandler struct {
	Store  *store.Store
	Config *config.Config
	Logger *slog.Logger
	Hub    EventBroadcaster
}

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = slugRe.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	return s
}

func channelScope(r *http.Request) string {
	return fmt.Sprintf("channel:%s", r.PathValue("channelId"))
}

func (h *ChannelHandler) RegisterRoutes(mux *http.ServeMux, authMw func(http.Handler) http.Handler) {
	wrap := func(f http.HandlerFunc) http.Handler { return authMw(f) }
	permWrap := func(perm string, f http.HandlerFunc) http.Handler {
		return authMw(auth.RequirePermission(h.Store, perm, channelScope)(f))
	}

	mux.Handle("GET /api/v1/channels", wrap(h.handleListChannels))
	mux.Handle("POST /api/v1/channels", authMw(auth.RequirePermission(h.Store, "channel.create", nil)(http.HandlerFunc(h.handleCreateChannel))))
	mux.Handle("GET /api/v1/channels/{channelId}", wrap(h.handleGetChannel))
	mux.Handle("GET /api/v1/channels/{channelId}/preview", wrap(h.handlePreviewChannel))
	mux.Handle("PUT /api/v1/channels/{channelId}", wrap(h.handleUpdateChannel))
	mux.Handle("PUT /api/v1/channels/{channelId}/topic", wrap(h.handleSetTopic))
	mux.Handle("POST /api/v1/channels/{channelId}/join", wrap(h.handleJoinChannel))
	mux.Handle("POST /api/v1/channels/{channelId}/leave", wrap(h.handleLeaveChannel))
	mux.Handle("POST /api/v1/channels/{channelId}/members", permWrap("channel.manage_members", h.handleAddMember))
	mux.Handle("DELETE /api/v1/channels/{channelId}/members/{userId}", wrap(h.handleRemoveMember))
	mux.Handle("PUT /api/v1/channels/{channelId}/members/{userId}/require-mention", wrap(h.handleSetMemberRequireMentionPolicy))
	mux.Handle("GET /api/v1/channels/{channelId}/members", wrap(h.handleListMembers))
	mux.Handle("PUT /api/v1/channels/{channelId}/read", wrap(h.handleMarkRead))
	mux.Handle("DELETE /api/v1/channels/{channelId}", permWrap("channel.delete", h.handleDeleteChannel))
	mux.Handle("PUT /api/v1/channels/reorder", wrap(h.handleReorderChannel))

	mux.Handle("GET /api/v1/channel-groups", wrap(h.handleListGroups))
	mux.Handle("POST /api/v1/channel-groups", wrap(h.handleCreateGroup))
	mux.Handle("PUT /api/v1/channel-groups/{groupId}", wrap(h.handleUpdateGroup))
	mux.Handle("DELETE /api/v1/channel-groups/{groupId}", wrap(h.handleDeleteGroup))
	mux.Handle("PUT /api/v1/channel-groups/reorder", wrap(h.handleReorderGroup))
}

func (h *ChannelHandler) handleListChannels(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}

	// ADM-0.3: user-rail lists membership-scoped channels only.
	// Cross-user enumeration is admin-rail (/admin-api/v1/channels).
	// CHN-13 设计 ②: optional ?q= 子串 filter (空 q 走既有 path
	// byte-identical; q!="" 加 LIKE COLLATE NOCASE).
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	channels, err := h.Store.ListChannelsWithUnread(user.ID, q)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to list channels")
		return
	}

	groups, err := h.Store.ListChannelGroups()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to list groups")
		return
	}

	if channels == nil {
		channels = []store.ChannelWithCounts{}
	}
	if groups == nil {
		groups = []store.ChannelGroup{}
	}

	writeJSONResponse(w, http.StatusOK, map[string]any{"channels": channels, "groups": groups})
}

func (h *ChannelHandler) handleCreateChannel(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}

	var body struct {
		Name       string   `json:"name"`
		Topic      string   `json:"topic"`
		MemberIDs  []string `json:"member_ids"`
		Visibility string   `json:"visibility"`
	}
	if err := readJSON(r, &body); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	slug := slugify(body.Name)
	if slug == "" {
		writeJSONError(w, http.StatusBadRequest, "Channel name is required")
		return
	}

	if body.Visibility == "" {
		body.Visibility = "public"
	}
	// CHN-9 设计 ① + ④: visibility 三态校验 byte-identical 跟 IsValidVisibility
	// 谓词单源 (原 'public'/'private' 二态扩到三态加 'creator_only').
	if !IsValidVisibility(body.Visibility) {
		writeJSONError(w, http.StatusBadRequest, VisibilityRejectMessage)
		return
	}
	if len(body.Topic) > 250 {
		writeJSONError(w, http.StatusBadRequest, "Topic must be 250 characters or less")
		return
	}

	if existing, _ := h.Store.GetChannelByNameInOrg(user.OrgID, slug); existing != nil {
		writeJSONError(w, http.StatusConflict, "Channel name already exists")
		return
	}

	lastPos := h.Store.GetLastChannelPosition()
	position := store.GenerateRankBetween(lastPos, "")

	ch := &store.Channel{
		Name:       slug,
		Topic:      body.Topic,
		Visibility: body.Visibility,
		CreatedBy:  user.ID,
		Type:       "channel",
		Position:   position,
		OrgID:      user.OrgID, // CM-3.1
	}
	if err := h.Store.CreateChannel(ch); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to create channel")
		return
	}

	// CHN-1.2 设计 ②: creator-only default member. POST /channels 后
	// channel_members count == 1 (只 creator). Public channels are
	// discoverable via GET (org-scoped) — no auto-fan-out join.
	h.Store.AddChannelMember(&store.ChannelMember{ChannelID: ch.ID, UserID: user.ID})

	if body.Visibility == "private" {
		for _, uid := range body.MemberIDs {
			if uid != user.ID {
				h.Store.AddChannelMember(&store.ChannelMember{ChannelID: ch.ID, UserID: uid})
			}
		}
	}

	if err := h.Store.GrantCreatorPermissions(user.ID, user.Role, ch.ID, user.OwnerID); err != nil {
		h.Logger.Error("failed to grant creator permissions", "error", err)
	}

	result, _ := h.Store.GetChannelWithCounts(ch.ID, user.ID)
	if result == nil {
		writeJSONResponse(w, http.StatusCreated, map[string]any{"channel": ch})
	} else {
		writeJSONResponse(w, http.StatusCreated, map[string]any{"channel": result})
	}

	h.Store.CreateEvent(&store.Event{
		Kind:    "channel_created",
		Payload: mustJSON(map[string]any{"channel": ch}),
	})
	if h.Hub != nil {
		// The POST response is the creator's source of truth, and explicit
		// member adds use channel_added below. Broadcasting channel_created to
		// every browser bypasses ListChannelsWithUnread's org/member scoping and
		// makes unrelated clients accumulate other users' channels under parallel
		// e2e load.
		h.Hub.BroadcastEventToChannel(ch.ID, "channel_created", map[string]any{"channel": ch})
	}
}

func (h *ChannelHandler) handleGetChannel(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}

	channelID := r.PathValue("channelId")
	// CM-3.2: cross-org 403 BEFORE membership check, otherwise private-channel
	// rejection 404s first and the 403 contract leaks.
	if orgID, err := h.Store.ChannelOrgID(channelID); err == nil && store.CrossOrg(user.OrgID, orgID) {
		writeJSONError(w, http.StatusForbidden, "Forbidden")
		return
	}
	// AP-1 设计 ①: 严格 403 — 非 member 也 403 (不再 404 隐藏存在性).
	// 跟 GitHub repo 私有路径同模式: "暴露存在但拒访问". 触发
	// REG-CHN1-007 ⏸️→🟢 flip (CHN-1 #286 既有 404 路径承袭, 改一处
	// status code, e2e 反向断言改 `status === 403`).
	//
	// 不存在 → 404 (区分: 真不存在 vs 存在但无权).
	if _, err := h.Store.GetChannelByID(channelID); err != nil {
		writeJSONError(w, http.StatusNotFound, "Channel not found")
		return
	}
	if !h.Store.CanAccessChannel(channelID, user.ID) {
		writeJSONError(w, http.StatusForbidden, "Forbidden")
		return
	}

	ch, err := h.Store.GetChannelWithCounts(channelID, user.ID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Channel not found")
		return
	}

	members, _ := h.Store.GetChannelDetail(channelID)
	if members == nil {
		members = []store.ChannelMemberInfo{}
	}

	resp := map[string]any{
		"channel": ch,
		"members": members,
	}
	writeJSONResponse(w, http.StatusOK, resp)
}

func (h *ChannelHandler) handlePreviewChannel(w http.ResponseWriter, r *http.Request) {
	_, ok := mustUser(w, r)
	if !ok {
		return
	}

	channelID := r.PathValue("channelId")
	ch, err := h.Store.GetChannelByID(channelID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Channel not found")
		return
	}

	if ch.Visibility == "private" {
		writeJSONError(w, http.StatusNotFound, "Channel not found")
		return
	}

	msgs, err := h.Store.GetPreviewMessages(channelID, 50)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to get messages")
		return
	}
	if msgs == nil {
		msgs = []store.PreviewMessage{}
	}

	writeJSONResponse(w, http.StatusOK, map[string]any{"messages": msgs, "channel": ch})
}

func (h *ChannelHandler) handleUpdateChannel(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}

	channelID := r.PathValue("channelId")
	ch, err := h.Store.GetChannelByID(channelID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Channel not found")
		return
	}

	var body struct {
		Name       *string `json:"name"`
		Topic      *string `json:"topic"`
		Visibility *string `json:"visibility"`
		// Archive flag (CHN-1.2 设计 ⑤): clients PATCH `archived: true` to soft
		// 退役 a channel. Setting to false un-archives. The actual archived_at
		// timestamp is server-stamped — clients cannot inject arbitrary times.
		Archived *bool `json:"archived"`
	}
	if err := readJSON(r, &body); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	topicOnly := body.Topic != nil && body.Name == nil && body.Visibility == nil && body.Archived == nil
	if topicOnly {
		if !h.Store.IsChannelMember(channelID, user.ID) {
			writeJSONError(w, http.StatusForbidden, "Must be a channel member to update topic")
			return
		}
	} else {
		if store.CrossOrg(user.OrgID, ch.OrgID) {
			writeJSONError(w, http.StatusForbidden, "Forbidden")
			return
		}
		if !h.hasChannelPermission(user, "channel.manage_visibility", channelID) {
			writeJSONError(w, http.StatusForbidden, "Forbidden")
			return
		}
	}
	if body.Archived != nil {
		if ch.Type == "dm" {
			writeJSONError(w, http.StatusBadRequest, "Cannot archive DM channels")
			return
		}
		if ch.Name == "general" {
			writeJSONError(w, http.StatusBadRequest, "Cannot archive #general")
			return
		}
		if ch.CreatedBy != user.ID {
			writeJSONError(w, http.StatusForbidden, "Forbidden")
			return
		}
	}

	updates := map[string]any{}
	if body.Name != nil {
		slug := slugify(*body.Name)
		if slug == "" {
			writeJSONError(w, http.StatusBadRequest, "Channel name is required")
			return
		}
		if slug != ch.Name {
			// CHN-1.2: per-org name uniqueness (channels.name is no longer
			// globally UNIQUE post v=11). Only collide within the same org.
			if existing, _ := h.Store.GetChannelByNameInOrg(ch.OrgID, slug); existing != nil {
				writeJSONError(w, http.StatusConflict, "Channel name already exists")
				return
			}
		}
		updates["name"] = slug
	}
	if body.Topic != nil {
		if len(*body.Topic) > 250 {
			writeJSONError(w, http.StatusBadRequest, "Topic must be 250 characters or less")
			return
		}
		updates["topic"] = *body.Topic
	}
	if body.Visibility != nil {
		// CHN-9 设计 ④: visibility 三态校验 byte-identical 跟 create handler 同源.
		if !IsValidVisibility(*body.Visibility) {
			writeJSONError(w, http.StatusBadRequest, VisibilityRejectMessage)
			return
		}
		updates["visibility"] = *body.Visibility
	}

	// CHN-1.2 设计 ⑤: archive flip — server stamps timestamp; emits per-member
	// system DM fanout reusing the ADM-0 §1.4 红线 ③ shape. Skipped if no
	// transition (already archived → ignored; un-archive nullifies).
	// CHN-5.2 设计 ③: unarchive 加互补 fanoutUnarchiveSystemMessage (跟 archive
	// 互补二式 byte-identical 跟 chn-5-content-lock.md §1).
	archiveTriggered := false
	unarchiveTriggered := false
	var archiveTs int64
	var unarchiveTs int64
	if body.Archived != nil {
		if *body.Archived {
			if ch.ArchivedAt == nil {
				archiveTs = nowMillis()
				updates["archived_at"] = archiveTs
				archiveTriggered = true
			}
		} else {
			if ch.ArchivedAt != nil {
				unarchiveTs = nowMillis()
				unarchiveTriggered = true
			}
			updates["archived_at"] = nil
		}
	}

	if len(updates) > 0 {
		if err := h.Store.UpdateChannel(channelID, updates); err != nil {
			writeJSONError(w, http.StatusInternalServerError, "Failed to update channel")
			return
		}
	}

	// Fanout the archive system DM after the row commits so members observe
	// the archive flag at the same time as the notification.
	if archiveTriggered {
		h.fanoutArchiveSystemMessage(channelID, ch.Name, user.ID, archiveTs)
	}
	if unarchiveTriggered {
		h.fanoutUnarchiveSystemMessage(channelID, ch.Name, user.ID, unarchiveTs)
	}

	result, _ := h.Store.GetChannelWithCounts(channelID, user.ID)
	writeJSONResponse(w, http.StatusOK, map[string]any{"channel": result})

	h.Store.CreateEvent(&store.Event{
		Kind:      "channel_updated",
		ChannelID: channelID,
		Payload:   mustJSON(map[string]any{"channel": result}),
	})
	if h.Hub != nil {
		h.Hub.BroadcastEventToAll("channel_updated", map[string]any{"channel": result})
	}
}

func (h *ChannelHandler) handleSetTopic(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}

	channelID := r.PathValue("channelId")
	if !h.Store.IsChannelMember(channelID, user.ID) {
		writeJSONError(w, http.StatusForbidden, "Must be a channel member")
		return
	}

	var body struct {
		Topic string `json:"topic"`
	}
	if err := readJSON(r, &body); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if len(body.Topic) > 250 {
		writeJSONError(w, http.StatusBadRequest, "Topic must be 250 characters or less")
		return
	}

	if err := h.Store.UpdateChannel(channelID, map[string]any{"topic": body.Topic}); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to update topic")
		return
	}

	result, _ := h.Store.GetChannelWithCounts(channelID, user.ID)
	writeJSONResponse(w, http.StatusOK, map[string]any{"channel": result})

	h.Store.CreateEvent(&store.Event{
		Kind:      "channel_updated",
		ChannelID: channelID,
		Payload:   mustJSON(map[string]any{"channel": result}),
	})
	if h.Hub != nil {
		h.Hub.BroadcastEventToChannel(channelID, "channel_updated", map[string]any{"channel": result})
	}
}

func (h *ChannelHandler) handleJoinChannel(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}

	if user.Role == "agent" {
		writeJSONError(w, http.StatusForbidden, "Agents cannot self-join channels")
		return
	}

	channelID := r.PathValue("channelId")
	ch, err := h.Store.GetChannelByID(channelID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Channel not found")
		return
	}

	if ch.Type == "dm" {
		writeJSONError(w, http.StatusBadRequest, "Cannot join DM channels")
		return
	}
	if ch.Visibility != "public" {
		writeJSONError(w, http.StatusForbidden, "Cannot join private channels")
		return
	}

	h.Store.AddChannelMember(&store.ChannelMember{ChannelID: channelID, UserID: user.ID})
	writeJSONResponse(w, http.StatusOK, map[string]any{"ok": true})

	h.Store.CreateEvent(&store.Event{
		Kind:      "user_joined",
		ChannelID: channelID,
		Payload:   mustJSON(map[string]any{"channel_id": channelID, "user_id": user.ID}),
	})
	if h.Hub != nil {
		h.Hub.BroadcastEventToChannel(channelID, "user_joined", map[string]any{"channel_id": channelID, "user_id": user.ID})
	}
}

func (h *ChannelHandler) handleLeaveChannel(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}

	channelID := r.PathValue("channelId")
	ch, err := h.Store.GetChannelByID(channelID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Channel not found")
		return
	}

	if ch.Type == "dm" {
		writeJSONError(w, http.StatusBadRequest, "Cannot leave DM channels")
		return
	}
	if ch.Name == "general" {
		writeJSONError(w, http.StatusBadRequest, "Cannot leave #general")
		return
	}
	if ch.CreatedBy == user.ID {
		writeJSONError(w, http.StatusBadRequest, "Channel creator cannot leave own channel")
		return
	}
	if !h.Store.IsChannelMember(channelID, user.ID) {
		writeJSONError(w, http.StatusForbidden, "Must be a channel member")
		return
	}

	h.Store.RemoveChannelMember(channelID, user.ID)
	writeJSONResponse(w, http.StatusOK, map[string]any{"ok": true})

	h.Store.CreateEvent(&store.Event{
		Kind:      "user_left",
		ChannelID: channelID,
		Payload:   mustJSON(map[string]any{"channel_id": channelID, "user_id": user.ID}),
	})
	if h.Hub != nil {
		h.Hub.BroadcastEventToChannel(channelID, "user_left", map[string]any{"channel_id": channelID, "user_id": user.ID})
	}
}

func (h *ChannelHandler) handleAddMember(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}

	channelID := r.PathValue("channelId")
	ch, err := h.Store.GetChannelByID(channelID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Channel not found")
		return
	}

	if ch.Type == "dm" {
		writeJSONError(w, http.StatusBadRequest, "Cannot add members to DM channels")
		return
	}
	if store.CrossOrg(user.OrgID, ch.OrgID) {
		writeJSONError(w, http.StatusForbidden, "Forbidden")
		return
	}
	if !h.Store.IsChannelMember(channelID, user.ID) {
		writeJSONError(w, http.StatusForbidden, "Must be a channel member")
		return
	}

	var body struct {
		UserID string `json:"user_id"`
	}
	if err := readJSON(r, &body); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if body.UserID == "" {
		writeJSONError(w, http.StatusBadRequest, "user_id is required")
		return
	}

	target, err := h.Store.GetUserByID(body.UserID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "User not found")
		return
	}

	// ADM-0.3: user-rail can only add agents the user owns. Cross-owner agent
	// channel-add is admin-rail (or not supported on user-rail at all).
	if target.Role == "agent" {
		isOwner := target.OwnerID != nil && *target.OwnerID == user.ID
		if !isOwner {
			writeJSONError(w, http.StatusForbidden, "Only the agent's owner can add it to a channel")
			return
		}
	}

	h.Store.AddChannelMember(&store.ChannelMember{ChannelID: channelID, UserID: body.UserID})

	// CHN-1.2 设计 ③: agent join 触发 system message 文案锁
	// `"{agent_name} joined"` — sender_id='system', kind=system. Human joins
	// continue to broadcast `user_joined` event only (no system message).
	if target.Role == "agent" {
		h.emitAgentJoinSystemMessage(channelID, target.DisplayName)
	}

	writeJSONResponse(w, http.StatusOK, map[string]any{"ok": true})

	h.Store.CreateEvent(&store.Event{
		Kind:      "user_joined",
		ChannelID: channelID,
		Payload:   mustJSON(map[string]any{"channel_id": channelID, "user_id": body.UserID}),
	})
	if h.Hub != nil {
		h.Hub.BroadcastEventToChannel(channelID, "user_joined", map[string]any{"channel_id": channelID, "user_id": body.UserID})
		// CHN-1.3 fix: send full channel object so the recipient's reducer
		// can ADD_CHANNEL without an extra round-trip. Previously this only
		// carried channel_id, which made `data.channel as Channel` resolve
		// to undefined and crashed AppProvider via reducer line 117.
		if added, _ := h.Store.GetChannelWithCounts(channelID, body.UserID); added != nil {
			h.Hub.BroadcastEventToUser(body.UserID, "channel_added", map[string]any{"channel": added})
		} else {
			h.Hub.BroadcastEventToUser(body.UserID, "channel_added", map[string]any{"channel_id": channelID})
		}
	}
}

func (h *ChannelHandler) handleRemoveMember(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}

	channelID := r.PathValue("channelId")
	targetID := r.PathValue("userId")

	ch, err := h.Store.GetChannelByID(channelID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Channel not found")
		return
	}
	if ch.Type == "dm" {
		writeJSONError(w, http.StatusBadRequest, "Cannot remove members from DM channels")
		return
	}

	if ch.Name == "general" {
		writeJSONError(w, http.StatusBadRequest, "Cannot remove members from #general")
		return
	}
	if !h.Store.IsChannelMember(channelID, user.ID) {
		writeJSONError(w, http.StatusForbidden, "Must be a channel member")
		return
	}
	if targetID == ch.CreatedBy {
		writeJSONError(w, http.StatusBadRequest, "Cannot remove channel creator")
		return
	}

	if targetID != user.ID {
		if store.CrossOrg(user.OrgID, ch.OrgID) {
			writeJSONError(w, http.StatusForbidden, "Forbidden")
			return
		}
		if !h.hasChannelPermission(user, "channel.manage_members", channelID) {
			writeJSONError(w, http.StatusForbidden, "Forbidden")
			return
		}
	}
	if !h.Store.IsChannelMember(channelID, targetID) {
		writeJSONError(w, http.StatusNotFound, "Channel member not found")
		return
	}

	h.Store.RemoveChannelMember(channelID, targetID)
	writeJSONResponse(w, http.StatusOK, map[string]any{"ok": true})

	h.Store.CreateEvent(&store.Event{
		Kind:      "user_left",
		ChannelID: channelID,
		Payload:   mustJSON(map[string]any{"channel_id": channelID, "user_id": targetID}),
	})
	if h.Hub != nil {
		h.Hub.BroadcastEventToChannel(channelID, "user_left", map[string]any{"channel_id": channelID, "user_id": targetID})
		h.Hub.BroadcastEventToUser(targetID, "channel_removed", map[string]any{"channel_id": channelID})
	}
}

func (h *ChannelHandler) handleListMembers(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}

	channelID := r.PathValue("channelId")
	if !h.Store.CanAccessChannel(channelID, user.ID) {
		writeJSONError(w, http.StatusNotFound, "Channel not found")
		return
	}

	members, err := h.Store.GetChannelDetail(channelID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to list members")
		return
	}
	if members == nil {
		members = []store.ChannelMemberInfo{}
	}

	writeJSONResponse(w, http.StatusOK, map[string]any{"members": members})
}

func (h *ChannelHandler) handleSetMemberRequireMentionPolicy(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}

	channelID := r.PathValue("channelId")
	targetID := r.PathValue("userId")
	if channelID == "" || targetID == "" {
		writeJSONError(w, http.StatusBadRequest, "channel_id and user_id are required")
		return
	}

	ch, err := h.Store.GetChannelByID(channelID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Channel not found")
		return
	}
	if ch.Type == "dm" {
		writeJSONError(w, http.StatusBadRequest, "Cannot set requireMention policy on DM channels")
		return
	}
	if store.CrossOrg(user.OrgID, ch.OrgID) {
		writeJSONError(w, http.StatusForbidden, "Forbidden")
		return
	}
	if !h.Store.IsChannelMember(channelID, user.ID) {
		writeJSONError(w, http.StatusForbidden, "Must be a channel member")
		return
	}
	if !h.hasChannelPermission(user, "channel.manage_members", channelID) {
		writeJSONError(w, http.StatusForbidden, "Forbidden")
		return
	}

	var body struct {
		Policy string `json:"policy"`
	}
	if err := readJSON(r, &body); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	rawPolicy := strings.TrimSpace(body.Policy)
	if rawPolicy == "" {
		writeJSONError(w, http.StatusBadRequest, "Invalid requireMention policy")
		return
	}
	policy, err := store.NormalizeRequireMentionPolicy(rawPolicy)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "Invalid requireMention policy")
		return
	}

	state, err := h.Store.SetChannelMemberRequireMentionPolicy(channelID, targetID, policy)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrRequireMentionPolicyTargetNotMember):
			writeJSONError(w, http.StatusNotFound, "Channel member not found")
		case errors.Is(err, store.ErrRequireMentionPolicyTargetNotAgent):
			writeJSONError(w, http.StatusBadRequest, "requireMention policy applies only to agent members")
		case errors.Is(err, store.ErrInvalidRequireMentionPolicy):
			writeJSONError(w, http.StatusBadRequest, "Invalid requireMention policy")
		case errors.Is(err, store.ErrRequireMentionPolicyOwnerCeiling):
			writeJSONError(w, http.StatusBadRequest, "requireMention policy cannot broaden agent attention beyond owner authorization")
		default:
			if h.Logger != nil {
				h.Logger.Error("failed to set requireMention policy", "error", err, "channel_id", channelID, "user_id", targetID)
			}
			writeJSONError(w, http.StatusInternalServerError, "Failed to update requireMention policy")
		}
		return
	}

	resp := map[string]any{
		"channel_id":                state.ChannelID,
		"user_id":                   state.UserID,
		"require_mention_policy":    state.RequireMentionPolicy,
		"effective_require_mention": state.EffectiveRequireMention,
	}
	writeJSONResponse(w, http.StatusOK, resp)

	h.Store.CreateEvent(&store.Event{
		Kind:      "channel_member_updated",
		ChannelID: channelID,
		Payload:   mustJSON(resp),
	})
	if h.Hub != nil {
		h.Hub.BroadcastEventToChannel(channelID, "channel_member_updated", resp)
	}
}

func (h *ChannelHandler) handleMarkRead(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}

	channelID := r.PathValue("channelId")
	if !h.Store.IsChannelMember(channelID, user.ID) {
		writeJSONError(w, http.StatusForbidden, "Must be a channel member")
		return
	}

	if err := h.Store.MarkChannelRead(channelID, user.ID); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to mark read")
		return
	}

	writeJSONResponse(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *ChannelHandler) handleDeleteChannel(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}

	channelID := r.PathValue("channelId")
	ch, err := h.Store.GetChannelByID(channelID)
	if err != nil {
		// Check if channel exists but is already deleted — idempotent 204
		chDeleted, errD := h.Store.GetChannelIncludingDeleted(channelID)
		if errD == nil && chDeleted != nil && chDeleted.DeletedAt != nil {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		writeJSONError(w, http.StatusNotFound, "Channel not found")
		return
	}

	if ch.Type == "dm" {
		writeJSONError(w, http.StatusBadRequest, "Cannot delete DM channels")
		return
	}
	if ch.Name == "general" {
		writeJSONError(w, http.StatusBadRequest, "Cannot delete #general")
		return
	}
	if store.CrossOrg(user.OrgID, ch.OrgID) {
		writeJSONError(w, http.StatusForbidden, "Forbidden")
		return
	}
	if ch.CreatedBy != user.ID {
		writeJSONError(w, http.StatusForbidden, "Forbidden")
		return
	}

	if err := h.Store.SoftDeleteChannel(channelID); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to delete channel")
		return
	}

	scope := fmt.Sprintf("channel:%s", channelID)
	h.Store.DeletePermissionsByScope(scope)

	h.Store.CreateEvent(&store.Event{
		Kind:      "channel_deleted",
		ChannelID: channelID,
		Payload:   mustJSON(map[string]any{"channel_id": channelID}),
	})
	if h.Hub != nil {
		h.Hub.BroadcastEventToAll("channel_deleted", map[string]any{"channel_id": channelID})
	}

	writeJSONResponse(w, http.StatusOK, map[string]any{"ok": true})
}

func (h *ChannelHandler) handleReorderChannel(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}

	// ADM-0.3: user-rail reorder gated by `channel.manage_visibility` (*, *).
	// Owner-default member fixtures hold this via the (*, *) wildcard granted
	// at registration. Admin-rail reorder is /admin-api/v1/channels.
	perms, _ := h.Store.ListUserPermissions(user.ID)
	isOwner := false
	for _, p := range perms {
		if (p.Permission == "channel.manage_visibility" || p.Permission == "*") && p.Scope == "*" {
			isOwner = true
			break
		}
	}
	if !isOwner {
		writeJSONError(w, http.StatusForbidden, "Forbidden")
		return
	}

	var body struct {
		ChannelID string  `json:"channel_id"`
		AfterID   *string `json:"after_id"`
		GroupID   *string `json:"group_id"`
	}
	if err := readJSON(r, &body); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if body.ChannelID == "" {
		writeJSONError(w, http.StatusBadRequest, "channel_id is required")
		return
	}

	before, after, err := h.Store.GetAdjacentChannelPositions(body.AfterID, body.GroupID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to calculate position")
		return
	}

	position := store.GenerateRankBetween(before, after)
	if err := h.Store.UpdateChannelPosition(body.ChannelID, position, body.GroupID); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to reorder channel")
		return
	}

	writeJSONResponse(w, http.StatusOK, map[string]any{
		"channel": map[string]any{"id": body.ChannelID, "position": position, "group_id": body.GroupID},
	})

	h.Store.CreateEvent(&store.Event{
		Kind:    "channels_reordered",
		Payload: mustJSON(map[string]any{"channel_id": body.ChannelID, "position": position, "group_id": body.GroupID}),
	})
	if h.Hub != nil {
		h.Hub.BroadcastEventToAll("channels_reordered", map[string]any{"channel_id": body.ChannelID, "position": position, "group_id": body.GroupID})
	}
}

// ─── Channel Groups ───────────────────────────────────

func (h *ChannelHandler) handleListGroups(w http.ResponseWriter, r *http.Request) {
	groups, err := h.Store.ListChannelGroups()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to list groups")
		return
	}
	if groups == nil {
		groups = []store.ChannelGroup{}
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{"groups": groups})
}

func (h *ChannelHandler) handleCreateGroup(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}

	var body struct {
		Name string `json:"name"`
	}
	if err := readJSON(r, &body); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	name := strings.TrimSpace(body.Name)
	if name == "" {
		writeJSONError(w, http.StatusBadRequest, "Group name is required")
		return
	}
	if len(name) > 50 {
		writeJSONError(w, http.StatusBadRequest, "Group name must be 50 characters or less")
		return
	}

	lastPos := h.Store.GetLastGroupPosition()
	position := store.GenerateRankBetween(lastPos, "")

	group := &store.ChannelGroup{
		Name:      name,
		Position:  position,
		CreatedBy: user.ID,
	}
	if err := h.Store.CreateChannelGroup(group); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to create group")
		return
	}

	h.Store.CreateEvent(&store.Event{
		Kind:    "group_created",
		Payload: mustJSON(map[string]any{"group": group}),
	})
	if h.Hub != nil {
		h.Hub.BroadcastEventToAll("group_created", map[string]any{"group": group})
	}

	writeJSONResponse(w, http.StatusCreated, map[string]any{"group": group})
}

func (h *ChannelHandler) handleUpdateGroup(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}

	groupID := r.PathValue("groupId")
	group, err := h.Store.GetChannelGroup(groupID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Group not found")
		return
	}

	if group.CreatedBy != user.ID {
		writeJSONError(w, http.StatusForbidden, "Only the group creator can rename it")
		return
	}

	var body struct {
		Name string `json:"name"`
	}
	if err := readJSON(r, &body); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	name := strings.TrimSpace(body.Name)
	if name == "" {
		writeJSONError(w, http.StatusBadRequest, "Group name is required")
		return
	}
	if len(name) > 50 {
		writeJSONError(w, http.StatusBadRequest, "Group name must be 50 characters or less")
		return
	}

	if err := h.Store.UpdateChannelGroup(groupID, name); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to update group")
		return
	}

	group.Name = name
	writeJSONResponse(w, http.StatusOK, map[string]any{"group": group})

	h.Store.CreateEvent(&store.Event{
		Kind:    "group_updated",
		Payload: mustJSON(map[string]any{"group": group}),
	})
	if h.Hub != nil {
		h.Hub.BroadcastEventToAll("group_updated", map[string]any{"group": group})
	}
}

func (h *ChannelHandler) handleDeleteGroup(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}

	groupID := r.PathValue("groupId")
	group, err := h.Store.GetChannelGroup(groupID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Group not found")
		return
	}

	if group.CreatedBy != user.ID {
		writeJSONError(w, http.StatusForbidden, "Only the group creator can delete it")
		return
	}

	ungroupedIDs, err := h.Store.UngroupChannels(groupID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to ungroup channels")
		return
	}

	if err := h.Store.DeleteChannelGroup(groupID); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to delete group")
		return
	}

	if ungroupedIDs == nil {
		ungroupedIDs = []string{}
	}

	h.Store.CreateEvent(&store.Event{
		Kind:    "group_deleted",
		Payload: mustJSON(map[string]any{"group_id": groupID}),
	})
	if h.Hub != nil {
		h.Hub.BroadcastEventToAll("group_deleted", map[string]any{"group_id": groupID})
	}

	writeJSONResponse(w, http.StatusOK, map[string]any{"ok": true, "ungrouped_channel_ids": ungroupedIDs})
}

func (h *ChannelHandler) handleReorderGroup(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}

	var body struct {
		GroupID string  `json:"group_id"`
		AfterID *string `json:"after_id"`
	}
	if err := readJSON(r, &body); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if body.GroupID == "" {
		writeJSONError(w, http.StatusBadRequest, "group_id is required")
		return
	}

	group, err := h.Store.GetChannelGroup(body.GroupID)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "Group not found")
		return
	}

	if group.CreatedBy != user.ID {
		writeJSONError(w, http.StatusForbidden, "Only the group creator can reorder")
		return
	}

	before, after, err := h.Store.GetAdjacentGroupPositions(body.AfterID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to calculate position")
		return
	}

	position := store.GenerateRankBetween(before, after)
	if err := h.Store.UpdateGroupPosition(body.GroupID, position); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "Failed to reorder group")
		return
	}

	group.Position = position
	writeJSONResponse(w, http.StatusOK, map[string]any{"group": group})

	h.Store.CreateEvent(&store.Event{
		Kind:    "groups_reordered",
		Payload: mustJSON(map[string]any{"group": group}),
	})
	if h.Hub != nil {
		h.Hub.BroadcastEventToAll("groups_reordered", map[string]any{"group": group})
	}
}

// ─── Helpers ──────────────────────────────────────────

func (h *ChannelHandler) hasChannelPermission(user *store.User, permission, channelID string) bool {
	// ADM-0.3: no role short-circuit. (*, *) wildcard below covers humans;
	// admin-rail uses /admin-api/v1/channels.
	perms, err := h.Store.ListUserPermissions(user.ID)
	if err != nil {
		return false
	}
	scope := fmt.Sprintf("channel:%s", channelID)
	for _, p := range perms {
		// AP-0: humans default to (*, *); bundle-narrowed accounts (AP-2) will
		// not have this row and fall through to the explicit match below.
		if p.Permission == "*" && p.Scope == "*" {
			return true
		}
		if p.Permission == permission && (p.Scope == "*" || p.Scope == scope) {
			return true
		}
	}
	return false
}

// nowMillis is the wall-clock now in milliseconds. Indirected so future tests
// can swap a clock if needed (CHN-1.2 archive ts is observed by clients via
// system DM; precision below ms is irrelevant for fanout ordering).
func nowMillis() int64 { return time.Now().UnixMilli() }

// emitAgentJoinSystemMessage inserts the agent-join system message
// (CHN-1.2 设计 ③, #265 acceptance #6). Format MUST be exactly
// `"{agent_name} joined"` — the suite greps it by string match. The message
// is sender_id='system' and content_type='text'; no quick_action attached.
//
// Failures are logged but do NOT roll back the channel_member insert: the
// audit row is the source of truth, the system message is best-effort.
func (h *ChannelHandler) emitAgentJoinSystemMessage(channelID, agentName string) {
	content := agentName + " joined"
	now := nowMillis()
	msg := &store.Message{
		ID:          idgen.NewID(),
		ChannelID:   channelID,
		SenderID:    "system",
		Content:     content,
		ContentType: "text",
		CreatedAt:   now,
	}
	if err := h.Store.CreateMessage(msg); err != nil {
		if h.Logger != nil {
			h.Logger.Error("emitAgentJoinSystemMessage failed", "channel_id", channelID, "error", err)
		}
		return
	}
	if h.Hub != nil {
		h.Hub.BroadcastEventToChannel(channelID, "system_message", map[string]any{
			"channel_id": channelID,
			"content":    content,
			"sender_id":  "system",
			"created_at": now,
		})
	}
}

// fanoutArchiveSystemMessage delivers a system DM to every member of the
// archived channel — CHN-1.2 设计 ⑤ (#265 acceptance #7). Content format:
//
//	"channel #{name} 已被 {owner_name} 关闭于 {ts}"
//
// where {ts} is the unix-milli archive timestamp formatted RFC3339. We send
// to the channel itself (not separate DMs) because the per-member DM channel
// resolver is heavier than necessary; clients render system messages with
// kind=system, sender=system as a global broadcast inside the archived
// channel — sufficient for the audit trail and matches the ADM-0 §1.4 红线
// ③ shape (one row per member fanout would duplicate noise).
func (h *ChannelHandler) fanoutArchiveSystemMessage(channelID, channelName, ownerID string, archiveTs int64) {
	h.fanoutChannelStateMessage(channelStateMessageArgs{
		channelID:    channelID,
		channelName:  channelName,
		ownerID:      ownerID,
		ts:           archiveTs,
		verbLiteral:  "关闭于",
		eventName:    "channel_archived",
		eventTSKey:   "archived_at",
		errLogPrefix: "fanoutArchiveSystemMessage failed",
	})
}
