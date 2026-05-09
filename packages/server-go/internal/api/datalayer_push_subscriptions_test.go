// Package api_test — dl_4_2_push_subscriptions_test.go: DL-4.2 server
// REST endpoint tests for web_push_subscriptions.
//
// Stance pins exercised (蓝图 client-shape.md L22 + dl-4-spec §0):
//   - 设计 ① POST upsert: 同 endpoint 重注册 → 不再插新 row, p256dh/auth
//     原地更新 (UNIQUE 严闭 + ON CONFLICT DO UPDATE).
//   - 设计 ① 反约束: secret 字段在 server env, 不接受 client 传 (此测试
//     不 inject secret, 验证 4 字面字段路径).
//   - 设计 ③ 退订单源: DELETE row 不开 PATCH enabled=false 双源.
//   - cross-user reject: REG-INV-002 fail-closed.
//   - idempotent unsubscribe: 不存在 endpoint DELETE 仍返 204 (跟 layout
//     DELETE 同模式).
package api_test

import (
	"net/http"
	"testing"

	"borgee-server/internal/testutil"
)

// TestDL_SubscribeRoundTrip pins acceptance §1 — POST 后 row 落库 +
// DELETE 后 row 消失 (round-trip 完整路径).
func TestDL_SubscribeRoundTrip(t *testing.T) {
	t.Parallel()
	ts, store, _ := testutil.NewTestServer(t)
	token := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	resp, body := testutil.JSON(t, "POST", ts.URL+"/api/v1/push/subscribe", token, map[string]any{
		"endpoint":   "https://fcm.googleapis.com/fcm/send/test-1",
		"p256dh":     "BNcRdreALRFXTkOOUHK1EtK2wtaz5Ry4YfYCA_0QTpQtUbVlUls0VJXg7A8u-Ts1XbjhazAkj7I99e8QcYP7DkM",
		"auth":       "tBHItJI5svbpez7KI4CCXg",
		"user_agent": "Mozilla/5.0 (TestUA)",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST expected 200, got %d: %v", resp.StatusCode, body)
	}
	if body["endpoint"] != "https://fcm.googleapis.com/fcm/send/test-1" {
		t.Errorf("response endpoint mismatch: got %v", body["endpoint"])
	}

	// Verify row in db.
	var count int64
	store.DB().Raw(`SELECT COUNT(*) FROM web_push_subscriptions WHERE endpoint=?`,
		"https://fcm.googleapis.com/fcm/send/test-1").Scan(&count)
	if count != 1 {
		t.Fatalf("expected 1 row in web_push_subscriptions, got %d", count)
	}

	// DELETE.
	resp, _ = testutil.JSON(t, "DELETE",
		ts.URL+"/api/v1/push/subscribe?endpoint=https://fcm.googleapis.com/fcm/send/test-1",
		token, nil)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("DELETE expected 204, got %d", resp.StatusCode)
	}

	store.DB().Raw(`SELECT COUNT(*) FROM web_push_subscriptions WHERE endpoint=?`,
		"https://fcm.googleapis.com/fcm/send/test-1").Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 rows after DELETE, got %d", count)
	}
}

