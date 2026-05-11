// Package api_test — hb_6_lag_test.go: HB-6 heartbeat lag percentile
// monitor acceptance §1+§2+§3.
//
// Pins:
//   REG-HB6-001 TestHost_NoSchemaChange + WindowSecondsByteIdentical
//   REG-HB6-002 TestHB_AggregateLag_PercentileCorrect
//   REG-HB6-003 TestHB_WindowCutoffExcludesStale
//   REG-HB6-004 TestHB_AdminHappyPath + _NonAdmin401 + _NoUserRailPath
//   REG-HB6-005 TestHB_AtRiskReasonByteIdentical
//   REG-HB6-006 TestHB_NoAdminWritePath + _NoLagSampleQueue (AST scan)
package api_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"borgee-server/internal/api"
	"borgee-server/internal/store"
	"borgee-server/internal/testutil"
)

// REG-HB6-001 — 0 schema 改 (grep 检查 migrations/hb_6_).
func TestHost_NoSchemaChange(t *testing.T) {
	t.Parallel()
	dir := filepath.Join("..", "migrations")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read migrations dir: %v", err)
	}
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, "hb_6_") {
			t.Errorf("HB-6 设计第 1 条 broken — found schema migration file %q (must be 0 schema)", name)
		}
	}
}

// REG-HB6-001b — WindowSeconds 字节级一致 跟 BPP-4 BPP_HEARTBEAT_TIMEOUT_SECONDS.
func TestHB_WindowSecondsByteIdentical(t *testing.T) {
	t.Parallel()
	if api.WindowSeconds != 30 {
		t.Errorf("WindowSeconds: got %d, want 30 (跟 BPP-4 BPP_HEARTBEAT_TIMEOUT_SECONDS 同源)", api.WindowSeconds)
	}
	// grep 检查 BPP-4 watchdog source — 锁 30 字面.
	body, err := os.ReadFile(filepath.Join("..", "bpp", "heartbeat_watchdog.go"))
	if err != nil {
		t.Fatalf("read bpp watchdog: %v", err)
	}
	if !strings.Contains(string(body), "BPP_HEARTBEAT_TIMEOUT_SECONDS = 30") {
		t.Error("BPP-4 watchdog 30s 字面漂移 — HB-6 WindowSeconds 双向锁 broken")
	}
}

// REG-HB6-002 — AggregateLag percentile correctness (5-sample HappyPath).
func TestHB_AggregateLag_PercentileCorrect(t *testing.T) {
	t.Parallel()
	// 5 samples 1k/5k/10k/15k/30k ms — P50≈10k, P95 ≈ 30k (top), count=5.
	lags := []int64{1000, 5000, 10000, 15000, 30000}
	snap := api.AggregateLag(lags, 1700000000000)
	if snap.Count != 5 {
		t.Errorf("count: got %d, want 5", snap.Count)
	}
	if snap.P50Ms < 9000 || snap.P50Ms > 11000 {
		t.Errorf("p50_ms: got %d, want ~10000", snap.P50Ms)
	}
	if snap.P95Ms < 25000 {
		t.Errorf("p95_ms: got %d, want >=25000 (top region)", snap.P95Ms)
	}
	if !snap.AtRisk {
		t.Errorf("at_risk: got %v, want true (P95>%d threshold)", snap.AtRisk, api.LagThresholdMs)
	}
	// REG-HB6-005 — at_risk reason 字节级一致 跟 reasons.NetworkUnreachable.
	if snap.ReasonIfAtRisk != "network_unreachable" {
		t.Errorf("reason_if_at_risk: got %q, want 'network_unreachable' (AL-1a 对齐链第 19 处)", snap.ReasonIfAtRisk)
	}
}

// REG-HB6-002b — empty samples → count=0, no at_risk.
func TestHB_AggregateLag_EmptyNoRisk(t *testing.T) {
	t.Parallel()
	snap := api.AggregateLag(nil, 1700000000000)
	if snap.Count != 0 {
		t.Errorf("count: got %d, want 0", snap.Count)
	}
	if snap.AtRisk {
		t.Error("at_risk: got true, want false (empty samples)")
	}
	if snap.ReasonIfAtRisk != "" {
		t.Errorf("reason_if_at_risk: got %q, want empty", snap.ReasonIfAtRisk)
	}
	if snap.WindowSeconds != 30 {
		t.Errorf("window_seconds: got %d, want 30", snap.WindowSeconds)
	}
	if snap.ThresholdMs != 15000 {
		t.Errorf("threshold_ms: got %d, want 15000", snap.ThresholdMs)
	}
}

