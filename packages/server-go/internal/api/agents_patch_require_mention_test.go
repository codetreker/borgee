package api_test

// Tests for owner-rail PATCH /api/v1/agents/{id} — `require_mention` toggle.
//
// Coverage matrix (paired with handler comment block in agents.go):
//   happy path     : owner flips require_mention, GET reflects new value
//   anti-IDOR      : user A PATCHes user B's agent → 404, DB unchanged
//   not owner      : non-owner member PATCH → 404, DB unchanged
//   not found      : missing id → 404
//   unknown field  : strict-decode → 400
//   wrong type     : require_mention non-bool → 400
//   empty body     : no mutable fields → 400
//   unauthenticated: no token → 401
//
// Anti-leak: every authz negative (IDOR / non-owner) returns the same 404
// shape as truly-missing, so callers can't probe existence of someone
// else's agent ids.

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"borgee-server/internal/auth"
	"borgee-server/internal/testutil"
)

// rawJSON sends a request with an arbitrary string body (bypasses JSON
// marshal so we can craft malformed payloads and unknown fields).
func rawJSON(t *testing.T, method, url, token, body string) (*http.Response, map[string]any) {
	t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, r)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.AddCookie(&http.Cookie{Name: auth.CookieName, Value: token})
		req.Header.Set("Authorization", "Bearer "+token)
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	bb, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	var out map[string]any
	_ = json.Unmarshal(bb, &out)
	_ = bytes.MinRead // keep "bytes" import live for future helpers
	return resp, out
}

