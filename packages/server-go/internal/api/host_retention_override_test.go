// Package api_test — hb_5_heartbeat_retention_override_test.go: HB-5.2
// admin-rail override endpoint acceptance §2.2.
//
// Pins:
//   REG-HB5-007 TestHB_OverrideEndpointWritesAudit
//   REG-HB5-008 TestHB_OverrideRejectsUserRail
//   REG-HB5-009 TestHB_OverrideClampsRetention
package api_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"borgee-server/internal/store"
	"borgee-server/internal/testutil"
)

// TestHB_OverrideEndpointWritesAudit pins acceptance §2.2 — admin POST
// writes one admin_actions row with action='audit_retention_override'
// (复用 AL-7 既有 action) + metadata.target='heartbeat'.
func TestHB_OverrideEndpointWritesAudit(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAsAdmin(t, ts.URL)

	resp, body := testutil.JSON(t, "POST",
		ts.URL+"/admin-api/v1/heartbeat-retention/override",
		adminToken,
		map[string]any{"retention_days": 60})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %v", resp.StatusCode, body)
	}
	if body["recorded"] != true {
		t.Errorf("recorded: got %v, want true", body["recorded"])
	}
	if body["target"] != "heartbeat" {
		t.Errorf("target: got %v, want %q", body["target"], "heartbeat")
	}

	// Audit row written 复用 AL-7 既有 action + metadata.target='heartbeat'.
	var rows []store.AdminAction
	s.DB().Where("action = ?", "audit_retention_override").Find(&rows)
	if len(rows) != 1 {
		t.Fatalf("audit_retention_override audit rows: got %d, want 1", len(rows))
	}
	r := rows[0]
	if r.ActorID == "" || r.ActorID == "system" {
		t.Errorf("actor_id: got %q, want admin id (ADM-0 §1.3)", r.ActorID)
	}
	var meta map[string]any
	if err := json.Unmarshal([]byte(r.Metadata), &meta); err != nil {
		t.Fatalf("metadata not valid JSON: %v", err)
	}
	if meta["target"] != "heartbeat" {
		t.Errorf("metadata.target: got %v, want %q (HB-5 设计 ②)", meta["target"], "heartbeat")
	}
}

// TestHB_OverrideRejectsUserRail — 设计 ③ admin-rail only.
func TestHB_OverrideRejectsUserRail(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	userToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	resp, _ := testutil.JSON(t, "POST",
		ts.URL+"/admin-api/v1/heartbeat-retention/override",
		userToken,
		map[string]any{"retention_days": 60})
	if resp.StatusCode == http.StatusOK {
		t.Errorf("expected non-200 (admin-rail only), got %d", resp.StatusCode)
	}
}

// TestHB_OverrideClampsRetention — 设计 ⑥ clamp 1..365 (复用 auth pkg
// const 跟 AL-7 同源).
func TestHB_OverrideClampsRetention(t *testing.T) {
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
				ts.URL+"/admin-api/v1/heartbeat-retention/override",
				adminToken, tc.body)
			if resp.StatusCode != http.StatusBadRequest {
				t.Errorf("%s: expected 400, got %d", tc.name, resp.StatusCode)
			}
		})
	}

	// Boundary 1 + 365 PASS.
	for _, days := range []int{1, 365} {
		resp, _ := testutil.JSON(t, "POST",
			ts.URL+"/admin-api/v1/heartbeat-retention/override",
			adminToken, map[string]any{"retention_days": days})
		if resp.StatusCode != http.StatusOK {
			t.Errorf("boundary %d: expected 200, got %d", days, resp.StatusCode)
		}
	}
}

// REG-HB5-cov — 401 unauthorized branch.
func TestHB_Override_NoAdmin401(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	resp, _ := testutil.JSON(t, http.MethodPost,
		ts.URL+"/admin-api/v1/heartbeat-retention/override", "",
		map[string]any{"retention_days": 30})
	if resp.StatusCode == http.StatusOK {
		t.Errorf("no-token: got 200, want non-200")
	}
}

// REG-HB5-cov — invalid JSON body 400.
func TestHB_Override_InvalidJSON(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAsAdmin(t, ts.URL)
	req, _ := http.NewRequest(http.MethodPost,
		ts.URL+"/admin-api/v1/heartbeat-retention/override",
		strings.NewReader("not json {"))
	req.AddCookie(&http.Cookie{Name: "borgee_admin_session", Value: adminToken})
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("invalid-json: got %d, want 400", resp.StatusCode)
	}
}

// REG-HB5-cov — TargetUserID specified (covers non-default target branch).
func TestHB_Override_WithTargetUserID(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAsAdmin(t, ts.URL)
	resp, _ := testutil.JSON(t, http.MethodPost,
		ts.URL+"/admin-api/v1/heartbeat-retention/override", adminToken,
		map[string]any{
			"retention_days":  30,
			"target_user_id":  "specific-user-123",
		})
	if resp.StatusCode != http.StatusOK {
		t.Errorf("with target: got %d, want 200", resp.StatusCode)
	}
}

