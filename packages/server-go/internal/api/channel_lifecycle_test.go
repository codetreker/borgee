package api_test

import (
	"fmt"
	"net/http"
	"testing"

	"borgee-server/internal/testutil"
)

func requireStatus(t *testing.T, resp *http.Response, want int, body map[string]any) {
	t.Helper()
	if resp.StatusCode != want {
		t.Fatalf("expected status %d, got %d, body %v", want, resp.StatusCode, body)
	}
}

func stringField(t *testing.T, m map[string]any, key string) string {
	t.Helper()
	v, ok := m[key].(string)
	if !ok || v == "" {
		t.Fatalf("expected non-empty string field %q in %v", key, m)
	}
	return v
}

func containsObjectWithID(items []any, id string) bool {
	for _, raw := range items {
		m, ok := raw.(map[string]any)
		if ok && m["id"] == id {
			return true
		}
	}
	return false
}

func TestP0ChannelLifecycle(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAs(t, ts.URL, "admin@test.com", "password123")
	memberToken := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")

	ch := testutil.CreateChannel(t, ts.URL, memberToken, "P0 Lifecycle", "public")
	channelID := stringField(t, ch, "id")
	if ch["name"] != "p0-lifecycle" {
		t.Fatalf("expected slug p0-lifecycle, got %v", ch["name"])
	}

	resp, data := testutil.JSON(t, http.MethodGet, ts.URL+"/api/v1/channels/"+channelID, memberToken, nil)
	requireStatus(t, resp, http.StatusOK, data)
	// CHN-1.2 设计 ②: creator-only default member. After POST /channels the
	// member set is exactly {creator}; org peers see the channel via GET (org-
	// scoped public discovery) but are NOT auto-joined.
	if members := data["members"].([]any); len(members) != 1 {
		t.Fatalf("creator-only default: expected 1 member, got %d", len(members))
	}

	resp, data = testutil.JSON(t, http.MethodPut, ts.URL+"/api/v1/channels/"+channelID+"/topic", memberToken, map[string]string{"topic": "release room"})
	requireStatus(t, resp, http.StatusOK, data)
	updated := data["channel"].(map[string]any)
	if updated["topic"] != "release room" {
		t.Fatalf("topic was not updated: %v", updated)
	}

	msg := testutil.PostMessage(t, ts.URL, memberToken, channelID, "lifecycle message")
	if stringField(t, msg, "channel_id") != channelID {
		t.Fatalf("message channel mismatch: %v", msg)
	}

	resp, data = testutil.JSON(t, http.MethodPut, ts.URL+"/api/v1/channels/"+channelID+"/read", memberToken, nil)
	requireStatus(t, resp, http.StatusOK, data)

	resp, data = testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/channels/"+channelID+"/join", adminToken, nil)
	requireStatus(t, resp, http.StatusOK, data)

	resp, data = testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/channels/"+channelID+"/leave", adminToken, nil)
	requireStatus(t, resp, http.StatusOK, data)

	resp, data = testutil.JSON(t, http.MethodPost, ts.URL+"/api/v1/channels/"+channelID+"/join", adminToken, nil)
	requireStatus(t, resp, http.StatusOK, data)

	resp, data = testutil.JSON(t, http.MethodDelete, fmt.Sprintf("%s/api/v1/channels/%s", ts.URL, channelID), memberToken, nil)
	requireStatus(t, resp, http.StatusOK, data)

	resp, data = testutil.JSON(t, http.MethodGet, ts.URL+"/api/v1/channels/"+channelID, memberToken, nil)
	requireStatus(t, resp, http.StatusNotFound, data)
}
