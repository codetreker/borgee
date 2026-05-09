package ws_test

import (
	"net/http"
	"testing"

	"borgee-server/internal/testutil"
)

func TestP1WebSocketPermissionChanges(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAs(t, ts.URL, "admin@test.com", "password123")
	memberToken := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")
	memberID := getWSUserIDByName(t, ts.URL, adminToken, "Member")

	resp, data := testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/channels", adminToken, map[string]any{
		"name":       "Permission Room",
		"visibility": "private",
		"member_ids": []string{memberID},
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create private channel status %d: %v", resp.StatusCode, data)
	}
	channelID := data["channel"].(map[string]any)["id"].(string)

	memberConn := testutil.DialWS(t, ts.URL, "/ws", memberToken)
	testutil.WSWriteJSON(t, memberConn, map[string]string{"type": "subscribe", "channel_id": channelID})
	testutil.WSReadUntil(t, memberConn, "subscribed")

	resp, data = testutil.JSON(t, http.MethodDelete, ts.URL+"/api/v1/channels/"+channelID+"/members/"+memberID, adminToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("remove member status %d: %v", resp.StatusCode, data)
	}
	removed := testutil.WSReadUntil(t, memberConn, "channel_removed")
	if removed["data"].(map[string]any)["channel_id"] != channelID {
		t.Fatalf("expected channel_removed for private room, got %v", removed)
	}

	testutil.WSWriteJSON(t, memberConn, map[string]string{
		"type":              "send_message",
		"channel_id":        channelID,
		"content":           "should be rejected",
		"client_message_id": "perm-1",
	})
	nack := testutil.WSReadUntil(t, memberConn, "message_nack")
	if nack["code"] != "NOT_FOUND" {
		t.Fatalf("expected private channel send to be rejected after removal, got %v", nack)
	}

	resp, data = testutil.JSON(t, http.MethodGet, ts.URL+"/api/v1/channels/"+channelID, memberToken, nil)
	// AP-1 设计 ① (REG-CHN1-007): removed member → 403 (不再 404).
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("removed member should get 403, status %d: %v", resp.StatusCode, data)
	}
}

func getWSUserIDByName(t *testing.T, serverURL, token, displayName string) string {
	t.Helper()
	return testutil.GetUserIDByName(t, serverURL, token, displayName)
}
