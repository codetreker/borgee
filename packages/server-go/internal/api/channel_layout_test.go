// Package api_test — chn_3_2_layout_test.go: CHN-3.2 server-side
// user_channel_layout REST acceptance tests (acceptance §2.* CHN-3.2).
//
// 覆盖的设计约束:
//   - 设计第 2 条 个人偏好两维 collapsed + position — GET 返本人 row;
//     PUT 批量 upsert + ON CONFLICT 复合 PK.
//   - 设计第 4 条 DM 永不参与分组 — DM channel_id PUT → 400
//     `layout.dm_not_grouped` 字节级一致 (5 源 #357/#353/#366/#402).
//   - 设计第 5 条 ADM-0 红线 — admin 不入业务路径 (本测试 + reverse grep
//     `admin.*user_channel_layout` 在 admin*.go count==0).
//   - 设计第 6 条 ordering client 端 — server 不算 MIN-1.0; 接受任意 REAL
//     position 值 (含负数, client 置顶算法).
//   - 设计第 7 条 non-member channel reject 403 (CHN-1 ACL 同源).
package api_test

import (
	"net/http"
	"testing"

	"borgee-server/internal/store"
	"borgee-server/internal/testutil"
)

func chn32FindUserID(t *testing.T, s *store.Store, email string) string {
	t.Helper()
	users, err := s.ListUsers()
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	for _, u := range users {
		if u.Email != nil && *u.Email == email {
			return u.ID
		}
	}
	t.Fatalf("user %s not found", email)
	return ""
}

func chn32CreateChannel(t *testing.T, ts string, token, name string) string {
	t.Helper()
	resp, data := testutil.JSON(t, "POST", ts+"/api/v1/channels", token,
		map[string]any{"name": name, "visibility": "private"})
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		t.Fatalf("channel create: %d %v", resp.StatusCode, data)
	}
	ch := data["channel"].(map[string]any)
	return ch["id"].(string)
}

// TestCHN_GetEmpty 覆盖 acceptance §2.1: GET /me/layout 返空数组 if
// 本人无任何 layout 行 (fallback ordering 是 client 端事, server 不补全).
func TestCHN_GetEmpty(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	token := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	resp, body := testutil.JSON(t, "GET", ts.URL+"/api/v1/me/layout", token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %v", resp.StatusCode, body)
	}
	layout, ok := body["layout"].([]any)
	if !ok {
		t.Fatalf("layout field missing or wrong type: %v", body)
	}
	if len(layout) != 0 {
		t.Fatalf("expected empty layout, got %v", layout)
	}
}

// TestCHN_PutBatchUpsertAndGet 覆盖 acceptance §2.1+§2.4 — PUT batch
// upsert, GET returns the rows ordered by position ASC.
func TestCHN_PutBatchUpsertAndGet(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	token := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	chA := chn32CreateChannel(t, ts.URL, token, "chn32-test-a")
	chB := chn32CreateChannel(t, ts.URL, token, "chn32-test-b")

	// PUT layout — chA pinned via MIN-1.0 negative position, chB collapsed.
	resp, body := testutil.JSON(t, "PUT", ts.URL+"/api/v1/me/layout", token, map[string]any{
		"layout": []map[string]any{
			{"channel_id": chA, "collapsed": 0, "position": -1.0},
			{"channel_id": chB, "collapsed": 1, "position": 5.0},
		},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT layout: %d %v", resp.StatusCode, body)
	}

	resp, body = testutil.JSON(t, "GET", ts.URL+"/api/v1/me/layout", token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET layout: %d", resp.StatusCode)
	}
	layout := body["layout"].([]any)
	if len(layout) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(layout))
	}
	// position ASC ordering — chA (-1.0) before chB (5.0).
	first := layout[0].(map[string]any)
	if first["channel_id"].(string) != chA {
		t.Errorf("first row should be chA (position=-1.0), got %v", first)
	}
	if first["position"].(float64) != -1.0 {
		t.Errorf("first row position should be -1.0, got %v", first["position"])
	}
	second := layout[1].(map[string]any)
	if second["collapsed"].(float64) != 1 {
		t.Errorf("second row collapsed should be 1, got %v", second["collapsed"])
	}

	// PUT again with updated values — UPSERT path (ON CONFLICT user_id+
	// channel_id DO UPDATE).
	resp, body = testutil.JSON(t, "PUT", ts.URL+"/api/v1/me/layout", token, map[string]any{
		"layout": []map[string]any{
			{"channel_id": chA, "collapsed": 1, "position": 0.5},
		},
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("PUT layout (upsert): %d %v", resp.StatusCode, body)
	}
	resp, body = testutil.JSON(t, "GET", ts.URL+"/api/v1/me/layout", token, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatal("GET after upsert")
	}
	layout = body["layout"].([]any)
	// chA still in list with new collapsed=1 + position=0.5 (chB unchanged).
	for _, r := range layout {
		row := r.(map[string]any)
		if row["channel_id"].(string) == chA {
			if row["collapsed"].(float64) != 1 {
				t.Errorf("upsert collapsed: got %v", row["collapsed"])
			}
			if row["position"].(float64) != 0.5 {
				t.Errorf("upsert position: got %v", row["position"])
			}
		}
	}
}

