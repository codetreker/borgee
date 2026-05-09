// adm_2_2_endpoints.go — ADM-2.2 user-rail + admin-rail audit + impersonate
// REST endpoints. 跟 ADM-1 spec §2 wire 衔接.
//
// User-rail (走 authMw, /api/v1/me/*):
//   - GET  /api/v1/me/admin-actions          (设计 ④ 只见自己)
//   - GET  /api/v1/me/impersonation-grant    (业主端红横幅查询当前 grant 状态)
//   - POST /api/v1/me/impersonation-grant    (业主授权 24h, 设计 ⑦)
//   - DELETE /api/v1/me/impersonation-grant  (业主主动撤销)
//
// Admin-rail (走 adminMw, /admin-api/v1/audit-log):
//   - GET  /admin-api/v1/audit-log           (设计 ③ admin 互可见 + 三 filter)
//
// 反约束 (stance §1 设计 ④ + ADM2-NEG-005 grep 检查):
//   - 不开 GET /api/v1/audit-log (无 /me/) — 全站 audit log 不对全体 user
//     公开 (蓝图 §1.4 字面 "避免跨 org 隐私泄漏"); CI grep
//     `GET /api/v1/audit-log[^/]` count==0 锁
//   - user-rail GET 忽略 ?target_user_id 参数 (跨业主 inject 防线)
package api

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"borgee-server/internal/admin"
	"borgee-server/internal/store"
)

// AdminEndpointsHandler hosts both user-rail (audit list + impersonate CRUD) and
// admin-rail (audit-log) endpoints. We keep them in one struct because
// they share the Store backend; routing is split via separate Register*
// methods called from server.go with the respective middleware.
type AdminEndpointsHandler struct {
	Store  *store.Store
	Logger *slog.Logger
}

// RegisterUserRoutes wires the user-rail endpoints behind authMw (走
// borgee_token cookie / Bearer). 设计 ④ + ⑦.
func (h *AdminEndpointsHandler) RegisterUserRoutes(mux *http.ServeMux, authMw func(http.Handler) http.Handler) {
	mux.Handle("GET /api/v1/me/admin-actions", authMw(http.HandlerFunc(h.handleListMyAdminActions)))
	mux.Handle("GET /api/v1/me/impersonation-grant", authMw(http.HandlerFunc(h.handleGetMyImpersonateGrant)))
	mux.Handle("POST /api/v1/me/impersonation-grant", authMw(http.HandlerFunc(h.handleCreateMyImpersonateGrant)))
	mux.Handle("DELETE /api/v1/me/impersonation-grant", authMw(http.HandlerFunc(h.handleRevokeMyImpersonateGrant)))
}

// RegisterAdminRoutes wires the admin-rail audit log endpoint behind adminMw
// (走 borgee_admin_session cookie). 设计 ③.
func (h *AdminEndpointsHandler) RegisterAdminRoutes(mux *http.ServeMux, adminMw func(http.Handler) http.Handler) {
	mux.Handle("GET /admin-api/v1/audit-log", adminMw(http.HandlerFunc(h.handleAdminAuditLog)))
}

