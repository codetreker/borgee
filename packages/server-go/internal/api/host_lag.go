// Package api — hb_6_lag.go: HB-6 heartbeat lag percentile monitor.
//
// Blueprint: admin-model.md ADM-0 §1.3 红线 (admin readonly only).
// Spec: docs/implementation/modules/hb-6-spec.md §1 拆段 HB-6.1 + HB-6.2.
//
// Public surface:
//   - HostLagHandler{Store, Logger}
//   - (h *HostLagHandler) RegisterAdminRoutes(mux, adminMw)
//   - WindowSeconds (= BPP-4 BPP_HEARTBEAT_TIMEOUT_SECONDS byte-identical)
//   - LagThresholdMs (= 15000, watchdog 周期一半)
//
// 反约束 (hb-6-spec.md §0 + 设计 ②③):
//   - admin-rail only — RegisterAdminRoutes 走 adminMw; grep 检查
//     `/api/v1/heartbeat-lag` user-rail 0 hit + POST/PATCH/PUT/DELETE
//     在 admin-api/v1/heartbeat-lag 0 hit (ADM-0 §1.3 红线 admin readonly).
//   - 不写表 / 不另起 admin_actions enum — lag derived metric, write 无意义.
//   - 0 sweeper goroutine — synchronous GET handler 即时聚合, 不 schedule.
//   - AL-1a reason 锁链第 19 处 — at_risk reason 字面 = reasons.NetworkUnreachable
//     (跟 BPP-4 watchdog timeout 同源).
package api

import (
	"context"
	"log/slog"
	"net/http"
	"sort"
	"time"

	"borgee-server/internal/admin"
	"borgee-server/internal/agent/reasons"
	"borgee-server/internal/store"
)

// WindowSeconds — 30s 滚窗 byte-identical 跟 BPP-4 BPP_HEARTBEAT_TIMEOUT_
// SECONDS 同源. 改一处 = 改两处反向锁守门 (TestHB61_WindowSecondsByteIdentical).
const WindowSeconds = 30

// LagThresholdMs — P95 lag 超此值 → at_risk=true (watchdog 周期一半,
// 体现接近超时风险). reason 复用 reasons.NetworkUnreachable byte-identical.
const LagThresholdMs = 15000

// HostLagHandler hosts the admin-rail GET endpoint that aggregates 30s
// rolling-window heartbeat lag percentiles from agent_runtimes.
type HostLagHandler struct {
	Store  *store.Store
	Logger *slog.Logger
}

// RegisterAdminRoutes wires the admin-rail GET endpoint behind adminMw.
// 设计 ③: admin-rail only. user-rail (`/api/v1/...`) 不挂.
func (h *HostLagHandler) RegisterAdminRoutes(mux *http.ServeMux, adminMw func(http.Handler) http.Handler) {
	mux.Handle("GET /admin-api/v1/heartbeat-lag",
		adminMw(http.HandlerFunc(h.handleGet)))
}

// LagSnapshot — response shape (no separate types pkg, single-source).
type LagSnapshot struct {
	Count          int    `json:"count"`
	P50Ms          int64  `json:"p50_ms"`
	P95Ms          int64  `json:"p95_ms"`
	P99Ms          int64  `json:"p99_ms"`
	ThresholdMs    int64  `json:"threshold_ms"`
	AtRisk         bool   `json:"at_risk"`
	SampledAt      int64  `json:"sampled_at"`
	WindowSeconds  int    `json:"window_seconds"`
	ReasonIfAtRisk string `json:"reason_if_at_risk,omitempty"`
}

// AggregateLag — exported for test; computes percentiles from the
// supplied lag_ms slice (already filtered by window + status). Caller
// passes nowMs so test fixtures can pin time.
func AggregateLag(lagMs []int64, nowMs int64) LagSnapshot {
	snap := LagSnapshot{
		Count:         len(lagMs),
		ThresholdMs:   LagThresholdMs,
		SampledAt:     nowMs,
		WindowSeconds: WindowSeconds,
	}
	if len(lagMs) == 0 {
		return snap
	}
	sorted := make([]int64, len(lagMs))
	copy(sorted, lagMs)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	snap.P50Ms = percentile(sorted, 50)
	snap.P95Ms = percentile(sorted, 95)
	snap.P99Ms = percentile(sorted, 99)
	if snap.P95Ms > LagThresholdMs {
		snap.AtRisk = true
		// AL-1a reason 锁链第 19 处 — byte-identical 跟 reasons.NetworkUnreachable.
		snap.ReasonIfAtRisk = reasons.NetworkUnreachable
	}
	return snap
}

// percentile — nearest-rank with linear interpolation between adjacent
// indices. p ∈ [0, 100]. sorted must be non-empty + sorted ASC.
func percentile(sorted []int64, p int) int64 {
	if len(sorted) == 0 {
		return 0
	}
	if len(sorted) == 1 {
		return sorted[0]
	}
	// Position in 1..N space using nearest-rank style; clamp upper.
	pos := float64(p) / 100.0 * float64(len(sorted)-1)
	lo := int(pos)
	hi := lo + 1
	if hi >= len(sorted) {
		return sorted[len(sorted)-1]
	}
	frac := pos - float64(lo)
	return int64(float64(sorted[lo]) + frac*float64(sorted[hi]-sorted[lo]))
}

// SampleLagFromStore — exported for test; queries agent_runtimes 30s
// rolling window WHERE status='running' AND last_heartbeat_at IS NOT
// NULL AND last_heartbeat_at >= cutoff. Returns lag_ms slice.
func SampleLagFromStore(ctx context.Context, s *store.Store, nowMs int64) ([]int64, error) {
	cutoff := nowMs - int64(WindowSeconds)*1000
	var lags []int64
	rows, err := s.DB().WithContext(ctx).Raw(`
		SELECT (? - last_heartbeat_at) AS lag_ms
		FROM agent_runtimes
		WHERE status = 'running'
		  AND last_heartbeat_at IS NOT NULL
		  AND last_heartbeat_at >= ?
	`, nowMs, cutoff).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var lag int64
		if err := rows.Scan(&lag); err != nil {
			return nil, err
		}
		if lag < 0 {
			lag = 0
		}
		lags = append(lags, lag)
	}
	return lags, rows.Err()
}

// handleGet — GET /admin-api/v1/heartbeat-lag.
//
// admin-rail only (adminMw + admin.AdminFromContext); aggregates 30s
// rolling-window lag from agent_runtimes table, returns LagSnapshot.
func (h *HostLagHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	a := admin.AdminFromContext(r.Context())
	if a == nil {
		writeJSONError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}
	nowMs := time.Now().UnixMilli()
	lags, err := SampleLagFromStore(r.Context(), h.Store, nowMs)
	if err != nil {
		if h.Logger != nil {
			h.Logger.Error("hb6.sample", "error", err)
		}
		writeJSONError(w, http.StatusInternalServerError, "Failed to sample heartbeat lag")
		return
	}
	snap := AggregateLag(lags, nowMs)
	writeJSONResponse(w, http.StatusOK, snap)
}
