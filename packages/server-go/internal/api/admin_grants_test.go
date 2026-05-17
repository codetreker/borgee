// covbump v5 — host-grants + AL-5 recover + impersonation grant nil.
// Pushes cov +0.1% (local 84.0% → 84.1%).
package api_test

import (
	"net/http"
	"testing"

	"borgee-server/internal/testutil"
)

// REG-covbump v5 — AL-5 recover error branches.
func TestAL_RecoverErrors(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	// 401 no auth.
	resp, _ := testutil.JSON(t, http.MethodPost,
		ts.URL+"/api/v1/agents/some-id/recover", "", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("no auth: got %d", resp.StatusCode)
	}
	// 404 unknown agent.
	resp, _ = testutil.JSON(t, http.MethodPost,
		ts.URL+"/api/v1/agents/00000000-0000-0000-0000-000000000000/recover",
		ownerToken, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("404 unknown: got %d", resp.StatusCode)
	}
	// Empty path -> route not matched -> 404 handler-level (Go ServeMux).
	// (skip — 404 from ServeMux already.)
}

// REG-covbump v5 — sanitizeImpersonateGrant nil branch via JSON GET.
// REMOVED in #975: the user-rail GET endpoint was deleted with the
// user-facing privacy UI; the orphan covbump test went away with it.

// REG-covbump v5 — host-grants validation + lifecycle.
func TestHostGrantsLifecycle(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	// 401.
	resp, _ := testutil.JSON(t, http.MethodGet,
		ts.URL+"/api/v1/host-grants", "", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("list 401: got %d", resp.StatusCode)
	}
	resp, _ = testutil.JSON(t, http.MethodPost,
		ts.URL+"/api/v1/host-grants", "", map[string]any{})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("post 401: got %d", resp.StatusCode)
	}
	// invalid JSON.
	resp, _ = testutil.JSON(t, http.MethodPost,
		ts.URL+"/api/v1/host-grants", ownerToken, "garbage")
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("invalid json: got %d", resp.StatusCode)
	}
	// invalid grant_type.
	resp, _ = testutil.JSON(t, http.MethodPost,
		ts.URL+"/api/v1/host-grants", ownerToken,
		map[string]any{"grant_type": "bogus", "scope": "x", "ttl_kind": "one_shot"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("invalid grant_type: got %d", resp.StatusCode)
	}
	// invalid ttl_kind.
	resp, _ = testutil.JSON(t, http.MethodPost,
		ts.URL+"/api/v1/host-grants", ownerToken,
		map[string]any{"grant_type": "filesystem", "scope": "/tmp", "ttl_kind": "forever"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("invalid ttl_kind: got %d", resp.StatusCode)
	}
	// missing scope.
	resp, _ = testutil.JSON(t, http.MethodPost,
		ts.URL+"/api/v1/host-grants", ownerToken,
		map[string]any{"grant_type": "filesystem", "ttl_kind": "one_shot"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("missing scope: got %d", resp.StatusCode)
	}
	// happy: filesystem one_shot.
	resp, body := testutil.JSON(t, http.MethodPost,
		ts.URL+"/api/v1/host-grants", ownerToken,
		map[string]any{"grant_type": "filesystem", "scope": "/tmp", "ttl_kind": "one_shot"})
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("happy filesystem: got %d", resp.StatusCode)
	}
	grantID, _ := body["id"].(string)
	// happy: install always (no expires).
	resp, _ = testutil.JSON(t, http.MethodPost,
		ts.URL+"/api/v1/host-grants", ownerToken,
		map[string]any{"grant_type": "install", "scope": "host", "ttl_kind": "always"})
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("happy install: got %d", resp.StatusCode)
	}
	// list.
	resp, _ = testutil.JSON(t, http.MethodGet,
		ts.URL+"/api/v1/host-grants", ownerToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("list: got %d", resp.StatusCode)
	}
	// delete unknown id (404).
	resp, _ = testutil.JSON(t, http.MethodDelete,
		ts.URL+"/api/v1/host-grants/nonexistent-id", ownerToken, nil)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("delete 404: got %d", resp.StatusCode)
	}
	// delete happy.
	if grantID != "" {
		resp, _ = testutil.JSON(t, http.MethodDelete,
			ts.URL+"/api/v1/host-grants/"+grantID, ownerToken, nil)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("delete happy: got %d", resp.StatusCode)
		}
		// idempotent revoke.
		resp, _ = testutil.JSON(t, http.MethodDelete,
			ts.URL+"/api/v1/host-grants/"+grantID, ownerToken, nil)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("delete idempotent: got %d", resp.StatusCode)
		}
	}
}