// REG-HB6-002c — single-sample percentile edge case (covers len==1 branch).
func TestHB_AggregateLag_SingleSample(t *testing.T) {
	t.Parallel()
	snap := api.AggregateLag([]int64{5000}, 1700000000000)
	if snap.Count != 1 {
		t.Errorf("count: got %d, want 1", snap.Count)
	}
	if snap.P50Ms != 5000 || snap.P95Ms != 5000 || snap.P99Ms != 5000 {
		t.Errorf("single-sample percentiles: got P50=%d P95=%d P99=%d, want all 5000",
			snap.P50Ms, snap.P95Ms, snap.P99Ms)
	}
	if snap.AtRisk {
		t.Error("5000ms < 15000 threshold; at_risk should be false")
	}
}

// REG-HB6-002d — empty store → SampleLagFromStore returns empty + handleGet
// shape correct (covers handleGet HappyPath through AggregateLag path).
func TestHB_GetPresence_EmptyStore(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAsAdmin(t, ts.URL)
	resp, body := testutil.JSON(t, http.MethodGet,
		ts.URL+"/admin-api/v1/heartbeat-lag", adminToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if got, ok := body["count"].(float64); !ok || int(got) != 0 {
		t.Errorf("count: got %v, want 0 (empty store)", body["count"])
	}
	if body["at_risk"] != false {
		t.Errorf("at_risk: got %v, want false (empty store)", body["at_risk"])
	}
}

// REG-HB6-003 — window cutoff excludes stale (>30s) samples + status≠running.
func TestHB_WindowCutoffExcludesStale(t *testing.T) {
	t.Parallel()
	_, s, _ := testutil.NewTestServer(t)
	nowMs := int64(1700000000000)
	cutoff := nowMs - int64(api.WindowSeconds)*1000

	// Seed 3 rows: in-window running / stale running / in-window error.
	mustExec(t, s, `INSERT INTO agent_runtimes
		(id, agent_id, endpoint_url, process_kind, status, last_heartbeat_at, created_at, updated_at)
		VALUES
		  ('rt-fresh','agent-fresh','ws://localhost/p','openclaw','running', ?, ?, ?),
		  ('rt-stale','agent-stale','ws://localhost/p','openclaw','running', ?, ?, ?),
		  ('rt-error','agent-error','ws://localhost/p','openclaw','error',   ?, ?, ?)`,
		nowMs-5000, nowMs, nowMs,
		cutoff-5000, nowMs, nowMs,
		nowMs-2000, nowMs, nowMs)

	lags, err := api.SampleLagFromStore(t.Context(), s, nowMs)
	if err != nil {
		t.Fatalf("sample: %v", err)
	}
	if len(lags) != 1 {
		t.Fatalf("sample count: got %d, want 1 (only in-window running) — got=%v", len(lags), lags)
	}
	if lags[0] != 5000 {
		t.Errorf("lag_ms: got %d, want 5000", lags[0])
	}
}

// REG-HB6-004 — admin GET happy path returns LagSnapshot JSON.
func TestHB_AdminHappyPath(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAsAdmin(t, ts.URL)

	resp, body := testutil.JSON(t, http.MethodGet,
		ts.URL+"/admin-api/v1/heartbeat-lag", adminToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %v", resp.StatusCode, body)
	}
	for _, k := range []string{"count", "p50_ms", "p95_ms", "p99_ms", "threshold_ms", "at_risk", "sampled_at", "window_seconds"} {
		if _, ok := body[k]; !ok {
			t.Errorf("missing key %q in response: %v", k, body)
		}
	}
	// window_seconds = 30 字节级一致.
	if got, ok := body["window_seconds"].(float64); !ok || int(got) != 30 {
		t.Errorf("window_seconds: got %v, want 30", body["window_seconds"])
	}
	if got, ok := body["threshold_ms"].(float64); !ok || int64(got) != 15000 {
		t.Errorf("threshold_ms: got %v, want 15000", body["threshold_ms"])
	}
}

// REG-HB6-004b — non-admin (no admin cookie) → 401.
func TestHB_NonAdmin401(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	resp, _ := testutil.JSON(t, http.MethodGet,
		ts.URL+"/admin-api/v1/heartbeat-lag", "", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

// REG-HB6-004c — user-rail /api/v1/heartbeat-lag NOT mounted (404).
func TestHB_NoUserRailPath(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	userToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	resp, _ := testutil.JSON(t, http.MethodGet,
		ts.URL+"/api/v1/heartbeat-lag", userToken, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 (user-rail not mounted), got %d", resp.StatusCode)
	}
}

// REG-HB6-005 — at-risk reason 字面字节级一致 via end-to-end (seed
// agent_runtimes 让 P95 > LagThresholdMs, 验证 reason='network_unreachable').
func TestHB_AtRiskReasonByteIdentical(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAsAdmin(t, ts.URL)

	// Seed 5 in-window running rows, mostly high-lag → P95 > 15s.
	highLags := []int64{20000, 22000, 25000, 27000, 28000}
	for i, lag := range highLags {
		mustExec(t, s, `INSERT INTO agent_runtimes
			(id, agent_id, endpoint_url, process_kind, status, last_heartbeat_at, created_at, updated_at)
			VALUES (?, ?, 'ws://localhost/p', 'openclaw', 'running',
			        strftime('%s','now')*1000 - ?,
			        strftime('%s','now')*1000,
			        strftime('%s','now')*1000)`,
			fmt.Sprintf("rt-%d", i), fmt.Sprintf("agent-%d", i), lag)
	}

	resp, body := testutil.JSON(t, http.MethodGet,
		ts.URL+"/admin-api/v1/heartbeat-lag", adminToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if body["at_risk"] != true {
		t.Errorf("at_risk: got %v, want true", body["at_risk"])
	}
	if body["reason_if_at_risk"] != "network_unreachable" {
		t.Errorf("reason_if_at_risk: got %v, want 'network_unreachable' (AL-1a 对齐链第 19 处)",
			body["reason_if_at_risk"])
	}
}

// REG-HB6-006a — admin god-mode 不挂 PATCH/POST/PUT/DELETE 在 admin-api/v1/heartbeat-lag.
func TestHB_NoAdminWritePath(t *testing.T) {
	t.Parallel()
	dirs := []string{filepath.Join("..", "api"), filepath.Join("..", "server")}
	pat := regexp.MustCompile(`mux\.Handle\("(POST|DELETE|PATCH|PUT)[^"]*admin-api/v[0-9]+/heartbeat-lag`)
	for _, dir := range dirs {
		_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}
			if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
				return nil
			}
			body, _ := os.ReadFile(p)
			if loc := pat.FindIndex(body); loc != nil {
				t.Errorf("HB-6 设计第 3 条 broken — admin write on heartbeat-lag in %s: %q",
					p, body[loc[0]:loc[1]])
			}
			return nil
		})
	}
}

