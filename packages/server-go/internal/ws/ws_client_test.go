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
	"github.com/google/uuid"
)

func TestWSSendMessageEdgeCases(t *testing.T) {
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

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	writeMsg(t, ctx, conn, map[string]string{"type": "subscribe", "channel_id": generalID})
	drainUntil(t, ctx, conn, "subscribed")

	t.Run("EmptyContent", func(t *testing.T) {
		writeMsg(t, ctx, conn, map[string]string{
			"type": "send_message", "channel_id": generalID, "content": "", "client_id": uuid.NewString(),
		})
		msg := drainUntil(t, ctx, conn, "message_nack")
		if msg["code"] != "INVALID_CONTENT_TYPE" {
			t.Fatalf("expected INVALID_CONTENT_TYPE, got %v", msg["code"])
		}
	})

	t.Run("NonexistentChannel", func(t *testing.T) {
		writeMsg(t, ctx, conn, map[string]string{
			"type": "send_message", "channel_id": "nonexistent", "content": "test", "client_id": uuid.NewString(),
		})
		msg := drainUntil(t, ctx, conn, "message_nack")
		if msg["code"] != "NOT_FOUND" {
			t.Fatalf("expected NOT_FOUND, got %v", msg["code"])
		}
	})

	t.Run("InvalidContentType", func(t *testing.T) {
		writeMsg(t, ctx, conn, map[string]any{
			"type": "send_message", "channel_id": generalID, "content": "test",
			"content_type": "video", "client_id": uuid.NewString(),
		})
		msg := drainUntil(t, ctx, conn, "message_nack")
		if msg["code"] != "INVALID_CONTENT_TYPE" {
			t.Fatalf("expected INVALID_CONTENT_TYPE, got %v", msg["code"])
		}
	})

	t.Run("CommandSlashPrefix", func(t *testing.T) {
		writeMsg(t, ctx, conn, map[string]any{
			"type": "send_message", "channel_id": generalID, "content": "/help",
			"client_message_id": uuid.NewString(),
		})
		msg := drainUntil(t, ctx, conn, "message_ack")
		if msg["message_id"] == nil {
			t.Fatal("expected message_id")
		}
	})

	t.Run("CommandContentTypeJSON", func(t *testing.T) {
		cmdJSON, _ := json.Marshal(map[string]any{"command": "test", "params": map[string]string{}})
		writeMsg(t, ctx, conn, map[string]any{
			"type": "send_message", "channel_id": generalID,
			"content": string(cmdJSON), "content_type": "command",
			"client_id": uuid.NewString(),
		})
		msg := drainUntil(t, ctx, conn, "message_ack")
		if msg["message_id"] == nil {
			t.Fatal("expected message_id")
		}
	})

	t.Run("CommandContentTypeInvalidJSON", func(t *testing.T) {
		writeMsg(t, ctx, conn, map[string]any{
			"type": "send_message", "channel_id": generalID,
			"content": "not json", "content_type": "command",
			"client_id": uuid.NewString(),
		})
		msg := drainUntil(t, ctx, conn, "message_nack")
		if msg["code"] != "INVALID_CONTENT_TYPE" {
			t.Fatalf("expected INVALID_CONTENT_TYPE, got %v", msg["code"])
		}
	})

	t.Run("ImageContentType", func(t *testing.T) {
		writeMsg(t, ctx, conn, map[string]any{
			"type": "send_message", "channel_id": generalID,
			"content": "image.png", "content_type": "image",
			"client_id": uuid.NewString(),
		})
		msg := drainUntil(t, ctx, conn, "message_ack")
		if msg["message_id"] == nil {
			t.Fatal("expected message_id")
		}
	})

	t.Run("WithReplyTo", func(t *testing.T) {
		writeMsg(t, ctx, conn, map[string]any{
			"type": "send_message", "channel_id": generalID,
			"content":   "reply with ref",
			"client_id": uuid.NewString(),
		})
		msg := drainUntil(t, ctx, conn, "message_ack")
		if msg["message_id"] == nil {
			t.Fatal("expected message_id")
		}
	})

	t.Run("WithMentions", func(t *testing.T) {
		writeMsg(t, ctx, conn, map[string]any{
			"type": "send_message", "channel_id": generalID,
			"content": "hello @user", "mentions": []string{"someone"},
			"client_id": uuid.NewString(),
		})
		msg := drainUntil(t, ctx, conn, "message_ack")
		if msg["message_id"] == nil {
			t.Fatal("expected message_id")
		}
	})
}

