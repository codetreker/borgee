// Package api — al_7_audit_retention_override.go: AL-7.2 admin-rail
// override endpoint POST /admin-api/v1/audit-retention/override.
//
// Blueprint: admin-model.md ADM-0 §1.3 红线 (admin 操作必走 audit row).
// Spec: docs/implementation/modules/al-7-spec.md §1 拆段 AL-7.2 设计 ②③.
//
// Public surface:
//   - AgentRetentionOverrideHandler{Store, Logger}
//   - (h *AgentRetentionOverrideHandler) RegisterAdminRoutes(mux, adminMw)
//
// 反约束 (al-7-spec.md §0 + 设计 ②③):
//   - admin-rail only — RegisterAdminRoutes 走 adminMw (admin cookie middleware
//     必经); grep 检查 `audit_retention_override` 在 user-rail handler 0 hit.
//   - admin override 必写 admin_actions audit row (ADM-0 §1.3 红线 — admin
//     操作必留痕); action='audit_retention_override' 字面 (auth.
//     ActionAuditRetentionOverride const 单源, 跟 al_7_1 migration CHECK
//     12-tuple 同源).
//   - retention_days clamp 1..365 (RetentionMinDays..RetentionMaxDays);
//     0 / 负 / >365 reject 400 — 反 0 / 负 / 非数 / >365 reject (设计 ⑥).
package api

import (
	"log/slog"
	"net/http"

	"borgee-server/internal/auth"
	"borgee-server/internal/store"
)

// AgentRetentionOverrideHandler hosts the admin-rail POST endpoint that
// (a) clamps + validates the proposed retention window and (b) writes
// one admin_actions audit row so the override is visible in the existing
// /admin-api/v1/audit-log feed (no new endpoint, 设计 ① 不裂表).
type AgentRetentionOverrideHandler struct {
	Store  *store.Store
	Logger *slog.Logger
}

// RegisterAdminRoutes wires the admin-rail endpoint behind adminMw.
// 设计 ③: admin-rail only. user-rail (`/api/v1/...`) 不挂 — grep 检查
// 在 user-rail handler 0 hit (AdminEndpointsHandler.RegisterUserRoutes 不挂此 path).
func (h *AgentRetentionOverrideHandler) RegisterAdminRoutes(mux *http.ServeMux, adminMw func(http.Handler) http.Handler) {
	mux.Handle("POST /admin-api/v1/audit-retention/override",
		adminMw(http.HandlerFunc(h.handleOverride)))
}

// (REFACTOR-1 R1.2: al7OverrideRequest 已合到 retentionOverrideRequest
// SSOT in admin_retention_helper.go.)

// handleOverride — POST /admin-api/v1/audit-retention/override.
//
// REFACTOR-1 R1.2: 走 helper-3 SSOT writeRetentionOverride
// (admin_retention_helper.go) — al_7 / hb_5 共享 5-step skeleton (admin
// nil 401 → JSON decode → clamp → InsertAdminAction → response).
// 设计 ⑥ 字面单源 — runtime hot-mutate 留 v3, v0 仅留痕 (RetentionSweeper
// 窗口仍 compile-time const RetentionDays).
func (h *AgentRetentionOverrideHandler) handleOverride(w http.ResponseWriter, r *http.Request) {
	writeRetentionOverride(w, r, h.Store, h.Logger,
		"al7.override",
		auth.ActionAuditRetentionOverride,
		nil, // al_7 不附 metadata.target — hb_5 才用 (设计 ② 字面区分)
		nil, // al_7 response 仅 retention_days + recorded — hb_5 加 target
	)
}
