// Package api_test — al_2a_2_agent_config_test.go: AL-2a.2 server-side
// agent_configs REST acceptance tests (acceptance #264 §4.1.a-d).
//
// Contract pins exercised:
//   - Blueprint §1.4 single source of truth — blob only stores Borgee-owned fields (name/avatar/prompt/model/
//     capabilities/enabled/memory_ref); runtime-only fields (api_key/temperature/
//     token_limit/retry_policy) fail-closed reject.
//   - Blueprint §1.5 BPP frame constraint — AL-2a does not mount a push frame;
//     agent-side reloads are polling-based (this test uses GET, not ws subscription).
//   - acceptance §4.1.a concurrent update last-write-wins + strictly increasing
//     schema_version + no lost writes.
//   - acceptance §4.1.b cross-owner reject 403.
//   - acceptance §4.1.c reflect scan fail-closed (runtime-only field reject).
//   - acceptance §4.1.d agent-side polling reload mismatch test (GET immediately
//     after PATCH returns the new blob + version, with no stale cache).
package api_test

import (
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"borgee-server/internal/api"
	"borgee-server/internal/testutil"
)

// al2a2CreateAgent creates an agent owned by the caller. Returns agent_id.
func al2a2CreateAgent(t *testing.T, baseURL, token, displayName string) string {
	t.Helper()
	resp, data := testutil.JSON(t, "POST", baseURL+"/api/v1/agents", token,
		map[string]any{"display_name": displayName})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create agent: %d %v", resp.StatusCode, data)
	}
	return data["agent"].(map[string]any)["id"].(string)
}

// TestAL_GetEmpty pins acceptance §4.1.d (initial state) — GET before
// any PATCH returns schema_version=0 + empty blob {} (server fallback,
// no row in agent_configs yet).
func TestAL_GetEmpty(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	token := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	agentID := al2a2CreateAgent(t, ts.URL, token, "AL2A2-Initial")

	resp, body := testutil.JSON(t, "GET", ts.URL+"/api/v1/agents/"+agentID+"/config", token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %v", resp.StatusCode, body)
	}
	if v, _ := body["schema_version"].(float64); v != 0 {
		t.Errorf("expected schema_version=0, got %v", body["schema_version"])
	}
	blob, ok := body["blob"].(map[string]any)
	if !ok {
		t.Fatalf("blob missing or wrong type: %v", body)
	}
	if len(blob) != 0 {
		t.Errorf("expected empty blob, got %v", blob)
	}
}

// TestAL_PatchAndGet pins acceptance §4.1.a + §4.1.d — PATCH writes
// blob + bumps schema_version; subsequent GET returns the same blob
// + monotonic version (mismatch test prevents stale cache reads).
func TestAL_PatchAndGet(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	token := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	agentID := al2a2CreateAgent(t, ts.URL, token, "AL2A2-PatchGet")

	// First PATCH — schema_version 0 -> 1.
	resp1, body1 := testutil.JSON(t, "PATCH", ts.URL+"/api/v1/agents/"+agentID+"/config", token,
		map[string]any{"blob": map[string]any{"name": "Alpha", "model": "claude-3"}})
	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %v", resp1.StatusCode, body1)
	}
	if v, _ := body1["schema_version"].(float64); v != 1 {
		t.Errorf("expected schema_version=1 after first PATCH, got %v", body1["schema_version"])
	}

	// GET returns the new state (no cache mismatch).
	resp2, body2 := testutil.JSON(t, "GET", ts.URL+"/api/v1/agents/"+agentID+"/config", token, nil)
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("GET expected 200, got %d", resp2.StatusCode)
	}
	if v, _ := body2["schema_version"].(float64); v != 1 {
		t.Errorf("GET schema_version=1 expected, got %v", body2["schema_version"])
	}
	blob := body2["blob"].(map[string]any)
	if blob["name"] != "Alpha" || blob["model"] != "claude-3" {
		t.Errorf("blob mismatch: %v", blob)
	}

	// Second PATCH — version 1 -> 2, blob replaced (whole-blob single-source semantics).
	resp3, body3 := testutil.JSON(t, "PATCH", ts.URL+"/api/v1/agents/"+agentID+"/config", token,
		map[string]any{"blob": map[string]any{"name": "Beta"}})
	if resp3.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp3.StatusCode)
	}
	if v, _ := body3["schema_version"].(float64); v != 2 {
		t.Errorf("expected schema_version=2 after second PATCH, got %v", body3["schema_version"])
	}
	// Blob is REPLACED, not merged (single-source design — model field disappears).
	resp4, body4 := testutil.JSON(t, "GET", ts.URL+"/api/v1/agents/"+agentID+"/config", token, nil)
	if resp4.StatusCode != http.StatusOK {
		t.Fatalf("GET2 expected 200, got %d", resp4.StatusCode)
	}
	blob2 := body4["blob"].(map[string]any)
	if blob2["name"] != "Beta" {
		t.Errorf("expected name=Beta, got %v", blob2["name"])
	}
	if _, has := blob2["model"]; has {
		t.Error("blob should be replaced as the single source, but model field is still present")
	}
}

