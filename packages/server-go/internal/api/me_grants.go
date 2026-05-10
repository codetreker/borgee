// Package api — me_grants.go: BPP-3.2.2 owner-side `POST /api/v1/me/grants`
// endpoint for one-click capability grant from owner DM (蓝图
// auth-permissions.md §1.3 主入口字面 + bpp-3.2-spec.md §1 设计 ②).
//
// Flow:
//   1. owner sees system DM written by BPP-3.2.1 CapabilityGrantHandler
//      with three quick_action buttons (content-lock §3 byte-identical:
//      "授权" / "拒绝" / "稍后").
//   2. SystemMessageBubble.tsx (BPP-3.2.2 client) renders the buttons,
//      click → POST /api/v1/me/grants with body
//      `{agent_id, capability, scope, request_id, action}`.
//   3. action="grant" → Store.GrantPermission(agent_id, capability, scope);
//      action="reject"/"snooze" → log only (v1 不持久化 deny list, spec §4
//      留作后续; v2+ deny list 实施时再加).
//
// 反向约束 (bpp-3.2-stance §2):
//   - capability MUST be in auth.Capabilities (14 项 const), 字典外值 reject
//     + log warn `bpp.grant_capability_disallowed` (跟 BPP-3.2.1 同源错码).
//   - scope MUST ∈ v1 三层 ({*, channel:<id>, artifact:<id>}); 反向约束 ⑦
//     `workspace:` / `org:` 等脱节值 reject.
//   - owner-only ACL: caller MUST be agent.OwnerID (反向约束 ⑥ admin 不入此
//     路径; admin grant 走 /admin-api 单独 mw).
//   - admin god-mode 不挂 — 此 endpoint 仅 user-rail 注册.
//
// Audit-only path for reject/snooze: log line records the dismissal so
// future v2+ deny-list-by-cap implementation can replay (no persistent
// state mutation in v1).

package api

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"borgee-server/internal/auth"
	"borgee-server/internal/store"
)

// MeGrantsAction enum byte-identical 跟 bpp-3.2-content-lock.md §2
// (改 = 改两处: content-lock + 此 const).
const (
	MeGrantsActionGrant   = "grant"
	MeGrantsActionReject  = "reject"
	MeGrantsActionSnooze  = "snooze"
)

// validMeGrantsActions is the 3-enum membership set (反向约束 content-lock §2).
var validMeGrantsActions = map[string]bool{
	MeGrantsActionGrant:  true,
	MeGrantsActionReject: true,
	MeGrantsActionSnooze: true,
}

// validMeGrantsScopes / scopePrefixes are the v1 三层 scope guards (反向约束 ⑦).
// `*` is the wildcard; `channel:<id>` / `artifact:<id>` are prefix-bound.
var validMeGrantsScopePrefixes = []string{"channel:", "artifact:"}

// MeGrantsErrCode* — error code literals byte-identical 跟
// bpp-3.2-content-lock.md §4 + BPP-3.2.1 同源错码 (跟
// bpp.grant_capability_disallowed 命名同模式).
const (
	MeGrantsErrCodeActionUnknown     = "bpp.grant_action_unknown"
	MeGrantsErrCodeScopeUnknown      = "bpp.grant_scope_unknown"
	MeGrantsErrCodeAgentNotFound     = "bpp.grant_agent_not_found"
	MeGrantsErrCodeNotOwner          = "bpp.grant_not_owner"
	MeGrantsErrCodeMissingFields     = "bpp.grant_missing_fields"
)

// MeGrantsHandler handles the owner-rail one-click grant endpoint.
type MeGrantsHandler struct {
	Store  *store.Store
	Logger *slog.Logger
}

// RegisterRoutes mounts POST /api/v1/me/grants behind the user-rail authMw.
// 反向约束: not mounted on /admin-api (admin god-mode 走单独 mw, ADM-0 §1.3).
func (h *MeGrantsHandler) RegisterRoutes(mux *http.ServeMux, authMw func(http.Handler) http.Handler) {
	mux.Handle("POST /api/v1/me/grants", authMw(http.HandlerFunc(h.handleGrant)))
}

type meGrantsRequest struct {
	AgentID    string `json:"agent_id"`
	Capability string `json:"capability"`
	Scope      string `json:"scope"`
	RequestID  string `json:"request_id"`
	Action     string `json:"action"`
}

