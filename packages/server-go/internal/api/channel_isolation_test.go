package api_test

import (
	"net/http"
	"testing"

	"borgee-server/internal/testutil"
)

func TestP0PrivateChannelIsolation(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAs(t, ts.URL, "admin@test.com", "password123")
	memberToken := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")

	ch := testutil.CreateChannel(t, ts.URL, adminToken, "Private Isolation", "private")
	channelID := stringField(t, ch, "id")
	testutil.PostMessage(t, ts.URL, adminToken, channelID, "private content")

	resp, data := testutil.JSON(t, http.MethodGet, ts.URL+"/api/v1/channels", memberToken, nil)
	requireStatus(t, resp, http.StatusOK, data)
	if containsObjectWithID(data["channels"].([]any), channelID) {
		t.Fatalf("non-member should not see private channel in list: %v", data)
	}

	resp, data = testutil.JSON(t, http.MethodGet, ts.URL+"/api/v1/channels/"+channelID, memberToken, nil)
	// AP-1 设计 ① (REG-CHN1-007): 非 member GET → 403 (不再 404 隐藏存在性).
	requireStatus(t, resp, http.StatusForbidden, data)

	resp, data = testutil.JSON(t, http.MethodGet, ts.URL+"/api/v1/channels/"+channelID+"/messages", memberToken, nil)
	requireStatus(t, resp, http.StatusNotFound, data)

	resp, data = testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/channels/"+channelID+"/messages", memberToken, map[string]string{"content": "intrusion"})
	requireStatus(t, resp, http.StatusNotFound, data)

	conn := testutil.DialWS(t, ts.URL, "/ws", memberToken)
	testutil.WSWriteJSON(t, conn, map[string]string{"type": "subscribe", "channel_id": channelID})
	msg := testutil.WSReadUntil(t, conn, "error")
	if msg["code"] != "NOT_MEMBER" {
		t.Fatalf("expected forbidden websocket subscribe, got %v", msg)
	}
}
