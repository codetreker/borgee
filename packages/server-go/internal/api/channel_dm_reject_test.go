// Package api_test — chn_2_1_dm_reject_test.go: CHN-2.1 server-side DM
// reject paths (acceptance §1.2/§1.3/§2.3, content-lock ④).
//
// Stance pins exercised:
//   - 设计 ② DM 永远 2 人 — POST /channels/:id/members on type='dm' → 400
//     "Cannot add members to DM channels" (既有 channels.go:522 实施,
//     本测试 lock-pin 防漂; 同源 Cannot join/leave/delete DM 同模式).
//   - 设计 ③ DM 没 workspace — POST /channels/:id/artifacts on type='dm'
//     → 403 with code "dm.workspace_not_supported" (蓝图 §1.2 字面禁;
//     本 PR 加守门, artifacts.go handleCreate 增 ch.Type=='dm' gate).
//
// 反约束: artifact create on DM channel 必须 status==403 + code=
// "dm.workspace_not_supported" (字面 byte-identical 跟错误码 enum 锁).
package api_test

import (
	"net/http"
	"testing"

	"borgee-server/internal/store"
	"borgee-server/internal/testutil"
)

// helperFindUserID returns the user ID for a given email from the seeded
// test fixtures (mirrors dm_test.go local pattern).
func helperFindUserID(t *testing.T, s *store.Store, email string) string {
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

func TestCHN_DMArtifactReject(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	memberID := helperFindUserID(t, s, "member@test.com")

	// Create a DM channel between owner and member.
	resp, data := testutil.JSON(t, "POST", ts.URL+"/api/v1/dm/"+memberID, ownerToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("DM create: %d %v", resp.StatusCode, data)
	}
	ch := data["channel"].(map[string]any)
	dmID := ch["id"].(string)

	t.Run("POST artifact on DM → 403 dm.workspace_not_supported", func(t *testing.T) {
		// 设计 ③ DM 无 workspace — 蓝图 §1.2 字面禁.
		resp, body := testutil.JSON(t, "POST",
			ts.URL+"/api/v1/channels/"+dmID+"/artifacts", ownerToken,
			map[string]any{"title": "t", "body": "b"})
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 403, got %d: %v", resp.StatusCode, body)
		}
		if code, _ := body["code"].(string); code != "dm.workspace_not_supported" {
			t.Fatalf("expected code dm.workspace_not_supported, got %v", body["code"])
		}
	})

	t.Run("POST artifact on DM with kind=code → 403 (gate runs before kind validate)", func(t *testing.T) {
		resp, body := testutil.JSON(t, "POST",
			ts.URL+"/api/v1/channels/"+dmID+"/artifacts", ownerToken,
			map[string]any{
				"title": "t", "body": "x", "type": "code",
				"metadata": map[string]any{"language": "go"},
			})
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 403, got %d: %v", resp.StatusCode, body)
		}
		if code, _ := body["code"].(string); code != "dm.workspace_not_supported" {
			t.Fatalf("expected code dm.workspace_not_supported, got %v", body["code"])
		}
	})
}

// TestCHN_DMAddMemberReject — re-pins the existing 设计 ② 反约束 (DM
// 永远 2 人, channels.go:522 既有 400). Test exists today as part of
// channel tests — duplicate here as CHN-2.1 stance lock so 重构 误删
// 守门时 grep 锚明确归属 CHN-2.1.
func TestCHN_DMAddMemberReject(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	memberID := helperFindUserID(t, s, "member@test.com")

	// Create DM.
	resp, data := testutil.JSON(t, "POST", ts.URL+"/api/v1/dm/"+memberID, ownerToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("DM create: %d", resp.StatusCode)
	}
	dmID := data["channel"].(map[string]any)["id"].(string)

	// Find a 3rd user (admin@test.com).
	thirdID := helperFindUserID(t, s, "admin@test.com")

	// 设计 ② DM 不可加人 — POST /channels/:id/members on dm → 400.
	resp2, body := testutil.JSON(t, "POST",
		ts.URL+"/api/v1/channels/"+dmID+"/members", ownerToken,
		map[string]any{"user_id": thirdID})
	if resp2.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 add-member-to-DM, got %d: %v", resp2.StatusCode, body)
	}
}
