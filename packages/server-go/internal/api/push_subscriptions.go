// Package api — push_subscriptions.go: DL-4 web push subscription REST
// endpoints (must-fix 收口).
//
// Blueprint: docs/blueprint/current/client-shape.md L22 (Mobile PWA + Web Push
// VAPID) + L37 ("没推送 = AI 团队像后台脚本不像同事") + L46 (实现路径).
// Spec: docs/implementation/modules/dl-4-spec.md (本 PR 同期 §1 DL-4.2).
//
// Endpoint surface:
//   - POST   /api/v1/push/subscribe       UPSERT subscription by endpoint
//     (body: {endpoint, p256dh, auth})
//   - DELETE /api/v1/push/subscribe       remove by endpoint query param
//     (?endpoint=...)
//
// Stance reverse-grep targets (蓝图 L22 + spec §0 立场 ①②③):
//   - 立场 ①: secret 在 server env (BORGEE_VAPID_PRIVATE_KEY), 不入此
//     handler request body (反约束: 不接受 client 传 vapid_secret /
//     api_key / token 字段, 服务端只读 endpoint+p256dh+auth 三键).
//   - 立场 ②: subscription 不挂 cursor (push 是 fire-and-forget, 不走
//     hub.cursors RT-1/CV-2/DM-2/CV-4/AL-2b/RT-3 6 frame 共序 sequence).
//   - 立场 ③: 退订单源 = DELETE row, 不开 PATCH enabled=false 双源.
//   - cross-user reject: subscription 归 user, 不允许跨 user owner 操作
//     (REG-INV-002 fail-closed 同源).
package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"borgee-server/internal/store"


	"borgee-server/internal/idgen"
)

// PushSubscriptionsHandler handles DL-4 web_push_subscriptions REST
// endpoints. Wired in server.go boot.
type PushSubscriptionsHandler struct {
	Store  *store.Store
	Logger *slog.Logger
	Now    func() time.Time // injectable clock for tests; defaults to time.Now.
}

func (h *PushSubscriptionsHandler) now() int64 {
	if h.Now != nil {
		return h.Now().UnixMilli()
	}
	return time.Now().UnixMilli()
}

// RegisterRoutes wires POST + DELETE under /api/v1/push/subscribe.
func (h *PushSubscriptionsHandler) RegisterRoutes(mux *http.ServeMux, authMw func(http.Handler) http.Handler) {
	wrap := func(f http.HandlerFunc) http.Handler { return authMw(f) }
	mux.Handle("POST /api/v1/push/subscribe", wrap(h.handleSubscribe))
	mux.Handle("DELETE /api/v1/push/subscribe", wrap(h.handleUnsubscribe))
}

// pushSubscribeRequest is the POST body shape. Server-side validation
// rejects empty endpoint / p256dh / auth. user_agent optional; server
// also reads request UA header as fallback (audit hint only).
//
// 反约束 (spec §0 立场 ①): 不接受 client 传 secret 字段 — server 只读
// 这 4 个字面字段, JSON 解析其他字段忽略 (encoding/json default).
type pushSubscribeRequest struct {
	Endpoint  string `json:"endpoint"`
	P256DH    string `json:"p256dh"`
	Auth      string `json:"auth"`
	UserAgent string `json:"user_agent"` // optional; falls back to request UA header
}

// handleSubscribe UPSERT a subscription for the authenticated user. Same
// endpoint twice is a revive (refresh p256dh/auth + bump created_at on
// fresh insert; existing row updates p256dh/auth in-place).
//
// Cross-user reject: endpoint UNIQUE prevents 2 users registering the
// same endpoint; if a row exists with a different user_id, 409 reject
// (跟 REG-INV-002 fail-closed 同源).
func (h *PushSubscriptionsHandler) handleSubscribe(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}

	var req pushSubscribeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSONErrorCode(w, http.StatusBadRequest, "push.endpoint_invalid", "invalid JSON body")
		return
	}
	if req.Endpoint == "" || req.P256DH == "" || req.Auth == "" {
		writeJSONErrorCode(w, http.StatusBadRequest, "push.endpoint_invalid",
			"endpoint, p256dh, auth all required")
		return
	}
	ua := req.UserAgent
	if ua == "" {
		ua = r.Header.Get("User-Agent")
	}

	now := h.now()

	// Cross-user check first — endpoint UNIQUE means we can read the
	// existing owner before INSERT.
	var existingUserID string
	row := h.Store.DB().Raw(`SELECT user_id FROM web_push_subscriptions WHERE endpoint = ?`, req.Endpoint).Row()
	if err := row.Scan(&existingUserID); err == nil && existingUserID != "" && existingUserID != user.ID {
		writeJSONErrorCode(w, http.StatusConflict, "push.cross_user_reject",
			"subscription endpoint owned by another user")
		return
	}

	// UPSERT: same endpoint by same user → refresh p256dh/auth/user_agent;
	// new endpoint → INSERT.
	if err := h.Store.DB().Exec(`INSERT INTO web_push_subscriptions
		(id, user_id, endpoint, p256dh_key, auth_key, user_agent, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(endpoint) DO UPDATE SET
		  p256dh_key = excluded.p256dh_key,
		  auth_key   = excluded.auth_key,
		  user_agent = excluded.user_agent`,
		idgen.NewID(), user.ID, req.Endpoint, req.P256DH, req.Auth, ua, now).Error; err != nil {
		h.logErr("push subscribe upsert", err)
		writeJSONError(w, http.StatusInternalServerError, "Failed to save subscription")
		return
	}

	writeJSONResponse(w, http.StatusOK, map[string]any{
		"endpoint":   req.Endpoint,
		"created_at": now,
	})
}

// handleUnsubscribe removes a subscription by endpoint query param.
// Cross-user reject: 行 user_id != current user → 403 (跟 anchor #360
// owner-only 同模式).
//
// 不存在 endpoint → 204 (idempotent: 重复退订不报错, 跟 layout DELETE
// 同模式).
func (h *PushSubscriptionsHandler) handleUnsubscribe(w http.ResponseWriter, r *http.Request) {
	user, ok := mustUser(w, r)
	if !ok {
		return
	}
	endpoint := r.URL.Query().Get("endpoint")
	if endpoint == "" {
		writeJSONErrorCode(w, http.StatusBadRequest, "push.endpoint_invalid",
			"endpoint query param required")
		return
	}

	// Cross-user check.
	var rowUserID string
	row := h.Store.DB().Raw(`SELECT user_id FROM web_push_subscriptions WHERE endpoint = ?`, endpoint).Row()
	if err := row.Scan(&rowUserID); err == nil && rowUserID != "" && rowUserID != user.ID {
		writeJSONErrorCode(w, http.StatusForbidden, "push.cross_user_reject",
			"subscription endpoint owned by another user")
		return
	}

	// Idempotent delete (rows-affected may be 0 if endpoint not registered;
	// still return 204).
	if err := h.Store.DB().Exec(`DELETE FROM web_push_subscriptions WHERE endpoint = ? AND user_id = ?`,
		endpoint, user.ID).Error; err != nil {
		h.logErr("push unsubscribe delete", err)
		writeJSONError(w, http.StatusInternalServerError, "Failed to delete subscription")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *PushSubscriptionsHandler) logErr(op string, err error) {
	if h.Logger != nil {
		h.Logger.Error("push_subscriptions error", "op", op, "err", err)
	}
}
