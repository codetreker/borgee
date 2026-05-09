// Package api_test — al_7_audit_retention_override_test.go: AL-7.2
// admin-rail override endpoint acceptance §2.2.
//
// Pins:
//   REG-AL7-007 TestAL_OverrideEndpointWritesAudit — POST writes admin_actions row
//   REG-AL7-008 TestAL_OverrideRejectsUserRail — user cookie 401
//   REG-AL7-009 TestAL_OverrideClampsRetention — 0/-5/999 reject 400
package api_test

import (
	"net/http"
	"testing"

	"borgee-server/internal/store"
	"borgee-server/internal/testutil"
)

// TestAL_OverrideEndpointWritesAudit pins acceptance §2.2 — admin POST
// writes one admin_actions row with action='audit_retention_override'.
func TestAL_OverrideEndpointWritesAudit(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAsAdmin(t, ts.URL)

	resp, body := testutil.JSON(t, "POST",
		ts.URL+"/admin-api/v1/audit-retention/override",
		adminToken,
		map[string]any{"retention_days": 30})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %v", resp.StatusCode, body)
	}
	if body["recorded"] != true {
		t.Errorf("recorded: got %v, want true", body["recorded"])
	}

	// Audit row written 跟 ADM-0 §1.3 红线: admin 操作必留痕.
	var rows []store.AdminAction
	s.DB().Where("action = ?", "audit_retention_override").Find(&rows)
	if len(rows) != 1 {
		t.Fatalf("audit_retention_override audit rows: got %d, want 1", len(rows))
	}
	r := rows[0]
	if r.ActorID == "" || r.ActorID == "system" {
		t.Errorf("actor_id: got %q, want admin id (ADM-0 §1.3 admin 操作必留痕)", r.ActorID)
	}
}

// TestAL_OverrideRejectsUserRail pins 设计 ③ admin-rail only — user
// cookie 调 admin-api 必 401 (admin.RequireAdmin middleware).
func TestAL_OverrideRejectsUserRail(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	userToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	resp, _ := testutil.JSON(t, "POST",
		ts.URL+"/admin-api/v1/audit-retention/override",
		userToken,
		map[string]any{"retention_days": 30})
	if resp.StatusCode == http.StatusOK {
		t.Errorf("expected non-200 (admin-rail only), got %d", resp.StatusCode)
	}
}

// TestAL_OverrideClampsRetention pins 设计 ⑥ — clamp 1..365.
//
// 0 / -5 / 999 / missing field (decoder 默认 0) 全 reject 400.
func TestAL_OverrideClampsRetention(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAsAdmin(t, ts.URL)

	cases := []struct {
		name string
		body map[string]any
	}{
		{"zero", map[string]any{"retention_days": 0}},
		{"negative", map[string]any{"retention_days": -5}},
		{"too_large", map[string]any{"retention_days": 999}},
		{"missing", map[string]any{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, _ := testutil.JSON(t, "POST",
				ts.URL+"/admin-api/v1/audit-retention/override",
				adminToken, tc.body)
			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("%s: expected 400, got %d", tc.name, resp.StatusCode)
			}
		})
	}

	// Boundary 1 + 365 PASS.
	for _, days := range []int{1, 365} {
		resp, _ := testutil.JSON(t, "POST",
			ts.URL+"/admin-api/v1/audit-retention/override",
			adminToken, map[string]any{"retention_days": days})
		if resp.StatusCode != http.StatusOK {
			t.Errorf("boundary %d: expected 200, got %d", days, resp.StatusCode)
		}
	}
}
