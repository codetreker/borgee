// Package api — chn_5_archived.go: CHN-5 archived-channel UI list + admin
// readonly view + matching unarchive system DM.
//
// Blueprint: channel-model.md §2 invariant #3: archive keeps history. Spec:
// docs/implementation/modules/chn-5-spec.md. No schema change: reuse the
// existing CHN-1.1 #267 channels.archived_at column.
//
// Public surface:
//   - (h *ChannelHandler) RegisterCHN5Routes(mux, authMw) — user-rail GET
//   - (h *ChannelHandler) RegisterCHN5AdminRoutes(mux, adminMw) — admin GET
//   - (h *ChannelHandler) fanoutUnarchiveSystemMessage(...) — archive complement
//
// Constraints (chn-5-spec.md §0):
//   - Design ② owner-only: GET /api/v1/me/archived-channels returns only
//     archived channels where the current user is a member; no admin PATCH route
//     is mounted.
//   - Design ③ unarchive system DM mirrors archive; text stays byte-identical
//     with content-lock §1 (`channel #{name} 已被 {owner} 恢复于 {ts}`).
//   - Design ④ admin rail is readonly: admin GET only, no PATCH/PUT/DELETE.
//   - Design ⑥ AST check site 10 requires zero matches for the three forbidden
//     tokens.
package api

import (
	"fmt"
	"net/http"
	"time"

	"borgee-server/internal/admin"
	"borgee-server/internal/store"


	"borgee-server/internal/idgen"
)

// RegisterCHN5Routes wires the user-rail archived channels GET endpoint.
// Design ② owner-only via current-user filter; no admin route is mounted.
func (h *ChannelHandler) RegisterCHN5Routes(mux *http.ServeMux, authMw func(http.Handler) http.Handler) {
	mux.Handle("GET /api/v1/me/archived-channels",
		authMw(http.HandlerFunc(h.handleListMyArchivedChannels)))
}

// RegisterCHN5AdminRoutes wires the admin-rail readonly archived channels
// GET endpoint. 设计 ④ readonly — no PATCH/PUT/DELETE on this path.
func (h *ChannelHandler) RegisterCHN5AdminRoutes(mux *http.ServeMux, adminMw func(http.Handler) http.Handler) {
	mux.Handle("GET /admin-api/v1/channels/archived",
		adminMw(http.HandlerFunc(h.handleAdminListArchivedChannels)))
}

// handleListMyArchivedChannels — GET /api/v1/me/archived-channels.
//
// Returns the user's archived channels (membership-scoped, cross-org
// filtered, matching ListChannelsWithUnread). Design ② owner-only.
func (h *ChannelHandler) handleListMyArchivedChannels(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}
	rows, err := h.Store.ListArchivedChannelsForUser(user.ID)
	if err != nil {
		if h.Logger != nil {
			h.Logger.Error("chn5.list archived for user", "error", err)
		}
		writeJSONError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{"channels": rows})
}

// handleAdminListArchivedChannels — GET /admin-api/v1/channels/archived.
//
// Admin all-org readonly view. Design ④: GET only, no PATCH/PUT/DELETE
// (ADM-0 §1.3: admin inspects audit/history rather than modifying directly).
func (h *ChannelHandler) handleAdminListArchivedChannels(w http.ResponseWriter, r *http.Request) {
	a := admin.AdminFromContext(r.Context())
	if a == nil {
		writeJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	rows, err := h.Store.ListAllArchivedChannelsForAdmin()
	if err != nil {
		if h.Logger != nil {
			h.Logger.Error("chn5.list archived for admin", "error", err)
		}
		writeJSONError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{"channels": rows})
}

// fanoutUnarchiveSystemMessage delivers a system DM to every member of
// the un-archived channel, mirroring fanoutArchiveSystemMessage under CHN-5.2
// design ③. Content format is byte-identical with content-lock §1:
//
//	"channel #{name} 已被 {owner_name} 恢复于 {ts}"
//
// Complements the CHN-1.2 archive literal (`关闭于`) with `恢复于`; timestamp is
// RFC3339 and owner DisplayName falls back to 'system', matching the existing
// fanoutArchiveSystemMessage behavior.
func (h *ChannelHandler) fanoutUnarchiveSystemMessage(channelID, channelName, ownerID string, unarchiveTs int64) {
	h.fanoutChannelStateMessage(channelStateMessageArgs{
		channelID:    channelID,
		channelName:  channelName,
		ownerID:      ownerID,
		ts:           unarchiveTs,
		verbLiteral:  "恢复于",
		eventName:    "channel_unarchived",
		eventTSKey:   "unarchived_at",
		errLogPrefix: "fanoutUnarchiveSystemMessage failed",
	})
}

// channelStateMessageArgs carries the per-call differences across the
// archive ↔ unarchive pair: verb literal, event name, payload key, and error log
// prefix. content-lock §1 literals `"关闭于"` / `"恢复于"` stay owned by callers;
// this helper does not change them.
type channelStateMessageArgs struct {
	channelID, channelName, ownerID string
	ts                              int64
	verbLiteral                     string // "关闭于" or "恢复于"
	eventName                       string // "channel_archived" or "channel_unarchived"
	eventTSKey                      string // "archived_at" or "unarchived_at"
	errLogPrefix                    string
}

// fanoutChannelStateMessage is the single shared body for archive and
// unarchive fanout. Behavior is byte-identical with the original two inline
// sites (REFACTOR-2 helper-8): owner DisplayName fallback 'system', RFC3339
// timestamp, system DM Create, and Hub broadcast event payload
// {channel_id, <eventTSKey>, content}.
func (h *ChannelHandler) fanoutChannelStateMessage(a channelStateMessageArgs) {
	owner, err := h.Store.GetUserByID(a.ownerID)
	ownerName := "system"
	if err == nil && owner != nil && owner.DisplayName != "" {
		ownerName = owner.DisplayName
	}
	tsLabel := time.UnixMilli(a.ts).UTC().Format(time.RFC3339)
	content := fmt.Sprintf("channel #%s 已被 %s %s %s", a.channelName, ownerName, a.verbLiteral, tsLabel)
	now := nowMillis()
	msg := &store.Message{
		ID:          idgen.NewID(),
		ChannelID:   a.channelID,
		SenderID:    "system",
		Content:     content,
		ContentType: "text",
		CreatedAt:   now,
	}
	if err := h.Store.CreateMessage(msg); err != nil {
		if h.Logger != nil {
			h.Logger.Error(a.errLogPrefix, "channel_id", a.channelID, "error", err)
		}
		return
	}
	if h.Hub != nil {
		h.Hub.BroadcastEventToChannel(a.channelID, a.eventName, map[string]any{
			"channel_id":  a.channelID,
			a.eventTSKey:  a.ts,
			"content":     content,
		})
	}
}
