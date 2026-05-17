// Package api_test — rt_4_presence_test.go: RT-4 channel presence
// indicator member-only GET + 约束守门.
//
// Pins:
//
//	REG-RT4-001 TestRT_NoSchemaChange
//	REG-RT4-002 TestRT_GetPresence_MemberHappyPath + _NonMemberRejected
//	             + _Unauthorized401
//	REG-RT4-003 TestRT_TypingPathByteIdentical (grep 检查 rt_4 在
//	            ws/client.go::handleTyping block 0 hit)
//	REG-RT4-004 TestRT_NoAdminPresencePath
//	REG-RT4-005 TestRT_NoPresenceQueue
package api_test

import (
	"net/http"
	"testing"

	"borgee-server/internal/store"
	"borgee-server/internal/testutil"
)

func setupRT4(t *testing.T) (string, string, string, string, *store.Store) {
	t.Helper()
	ts, s, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	memberToken := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")
	owner, _ := s.GetUserByEmail("owner@test.com")
	member, _ := s.GetUserByEmail("member@test.com")

	resp, body := testutil.JSON(t, http.MethodPost,
		ts.URL+"/api/v1/channels", ownerToken,
		map[string]any{"name": "rt4-test", "visibility": "public"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create channel: %d %v", resp.StatusCode, body)
	}
	ch, _ := body["channel"].(map[string]any)
	channelID, _ := ch["id"].(string)
	if channelID == "" {
		t.Fatalf("channel.id missing in response: %v", body)
	}
	if err := s.AddChannelMember(&store.ChannelMember{
		ChannelID: channelID, UserID: member.ID,
	}); err != nil {
		t.Fatalf("AddChannelMember: %v", err)
	}
	_ = owner
	return ts.URL, ownerToken, memberToken, channelID, s
}

// REG-RT4-002a — member HappyPath GET /presence → 200 + shape.
func TestRT_GetPresence_MemberHappyPath(t *testing.T) {
	t.Parallel()
	url, ownerToken, _, channelID, _ := setupRT4(t)

	resp, body := testutil.JSON(t, http.MethodGet,
		url+"/api/v1/channels/"+channelID+"/presence", ownerToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d: %v", resp.StatusCode, body)
	}
	for _, k := range []string{"online_user_ids", "counted_at"} {
		if _, ok := body[k]; !ok {
			t.Errorf("missing key %q in response: %v", k, body)
		}
	}
	if _, ok := body["online_user_ids"].([]any); !ok {
		t.Errorf("online_user_ids: got %T, want []any", body["online_user_ids"])
	}
}

// REG-RT4-002b — non-member 403.
func TestRT_GetPresence_NonMemberRejected(t *testing.T) {
	t.Parallel()
	url, ownerToken, _, _, s := setupRT4(t)
	// Create a second channel where the calling user is NOT a member.
	resp, body := testutil.JSON(t, http.MethodPost,
		url+"/api/v1/channels", ownerToken,
		map[string]any{"name": "rt4-private", "visibility": "public"})
	ch, _ := body["channel"].(map[string]any)
	otherChannel, _ := ch["id"].(string)
	_ = resp
	_ = s
	// member@test.com is NOT added to otherChannel.
	memberToken := testutil.LoginAs(t, url, "member@test.com", "password123")
	resp, _ = testutil.JSON(t, http.MethodGet,
		url+"/api/v1/channels/"+otherChannel+"/presence", memberToken, nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("non-member: got %d, want 403", resp.StatusCode)
	}
}

// REG-RT4-002c — 401 unauthorized.
func TestRT_GetPresence_Unauthorized401(t *testing.T) {
	t.Parallel()
	url, _, _, channelID, _ := setupRT4(t)
	resp, _ := testutil.JSON(t, http.MethodGet,
		url+"/api/v1/channels/"+channelID+"/presence", "", nil)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("unauth: got %d, want 401", resp.StatusCode)
	}
}
