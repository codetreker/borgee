// covbump v3+v4 — AL-8 audit-log filters + impersonation grant lifecycle
// + AL-7 audit-retention override. Bumps cov +0.1% (local 83.9% → 84.0%).
package api_test

import (
	"net/http"
	"testing"

	"borgee-server/internal/testutil"
)

// REG-covbump v3 — AL-8 audit-log filter param branches.
func TestAuditLogFilters(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAsAdmin(t, ts.URL)

	// since invalid (negative).
	resp, _ := testutil.JSON(t, http.MethodGet,
		ts.URL+"/admin-api/v1/audit-log?since=-1", adminToken, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("since=-1: got %d", resp.StatusCode)
	}
	// since invalid (non-int).
	resp, _ = testutil.JSON(t, http.MethodGet,
		ts.URL+"/admin-api/v1/audit-log?since=abc", adminToken, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("since=abc: got %d", resp.StatusCode)
	}
	// until invalid.
	resp, _ = testutil.JSON(t, http.MethodGet,
		ts.URL+"/admin-api/v1/audit-log?until=xyz", adminToken, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("until=xyz: got %d", resp.StatusCode)
	}
	// since > until → inverted.
	resp, _ = testutil.JSON(t, http.MethodGet,
		ts.URL+"/admin-api/v1/audit-log?since=2000&until=1000", adminToken, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("inverted: got %d", resp.StatusCode)
	}
	// archived invalid.
	resp, _ = testutil.JSON(t, http.MethodGet,
		ts.URL+"/admin-api/v1/audit-log?archived=foo", adminToken, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("archived=foo: got %d", resp.StatusCode)
	}
	// archived=all happy.
	resp, _ = testutil.JSON(t, http.MethodGet,
		ts.URL+"/admin-api/v1/audit-log?archived=all", adminToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("archived=all: got %d", resp.StatusCode)
	}
	// archived=archived happy.
	resp, _ = testutil.JSON(t, http.MethodGet,
		ts.URL+"/admin-api/v1/audit-log?archived=archived", adminToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("archived=archived: got %d", resp.StatusCode)
	}
	// multi-action.
	resp, _ = testutil.JSON(t, http.MethodGet,
		ts.URL+"/admin-api/v1/audit-log?action=a&action=b", adminToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("multi-action: got %d", resp.StatusCode)
	}
	// since/until happy.
	resp, _ = testutil.JSON(t, http.MethodGet,
		ts.URL+"/admin-api/v1/audit-log?since=0&until=999999999999", adminToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("since/until happy: got %d", resp.StatusCode)
	}
}

// REG-covbump v3 — impersonation-grant lifecycle test REMOVED in #975 with
// the user-rail GET/POST/DELETE routes that it exercised.

// REG-covbump v3 — AL-7 audit-retention/override branches.
func TestAL_AuditRetentionOverride(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAsAdmin(t, ts.URL)

	// Invalid JSON body.
	resp, _ := testutil.JSON(t, http.MethodPost,
		ts.URL+"/admin-api/v1/audit-retention/override", adminToken, "not-an-object")
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("invalid json: got %d", resp.StatusCode)
	}
	// Out-of-range: 0 (reject ZeroValue).
	resp, _ = testutil.JSON(t, http.MethodPost,
		ts.URL+"/admin-api/v1/audit-retention/override", adminToken,
		map[string]any{"retention_days": 0})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("0 days: got %d", resp.StatusCode)
	}
	// Out-of-range: negative.
	resp, _ = testutil.JSON(t, http.MethodPost,
		ts.URL+"/admin-api/v1/audit-retention/override", adminToken,
		map[string]any{"retention_days": -5})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("negative: got %d", resp.StatusCode)
	}
	// Out-of-range: >365.
	resp, _ = testutil.JSON(t, http.MethodPost,
		ts.URL+"/admin-api/v1/audit-retention/override", adminToken,
		map[string]any{"retention_days": 9999})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf(">365: got %d", resp.StatusCode)
	}
	// Happy: 30 days, default target (system).
	resp, _ = testutil.JSON(t, http.MethodPost,
		ts.URL+"/admin-api/v1/audit-retention/override", adminToken,
		map[string]any{"retention_days": 30})
	if resp.StatusCode != http.StatusOK {
		t.Errorf("happy 30d: got %d", resp.StatusCode)
	}
	// Happy: 90 days with explicit target_user_id.
	resp, _ = testutil.JSON(t, http.MethodPost,
		ts.URL+"/admin-api/v1/audit-retention/override", adminToken,
		map[string]any{"retention_days": 90, "target_user_id": "some-user"})
	if resp.StatusCode != http.StatusOK {
		t.Errorf("happy 90d w/target: got %d", resp.StatusCode)
	}
	// 401: no admin token.
	resp, _ = testutil.JSON(t, http.MethodPost,
		ts.URL+"/admin-api/v1/audit-retention/override", "",
		map[string]any{"retention_days": 30})
	if resp.StatusCode != http.StatusUnauthorized && resp.StatusCode != http.StatusForbidden {
		t.Errorf("no auth: got %d", resp.StatusCode)
	}
}