func TestPatchAgentRequireMention(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	ownerTok := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	memberTok := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")

	// Owner creates an agent (require_mention defaults to true).
	resp, data := testutil.JSON(t, "POST", ts.URL+"/api/v1/agents", ownerTok, map[string]any{
		"display_name": "PatchBot",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create: %d %v", resp.StatusCode, data)
	}
	agent := data["agent"].(map[string]any)
	agentID := agent["id"].(string)
	if rm, ok := agent["require_mention"].(bool); !ok || rm != true {
		t.Fatalf("expected default require_mention=true, got %v", agent["require_mention"])
	}

	// Member creates their own agent — we'll use this in IDOR tests as
	// memberTok's "legit" agent, and also as a non-owner trying to PATCH
	// owner's agent.
	resp, data = testutil.JSON(t, "POST", ts.URL+"/api/v1/agents", memberTok, map[string]any{
		"display_name": "MemberBot",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("member create: %d %v", resp.StatusCode, data)
	}
	memberAgent := data["agent"].(map[string]any)
	memberAgentID := memberAgent["id"].(string)

	t.Run("HappyPath_FlipFalseThenBack", func(t *testing.T) {
		// Flip to false.
		resp, data := testutil.JSON(t, "PATCH", ts.URL+"/api/v1/agents/"+agentID, ownerTok, map[string]any{
			"require_mention": false,
		})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("PATCH false: %d %v", resp.StatusCode, data)
		}
		ag := data["agent"].(map[string]any)
		if rm, _ := ag["require_mention"].(bool); rm != false {
			t.Fatalf("response require_mention != false: %v", ag["require_mention"])
		}

		// GET — DB really changed (anti "UI flipped but DB didn't").
		resp, data = testutil.JSON(t, "GET", ts.URL+"/api/v1/agents/"+agentID, ownerTok, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET: %d %v", resp.StatusCode, data)
		}
		ag = data["agent"].(map[string]any)
		if rm, _ := ag["require_mention"].(bool); rm != false {
			t.Fatalf("DB require_mention != false: %v", ag["require_mention"])
		}

		// Flip back to true (idempotency of forward + back path).
		resp, _ = testutil.JSON(t, "PATCH", ts.URL+"/api/v1/agents/"+agentID, ownerTok, map[string]any{
			"require_mention": true,
		})
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("PATCH true: %d", resp.StatusCode)
		}
		resp, data = testutil.JSON(t, "GET", ts.URL+"/api/v1/agents/"+agentID, ownerTok, nil)
		ag = data["agent"].(map[string]any)
		if rm, _ := ag["require_mention"].(bool); rm != true {
			t.Fatalf("after flip-back require_mention != true: %v", ag["require_mention"])
		}
	})

	t.Run("AntiIDOR_MemberCannotPatchOwnersAgent", func(t *testing.T) {
		// Read current DB value first so we can assert it didn't move.
		resp, data := testutil.JSON(t, "GET", ts.URL+"/api/v1/agents/"+agentID, ownerTok, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("pre-GET: %d", resp.StatusCode)
		}
		before := data["agent"].(map[string]any)["require_mention"].(bool)

		// memberTok tries to flip owner's agent → 404 (anti existence leak).
		resp, _ = testutil.JSON(t, "PATCH", ts.URL+"/api/v1/agents/"+agentID, memberTok, map[string]any{
			"require_mention": !before,
		})
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("expected 404 (anti-IDOR), got %d", resp.StatusCode)
		}

		// DB unchanged.
		resp, data = testutil.JSON(t, "GET", ts.URL+"/api/v1/agents/"+agentID, ownerTok, nil)
		after := data["agent"].(map[string]any)["require_mention"].(bool)
		if before != after {
			t.Fatalf("DB moved despite IDOR attempt: before=%v after=%v", before, after)
		}
	})

	t.Run("AntiIDOR_OwnerCannotPatchMembersAgent", func(t *testing.T) {
		// Read member's agent state via memberTok (owner can't GET it).
		resp, data := testutil.JSON(t, "GET", ts.URL+"/api/v1/agents/"+memberAgentID, memberTok, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("pre-GET memberAgent: %d", resp.StatusCode)
		}
		before := data["agent"].(map[string]any)["require_mention"].(bool)

		// Owner tries to flip member's agent → 404.
		resp, _ = testutil.JSON(t, "PATCH", ts.URL+"/api/v1/agents/"+memberAgentID, ownerTok, map[string]any{
			"require_mention": !before,
		})
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", resp.StatusCode)
		}

		resp, data = testutil.JSON(t, "GET", ts.URL+"/api/v1/agents/"+memberAgentID, memberTok, nil)
		after := data["agent"].(map[string]any)["require_mention"].(bool)
		if before != after {
			t.Fatalf("member DB moved: before=%v after=%v", before, after)
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		resp, _ := testutil.JSON(t, "PATCH", ts.URL+"/api/v1/agents/does-not-exist", ownerTok, map[string]any{
			"require_mention": false,
		})
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", resp.StatusCode)
		}
	})

	t.Run("UnknownFieldRejected", func(t *testing.T) {
		// Extra field "disabled" — strict decode must reject so we never
		// silently widen the surface.
		resp, _ := rawJSON(t, "PATCH", ts.URL+"/api/v1/agents/"+agentID, ownerTok,
			`{"require_mention": false, "disabled": true}`)
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400 for unknown field, got %d", resp.StatusCode)
		}
	})

	t.Run("WrongTypeRejected", func(t *testing.T) {
		// require_mention as string.
		resp, _ := rawJSON(t, "PATCH", ts.URL+"/api/v1/agents/"+agentID, ownerTok,
			`{"require_mention": "yes"}`)
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400 for non-bool, got %d", resp.StatusCode)
		}
	})

	t.Run("EmptyBodyRejected", func(t *testing.T) {
		// No mutable fields supplied.
		resp, _ := rawJSON(t, "PATCH", ts.URL+"/api/v1/agents/"+agentID, ownerTok, `{}`)
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400 for empty body, got %d", resp.StatusCode)
		}
	})

	t.Run("Unauthenticated", func(t *testing.T) {
		resp, _ := rawJSON(t, "PATCH", ts.URL+"/api/v1/agents/"+agentID, "",
			`{"require_mention": false}`)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", resp.StatusCode)
		}
	})
}