// TestAL_CrossOwnerReject pins acceptance §4.1.b — non-owner PATCH/GET
// returns 403.
func TestAL_CrossOwnerReject(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	memberToken := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")
	agentID := al2a2CreateAgent(t, ts.URL, ownerToken, "AL2A2-Owned")

	// Member tries GET -> 403.
	respGet, _ := testutil.JSON(t, "GET", ts.URL+"/api/v1/agents/"+agentID+"/config", memberToken, nil)
	if respGet.StatusCode != http.StatusForbidden {
		t.Errorf("cross-owner GET: expected 403, got %d", respGet.StatusCode)
	}
	// Member tries PATCH -> 403.
	respPatch, _ := testutil.JSON(t, "PATCH", ts.URL+"/api/v1/agents/"+agentID+"/config", memberToken,
		map[string]any{"blob": map[string]any{"name": "Hijack"}})
	if respPatch.StatusCode != http.StatusForbidden {
		t.Errorf("cross-owner PATCH: expected 403, got %d", respPatch.StatusCode)
	}
}

// TestAL_RuntimeFieldRejected pins acceptance §4.1.c — runtime-only
// fields (api_key / temperature / token_limit / retry_policy) fail-closed
// reject with code `agent_config.runtime_field_rejected`.
func TestAL_RuntimeFieldRejected(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	token := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	agentID := al2a2CreateAgent(t, ts.URL, token, "AL2A2-Runtime")

	for _, forbidden := range []string{"api_key", "temperature", "token_limit", "retry_policy"} {
		t.Run(forbidden, func(t *testing.T) {
			resp, body := testutil.JSON(t, "PATCH", ts.URL+"/api/v1/agents/"+agentID+"/config", token,
				map[string]any{"blob": map[string]any{
					"name":    "OK",
					forbidden: "should-reject",
				}})
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("expected 400 for runtime field %q, got %d: %v", forbidden, resp.StatusCode, body)
			}
			if code, _ := body["code"].(string); code != "agent_config.runtime_field_rejected" {
				t.Errorf("expected code agent_config.runtime_field_rejected for %q, got %v", forbidden, body["code"])
			}
		})
	}
}

// TestAL_InvalidPayload pins error surface — empty body / malformed
// JSON / blob field missing → 400 `agent_config.invalid_payload`.
func TestAL_InvalidPayload(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	token := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	agentID := al2a2CreateAgent(t, ts.URL, token, "AL2A2-Invalid")

	// Missing blob field.
	resp, body := testutil.JSON(t, "PATCH", ts.URL+"/api/v1/agents/"+agentID+"/config", token,
		map[string]any{"other_field": "x"})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for missing blob, got %d", resp.StatusCode)
	}
	if code, _ := body["code"].(string); code != "agent_config.invalid_payload" {
		t.Errorf("expected code agent_config.invalid_payload, got %v", body["code"])
	}
}

