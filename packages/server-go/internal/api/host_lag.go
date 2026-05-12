// Package api — hb_6_lag.go: HB-6 heartbeat lag percentile monitor.
//
// Blueprint: admin-model.md ADM-0 §1.3 constraint: admin readonly only.
// Spec: docs/implementation/modules/hb-6-spec.md §1 split HB-6.1 + HB-6.2.
//
// Public surface:
//   - HostLagHandler{Store, Logger}
//   - (h *HostLagHandler) RegisterAdminRoutes(mux, adminMw)
//   - WindowSeconds (= BPP-4 BPP_HEARTBEAT_TIMEOUT_SECONDS byte-identical)
//   - LagThresholdMs (= 15000, half the watchdog period)
//
// Constraints (hb-6-spec.md §0 + designs ②③):
//   - Admin rail only: RegisterAdminRoutes uses adminMw; grep checks require
//     zero `/api/v1/heartbeat-lag` user-rail matches and zero
//     POST/PATCH/PUT/DELETE matches for admin-api/v1/heartbeat-lag
//     (ADM-0 §1.3 admin readonly).
//   - Do not write tables or add an admin_actions enum; lag is a derived metric.
//   - Do not start a sweeper goroutine; the synchronous GET handler aggregates
//     on demand.
//   - AL-1a reference site 19: at_risk reason literal =
//     reasons.NetworkUnreachable, shared with the BPP-4 watchdog timeout.
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

// WindowSeconds is the 30s rolling window. It must stay byte-identical with
// BPP-4 BPP_HEARTBEAT_TIMEOUT_SECONDS; TestHB61_WindowSecondsByteIdentical
// covers this cross-reference.
const WindowSeconds = 30

// LagThresholdMs marks at_risk=true when P95 lag exceeds half the watchdog
// period. The reason literal reuses reasons.NetworkUnreachable byte-for-byte.
const LagThresholdMs = 15000

// HostLagHandler hosts the admin-rail GET endpoint that aggregates 30s
// rolling-window heartbeat lag percentiles from agent_runtimes.
type HostLagHandler struct {
	Store  *store.Store
	Logger *slog.Logger
}

// RegisterAdminRoutes wires the admin-rail GET endpoint behind adminMw.
// Design ③: admin rail only; no user-rail (`/api/v1/...`) route is mounted.
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
		// AL-1a reference site 19: byte-identical with reasons.NetworkUnreachable.
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
