// Package api — hb_5_heartbeat_retention_override.go: HB-5.2 admin-rail
// override endpoint POST /admin-api/v1/heartbeat-retention/override.
//
// Blueprint: agent-lifecycle.md §2.3 forward-only state log + AL-7 #533
// retention 模式延伸. Spec: docs/implementation/modules/hb-5-spec.md §1
// 拆段 HB-5.2 设计 ②③.
//
// Public surface:
//   - HostRetentionOverrideHandler{Store, Logger}
//   - (h *HostRetentionOverrideHandler) RegisterAdminRoutes(mux, adminMw)
//
// 反约束 (hb-5-spec.md §0 + 设计 ②③):
//   - admin-rail only — RegisterAdminRoutes 走 adminMw; user-rail 不挂.
//   - admin override 必写 admin_actions audit row (ADM-0 §1.3 红线);
//     action 复用 AL-7 既有 audit retention override action const
//     (auth.ActionAuditRetentionOverride 单源, 设计 ② 不挂第 13 项 enum).
//   - retention_days clamp 1..365 (复用 auth.RetentionMinDays /
//     RetentionMaxDays 跟 AL-7 同源).
//   - metadata.target='heartbeat' 字面区分 跟 AL-7 audit override
//     target='admin_actions' 二选一.
package api

import (
	"log/slog"
	"net/http"

	"borgee-server/internal/auth"
	"borgee-server/internal/store"
)

// HostRetentionOverrideHandler hosts the admin-rail POST endpoint that
// validates the proposed heartbeat retention window and writes one
// admin_actions audit row (reusing AL-7 既有 action; metadata.target=
// 'heartbeat' 二选一字面区分).
type HostRetentionOverrideHandler struct {
	Store  *store.Store
	Logger *slog.Logger
}

// RegisterAdminRoutes wires the admin-rail endpoint behind adminMw.
// 设计 ③: admin-rail only.
func (h *HostRetentionOverrideHandler) RegisterAdminRoutes(mux *http.ServeMux, adminMw func(http.Handler) http.Handler) {
	mux.Handle("POST /admin-api/v1/heartbeat-retention/override",
		adminMw(http.HandlerFunc(h.handleOverride)))
}

// (REFACTOR-1 R1.2: hb5OverrideRequest 已合到 retentionOverrideRequest
// SSOT in admin_retention_helper.go.)

// handleOverride — POST /admin-api/v1/heartbeat-retention/override.
//
// REFACTOR-1 R1.2: 走 helper-3 SSOT writeRetentionOverride
// (admin_retention_helper.go) — al_7 / hb_5 共享 5-step skeleton. metadata.
// target='heartbeat' 字面区分 (设计 ②) 通过 extraMeta 传入; response 加
// target 字段 (HB-5 acceptance 锚 byte-identical).
func (h *HostRetentionOverrideHandler) handleOverride(w http.ResponseWriter, r *http.Request) {
	writeRetentionOverride(w, r, h.Store, h.Logger,
		"hb5.override",
		auth.ActionAuditRetentionOverride,
		map[string]any{"target": auth.HeartbeatTargetLabel}, // 设计 ② byte-identical
		map[string]any{"target": auth.HeartbeatTargetLabel}, // response also exposes target
	)
}