func TestWSSubscribePrivateChannel(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")
	memberToken := testutil.LoginAs(t, ts.URL, "member@test.com", "password123")

	priv := testutil.CreateChannel(t, ts.URL, adminToken, "ws-priv", "private")
	privID := priv["id"].(string)

	conn := dialWS(t, ts.URL, memberToken)
	defer conn.Close(websocket.StatusNormalClosure, "")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	writeMsg(t, ctx, conn, map[string]string{"type": "subscribe", "channel_id": privID})
	msg := drainUntil(t, ctx, conn, "error")
	if msg["code"] != "NOT_MEMBER" {
		t.Fatalf("expected NOT_MEMBER, got %v", msg["code"])
	}
}

func TestWSRegisterCommands(t *testing.T) {
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
	agent := &store.User{DisplayName: "CmdBot", Role: "agent", OwnerID: &adminID, APIKey: &apiKey}
	s.CreateUser(agent)
	s.GrantDefaultPermissions(agent.ID, "agent")
	s.AddUserToPublicChannels(agent.ID)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader: http.Header{"Authorization": []string{"Bearer " + apiKey}},
	})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	drainUntil(t, ctx, conn, "presence")

	writeMsg(t, ctx, conn, map[string]any{
		"type": "register_commands",
		"commands": []map[string]any{
			{"name": "valid-cmd", "description": "A valid command"},
			{"name": "help", "description": "Builtin conflict"},
			{"name": "INVALID NAME!", "description": "Bad name"},
			{"name": "long-desc", "description": strings.Repeat("a", 201)},
		},
	})

	msg := drainUntil(t, ctx, conn, "commands_registered")
	registered := msg["registered"].([]any)
	skipped := msg["skipped"].([]any)

	if len(registered) != 1 || registered[0] != "valid-cmd" {
		t.Fatalf("expected [valid-cmd], got %v", registered)
	}
	if len(skipped) != 3 {
		t.Fatalf("expected 3 skipped, got %v", skipped)
	}
}

func TestWSRegisterCommandsNonAgent(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	token := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	conn := dialWS(t, ts.URL, token)
	defer conn.Close(websocket.StatusNormalClosure, "")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	drainUntil(t, ctx, conn, "presence")

	writeMsg(t, ctx, conn, map[string]any{
		"type": "register_commands",
		"commands": []map[string]any{
			{"name": "test-cmd", "description": "test"},
		},
	})

	msg := drainUntil(t, ctx, conn, "error")
	if msg["code"] != "FORBIDDEN" {
		t.Fatalf("expected FORBIDDEN, got %v", msg["code"])
	}
}

func TestWSAuthenticateWSExported(t *testing.T) {
	t.Parallel()
	hub, s := setupTestHub(t)

	u := &store.User{ID: "auth-ws-test", DisplayName: "AuthWSTest", Role: "member"}
	s.CreateUser(u)

	apiKey, _ := store.GenerateAPIKey()
	s.SetAPIKey(u.ID, apiKey)

	// WS-auth-unify: the `?token=` apiKey query form is removed from /ws.
	// A query token must no longer authenticate.
	r, _ := http.NewRequest("GET", "/ws?token="+apiKey, nil)
	user := ws.AuthenticateWS(hub, r)
	if user != nil {
		t.Fatal("expected nil — ?token= apiKey query form must be rejected on /ws")
	}

	r2, _ := http.NewRequest("GET", "/ws", nil)
	r2.Header.Set("Authorization", "Bearer "+apiKey)
	user2 := ws.AuthenticateWS(hub, r2)
	if user2 == nil {
		t.Fatal("expected user via bearer")
	}
	if user2.ID != u.ID {
		t.Fatalf("expected user %s, got %s", u.ID, user2.ID)
	}

	r3, _ := http.NewRequest("GET", "/ws", nil)
	r3.Header.Set("Sec-WebSocket-Protocol", "Bearer, "+apiKey)
	user3 := ws.AuthenticateWS(hub, r3)
	if user3 == nil {
		t.Fatal("expected user via sec-websocket-protocol")
	}

	r4, _ := http.NewRequest("GET", "/ws", nil)
	user4 := ws.AuthenticateWS(hub, r4)
	if user4 != nil {
		t.Fatal("expected nil for no auth")
	}
}

func TestWSListCommands(t *testing.T) {
	t.Parallel()
	ts, _, _ := testutil.NewTestServer(t)
	adminToken := testutil.LoginAs(t, ts.URL, "owner@test.com", "password123")

	resp, _ := testutil.JSON(t, "GET", ts.URL+"/api/v1/commands", adminToken, nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}
