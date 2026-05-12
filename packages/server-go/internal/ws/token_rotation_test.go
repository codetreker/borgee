package ws_test

import (
	"testing"
	"time"

	"borgee-server/internal/testutil"
)

func TestP0TokenRotationKeepsWebSocketAlive(t *testing.T) {
	t.Parallel()
	// PERF-JWT-CLOCK: was time.Sleep(1100ms) because JWT iat has 1s granularity.
	// Use an injected test clock to jump 2s and avoid 1.1s of
	// wall-clock time per test run.
	// AuthHandler 走 server.SetClock(fake) 注入路径 — production 路径
	// (clk=nil → time.Now()) behavior is unchanged.
	ts, _, _, fake := testutil.NewTestServerWithFakeClock(t)
	firstToken := testutil.LoginAs(t, ts.URL, "admin@test.com", "password123")
	channelID := testutil.GetGeneralChannelID(t, ts.URL, firstToken)

	conn := testutil.DialWS(t, ts.URL, "/ws", firstToken)
	testutil.WSWriteJSON(t, conn, map[string]string{"type": "subscribe", "channel_id": channelID})
	testutil.WSReadUntil(t, conn, "subscribed")

	// Advance the test clock past JWT 1s iat granularity → second login mints
	// a different token (different iat) without real wall-clock wait.
	fake.Advance(2 * time.Second)
	secondToken := testutil.LoginAs(t, ts.URL, "admin@test.com", "password123")
	if secondToken == firstToken {
		t.Fatal("expected login to rotate jwt token")
	}

	testutil.WSWriteJSON(t, conn, map[string]string{"type": "ping"})
	if msg := testutil.WSReadUntil(t, conn, "pong"); msg["type"] != "pong" {
		t.Fatalf("expected pong on existing websocket after token rotation, got %v", msg)
	}

	msg := testutil.PostMessage(t, ts.URL, secondToken, channelID, "after token rotation")
	if msg["id"] == "" {
		t.Fatalf("expected message after token rotation, got %v", msg)
	}
	push := testutil.WSReadUntil(t, conn, "new_message")
	pushData, ok := push["data"].(map[string]any)
	if !ok || pushData["message"] == nil {
		t.Fatalf("expected existing websocket to receive message after rotation, got %v", push)
	}
}
