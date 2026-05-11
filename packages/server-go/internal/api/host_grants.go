// Package api — host_grants.go: HB-3.1 host_grants 单一来源 REST endpoints
// (情境化授权 4 类). Owner-only ACL gate (anchor #360 同模式).
//
// Spec: docs/implementation/modules/hb-3-spec.md §1 HB-3.1.
// Acceptance: docs/qa/acceptance-templates/hb-3.md §1.
// 原则: docs/qa/hb-3-stance-checklist.md §1+§2+§3.
// Blueprint出处: docs/blueprint/current/host-bridge.md §1.3 (4 类: install/exec/
// filesystem/network) + §1.5 release gate 第 5 行 (撤销 < 100ms) + §2
// 信任五支柱第 3 条 (可审计日志).
//
// Endpoint surface:
//   - POST   /api/v1/host-grants              create grant (insert row)
//   - GET    /api/v1/host-grants              list active grants for caller
//   - DELETE /api/v1/host-grants/{id}         revoke (stamp revoked_at,
//                                              forward-only — 不真删行,
//                                              留作 audit; HB-4 §1.5
//                                              release gate 第 5 行
//                                              撤销 < 100ms 真测)
//
// Design constraints (原则 §0+§1+§2+§3):
//   - 设计 ① schema 单一来源 — server-go 唯一 INSERT/UPDATE/DELETE 路径; HB-2
//     daemon (Go module packages/borgee-helper) read-only.
//   - 设计 ② 字典分立 — 不复用 user_permissions schema (host vs runtime);
//     handler code must not join `host_grants` to `user_permissions`.
//   - 设计 ⑤ ttl_kind 2-enum matches permission UX values (one_shot/always
//     ↔ data-action="grant_one_shot"/"grant_always").
//   - 设计 ⑦ no admin-wide host grant path — 用户主权 (蓝图 §1.3 + ADM-0 §1.3 红线).

package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"borgee-server/internal/store"


	"borgee-server/internal/idgen"
	"gorm.io/gorm"
)

// HostGrantsHandler handles host_grants 单一来源 endpoints (HB-3.1).
type HostGrantsHandler struct {
	Store  *store.Store
	Logger *slog.Logger
	Now    func() time.Time // injectable clock for tests; defaults to time.Now.
}

func (h *HostGrantsHandler) now() int64 {
	if h.Now != nil {
		return h.Now().UnixMilli()
	}
	return time.Now().UnixMilli()
}

func (h *HostGrantsHandler) RegisterRoutes(mux *http.ServeMux, authMw func(http.Handler) http.Handler) {
	wrap := func(f http.HandlerFunc) http.Handler { return authMw(f) }
	mux.Handle("POST /api/v1/host-grants", wrap(h.handlePost))
	mux.Handle("GET /api/v1/host-grants", wrap(h.handleList))
	mux.Handle("DELETE /api/v1/host-grants/{id}", wrap(h.handleDelete))
}

// hostGrantRow mirrors migration v=27 (HB-3.1 schema 9 列).
type hostGrantRow struct {
	ID        string `gorm:"column:id"          json:"id"`
	UserID    string `gorm:"column:user_id"     json:"user_id"`
	AgentID   string `gorm:"column:agent_id"    json:"agent_id,omitempty"`
	GrantType string `gorm:"column:grant_type"  json:"grant_type"`
	Scope     string `gorm:"column:scope"       json:"scope"`
	TtlKind   string `gorm:"column:ttl_kind"    json:"ttl_kind"`
	GrantedAt int64  `gorm:"column:granted_at"  json:"granted_at"`
	ExpiresAt *int64 `gorm:"column:expires_at"  json:"expires_at,omitempty"`
	RevokedAt *int64 `gorm:"column:revoked_at"  json:"revoked_at,omitempty"`
}

func (hostGrantRow) TableName() string { return "host_grants" }

// hostGrantTypeWhitelist lists the 4 grant_type values from blueprint §1.3 and
// the DB CHECK constraint. Keep this map, migration CHECK, and UI content rules aligned.
var hostGrantTypeWhitelist = map[string]bool{
	"install":    true, // 装机时授权 (Helper 装/卸 runtime 二进制)
	"exec":       true, // 装机时授权 (启动 runtime 进程)
	"filesystem": true, // 触发时授权 (agent 读用户目录)
	"network":    true, // 触发时授权 (agent 出站非 Borgee 域)
}

// hostGrantTtlWhitelist lists the 2 ttl_kind values used by the permission UX:
// one_shot ↔ grant_one_shot, always ↔ grant_always.
var hostGrantTtlWhitelist = map[string]bool{
	"one_shot": true, // "仅这一次" — expires_at = now + 1h
	"always":   true, // "始终允许" — expires_at NULL
}

// oneShotTtlMs is the 1h TTL for "仅这一次"; keep it aligned with the permission UX.
const oneShotTtlMs int64 = 60 * 60 * 1000

const hostGrantSaveErrorMsg = "host grant 保存失败, 请重试"

type hostGrantPostRequest struct {
	AgentID   string `json:"agent_id"` // optional — install/exec is user-level (empty)
	GrantType string `json:"grant_type"`
	Scope     string `json:"scope"`
	TtlKind   string `json:"ttl_kind"`
}

