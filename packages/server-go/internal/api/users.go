package api

import (
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"borgee-server/internal/auth"
	"borgee-server/internal/datalayer"
	"borgee-server/internal/store"
)

// UserHandler handles user-related endpoints.
type UserHandler struct {
	Store *store.Store
	// DataLayer — DL-1.2 单一来源 4-interface bundle (Storage / Presence /
	// EventBus / 3 Repository). Optional in v1 (nil-safe; 历史 paths
	// still walk Store directly until ArtifactRepo + remaining surface
	// migrate in DL-1.5+). When non-nil, prefer DL-1 Repository methods
	// over store.Store equivalents (interface seam 锁定未来换实现).
	DataLayer *datalayer.DataLayer
	Logger    *slog.Logger
}

func (h *UserHandler) RegisterRoutes(mux *http.ServeMux, authMw func(http.Handler) http.Handler) {
	mux.Handle("GET /api/v1/me/permissions", authMw(http.HandlerFunc(h.handleMyPermissions)))
	mux.Handle("GET /api/v1/online", authMw(http.HandlerFunc(h.handleOnlineUsers)))
}

// GET /api/v1/me/permissions
func (h *UserHandler) handleMyPermissions(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}

	var permissions []string
	var details []map[string]any

	// ADM-0.3: no role short-circuit. Member humans hold (*, *) by AP-0
	// default; agents/bundle-narrowed accounts list explicit rows. Admin
	// permissions live on /admin-api/v1/* and are not addressed here.
	if user.Role == "member" {
		permissions = []string{"*"}
		details = []map[string]any{{"id": 0, "permission": "*", "scope": "*", "granted_by": nil, "granted_at": 0}}
	} else {
		perms, err := h.Store.ListUserPermissions(user.ID)
		if err != nil {
			h.Logger.Error("failed to list permissions", "error", err)
			writeJSONError(w, http.StatusInternalServerError, "Internal server error")
			return
		}
		for _, p := range perms {
			permissions = append(permissions, fmt.Sprintf("%s:%s", p.Permission, p.Scope))
			details = append(details, map[string]any{
				"id":         p.ID,
				"permission": p.Permission,
				"scope":      p.Scope,
				"granted_by": p.GrantedBy,
				"granted_at": p.GrantedAt,
			})
		}
	}
	if details == nil {
		details = []map[string]any{}
	}

	// AP-2 设计 ② — capability 透明 UI: response 加 `capabilities` 数组,
	// 走 14 const 单一来源 byte-identical (`auth.ALL`). UI 走 capability token
	// 渲染, 反 role 名 (admin/editor/viewer/owner) bleed. Member humans
	// 全权 → 全 14 const; agents/bundle-narrowed 仅 derive permissions
	// 中已授权的 token (grep 检查 `"role":\s*"(admin|editor|viewer|owner)"`
	// 0 hit in this response — `role` 字段保留 历史 caller 兼容, 但
	// `capabilities` 是 AP-2 单一来源).
	capabilities := deriveAP2Capabilities(user.Role, permissions)

	writeJSONResponse(w, http.StatusOK, map[string]any{
		"user_id": user.ID,
		// role kept for 历史 callers; AP-2 client UI 不显此字段
		// (反 role bleed; 设计 ② content-lock §1).
		"role":         user.Role,
		"permissions":  permissions,
		"details":      details,
		"capabilities": capabilities,
	})
}

// deriveAP2Capabilities maps user.Role + permissions[] → 14-const capability
// tokens (AP-2 设计 ② 单一来源).
//
//   - Member humans (Role=="member" + permissions=["*"]) → full 14 const
//     (蓝图 §1.1 + AP-0 default 全权)
//   - Agents / bundle-narrowed → filter `auth.ALL` keep only granted tokens
//     (走 capability part before `:` of `permissions[]` entries like
//     `read_channel:*` or `commit_artifact:channel:abc`)
//
// 反向约束: 不返回 role-derived 字面 (反 admin/editor/viewer/owner bleed).
func deriveAP2Capabilities(role string, permissions []string) []string {
	if role == "member" && len(permissions) == 1 && permissions[0] == "*" {
		// Full grant — return 14-const 单一来源 byte-identical 跟 auth.ALL.
		out := make([]string, 0, len(auth.ALL))
		out = append(out, auth.ALL...)
		return out
	}
	// Bundle-narrowed: derive token from `permission:scope` entries.
	seen := make(map[string]bool, len(permissions))
	out := make([]string, 0, len(permissions))
	for _, entry := range permissions {
		idx := strings.Index(entry, ":")
		var token string
		if idx >= 0 {
			token = entry[:idx]
		} else {
			token = entry
		}
		if !auth.IsValidCapability(token) {
			// Forward-compat: drop unknown tokens (反 leak v3+ 字面).
			continue
		}
		if !seen[token] {
			seen[token] = true
			out = append(out, token)
		}
	}
	return out
}

// GET /api/v1/online
func (h *UserHandler) handleOnlineUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.Store.GetOnlineUsers()
	if err != nil {
		h.Logger.Error("failed to get online users", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "Internal server error")
		return
	}

	userIDs := make([]string, len(users))
	for i, u := range users {
		userIDs[i] = u.ID
	}

	writeJSONResponse(w, http.StatusOK, map[string]any{"user_ids": userIDs})
}

// sanitizeUserPublic returns a public-safe user representation.
func sanitizeUserPublic(u *store.User) map[string]any {
	m := map[string]any{
		"id":              u.ID,
		"display_name":    u.DisplayName,
		"role":            u.Role,
		"avatar_url":      u.AvatarURL,
		"require_mention": u.RequireMention,
		"created_at":      u.CreatedAt,
	}
	if u.OwnerID != nil {
		m["owner_id"] = *u.OwnerID
	}
	if u.LastSeenAt != nil {
		m["last_seen_at"] = *u.LastSeenAt
	}
	return m
}
