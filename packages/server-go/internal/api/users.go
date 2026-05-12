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
	// DataLayer is the DL-1.2 single source for Storage, Presence,
	// EventBus, and the three repositories. It is optional in v1; legacy
	// paths still call Store directly until ArtifactRepo and the remaining
	// surface migrate in DL-1.5+. When non-nil, prefer DL-1 Repository
	// methods over store.Store equivalents so implementation swaps stay behind
	// the repository interfaces.
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

	// AP-2 design ②: the response includes `capabilities` for capability-based
	// UI. The 14 values must stay byte-identical with the `auth.ALL` single
	// source. UI renders capability tokens, not role names
	// (admin/editor/viewer/owner). Member humans receive all 14 capabilities;
	// agents and bundle-narrowed accounts receive only tokens derived from their
	// granted permissions. The `role` field remains for legacy callers, but
	// `capabilities` is the AP-2 source of truth.
	capabilities := deriveAP2Capabilities(user.Role, permissions)

	writeJSONResponse(w, http.StatusOK, map[string]any{
		"user_id": user.ID,
		// role is kept for legacy callers; AP-2 client UI must not render it
		// as a role label (design ②, content-lock §1).
		"role":         user.Role,
		"permissions":  permissions,
		"details":      details,
		"capabilities": capabilities,
	})
}

// deriveAP2Capabilities maps user.Role + permissions[] to the 14 capability
// tokens from the AP-2 design ② single source.
//
//   - Member humans (Role=="member" + permissions=["*"]) → full 14 const
//     (blueprint §1.1 + AP-0 default full grant)
//   - Agents / bundle-narrowed → filter `auth.ALL` to keep only granted tokens
//     (using the capability part before `:` of `permissions[]` entries like
//     `read_channel:*` or `commit_artifact:channel:abc`)
//
// Constraint: do not return role-derived labels such as
// admin/editor/viewer/owner.
func deriveAP2Capabilities(role string, permissions []string) []string {
	if role == "member" && len(permissions) == 1 && permissions[0] == "*" {
		// Full grant: return the 14-value list byte-identical with auth.ALL.
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
			// Forward compatibility: drop unknown tokens so v3+ literals are not
			// exposed early.
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
