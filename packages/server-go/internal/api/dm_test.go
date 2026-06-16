package api_test

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"borgee-server/internal/testutil"
	"github.com/coder/websocket"
)

func TestDMCreate(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	memberToken := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")

	users, _ := s.ListUsers()
	var adminID, memberID string
	for _, u := range users {
		if u.Email != nil && *u.Email == "owner@test.com" {
			adminID = u.ID
		} else if u.Email != nil && *u.Email == "member@test.com" {
			memberID = u.ID
		}
	}

	t.Run("CreateDM", func(t *testing.T) {
		resp, data := testutil.JSON(t, "POST", ts.URL+"/api/v1/dm/"+memberID, adminToken, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d: %v", resp.StatusCode, data)
		}
		ch := data["channel"].(map[string]any)
		if ch["type"] != "dm" {
			t.Fatalf("expected dm type, got %v", ch["type"])
		}
		peer := data["peer"].(map[string]any)
		if peer["id"] != memberID {
			t.Fatalf("expected peer %s, got %v", memberID, peer["id"])
		}
	})

	t.Run("Idempotent", func(t *testing.T) {
		resp1, data1 := testutil.JSON(t, "POST", ts.URL+"/api/v1/dm/"+memberID, adminToken, nil)
		resp2, data2 := testutil.JSON(t, "POST", ts.URL+"/api/v1/dm/"+adminID, memberToken, nil)
		if resp1.StatusCode != http.StatusOK || resp2.StatusCode != http.StatusOK {
			t.Fatal("both DM creates should succeed")
		}
		ch1 := data1["channel"].(map[string]any)
		ch2 := data2["channel"].(map[string]any)
		if ch1["id"] != ch2["id"] {
			t.Fatalf("expected same channel, got %v and %v", ch1["id"], ch2["id"])
		}
	})

	t.Run("CannotDMSelf", func(t *testing.T) {
		resp, _ := testutil.JSON(t, "POST", ts.URL+"/api/v1/dm/"+adminID, adminToken, nil)
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp.StatusCode)
		}
	})

	t.Run("NonexistentUser", func(t *testing.T) {
		resp, _ := testutil.JSON(t, "POST", ts.URL+"/api/v1/dm/nonexistent", adminToken, nil)
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", resp.StatusCode)
		}
	})

	t.Run("ListDMs", func(t *testing.T) {
		resp, data := testutil.JSON(t, "GET", ts.URL+"/api/v1/dm", adminToken, nil)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		channels := data["channels"].([]any)
		if len(channels) == 0 {
			t.Fatal("expected at least 1 DM channel")
		}
	})
}

func TestDMListAgentPeerIncludesOnlineState(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	ownerToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	resp, data := testutil.JSON(t, "POST", ts.URL+"/api/v1/agents", ownerToken, map[string]any{
		"display_name": "Sidebar Bot",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create agent: %d (%v)", resp.StatusCode, data)
	}
	agent := data["agent"].(map[string]any)
	agentID := agent["id"].(string)
	agentKey := agent["api_key"].(string)

	resp, data = testutil.JSON(t, "POST", ts.URL+"/api/v1/dm/"+agentID, ownerToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create agent dm: %d (%v)", resp.StatusCode, data)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/plugin"
	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader: http.Header{"Authorization": []string{"Bearer " + agentKey}},
	})
	if err != nil {
		t.Fatalf("dial plugin ws: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	deadline := time.Now().Add(2 * time.Second)
	for {
		resp, data = testutil.JSON(t, "GET", ts.URL+"/api/v1/agents/"+agentID+"/status", ownerToken, nil)
		if resp.StatusCode == http.StatusOK && data["state"] == "online" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("agent status did not become online: %d (%v)", resp.StatusCode, data)
		}
		time.Sleep(5 * time.Millisecond)
	}

	resp, data = testutil.JSON(t, "GET", ts.URL+"/api/v1/dm", ownerToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list dms: %d (%v)", resp.StatusCode, data)
	}
	channels := data["channels"].([]any)
	if len(channels) == 0 {
		t.Fatal("expected one agent DM channel")
	}
	peer := channels[0].(map[string]any)["peer"].(map[string]any)
	if peer["id"] != agentID {
		t.Fatalf("expected agent peer %s, got %v", agentID, peer)
	}
	if peer["state"] != "online" {
		t.Fatalf("agent DM peer state = %v, want online", peer["state"])
	}
	if _, has := peer["reason"]; has {
		t.Fatalf("online agent DM peer must not include reason: %v", peer)
	}
}
