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

