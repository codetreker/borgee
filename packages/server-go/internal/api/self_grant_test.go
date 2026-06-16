// Package api_test — self_grant_test.go: AP-2 #970 self-grant endpoint
// (PUT /api/v1/permissions) unit tests. This is the caller-driven fan-out
// target for the BundleSelector: one PUT per selected capability token.
//
// Pins:
//   - happy path: a confirmed token lands a user_permissions row for the
//     signed-in user (scope defaults to "*").
//   - capability allowlist guard: token outside the 14-const → 400.
//   - scope guard: drifted scope value → 400.
//   - auth gate: unauthenticated → 401.
package api_test

import (
	"net/http"
	"testing"

	"borgee-server/internal/auth"
	"borgee-server/internal/testutil"
)

// TestSelfGrant_HappyPath — member self-grants a capability token; the row
// lands in user_permissions for the signed-in user with the default scope.
func TestSelfGrant_HappyPath(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	tok := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")

	// Resolve the member's user id so we can verify the row lands on them.
	resp0, body0 := testutil.JSON(t, "GET", ts.URL+"/api/v1/me/permissions", tok, nil)
	if resp0.StatusCode != http.StatusOK {
		t.Fatalf("me/permissions expected 200, got %d", resp0.StatusCode)
	}
	userID, _ := body0["user_id"].(string)
	if userID == "" {
		t.Fatalf("could not resolve member user_id from %v", body0)
	}

	resp, body := testutil.JSON(t, "PUT", ts.URL+"/api/v1/permissions", tok, map[string]any{
		"permission": auth.SendDM,
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%v", resp.StatusCode, body)
	}
	if got, _ := body["granted"].(bool); !got {
		t.Errorf("body.granted = %v, want true", body["granted"])
	}
	if got, _ := body["scope"].(string); got != "*" {
		t.Errorf("default scope = %q, want \"*\"", got)
	}

	perms, err := s.ListUserPermissions(userID)
	if err != nil {
		t.Fatalf("list perms: %v", err)
	}
	found := false
	for _, p := range perms {
		if p.Permission == auth.SendDM && p.Scope == "*" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("self-grant row missing from user_permissions for user=%q", userID)
	}
}

// TestSelfGrant_FanOutManyTokens — a bundle confirm fans out one PUT per
// token; every token lands its own row (idempotent FirstOrCreate).
func TestSelfGrant_FanOutManyTokens(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	tok := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")
	_, body0 := testutil.JSON(t, "GET", ts.URL+"/api/v1/me/permissions", tok, nil)
	userID, _ := body0["user_id"].(string)

	tokens := []string{auth.ReadChannel, auth.ReadArtifact, auth.ReadDM}
	for _, tk := range tokens {
		resp, _ := testutil.JSON(t, "PUT", ts.URL+"/api/v1/permissions", tok, map[string]any{
			"permission": tk,
			"scope":      "*",
		})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("token=%q expected 200, got %d", tk, resp.StatusCode)
		}
	}
	perms, _ := s.ListUserPermissions(userID)
	for _, want := range tokens {
		found := false
		for _, p := range perms {
			if p.Permission == want && p.Scope == "*" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("fan-out token %q did not land a row for user=%q", want, userID)
		}
	}
}

// TestSelfGrant_CapabilityAllowlistGuard — tokens outside the 14-const are
// rejected with 400 (no role-name literals enter the grant path).
func TestSelfGrant_CapabilityAllowlistGuard(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	tok := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")

	for _, bad := range []string{"admin", "owner", "workspace.create", "foo_bar", ""} {
		resp, _ := testutil.JSON(t, "PUT", ts.URL+"/api/v1/permissions", tok, map[string]any{
			"permission": bad,
			"scope":      "*",
		})
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("permission=%q expected 400, got %d", bad, resp.StatusCode)
		}
	}
}

// TestSelfGrant_ScopeGuard — drifted scope values (workspace:/org:/empty
// prefix) are rejected with 400 (v1 three-level scope rule).
func TestSelfGrant_ScopeGuard(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	tok := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")

	// Valid scopes pass.
	for _, ok := range []string{"*", "channel:c1", "artifact:a9"} {
		resp, _ := testutil.JSON(t, "PUT", ts.URL+"/api/v1/permissions", tok, map[string]any{
			"permission": auth.WriteChannel,
			"scope":      ok,
		})
		if resp.StatusCode != http.StatusOK {
			t.Errorf("scope=%q expected 200, got %d", ok, resp.StatusCode)
		}
	}
	// Drifted scopes rejected.
	for _, bad := range []string{"workspace:w1", "org:o1", "channel:", "artifact:"} {
		resp, _ := testutil.JSON(t, "PUT", ts.URL+"/api/v1/permissions", tok, map[string]any{
			"permission": auth.WriteChannel,
			"scope":      bad,
		})
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("scope=%q expected 400, got %d", bad, resp.StatusCode)
		}
	}
}

// TestSelfGrant_RequiresAuth — unauthenticated PUT is rejected with 401.
func TestSelfGrant_RequiresAuth(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	resp, _ := testutil.JSON(t, "PUT", ts.URL+"/api/v1/permissions", "", map[string]any{
		"permission": auth.SendDM,
		"scope":      "*",
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("unauthenticated expected 401, got %d", resp.StatusCode)
	}
}