// REG-HB6-006b — AST 对齐链延伸第 16 处 forbidden 3 token.
func TestHB_NoLagSampleQueue(t *testing.T) {
	t.Parallel()
	forbidden := []string{
		"pendingLagSample",
		"lagSampleQueue",
		"deadLetterLag",
	}
	dir := filepath.Join("..", "api")
	_ = filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		body, _ := os.ReadFile(p)
		for _, tok := range forbidden {
			if strings.Contains(string(body), tok) {
				t.Errorf("AST 对齐链延伸第 16 处 broken — token %q in %s", tok, p)
			}
		}
		return nil
	})
}

// REG-HB6-006c — 0 client UI v1 (grep 检查 client/src/).
func TestHB_NoClientUIv1(t *testing.T) {
	t.Parallel()
	clientDir := filepath.Join("..", "..", "..", "client", "src")
	if _, err := os.Stat(clientDir); err != nil {
		t.Skipf("client dir not present: %v", err)
	}
	forbidden := []string{"useHeartbeatLag", "HeartbeatLagPanel"}
	_ = filepath.Walk(clientDir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(p, ".ts") && !strings.HasSuffix(p, ".tsx") {
			return nil
		}
		body, _ := os.ReadFile(p)
		for _, tok := range forbidden {
			if strings.Contains(string(body), tok) {
				t.Errorf("HB-6 设计第 6 条 broken — client UI v1 token %q in %s", tok, p)
			}
		}
		return nil
	})
}

// helpers ----------------------------------------------------------------

func mustExec(t *testing.T, s *store.Store, q string, args ...any) {
	t.Helper()
	if err := s.DB().Exec(q, args...).Error; err != nil {
		t.Fatalf("exec %q: %v", q, err)
	}
}

// Ensure encoding/json is wired so future test additions can decode.
var _ = json.NewDecoder
