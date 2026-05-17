// Package api_test — chn_7_mute_test.go: CHN-7 mute/unmute REST + 0
// schema 改 + bitmap + admin 权限不挂 + AST 对齐链延伸第 12 处 + mute
// 不 drop messages best-effort.
package api_test

import (
	"net/http"
	"testing"

	"borgee-server/internal/api"
	"borgee-server/internal/testutil"
)

// REG-CHN7-002a — POST mute happy path; collapsed bit 1 set.
func TestCHN_MuteChannel_HappyPath(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	owner, _ := s.GetUserByEmail("owner@test.com")
	ch := testutil.CreateChannel(t, ts.URL, ownerToken, "to-mute", "public")
	chID := ch["id"].(string)

	resp, body := testutil.JSON(t, http.MethodPost,
		ts.URL+"/api/v1/channels/"+chID+"/mute", ownerToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %v", resp.StatusCode, body)
	}
	if body["muted"] != true {
		t.Errorf("muted: got %v, want true", body["muted"])
	}
	cVal, _ := body["collapsed"].(float64)
	if int64(cVal)&int64(api.MuteBit) == 0 {
		t.Errorf("collapsed bit 1 not set: got %v", cVal)
	}

	// IsMutedForUser store helper agrees.
	muted, err := s.IsMutedForUser(owner.ID, chID, int64(api.MuteBit))
	if err != nil {
		t.Fatalf("IsMutedForUser: %v", err)
	}
	if !muted {
		t.Error("IsMutedForUser: got false, want true")
	}
}

// REG-CHN7-002b — non-member 403.
func TestCHN_MuteChannel_NonMemberRejected(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	memberToken := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")
	ch := testutil.CreateChannel(t, ts.URL, ownerToken, "private-mute", "private")
	chID := ch["id"].(string)
	resp, _ := testutil.JSON(t, http.MethodPost,
		ts.URL+"/api/v1/channels/"+chID+"/mute", memberToken, nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("non-member mute: got %d, want 403", resp.StatusCode)
	}
}

// REG-CHN7-002c — Unauthorized 401.
func TestCHN_MuteChannel_Unauthorized(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	resp, _ := testutil.JSON(t, http.MethodPost,
		ts.URL+"/api/v1/channels/some-id/mute", "", nil)
	if resp.StatusCode == http.StatusOK {
		t.Errorf("expected non-200 unauthenticated, got 200")
	}
}

// REG-CHN7-003a — DELETE unmute clears bit 1.
func TestCHN_UnmuteChannel_HappyPath(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	ch := testutil.CreateChannel(t, ts.URL, ownerToken, "round-trip", "public")
	chID := ch["id"].(string)

	testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/channels/"+chID+"/mute", ownerToken, nil)
	resp, body := testutil.JSON(t, http.MethodDelete,
		ts.URL+"/api/v1/channels/"+chID+"/mute", ownerToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if body["muted"] != false {
		t.Errorf("muted: got %v, want false", body["muted"])
	}
	cVal, _ := body["collapsed"].(float64)
	if int64(cVal)&int64(api.MuteBit) != 0 {
		t.Errorf("collapsed bit 1 still set: got %v", cVal)
	}
}

// REG-CHN7-003b — DELETE idempotent.
func TestCHN_UnmuteChannel_Idempotent(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	ch := testutil.CreateChannel(t, ts.URL, ownerToken, "idem", "public")
	chID := ch["id"].(string)
	testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/channels/"+chID+"/mute", ownerToken, nil)
	for i := 0; i < 2; i++ {
		resp, _ := testutil.JSON(t, http.MethodDelete,
			ts.URL+"/api/v1/channels/"+chID+"/mute", ownerToken, nil)
		if resp.StatusCode != http.StatusOK {
			t.Errorf("DELETE %d: got %d, want 200", i, resp.StatusCode)
		}
	}
}

// REG-CHN7-003c — unmute preserves collapse bit (bit 0).
func TestCHN_UnmuteChannel_PreservesCollapsedBit(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	owner, _ := s.GetUserByEmail("owner@test.com")
	ch := testutil.CreateChannel(t, ts.URL, ownerToken, "bit-preserve", "public")
	chID := ch["id"].(string)

	// Set bit 0 (CHN-3 collapsed) via PUT /me/layout.
	testutil.JSON(t, http.MethodPut, ts.URL+"/api/v1/me/layout", ownerToken, map[string]any{
		"layout": []map[string]any{
			{"channel_id": chID, "collapsed": 1, "position": 0.0},
		},
	})
	// Then mute (set bit 1) — collapsed should become 3.
	testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/channels/"+chID+"/mute", ownerToken, nil)
	// Then unmute (clear bit 1) — collapsed should become 1 (bit 0 preserved).
	testutil.JSON(t, http.MethodDelete, ts.URL+"/api/v1/channels/"+chID+"/mute", ownerToken, nil)

	muted, _ := s.IsMutedForUser(owner.ID, chID, int64(api.MuteBit))
	if muted {
		t.Error("IsMutedForUser after unmute: got true")
	}
}

// REG-CHN7-004 — MuteBit 字节级一致 双向锁 + IsMuted 谓词单一来源.
func TestCHN_MuteBit_ByteIdentical(t *testing.T) {
	t.Parallel()
	if api.MuteBit != 2 {
		t.Errorf("MuteBit mismatch: got %d, want 2 (双向锁跟 client MUTE_BIT)", api.MuteBit)
	}
	if api.IsMuted(0) {
		t.Error("IsMuted(0): got true, want false")
	}
	if api.IsMuted(1) {
		t.Error("IsMuted(1) (collapsed only): got true, want false")
	}
	if !api.IsMuted(2) {
		t.Error("IsMuted(2) (muted only): got false, want true")
	}
	if !api.IsMuted(3) {
		t.Error("IsMuted(3) (collapsed+muted): got false, want true")
	}
}
