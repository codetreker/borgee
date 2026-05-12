// Package api — al_7_audit_retention_override.go: AL-7.2 admin-rail
// override endpoint POST /admin-api/v1/audit-retention/override.
//
// Blueprint: admin-model.md ADM-0 §1.3 constraint: admin actions must write
// an audit row.
// Spec: docs/implementation/modules/al-7-spec.md §1 split AL-7.2 designs ②③.
//
// Public surface:
//   - AgentRetentionOverrideHandler{Store, Logger}
//   - (h *AgentRetentionOverrideHandler) RegisterAdminRoutes(mux, adminMw)
//
// Constraints (al-7-spec.md §0 + designs ②③):
//   - Admin rail only: RegisterAdminRoutes uses adminMw, so the admin cookie
//     middleware always runs; grep checks require zero `audit_retention_override`
//     matches in user-rail handlers.
//   - Admin overrides must write an admin_actions audit row. The
//     action='audit_retention_override' literal comes from the
//     auth.ActionAuditRetentionOverride single source and matches the al_7_1
//     migration CHECK 12-tuple.
//   - retention_days clamp 1..365 (RetentionMinDays..RetentionMaxDays);
//     0 / negative / non-numeric / >365 values reject with 400 (design ⑥).
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
// /admin-api/v1/audit-log feed (design ①: no new endpoint or table).
type AgentRetentionOverrideHandler struct {
	Store  *store.Store
	Logger *slog.Logger
}

// RegisterAdminRoutes wires the admin-rail endpoint behind adminMw.
// Design ③: admin rail only; no user-rail (`/api/v1/...`) route is mounted.
// Grep checks require zero user-rail handler matches.
func (h *AgentRetentionOverrideHandler) RegisterAdminRoutes(mux *http.ServeMux, adminMw func(http.Handler) http.Handler) {
	mux.Handle("POST /admin-api/v1/audit-retention/override",
		adminMw(http.HandlerFunc(h.handleOverride)))
}

// (REFACTOR-1 R1.2: al7OverrideRequest was merged into retentionOverrideRequest
// SSOT in admin_retention_helper.go.)

// handleOverride — POST /admin-api/v1/audit-retention/override.
//
// REFACTOR-1 R1.2: use helper-3 SSOT writeRetentionOverride
// (admin_retention_helper.go). al_7 and hb_5 share the same 5-step flow:
// missing admin context returns 401 → JSON decode → clamp → InsertAdminAction →
// response. Design ⑥ literal source: runtime hot-mutate is reserved for v3; v0
// only records the override. RetentionSweeper still uses compile-time const
// RetentionDays.
func (h *AgentRetentionOverrideHandler) handleOverride(w http.ResponseWriter, r *http.Request) {
	writeRetentionOverride(w, r, h.Store, h.Logger,
		"al7.override",
		auth.ActionAuditRetentionOverride,
		nil, // al_7 omits metadata.target; hb_5 owns that field (design ② distinction)
		nil, // al_7 response only has retention_days + recorded; hb_5 adds target
	)
}