// TestDL_UpsertSameEndpoint pins acceptance §2 — 同 endpoint 重注册 →
// 行原地更新 p256dh/auth, 不插新 row (UNIQUE 严闭).
func TestDL_UpsertSameEndpoint(t *testing.T) {
	t.Parallel()
	ts, store, _ := testutil.NewTestServer(t)
	token := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	endpoint := "https://fcm.googleapis.com/fcm/send/upsert-1"

	// First registration.
	resp, _ := testutil.JSON(t, "POST", ts.URL+"/api/v1/push/subscribe", token, map[string]any{
		"endpoint": endpoint,
		"p256dh":   "p256dh-v1",
		"auth":     "auth-v1",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST 1 expected 200, got %d", resp.StatusCode)
	}

	// Re-register same endpoint with new p256dh/auth (browser refresh).
	resp, _ = testutil.JSON(t, "POST", ts.URL+"/api/v1/push/subscribe", token, map[string]any{
		"endpoint": endpoint,
		"p256dh":   "p256dh-v2-refreshed",
		"auth":     "auth-v2-refreshed",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST 2 (upsert) expected 200, got %d", resp.StatusCode)
	}

	// Still exactly 1 row, with refreshed values.
	var count int64
	store.DB().Raw(`SELECT COUNT(*) FROM web_push_subscriptions WHERE endpoint=?`, endpoint).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 row after upsert, got %d (UNIQUE constraint missing?)", count)
	}

	var p256dh string
	store.DB().Raw(`SELECT p256dh_key FROM web_push_subscriptions WHERE endpoint=?`, endpoint).Scan(&p256dh)
	if p256dh != "p256dh-v2-refreshed" {
		t.Errorf("p256dh not refreshed on upsert: got %q, want %q", p256dh, "p256dh-v2-refreshed")
	}
}

// TestDL_CrossUserReject pins REG-INV-002 fail-closed — user-B 不能
// 操作 user-A 的 endpoint subscription (POST 409 / DELETE 403).
func TestDL_CrossUserReject(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	tokenA := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	tokenB := testutil.LoginAs(t, ts.URL, "admin@test.com", "password123")

	endpoint := "https://fcm.googleapis.com/fcm/send/cross-user"

	// User A registers.
	resp, _ := testutil.JSON(t, "POST", ts.URL+"/api/v1/push/subscribe", tokenA, map[string]any{
		"endpoint": endpoint,
		"p256dh":   "userA-p256dh",
		"auth":     "userA-auth",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST userA expected 200, got %d", resp.StatusCode)
	}

	// User B tries POST same endpoint → 409 cross_user_reject.
	resp, body := testutil.JSON(t, "POST", ts.URL+"/api/v1/push/subscribe", tokenB, map[string]any{
		"endpoint": endpoint,
		"p256dh":   "userB-p256dh",
		"auth":     "userB-auth",
	})
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("POST userB cross-user expected 409, got %d: %v", resp.StatusCode, body)
	}
	if code, _ := body["code"].(string); code != "push.cross_user_reject" {
		t.Errorf("expected code=push.cross_user_reject, got %v", body["code"])
	}

	// User B tries DELETE userA's endpoint → 403.
	resp, body = testutil.JSON(t, "DELETE",
		ts.URL+"/api/v1/push/subscribe?endpoint="+endpoint, tokenB, nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("DELETE userB cross-user expected 403, got %d: %v", resp.StatusCode, body)
	}
}

// TestDL_InvalidPayload pins acceptance §1 — 4 字面字段缺一即 reject
// (push.endpoint_invalid).
func TestDL_InvalidPayload(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	token := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	cases := []struct {
		name string
		body map[string]any
	}{
		{"empty endpoint", map[string]any{"endpoint": "", "p256dh": "x", "auth": "y"}},
		{"empty p256dh", map[string]any{"endpoint": "https://x.test", "p256dh": "", "auth": "y"}},
		{"empty auth", map[string]any{"endpoint": "https://x.test", "p256dh": "x", "auth": ""}},
		{"all empty", map[string]any{}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			resp, body := testutil.JSON(t, "POST", ts.URL+"/api/v1/push/subscribe", token, c.body)
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("expected 400 for %s, got %d", c.name, resp.StatusCode)
			}
			if code, _ := body["code"].(string); code != "push.endpoint_invalid" {
				t.Errorf("expected code=push.endpoint_invalid, got %v", body["code"])
			}
		})
	}
}

// TestDL_UnsubscribeIdempotent pins acceptance §3 — DELETE 不存在
// endpoint 仍返 204 (跟 layout DELETE 同模式 idempotent).
func TestDL_UnsubscribeIdempotent(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	token := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	// DELETE without ever POSTing — should still 204 (idempotent retry).
	resp, _ := testutil.JSON(t, "DELETE",
		ts.URL+"/api/v1/push/subscribe?endpoint=https://never-registered.test/", token, nil)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("idempotent DELETE expected 204, got %d", resp.StatusCode)
	}
}

// TestDL_UnsubscribeRequiresEndpoint pins endpoint query param required.
func TestDL_UnsubscribeRequiresEndpoint(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	token := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	resp, body := testutil.JSON(t, "DELETE", ts.URL+"/api/v1/push/subscribe", token, nil)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("DELETE without endpoint expected 400, got %d", resp.StatusCode)
	}
	if code, _ := body["code"].(string); code != "push.endpoint_invalid" {
		t.Errorf("expected code=push.endpoint_invalid, got %v", body["code"])
	}
}

// TestDL_UnauthorizedNoToken pins auth requirement — POST/DELETE without
// borgee_token cookie → 401 (跟 agent_config / layout 同模式).
func TestDL_UnauthorizedNoToken(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)

	resp, _ := testutil.JSON(t, "POST", ts.URL+"/api/v1/push/subscribe", "", map[string]any{
		"endpoint": "https://x.test/", "p256dh": "x", "auth": "y",
	})
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("POST without token expected 401, got %d", resp.StatusCode)
	}

	resp, _ = testutil.JSON(t, "DELETE", ts.URL+"/api/v1/push/subscribe?endpoint=x", "", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("DELETE without token expected 401, got %d", resp.StatusCode)
	}
}
