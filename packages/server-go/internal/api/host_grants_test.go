// Package api_test — host_grants_test.go: HB-3.1 REST CRUD acceptance
// tests + 字典分立 / audit schema / AST scan 约束.
//
// Acceptance: docs/qa/acceptance-templates/hb-3.md §1.
// 设计约束: docs/qa/hb-3-stance-checklist.md §1+§2+§3.
package api_test

import (
	"net/http"
	"reflect"
	"testing"

	"borgee-server/internal/testutil"
)

// ---- §1 schema + REST CRUD (7 tests) ----

func TestHB_POST_HappyPath_Filesystem(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	token := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	resp, body := testutil.JSON(t, "POST", ts.URL+"/api/v1/host-grants", token,
		map[string]any{
			"grant_type": "filesystem",
			"scope":      "/home/user/code",
			"ttl_kind":   "always",
		})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %v", resp.StatusCode, body)
	}
	if body["grant_type"] != "filesystem" {
		t.Errorf("grant_type=%v, want filesystem", body["grant_type"])
	}
	if body["ttl_kind"] != "always" {
		t.Errorf("ttl_kind=%v, want always", body["ttl_kind"])
	}
	// "always" → expires_at NULL (omitempty 不出现在 body)
	if _, ok := body["expires_at"]; ok {
		t.Errorf("ttl_kind=always should have no expires_at, got %v", body["expires_at"])
	}
}

func TestHB_POST_OneShot_HasExpiresAt(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	token := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	resp, body := testutil.JSON(t, "POST", ts.URL+"/api/v1/host-grants", token,
		map[string]any{
			"grant_type": "network",
			"scope":      "api.example.com",
			"ttl_kind":   "one_shot",
		})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %v", resp.StatusCode, body)
	}
	exp, ok := body["expires_at"].(float64)
	if !ok || exp == 0 {
		t.Errorf("ttl_kind=one_shot should have expires_at > 0, got %v", body["expires_at"])
	}
	granted := body["granted_at"].(float64)
	if exp <= granted {
		t.Errorf("expires_at %v should be > granted_at %v", exp, granted)
	}
}

func TestHB_POST_GrantTypeEnumReject(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	token := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	for _, bad := range []string{"admin", "sudo", "root", ""} {
		resp, body := testutil.JSON(t, "POST", ts.URL+"/api/v1/host-grants", token,
			map[string]any{
				"grant_type": bad,
				"scope":      "x",
				"ttl_kind":   "always",
			})
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("grant_type=%q should be 400, got %d: %v", bad, resp.StatusCode, body)
		}
	}
}

func TestHB_POST_TtlKindEnumReject(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	token := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	for _, bad := range []string{"once", "forever", "permanent", ""} {
		resp, body := testutil.JSON(t, "POST", ts.URL+"/api/v1/host-grants", token,
			map[string]any{
				"grant_type": "filesystem",
				"scope":      "/x",
				"ttl_kind":   bad,
			})
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("ttl_kind=%q should be 400, got %d: %v", bad, resp.StatusCode, body)
		}
	}
}

func TestHB_GET_ListActive(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	token := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	for _, scope := range []string{"/a", "/b"} {
		resp, _ := testutil.JSON(t, "POST", ts.URL+"/api/v1/host-grants", token,
			map[string]any{
				"grant_type": "filesystem",
				"scope":      scope,
				"ttl_kind":   "always",
			})
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("seed grant: %d", resp.StatusCode)
		}
	}
	resp, body := testutil.JSON(t, "GET", ts.URL+"/api/v1/host-grants", token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET: %d %v", resp.StatusCode, body)
	}
	grants, ok := body["grants"].([]any)
	if !ok {
		t.Fatalf("grants array missing: %v", body)
	}
	if len(grants) != 2 {
		t.Errorf("expected 2 active grants, got %d", len(grants))
	}
}

func TestHB_DELETE_RevokeStampsRevokedAt(t *testing.T) {
	t.Parallel()
	// Acceptance §1.4: revoke → revoked_at NOT NULL; daemon 不缓存 路径
	// (HB-4 §1.5 release gate 第 5 行 < 100ms 的 v1 实现).
	ts, _, _ := testutil.NewTestServer(t)
	token := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	resp, body := testutil.JSON(t, "POST", ts.URL+"/api/v1/host-grants", token,
		map[string]any{
			"grant_type": "filesystem",
			"scope":      "/revoke-test",
			"ttl_kind":   "always",
		})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("seed: %d", resp.StatusCode)
	}
	id := body["id"].(string)

	resp, body = testutil.JSON(t, "DELETE", ts.URL+"/api/v1/host-grants/"+id, token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("DELETE: %d %v", resp.StatusCode, body)
	}
	if rev, ok := body["revoked_at"].(float64); !ok || rev == 0 {
		t.Errorf("expected revoked_at > 0 after DELETE, got %v", body["revoked_at"])
	}

	// GET should now exclude revoked grant.
	resp, body = testutil.JSON(t, "GET", ts.URL+"/api/v1/host-grants", token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET: %d", resp.StatusCode)
	}
	grants, _ := body["grants"].([]any)
	for _, g := range grants {
		gm := g.(map[string]any)
		if gm["id"] == id {
			t.Errorf("revoked grant still in active list: %v", gm)
		}
	}
}

func TestHB_DELETE_CrossUser403(t *testing.T) {
	t.Parallel()
	// 设计约束 §0 设计第 7 条 admin 权限不入 + cross-user reject 403
	// (anchor #360 同模式).
	ts, _, _ := testutil.NewTestServer(t)
	token := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	// Owner creates grant.
	resp, body := testutil.JSON(t, "POST", ts.URL+"/api/v1/host-grants", token,
		map[string]any{
			"grant_type": "filesystem",
			"scope":      "/x",
			"ttl_kind":   "always",
		})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("seed: %d %v", resp.StatusCode, body)
	}
	id := body["id"].(string)

	// Different user attempts DELETE — should 403.
	otherToken := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")
	resp, _ = testutil.JSON(t, "DELETE", ts.URL+"/api/v1/host-grants/"+id, otherToken, nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("cross-user DELETE should be 403, got %d", resp.StatusCode)
	}
}

// ---- §3 约束 — host vs runtime 字典分立 + AST scan ----

// guard linter: keep reflect import live for future schema ref tests
var _ = reflect.TypeOf