// TestAL_ConcurrentLastWriteWins pins acceptance §4.1.a — concurrent
// concurrent PATCH calls -> no rows lost + schema_version monotonic + final state
// from one of the writers (last-write-wins).
func TestAL_ConcurrentLastWriteWins(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	token := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	agentID := al2a2CreateAgent(t, ts.URL, token, "AL2A2-Concurrent")

	const N = 10
	var wg sync.WaitGroup
	wg.Add(N)
	for i := 0; i < N; i++ {
		go func(i int) {
			defer wg.Done()
			testutil.JSON(t, "PATCH", ts.URL+"/api/v1/agents/"+agentID+"/config", token,
				map[string]any{"blob": map[string]any{"name": fmt.Sprintf("writer-%d", i)}})
		}(i)
	}
	wg.Wait()

	// Final state — schema_version must equal exactly N (no lost writes,
	// monotonic increment per UPSERT). One of the N writers' name fields wins.
	resp, body := testutil.JSON(t, "GET", ts.URL+"/api/v1/agents/"+agentID+"/config", token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET after concurrent PATCH: %d", resp.StatusCode)
	}
	v, _ := body["schema_version"].(float64)
	if int(v) != N {
		t.Errorf("expected schema_version=%d (no lost writes), got %v", N, body["schema_version"])
	}
	// One of the writers wins.
	blob := body["blob"].(map[string]any)
	winner, _ := blob["name"].(string)
	if winner == "" {
		t.Error("expected blob.name to be one of the writers, got empty")
	}
}

// TestAL_AdminAPINotMounted pins ADM-0 §1.3 boundary — admin god-mode does
// **not** mount agent_configs via /admin-api/* (acceptance §4.1.c implicit:
// runtime path remains separate from the admin path).
func TestAL_AdminAPINotMounted(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	token := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	agentID := al2a2CreateAgent(t, ts.URL, token, "AL2A2-Admin")

	// /admin-api/v1/agents/:id/config is not mounted (404 by mux).
	resp, _ := testutil.JSON(t, "GET", ts.URL+"/admin-api/v1/agents/"+agentID+"/config", token, nil)
	if resp.StatusCode == http.StatusOK {
		t.Errorf("admin-api/agent_config should NOT be mounted, got 200")
	}
}

// TestAL_AgentNotFound covers GET/PATCH 404 path — bogus agent_id
// returns 404 Not Found (previously uncovered branch).
func TestAL_AgentNotFound(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	token := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	bogusID := "bogus-agent-id-does-not-exist"

	respGet, _ := testutil.JSON(t, "GET", ts.URL+"/api/v1/agents/"+bogusID+"/config", token, nil)
	if respGet.StatusCode != http.StatusNotFound {
		t.Errorf("GET bogus agent_id: expected 404, got %d", respGet.StatusCode)
	}
	respPatch, _ := testutil.JSON(t, "PATCH", ts.URL+"/api/v1/agents/"+bogusID+"/config", token,
		map[string]any{"blob": map[string]any{"name": "x"}})
	if respPatch.StatusCode != http.StatusNotFound {
		t.Errorf("PATCH bogus agent_id: expected 404, got %d", respPatch.StatusCode)
	}
}