func (h *MeGrantsHandler) handleGrant(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}

	var req meGrantsRequest
	if err := readJSON(r, &req); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	for name, v := range map[string]string{
		"agent_id":   req.AgentID,
		"capability": req.Capability,
		"scope":      req.Scope,
		"request_id": req.RequestID,
		"action":     req.Action,
	} {
		if strings.TrimSpace(v) == "" {
			h.errCode(w, http.StatusBadRequest, MeGrantsErrCodeMissingFields,
				fmt.Sprintf("field %q empty", name))
			return
		}
	}
	// action ∈ 3-enum (content-lock §2 + 反向约束 #1 原则 §2).
	if !validMeGrantsActions[req.Action] {
		h.errCode(w, http.StatusBadRequest, MeGrantsErrCodeActionUnknown,
			fmt.Sprintf("action=%q (3-enum: grant/reject/snooze)", req.Action))
		return
	}
	// capability ∈ AP-1 14 项 const (反向约束 #1 + spec §3 #1, 跟 BPP-3.2.1 同源).
	if !auth.IsValidCapability(req.Capability) {
		h.errCode(w, http.StatusBadRequest, CapabilityGrantErrCodeCapabilityDisallowed,
			fmt.Sprintf("capability=%q (AP-1 Capabilities 14 项)", req.Capability))
		return
	}
	// scope ∈ v1 三层 (反向约束 ⑦ 原则 §2 + content-lock §2).
	if !meGrantsScopeValid(req.Scope) {
		h.errCode(w, http.StatusBadRequest, MeGrantsErrCodeScopeUnknown,
			fmt.Sprintf("scope=%q (v1 三层: */channel:<id>/artifact:<id>)", req.Scope))
		return
	}

	// owner-only ACL (反向约束 ⑥ 原则 §2).
	agent, err := h.Store.GetUserByID(req.AgentID)
	if err != nil {
		h.errCode(w, http.StatusNotFound, MeGrantsErrCodeAgentNotFound,
			fmt.Sprintf("agent_id=%q not found", req.AgentID))
		return
	}
	if agent.OwnerID == nil || *agent.OwnerID != user.ID {
		h.errCode(w, http.StatusForbidden, MeGrantsErrCodeNotOwner,
			fmt.Sprintf("user=%q is not owner of agent=%q", user.ID, req.AgentID))
		return
	}

	switch req.Action {
	case MeGrantsActionGrant:
		// Real grant via existing AP-0 path (idempotent — FirstOrCreate
		// inside store.GrantPermission).
		if err := h.Store.GrantPermission(&store.UserPermission{
			UserID:     req.AgentID,
			Permission: req.Capability,
			Scope:      req.Scope,
			GrantedBy:  &user.ID,
		}); err != nil {
			writeJSONError(w, http.StatusInternalServerError, "grant write failed")
			return
		}
		h.logInfo("bpp.grant.granted", req, user.ID)
		writeJSONResponse(w, http.StatusOK, map[string]any{
			"granted":    true,
			"action":     req.Action,
			"agent_id":   req.AgentID,
			"capability": req.Capability,
			"scope":      req.Scope,
		})
	case MeGrantsActionReject, MeGrantsActionSnooze:
		// v1 audit-only: spec §4 留作后续 — 不持久化 deny list, 仅记 log
		// 供 v2+ replay. Future BPP-3.2.3 plugin retry cache 自动 abort
		// on owner reject (BPP-3.2.3 后续 实施 trigger).
		h.logInfo("bpp.grant."+req.Action, req, user.ID)
		writeJSONResponse(w, http.StatusOK, map[string]any{
			"granted": false,
			"action":  req.Action,
		})
	}
}

// meGrantsScopeValid validates v1 三层 scope (反向约束 ⑦ 原则 §2).
func meGrantsScopeValid(scope string) bool {
	if scope == "*" {
		return true
	}
	for _, p := range validMeGrantsScopePrefixes {
		if strings.HasPrefix(scope, p) && len(scope) > len(p) {
			return true
		}
	}
	return false
}

// errCode writes a structured error body with `error_code` (跟
// BPP-2.2/2.3/3.2.1 错码体系同模式).
func (h *MeGrantsHandler) errCode(w http.ResponseWriter, status int, code, detail string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	fmt.Fprintf(w, `{"error":"Bad Request","error_code":%q,"detail":%q}`, code, detail)
}

// logInfo logs the audit-only outcome line for grant/reject/snooze.
// nil-safe Logger.
func (h *MeGrantsHandler) logInfo(event string, req meGrantsRequest, ownerID string) {
	if h.Logger == nil {
		return
	}
	h.Logger.Info(event,
		"owner_id", ownerID,
		"agent_id", req.AgentID,
		"capability", req.Capability,
		"scope", req.Scope,
		"request_id", req.RequestID,
	)
}

// IsMeGrantsActionUnknown / IsMeGrantsScopeUnknown — sentinel matchers
// (parallel to bpp.IsSemanticOpUnknown — exposed for future BPP-3.2.3
// plugin SDK retry path that may need to map 400 codes to local cache
// state machines).
var (
	errMeGrantsActionUnknown = errors.New("bpp: grant action unknown")
	errMeGrantsScopeUnknown  = errors.New("bpp: grant scope unknown")
)

func IsMeGrantsActionUnknown(err error) bool { return errors.Is(err, errMeGrantsActionUnknown) }
func IsMeGrantsScopeUnknown(err error) bool  { return errors.Is(err, errMeGrantsScopeUnknown) }
