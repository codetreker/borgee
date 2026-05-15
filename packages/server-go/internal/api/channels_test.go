package api_test

import (
	"net/http"
	"testing"

	"borgee-server/internal/testutil"
)

func TestChannelCRUD(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	memberToken := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")

	var channelID string

	t.Run("CreateChannel", func(t *testing.T) {
		ch := testutil.CreateChannel(t, ts.URL, memberToken, "test-channel", "public")
		channelID = ch["id"].(string)
		if channelID == "" {
			t.Fatal("expected channel id")
		}
	})

	t.Run("CreateChannelDuplicate", func(t *testing.T) {
		resp, _ := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels", memberToken, map[string]string{
			"name": "test-channel", "visibility": "public",
		})
		if resp.StatusCode != http.StatusConflict {
			t.Fatalf("expected 409, got %d", resp.StatusCode)
		}
	})

	t.Run("ListChannels", func(t *testing.T) {
		_, data := testutil.JSON(t, "GET", ts.URL+"/api/v1/channels", memberToken, nil)
		channels, ok := data["channels"].([]any)
		if !ok || len(channels) == 0 {
			t.Fatal("expected channels list")
		}
		found := false
		for _, c := range channels {
			cm := c.(map[string]any)
			if cm["name"] == "test-channel" {
				found = true
				break
			}
		}
		if !found {
			t.Fatal("test-channel not in list")
		}
	})

	t.Run("GetChannel", func(t *testing.T) {
		_, data := testutil.JSON(t, "GET", ts.URL+"/api/v1/channels/"+channelID, adminToken, nil)
		ch, ok := data["channel"].(map[string]any)
		if !ok {
			t.Fatal("expected channel object")
		}
		if ch["name"] != "test-channel" {
			t.Fatalf("expected test-channel, got %v", ch["name"])
		}
		members, ok := data["members"].([]any)
		if !ok || len(members) == 0 {
			t.Fatal("expected members list")
		}
	})

	t.Run("UpdateChannel", func(t *testing.T) {
		topic := "new topic"
		_, data := testutil.JSON(t, "PUT", ts.URL+"/api/v1/channels/"+channelID, memberToken, map[string]any{
			"topic": topic,
		})
		ch := data["channel"].(map[string]any)
		if ch["topic"] != topic {
			t.Fatalf("expected topic %q, got %v", topic, ch["topic"])
		}
	})

	t.Run("DeleteChannel", func(t *testing.T) {
		resp, _ := testutil.JSON(t, "DELETE", ts.URL+"/api/v1/channels/"+channelID, memberToken, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
	})
}

func TestChannelMembers(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	memberToken := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")

	pubCh := testutil.CreateChannel(t, ts.URL, adminToken, "pub-join-test", "public")
	pubID := pubCh["id"].(string)

	privCh := testutil.CreateChannel(t, ts.URL, adminToken, "priv-test", "private")
	privID := privCh["id"].(string)
	_ = s

	t.Run("JoinPublicChannel", func(t *testing.T) {
		resp, _ := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+pubID+"/join", memberToken, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("LeaveChannel", func(t *testing.T) {
		resp, _ := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+pubID+"/leave", memberToken, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
	})

	t.Run("CannotJoinPrivateChannel", func(t *testing.T) {
		resp, _ := testutil.JSON(t, "POST", ts.URL+"/api/v1/channels/"+privID+"/join", memberToken, nil)
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", resp.StatusCode)
		}
	})
}
