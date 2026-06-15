package ws_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"borgee-server/internal/store"
	"borgee-server/internal/testutil"
	"borgee-server/internal/ws"

	"github.com/coder/websocket"
)

func TestPluginWSConnect(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	// Create an agent with API key
	resp, data := testutil.JSON(t, "POST", ts.URL+"/api/v1/agents", adminToken, map[string]any{
		"display_name": "PluginTestBot",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("agent creation failed: %d %v", resp.StatusCode, data)
	}
	agent := data["agent"].(map[string]any)
	apiKey := agent["api_key"].(string)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/plugin"
	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader: http.Header{
			"Authorization": []string{"Bearer " + apiKey},
		},
	})
	if err != nil {
		t.Fatalf("plugin ws dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Send ping
	pingMsg, _ := json.Marshal(map[string]string{"type": "ping"})
	conn.Write(ctx, websocket.MessageText, pingMsg)

	// Read pong
	_, pongData, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read pong: %v", err)
	}
	var pong map[string]any
	json.Unmarshal(pongData, &pong)
	if pong["type"] != "pong" {
		t.Fatalf("expected pong, got %v", pong["type"])
	}

	// Send api_request to list channels
	apiReq, _ := json.Marshal(map[string]any{
		"type": "api_request",
		"id":   "req-1",
		"data": map[string]any{
			"method": "GET",
			"path":   "/api/v1/channels",
		},
	})
	conn.Write(ctx, websocket.MessageText, apiReq)

	// Read api_response
	for i := 0; i < 5; i++ {
		_, respData, err := conn.Read(ctx)
		if err != nil {
			t.Fatalf("read api_response: %v", err)
		}
		var apiResp map[string]any
		json.Unmarshal(respData, &apiResp)
		if apiResp["type"] == "api_response" && apiResp["id"] == "req-1" {
			d := apiResp["data"].(map[string]any)
			status := d["status"].(float64)
			if status != 200 {
				t.Fatalf("expected status 200, got %v", status)
			}
			return
		}
	}
	t.Fatal("did not receive api_response")
}

func TestPluginWSUnauthorized(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/plugin"
	_, _, err := websocket.Dial(ctx, wsURL, nil)
	if err == nil {
		t.Fatal("expected error for unauthorized plugin WS")
	}
}

func TestRemoteWSUnauthorized(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/remote"
	_, _, err := websocket.Dial(ctx, wsURL, nil)
	if err == nil {
		t.Fatal("expected error for unauthorized remote WS")
	}
}

func TestRemoteWSConnect(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)

	users, _ := s.ListUsers()
	var adminID string
	for _, u := range users {
		if u.Email != nil && *u.Email == "owner@test.com" {
			adminID = u.ID
			break
		}
	}

	node, err := s.CreateRemoteNode(adminID, "test-remote-machine")
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/remote"
	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader: http.Header{
			"Authorization": []string{"Bearer " + node.ConnectionToken},
		},
	})
	if err != nil {
		t.Fatalf("remote ws dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	pingMsg, _ := json.Marshal(map[string]string{"type": "ping"})
	conn.Write(ctx, websocket.MessageText, pingMsg)

	_, pongData, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("read pong: %v", err)
	}
	var pong map[string]any
	json.Unmarshal(pongData, &pong)
	if pong["type"] != "pong" {
		t.Fatalf("expected pong, got %v", pong["type"])
	}
}

func TestWSTypingAndUnsubscribe(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	token := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	_, chData := testutil.JSON(t, "GET", ts.URL+"/api/v1/channels", token, nil)
	channels := chData["channels"].([]any)
	var generalID string
	for _, c := range channels {
		cm := c.(map[string]any)
		if cm["name"] == "general" {
			generalID = cm["id"].(string)
			break
		}
	}

	conn := dialWS(t, ts.URL, token)
	defer conn.Close(websocket.StatusNormalClosure, "")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Subscribe
	writeMsg(t, ctx, conn, map[string]string{"type": "subscribe", "channel_id": generalID})
	drainUntil(t, ctx, conn, "subscribed")

	// Type
	writeMsg(t, ctx, conn, map[string]string{"type": "typing", "channel_id": generalID})

	// Unsubscribe
	writeMsg(t, ctx, conn, map[string]string{"type": "unsubscribe", "channel_id": generalID})
	msg := drainUntil(t, ctx, conn, "unsubscribed")
	if msg["channel_id"] != generalID {
		t.Fatalf("expected channel_id %s", generalID)
	}
}

func TestWSPingPong(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	token := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	conn := dialWS(t, ts.URL, token)
	defer conn.Close(websocket.StatusNormalClosure, "")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	writeMsg(t, ctx, conn, map[string]string{"type": "ping"})
	msg := drainUntil(t, ctx, conn, "pong")
	if msg["type"] != "pong" {
		t.Fatal("expected pong")
	}
}

func TestWSSubscribeNonexistent(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	token := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	conn := dialWS(t, ts.URL, token)
	defer conn.Close(websocket.StatusNormalClosure, "")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	writeMsg(t, ctx, conn, map[string]string{"type": "subscribe", "channel_id": "nonexistent"})
	msg := drainUntil(t, ctx, conn, "error")
	if msg["code"] != "NOT_FOUND" {
		t.Fatalf("expected NOT_FOUND, got %v", msg["code"])
	}
}

func TestWSAuthMethods(t *testing.T) {
	t.Parallel()
	ts, s, _ := testutil.NewTestServer(t)

	users, _ := s.ListUsers()
	var adminID string
	for _, u := range users {
		if u.Email != nil && *u.Email == "owner@test.com" {
			adminID = u.ID
			break
		}
	}

	apiKey, _ := store.GenerateAPIKey()
	s.SetAPIKey(adminID, apiKey)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// WS-auth-unify: /ws no longer accepts the `?token=` apiKey query form.
	// Header Bearer is the canonical process-client auth path.
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader: http.Header{"Authorization": []string{"Bearer " + apiKey}},
	})
	if err != nil {
		t.Fatalf("ws dial with bearer header: %v", err)
	}
	conn.Close(websocket.StatusNormalClosure, "")
}

func TestHubBroadcasting(t *testing.T) {
	t.Parallel()
	hub, s := setupTestHub(t)

	user := &store.User{ID: "hub-test", DisplayName: "HubTest", Role: "member"}
	s.CreateUser(user)

	hub.BroadcastToAll(map[string]string{"type": "test"})
	hub.BroadcastToUser("hub-test", map[string]string{"type": "test"})

	count := hub.ClientCount()
	if count != 0 {
		t.Fatalf("expected 0 clients, got %d", count)
	}

	ids := hub.GetOnlineUserIDs()
	if len(ids) != 0 {
		t.Fatalf("expected 0 online users, got %d", len(ids))
	}
}

func TestCommandStoreGetByName(t *testing.T) {
	t.Parallel()
	cs := ws.NewCommandStore()
	cs.Register("conn-1", "agent-1", "Bot", []ws.AgentCommand{
		{Name: "test-cmd", Description: "test"},
	})

	cmds := cs.GetByName("test-cmd")
	if len(cmds) != 1 {
		t.Fatalf("expected 1 command, got %d", len(cmds))
	}

	cmds2 := cs.GetByName("nonexistent")
	if len(cmds2) != 0 {
		t.Fatalf("expected 0 commands, got %d", len(cmds2))
	}
}