// TestCHN_DMReject 验证设计第 4 条 + content-lock 约束 — DM channel
// PUT → 400 with code `layout.dm_not_grouped` 字节级一致 (5 源
// #357/#353/#366/#402 同源).
func TestCHN_DMReject(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	memberID := chn32FindUserID(t, s, "member@test.com")

	// Create DM channel between owner and member.
	resp, data := testutil.JSON(t, "POST", ts.URL+"/api/v1/dm/"+memberID, ownerToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("DM create: %d %v", resp.StatusCode, data)
	}
	dmID := data["channel"].(map[string]any)["id"].(string)

	resp, body := testutil.JSON(t, "PUT", ts.URL+"/api/v1/me/layout", ownerToken, map[string]any{
		"layout": []map[string]any{
			{"channel_id": dmID, "collapsed": 0, "position": 1.0},
		},
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 DM reject, got %d: %v", resp.StatusCode, body)
	}
	if code, _ := body["code"].(string); code != "layout.dm_not_grouped" {
		t.Fatalf("expected code layout.dm_not_grouped (5 源 字节级一致), got %v", body["code"])
	}
}

// TestCHN_NonMemberReject 覆盖 acceptance §2.3 — non-member channel
// PUT → 403 (CHN-1 ACL 同源).
func TestCHN_NonMemberReject(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	memberToken := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")

	// Owner creates a private channel; member is NOT added.
	chID := chn32CreateChannel(t, ts.URL, ownerToken, "chn32-nonmember")

	resp, body := testutil.JSON(t, "PUT", ts.URL+"/api/v1/me/layout", memberToken, map[string]any{
		"layout": []map[string]any{
			{"channel_id": chID, "collapsed": 0, "position": 1.0},
		},
	})
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 non-member, got %d: %v", resp.StatusCode, body)
	}
}

// TestCHN_InvalidPayload 覆盖 acceptance §2.5 — empty body / malformed
// JSON → 400 with code `layout.invalid_payload`.
func TestCHN_InvalidPayload(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	token := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	// Missing layout field.
	resp, body := testutil.JSON(t, "PUT", ts.URL+"/api/v1/me/layout", token, map[string]any{})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 invalid_payload, got %d: %v", resp.StatusCode, body)
	}
	if code, _ := body["code"].(string); code != "layout.invalid_payload" {
		t.Errorf("expected code layout.invalid_payload, got %v", body["code"])
	}

	// Empty channel_id.
	resp, body = testutil.JSON(t, "PUT", ts.URL+"/api/v1/me/layout", token, map[string]any{
		"layout": []map[string]any{
			{"channel_id": "", "collapsed": 0, "position": 1.0},
		},
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 empty channel_id, got %d", resp.StatusCode)
	}
}

