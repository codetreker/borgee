// adm_2_2_endpoints.go — ADM-2.2 admin-rail audit endpoint
// (/admin-api/v1/audit-log). 跟 ADM-1 spec §2 wire 衔接.
//
// Admin-rail (走 adminMw, /admin-api/v1/audit-log):
//   - GET  /admin-api/v1/audit-log           (设计 ③ admin 互可见 + 三 filter)
//
// User-rail surface (the three impersonation-grant GET/POST/DELETE routes
// and the /api/v1/me/admin-actions list) was removed in #975 with the
// user-facing privacy UI: the client SPA no longer has any consumer. The
// admin audit-log (and the 5 admin write handlers that write audit rows
// via EmitAdminActionAudit) remain — those are admin-rail and untouched.
//
// 反约束 (stance §1 设计 ④ + ADM2-NEG-005 grep 检查):
//   - 不开 GET /api/v1/audit-log (无 /me/) — 全站 audit log 不对全体 user
//     公开 (蓝图 §1.4 字面 "避免跨 org 隐私泄漏"); CI grep
//     `GET /api/v1/audit-log[^/]` count==0 锁
package api

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"borgee-server/internal/admin"
	"borgee-server/internal/store"
)

// AdminEndpointsHandler hosts the admin-rail audit-log endpoint.
type AdminEndpointsHandler struct {
	Store  *store.Store
	Logger *slog.Logger
}

// RegisterAdminRoutes wires the admin-rail audit log endpoint behind adminMw
// (走 borgee_admin_session cookie). 设计 ③.
func (h *AdminEndpointsHandler) RegisterAdminRoutes(mux *http.ServeMux, adminMw func(http.Handler) http.Handler) {
	mux.Handle("GET /admin-api/v1/audit-log", adminMw(http.HandlerFunc(h.handleAdminAuditLog)))
}

// handleAdminAuditLog — GET /admin-api/v1/audit-log.
//
// 设计 ③ admin 之间互可见: 默认无 WHERE; ?actor_id / ?action / ?target_user_id
// 三 filter 是 UI 收敛, 不是分桶. user cookie 走 admin-rail → admin.RequireAdmin
// middleware 已 401 (REG-ADM0-002 共享底线, 设计 ⑥ admin/user 二轨拆死).
//
// AL-8 additive filter (al-8-spec.md §0): ?since / ?until int64 ms epoch
// BETWEEN created_at; ?archived 三态 ("" or "active" 默认 / "archived" /
// "all"); ?action 多值 (重复 query param) 走 IN slice. 既有 3-filter
// (actor_id/action/target_user_id) byte-identical 不动 (设计 ①).
func (h *AdminEndpointsHandler) handleAdminAuditLog(w http.ResponseWriter, r *http.Request) {
	a := admin.AdminFromContext(r.Context())
	if a == nil {
		writeJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	q := r.URL.Query()
	filters := store.AdminActionListFilters{
		ActorID:      q.Get("actor_id"),
		Action:       q.Get("action"), // ADM-2.2 single-value backward-compat
		TargetUserID: q.Get("target_user_id"),
	}
	// AL-8 §0 设计 ⑤ — actions 多值 (重复 ?action=a&action=b). 走 IN slice;
	// 单值 ADM-2.2 既有 path byte-identical 走 filters.Action 单字段.
	if actions := q["action"]; len(actions) > 1 {
		filters.Actions = actions
		filters.Action = "" // mutex — Actions 优先
	}
	// AL-8 §0 设计 ④ — since/until int64 ms epoch; reject negative / non-int.
	if v := q.Get("since"); v != "" {
		ms, err := al8ParseEpochMs(v)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "audit_log.time_range_invalid")
			return
		}
		filters.Since = &ms
	}
	if v := q.Get("until"); v != "" {
		ms, err := al8ParseEpochMs(v)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "audit_log.time_range_invalid")
			return
		}
		filters.Until = &ms
	}
	if filters.Since != nil && filters.Until != nil && *filters.Since > *filters.Until {
		writeJSONError(w, http.StatusBadRequest, "audit_log.time_range_inverted")
		return
	}
	// AL-8 §0 设计 ③ — archived 三态 (active 默认 / archived / all).
	if v := q.Get("archived"); v != "" {
		switch v {
		case "active", "archived", "all":
			filters.ArchivedView = v
		default:
			writeJSONError(w, http.StatusBadRequest, "audit_log.archived_view_invalid")
			return
		}
	}
	limit := parseLimit(r, 100, 500)
	rows, err := h.Store.ListAdminActionsForAdmin(filters, limit)
	if err != nil {
		h.Logger.Error("list admin_actions for admin", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	out := make([]map[string]any, len(rows))
	for i, r := range rows {
		out[i] = sanitizeAdminAction(r, true /* admin_view */)
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{"actions": out})
}

// al8ParseEpochMs parses int64 ms epoch from a query string. Rejects
// negative + non-int + empty (caller must guard "" vs zero). 设计 ④
// time_range_invalid 单源.
func al8ParseEpochMs(s string) (int64, error) {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, err
	}
	if n < 0 {
		return 0, errAL8NegativeMs
	}
	return n, nil
}

var errAL8NegativeMs = errors.New("al8: negative ms epoch")

// sanitizeAdminAction renders an admin_actions row for JSON. adminView=true
// includes actor_id (admin-rail 互可见). Called from handleAdminAuditLog only
// after the user-rail consumer was removed in #975.
//
// 反约束 (stance §2 ADM2-NEG-001): 此函数不渲染 raw UUID 包装的"模板字面"
// (e.g. `{admin_id}`); body 渲染走 store helper RenderAdminActionDMBody.
func sanitizeAdminAction(row store.AdminAction, adminView bool) map[string]any {
	out := map[string]any{
		"id":             row.ID,
		"target_user_id": row.TargetUserID,
		"action":         row.Action,
		"metadata":       row.Metadata,
		"created_at":     row.CreatedAt,
	}
	if adminView {
		out["actor_id"] = row.ActorID
	}
	// ADMIN-SPA-SHAPE-FIX D4 (走 A): AL-8 §0 条原则③ archived 三态 surface.
	// nil-safe — null/缺 = active row (不写字段); non-null = archived row
	// (写 archived_at: int64 ms epoch). client UI 走 row class 三态渲染.
	if row.ArchivedAt != nil {
		out["archived_at"] = *row.ArchivedAt
	}
	return out
}

// parseLimit reads ?limit= with sensible defaults + caps.
func parseLimit(r *http.Request, def, max int) int {
	q := r.URL.Query().Get("limit")
	if q == "" {
		return def
	}
	n, err := strconv.Atoi(q)
	if err != nil || n <= 0 {
		return def
	}
	if n > max {
		return max
	}
	return n
}