// handleListMyAdminActions — GET /api/v1/me/admin-actions.
//
// 设计 ④ user 只见自己: WHERE target_user_id = current_user_id.
// 反约束: ?target_user_id 参数 server 忽略 (跨业主 inject 防线 — 测试反向断言).
func (h *AdminEndpointsHandler) handleListMyAdminActions(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}
	limit := parseLimit(r, 50, 200)
	rows, err := h.Store.ListAdminActionsForTargetUser(user.ID, limit)
	if err != nil {
		h.Logger.Error("list admin_actions for user", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	out := make([]map[string]any, len(rows))
	for i, r := range rows {
		out[i] = sanitizeAdminAction(r, false /* admin_view */)
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{"actions": out})
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

// handleGetMyImpersonateGrant — GET /api/v1/me/impersonation-grant.
//
// Returns the user's currently active grant (or `null` body) — used by
// client BannerImpersonate.tsx to render the 24h red banner with countdown.
// 设计 ⑦ + content-lock §2.
func (h *AdminEndpointsHandler) handleGetMyImpersonateGrant(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}
	g, err := h.Store.ActiveImpersonationGrant(user.ID)
	if err != nil {
		h.Logger.Error("active impersonate grant", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{
		"grant": sanitizeImpersonateGrant(g),
	})
}

// handleCreateMyImpersonateGrant — POST /api/v1/me/impersonation-grant.
//
// 蓝图 §3 字面 "由 user 创建" — 业主自己 grant. 24h 固定期限 (server 端,
// 设计 ⑦ 反约束: 不接受 client 传 expires_at). 重复 grant in-cooldown → 409.
//
// REG-ADM2-010 (ADM-2-FOLLOWUP wire) — grant 创建后 emit audit row 进
// admin_actions 表 (跟 ADM-2.2 既有 5/5 admin write handler 同精神).
// actor=user.ID + action="impersonate.start" + target=user.ID (业主自签
// 给自己; 跟 GrantImpersonation store SSOT 设计沿用).
func (h *AdminEndpointsHandler) handleCreateMyImpersonateGrant(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}
	g, err := h.Store.GrantImpersonation(user.ID)
	if err != nil {
		// store err is either grant_already_active (409) or db (500).
		if strings.Contains(err.Error(), "grant_already_active") {
			writeJSONError(w, http.StatusConflict, "impersonate.grant_already_active")
			return
		}
		h.Logger.Error("grant impersonate", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	// REG-ADM2-010 audit hook — fire AFTER successful GrantImpersonation
	// commit. actor + target = user.ID (业主自签). 跟 5/5 既有 admin write
	// handler audit 同精神 byte-identical (delete_channel / suspend_user /
	// change_role / reset_password / patch_disabled 同模式).
	if _, auditErr := h.Store.InsertAdminAction(user.ID, user.ID,
		"start_impersonation",
		`{"grant_id":"`+g.ID+`","expires_at":`+strconv.FormatInt(g.ExpiresAt, 10)+`}`,
	); auditErr != nil {
		// Best-effort — don't fail the grant on audit error; log only.
		h.Logger.Error("audit impersonate.start", "error", auditErr, "grant_id", g.ID)
	}
	writeJSONResponse(w, http.StatusCreated, map[string]any{
		"grant": sanitizeImpersonateGrant(g),
	})
}

// handleRevokeMyImpersonateGrant — DELETE /api/v1/me/impersonation-grant.
// 业主主动撤销; no-op if no active grant.
func (h *AdminEndpointsHandler) handleRevokeMyImpersonateGrant(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}
	if err := h.Store.RevokeImpersonation(user.ID); err != nil {
		h.Logger.Error("revoke impersonate", "error", err)
		writeJSONError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// sanitizeAdminAction renders an admin_actions row for JSON. adminView=true
// includes actor_id (admin-rail 互可见); adminView=false omits actor_id raw
// (user-rail 只见自己, 渲染走 client 端 lookup admin_username).
//
// 反约束 (stance §2 ADM2-NEG-001): 此函数不渲染 raw UUID 包装的"模板字面"
// (e.g. `{admin_id}`); body 渲染走 client RenderAdminActionDMBody (server
// 端 system DM 走 store helper RenderAdminActionDMBody).
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
	// user-rail 不返 actor_id raw — client 渲染时调 admin lookup endpoint
	// 把 UUID 翻成 admin_username (跟 system DM body 同源避免 UUID 漏出).
	return out
}

// sanitizeImpersonateGrant renders a grant for JSON, or null when nil.
// expires_at 是 Unix ms — client 走 setInterval(1000) 重算 remaining
// (跟 content-lock §2 红横幅 24h 倒计时 wire).
func sanitizeImpersonateGrant(g *store.ImpersonationGrant) map[string]any {
	if g == nil {
		return nil
	}
	out := map[string]any{
		"id":         g.ID,
		"user_id":    g.UserID,
		"granted_at": g.GrantedAt,
		"expires_at": g.ExpiresAt,
	}
	if g.RevokedAt != nil {
		out["revoked_at"] = *g.RevokedAt
	} else {
		out["revoked_at"] = nil
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