// handlePost — POST /api/v1/host-grants. Owner-only; callers write their own
// grants and there is no admin-wide host grant path.
func (h *HostGrantsHandler) handlePost(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}
	var req hostGrantPostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONErrorCode(w, http.StatusBadRequest, "host_grants.invalid_payload", "invalid JSON body")
		return
	}
	if !hostGrantTypeWhitelist[req.GrantType] {
		writeJSONErrorCode(w, http.StatusBadRequest, "host_grants.grant_type_invalid",
			"grant_type must be one of: install/exec/filesystem/network")
		return
	}
	if !hostGrantTtlWhitelist[req.TtlKind] {
		writeJSONErrorCode(w, http.StatusBadRequest, "host_grants.ttl_kind_invalid",
			"ttl_kind must be one of: one_shot/always")
		return
	}
	if req.Scope == "" {
		writeJSONErrorCode(w, http.StatusBadRequest, "host_grants.scope_required",
			"scope must not be empty")
		return
	}

	now := h.now()
	row := hostGrantRow{
		ID:        idgen.NewID(),
		UserID:    user.ID,
		AgentID:   req.AgentID,
		GrantType: req.GrantType,
		Scope:     req.Scope,
		TtlKind:   req.TtlKind,
		GrantedAt: now,
	}
	if req.TtlKind == "one_shot" {
		exp := now + oneShotTtlMs
		row.ExpiresAt = &exp
	}

	if err := h.Store.DB().Create(&row).Error; err != nil {
		h.logErr("host_grants insert", err)
		writeJSONError(w, http.StatusInternalServerError, hostGrantSaveErrorMsg)
		return
	}

	if h.Logger != nil {
		// HB-3 audit log: 5 fields aligned with HB-1 / HB-2 / BPP-4 dead-letter audit.
		h.Logger.Info("host_grants.granted",
			"actor", user.ID,
			"action", "grant",
			"target", req.GrantType+":"+req.Scope,
			"when", now,
			"scope", row.ID)
	}

	writeJSONResponse(w, http.StatusCreated, row)
}

// handleList — GET /api/v1/host-grants. Returns active (revoked_at IS
// NULL AND (expires_at IS NULL OR expires_at > now)) grants for caller.
func (h *HostGrantsHandler) handleList(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}
	now := h.now()
	var rows []hostGrantRow
	if err := h.Store.DB().
		Where("user_id = ? AND revoked_at IS NULL AND (expires_at IS NULL OR expires_at > ?)",
			user.ID, now).
		Order("granted_at DESC").
		Find(&rows).Error; err != nil {
		h.logErr("host_grants list", err)
		writeJSONError(w, http.StatusInternalServerError, "Failed to list host grants")
		return
	}
	writeJSONResponse(w, http.StatusOK, map[string]any{"grants": rows})
}

// handleDelete — DELETE /api/v1/host-grants/{id}. Forward-only revoke
// (stamp revoked_at, 不真删行 — 留作 audit). owner-only (cross-user 403
// — 设计 ⑦).
//
// HB-4 §1.5 release gate 第 5 行: 撤销 → daemon 立即拒绝 < 100ms. v1
// 实现 = REST DELETE → revoked_at NOT NULL + daemon 每次 IPC 重查
// daemon rechecks on every IPC call; it does not cache grant state.
func (h *HostGrantsHandler) handleDelete(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}
	id := r.PathValue("id")
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "id required")
		return
	}
	var row hostGrantRow
	if err := h.Store.DB().Where("id = ?", id).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			writeJSONError(w, http.StatusNotFound, "Host grant not found")
			return
		}
		h.logErr("host_grants get", err)
		writeJSONError(w, http.StatusInternalServerError, "Failed to load host grant")
		return
	}
	// Owner-only ACL (cross-user reject 403, 设计 ⑦ 跟 anchor #360 同源).
	if row.UserID != user.ID {
		writeJSONError(w, http.StatusForbidden, "Forbidden")
		return
	}
	if row.RevokedAt != nil {
		// Idempotent: already revoked.
		writeJSONResponse(w, http.StatusOK, map[string]any{"id": id, "revoked_at": *row.RevokedAt})
		return
	}
	now := h.now()
	if err := h.Store.DB().Model(&hostGrantRow{}).
		Where("id = ?", id).
		Update("revoked_at", now).Error; err != nil {
		h.logErr("host_grants revoke", err)
		writeJSONError(w, http.StatusInternalServerError, hostGrantSaveErrorMsg)
		return
	}

	if h.Logger != nil {
		// HB-3 audit log: 5 fields aligned with HB-1 / HB-2 / BPP-4 audit.
		h.Logger.Info("host_grants.revoked",
			"actor", user.ID,
			"action", "revoke",
			"target", row.GrantType+":"+row.Scope,
			"when", now,
			"scope", id)
	}

	writeJSONResponse(w, http.StatusOK, map[string]any{"id": id, "revoked_at": now})
}

func (h *HostGrantsHandler) logErr(op string, err error) {
	if h.Logger != nil {
		h.Logger.Error("host_grants error", "op", op, "err", err)
	}
}
