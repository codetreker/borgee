// Package api — chn_5_archived.go: CHN-5 channel archived UI 列表 + admin
// readonly + unarchive system DM 互补二式.
//
// Blueprint: channel-model.md §2 不变量 #3 archive 留 history. Spec:
// docs/implementation/modules/chn-5-spec.md (战马D v0). 0 schema 改 —
// channels.archived_at 列复用 CHN-1.1 #267 既有.
//
// Public surface:
//   - (h *ChannelHandler) RegisterCHN5Routes(mux, authMw) — user-rail GET
//   - (h *ChannelHandler) RegisterCHN5AdminRoutes(mux, adminMw) — admin GET
//   - (h *ChannelHandler) fanoutUnarchiveSystemMessage(...) — 互补 archive
//
// 反约束 (chn-5-spec.md §0):
//   - 设计 ② owner-only — GET /api/v1/me/archived-channels 只见 user 自己
//     member 的 archived 频道; admin god-mode **不挂 PATCH** 路径.
//   - 设计 ③ unarchive system DM 互补二式 — 文案 byte-identical 跟
//     content-lock §1 (`channel #{name} 已被 {owner} 恢复于 {ts}`).
//   - 设计 ④ admin-rail readonly — admin GET only, 无 PATCH/PUT/DELETE.
//   - 设计 ⑥ AST 锁链延伸第 10 处 forbidden 3 token 0 hit.
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
// 设计 ② owner-only via current-user filter (no admin god-mode 路径).
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
// filtered跟 ListChannelsWithUnread 同精神). 设计 ② owner-only.
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
// admin 全 org readonly 视图. 设计 ④: GET only, 无 PATCH/PUT/DELETE
// (admin god-mode ADM-0 §1.3 红线 — admin 看 audit, 不直接改).
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
// the un-archived channel — CHN-5.2 设计 ③ 互补 fanoutArchiveSystemMessage
// 二式. Content format byte-identical 跟 content-lock §1:
//
//	"channel #{name} 已被 {owner_name} 恢复于 {ts}"
//
// 跟 CHN-1.2 archive (`关闭于`) 互补字面 (`恢复于`); ts RFC3339 + owner
// DisplayName fallback 'system' 跟既有 fanoutArchiveSystemMessage 同源.
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
// archive ↔ unarchive互补二式 (4 字面: 动词 / event 名 / payload key /
// error log prefix). content-lock §1 字面 `"关闭于"` / `"恢复于"` 严守
// caller 端; helper 不漂.
type channelStateMessageArgs struct {
	channelID, channelName, ownerID string
	ts                              int64
	verbLiteral                     string // "关闭于" or "恢复于"
	eventName                       string // "channel_archived" or "channel_unarchived"
	eventTSKey                      string // "archived_at" or "unarchived_at"
	errLogPrefix                    string
}

// fanoutChannelStateMessage is the SSOT body shared by archive and
// unarchive fanout. Behavior byte-identical 跟原 2 处 inline (REFACTOR-2
// helper-8): owner DisplayName fallback 'system' / RFC3339 ts / system
// DM Create + Hub broadcast event payload {channel_id, <eventTSKey>, content}.
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