// TestCHN_AcceptsNegativePosition 验证设计第 3+6 条 — server 接受任意
// REAL position (含负数, client 用 MIN-1.0 表示置顶). server 不 reject 负数.
func TestCHN_AcceptsNegativePosition(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	token := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	chID := chn32CreateChannel(t, ts.URL, token, "chn32-neg-pos")

	for _, pos := range []float64{-100.5, -1.0, 0.0, 0.5, 1e6} {
		resp, body := testutil.JSON(t, "PUT", ts.URL+"/api/v1/me/layout", token, map[string]any{
			"layout": []map[string]any{
				{"channel_id": chID, "collapsed": 0, "position": pos},
			},
		})
		if resp.StatusCode != http.StatusOK {
			t.Errorf("position=%v rejected: %d %v", pos, resp.StatusCode, body)
		}
	}
}

// TestCHN_PerUserIsolation 验证设计第 2 条 — same channel, different
// users → independent rows; PK (user_id, channel_id) 复合 enforces.
func TestCHN_PerUserIsolation(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	memberToken := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")

	// Create a public channel both users join.
	chResp, chBody := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels", ownerToken, map[string]any{
		"name": "chn32-shared", "visibility": "public",
	})
	if chResp.StatusCode != http.StatusOK && chResp.StatusCode != http.StatusCreated {
		t.Fatalf("create channel: %d %v", chResp.StatusCode, chBody)
	}
	chID := chBody["channel"].(map[string]any)["id"].(string)

	resp, _ := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+chID+"/join", memberToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("member join: %d", resp.StatusCode)
	}

	// Owner sets position=1.0; member sets position=42.0 — independent.
	if resp, body := testutil.JSON(t, "PUT", ts.URL+"/api/v1/me/layout", ownerToken, map[string]any{
		"layout": []map[string]any{{"channel_id": chID, "collapsed": 0, "position": 1.0}},
	}); resp.StatusCode != http.StatusOK {
		t.Fatalf("owner PUT: %d %v", resp.StatusCode, body)
	}
	if resp, body := testutil.JSON(t, "PUT", ts.URL+"/api/v1/me/layout", memberToken, map[string]any{
		"layout": []map[string]any{{"channel_id": chID, "collapsed": 1, "position": 42.0}},
	}); resp.StatusCode != http.StatusOK {
		t.Fatalf("member PUT: %d %v", resp.StatusCode, body)
	}

	// Owner sees position=1.0; member sees position=42.0 collapsed=1.
	_, ownerBody := testutil.JSON(t, "GET", ts.URL+"/api/v1/me/layout", ownerToken, nil)
	ownerLayout := ownerBody["layout"].([]any)
	if len(ownerLayout) != 1 || ownerLayout[0].(map[string]any)["position"].(float64) != 1.0 {
		t.Errorf("owner layout mismatch: %v", ownerLayout)
	}
	_, memberBody := testutil.JSON(t, "GET", ts.URL+"/api/v1/me/layout", memberToken, nil)
	memberLayout := memberBody["layout"].([]any)
	if len(memberLayout) != 1 || memberLayout[0].(map[string]any)["position"].(float64) != 42.0 {
		t.Errorf("member layout mismatch: %v", memberLayout)
	}
	if memberLayout[0].(map[string]any)["collapsed"].(float64) != 1 {
		t.Errorf("member collapsed mismatch: %v", memberLayout[0])
	}
}

// TestCHN_AdminAPINotMounted ensures admin-api server doesn't expose
// /me/layout (设计第 5 条 ADM-0 §1.3 红线 — admin 不读业务数据).
func TestCHN_AdminAPINotMounted(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	// /me/layout is mounted on /api/v1/* user-rail mux. /admin-api/* is
	// a separate mux via admin handler — verify hitting it returns 404 +
	// admin auth not granted any layout-data path.
	resp, _ := testutil.JSON(t, "GET", ts.URL+"/admin-api/v1/users/owner/layout", "", nil)
	if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusUnauthorized && resp.StatusCode != http.StatusMethodNotAllowed {
		// If the admin API ever exposes a layout path, test fails — the
		// source scan `admin.*user_channel_layout` in admin*.go should
		// also be 0 hit. 这条测试 + 源码扫描双重检查 ADM-0 红线.
		t.Errorf("admin-api should NOT expose user_channel_layout (got %d)", resp.StatusCode)
	}
}