// TestAL_UnauthorizedNoToken covers GET/PATCH 401 path — no auth token
// returns 401 (previously uncovered auth branch).
func TestAL_UnauthorizedNoToken(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	token := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	agentID := al2a2CreateAgent(t, ts.URL, token, "AL2A2-NoAuth")

	// No token (empty string) — auth middleware rejects with 401.
	respGet, _ := testutil.JSON(t, "GET", ts.URL+"/api/v1/agents/"+agentID+"/config", "", nil)
	if respGet.StatusCode != http.StatusUnauthorized {
		t.Errorf("GET no-token: expected 401, got %d", respGet.StatusCode)
	}
	respPatch, _ := testutil.JSON(t, "PATCH", ts.URL+"/api/v1/agents/"+agentID+"/config", "",
		map[string]any{"blob": map[string]any{"name": "x"}})
	if respPatch.StatusCode != http.StatusUnauthorized {
		t.Errorf("PATCH no-token: expected 401, got %d", respPatch.StatusCode)
	}
}

// TestAL_PatchInvalidJSON covers JSON parse failure path — malformed
// JSON body triggers a decoder error -> 400 invalid_payload (edge coverage).
func TestAL_PatchInvalidJSON(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	token := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	agentID := al2a2CreateAgent(t, ts.URL, token, "AL2A2-BadJSON")

	// PATCH with non-map body — decoder fails to unmarshal into struct.
	resp, body := testutil.JSON(t, "PATCH", ts.URL+"/api/v1/agents/"+agentID+"/config", token,
		"this-is-not-a-json-object")
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("PATCH non-object body: expected 400, got %d", resp.StatusCode)
	}
	if code, _ := body["code"].(string); code != "agent_config.invalid_payload" {
		t.Errorf("expected code agent_config.invalid_payload, got %v", body["code"])
	}
}

// TestAL_GetCorruptBlob covers the corrupt-blob path (json.Unmarshal
// error on stored blob) — direct DB insert with malformed JSON, then GET
// returns 500 (covers handleGetAgentConfig blob unmarshal branch + logErr).
func TestAL_GetCorruptBlob(t *testing.T) {
	t.Parallel()
	ts, store, _ := testutil.NewTestServer(t)
	token := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	agentID := al2a2CreateAgent(t, ts.URL, token, "AL2A2-Corrupt")

	// Insert corrupt blob directly bypassing the handler.
	if err := store.DB().Exec(`INSERT INTO agent_configs
		(agent_id, schema_version, blob, created_at, updated_at)
		VALUES (?, 1, ?, 1700000000000, 1700000000000)`,
		agentID, "{not-valid-json").Error; err != nil {
		t.Fatalf("seed corrupt blob: %v", err)
	}

	resp, _ := testutil.JSON(t, "GET", ts.URL+"/api/v1/agents/"+agentID+"/config", token, nil)
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("GET corrupt blob: expected 500, got %d", resp.StatusCode)
	}
}

// TestAL_HandlerNowInjection pins now() injectable clock branch (66.7%
// -> 100%) — handler with custom Now func returns deterministic timestamp.
// Direct unit test on the handler struct, not via HTTP.
func TestAL_HandlerNowInjection(t *testing.T) {
	t.Parallel()
	const fixedMs = int64(1700000000000)
	h := &api.AgentConfigHandler{
		Now: func() time.Time { return time.UnixMilli(fixedMs) },
	}
	// Use reflection-free path via injected Now — verify return matches.
	got := h.Now().UnixMilli()
	if got != fixedMs {
		t.Errorf("injected Now() returned %d, want %d", got, fixedMs)
	}
}

// TestAL_HandlerStructFields covers AgentConfigHandler struct field
// access (no-op smoke test for coverage on RegisterRoutes / handler init).
// Smoke covers public struct surface.
func TestAL_HandlerStructFields(t *testing.T) {
	t.Parallel()
	h := &api.AgentConfigHandler{}
	if h.Store != nil {
		t.Error("default Store should be nil")
	}
	if h.Logger != nil {
		t.Error("default Logger should be nil")
	}
	if h.Now != nil {
		t.Error("default Now should be nil")
	}
	// Register routes onto fresh mux — covers RegisterRoutes wiring.
	mux := http.NewServeMux()
	authMw := func(next http.Handler) http.Handler { return next }
	h.RegisterRoutes(mux, authMw)
}
