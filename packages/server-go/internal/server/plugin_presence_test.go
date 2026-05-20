// plugin_presence_test.go — agent 上线 UI 灰头像 bug 回归.
//
// 真因 (fix/agent-presence-online): ws/plugin.go::HandlePlugin 漏写
// hub.store.UpdateLastSeen + BroadcastToAll(presence online), 跟
// ws/client.go::HandleClient 路径不对齐. 后果:
//   1. REST GET /api/v1/online (查 users.last_seen_at > now-5min)
//      永远不含通过 plugin WS 上线的 agent → 客户端 AppContext.onlineUserIds
//      缺这个 ID → Sidebar 灰头像;
//   2. 其它已连 client 收不到 presence online 帧 → 它们的 usePresence cache
//      永远空 → ChannelMembersModal 的 PresenceDot fallback 为 offline (灰).
//
// 本测试用真 HTTP server + 真 WebSocket dial 走 /ws/plugin 路径, 断言:
//   §1 plugin 连接成功后 store.GetOnlineUsers() 包含 agent.ID;
//   §2 已连的 client 收到 {type:"presence", user_id:agent.ID, status:"online"} 帧;
//   §3 plugin 断开后 client 收到 status:"offline" 帧.
package server

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"borgee-server/internal/store"

	"github.com/gorilla/websocket"
)

func TestPluginPresence_UpdateLastSeenAndBroadcast(t *testing.T) {
	t.Parallel()
	srv, s := testServer(t)

	apiKey := "bgr_plugin_presence_agent"
	agent := &store.User{DisplayName: "Presence Bot", Role: "agent", APIKey: &apiKey}
	if err := s.CreateUser(agent); err != nil {
		t.Fatalf("create agent: %v", err)
	}
	// human observer that will receive the presence broadcast.
	humanKey := "bgr_plugin_presence_human"
	human := &store.User{DisplayName: "Human Watcher", Role: "member", APIKey: &humanKey}
	if err := s.CreateUser(human); err != nil {
		t.Fatalf("create human: %v", err)
	}

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// Connect the human first so it is on the hub before the agent's
	// presence-online broadcast fires. /ws (HandleClient) accepts the
	// Authorization Bearer apiKey path via authenticateWS.
	humanURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws?token=" + humanKey
	humanConn, _, err := websocket.DefaultDialer.Dial(humanURL, nil)
	if err != nil {
		t.Fatalf("dial human ws: %v", err)
	}
	defer humanConn.Close()

	// Drain the human's own presence-online frame (HandleClient broadcasts
	// it on its own Register) so the next ReadJSON is the agent's frame.
	deadline := time.Now().Add(2 * time.Second)
	if err := humanConn.SetReadDeadline(deadline); err != nil {
		t.Fatalf("set human read deadline: %v", err)
	}
	if err := drainOwnPresence(humanConn, human.ID); err != nil {
		t.Fatalf("drain human self-presence: %v", err)
	}

	// Now dial the plugin path as the agent.
	pluginURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/plugin?apiKey=" + apiKey
	pluginConn, _, err := websocket.DefaultDialer.Dial(pluginURL, nil)
	if err != nil {
		t.Fatalf("dial plugin ws: %v", err)
	}

	// §2 — wait for the human observer to receive the agent's
	// presence-online broadcast FIRST. The plugin path runs (synchronously
	// on the WS goroutine):
	//   RegisterPlugin  → UpdateLastSeen  → BroadcastToAll(presence)
	// so once the broadcast lands on the observer's read loop, both
	// UpdateLastSeen and RegisterPlugin must have completed — which is
	// the sync point we need before reading GetOnlineUsers. Polling on
	// hub.GetPlugin alone races with UpdateLastSeen under -race (the
	// observer could miss the in-between-lock window otherwise).
	if err := humanConn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("set human read deadline (online): %v", err)
	}
	frame, err := readPresenceFrame(humanConn, agent.ID)
	if err != nil {
		pluginConn.Close()
		t.Fatalf("read agent presence online frame: %v", err)
	}
	if frame.Status != "online" {
		t.Fatalf("expected status=online, got %q", frame.Status)
	}

	// §1 — REST /api/v1/online surface: agent must show as online (now
	// that the broadcast has been observed, UpdateLastSeen is guaranteed
	// to have completed).
	online, err := s.GetOnlineUsers()
	if err != nil {
		t.Fatalf("GetOnlineUsers: %v", err)
	}
	found := false
	for _, u := range online {
		if u.ID == agent.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("agent %s missing from GetOnlineUsers — UpdateLastSeen not called on plugin connect", agent.ID)
	}

	// §3 — close plugin, observer must receive presence offline frame.
	pluginConn.Close()
	if err := humanConn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("set human read deadline (offline): %v", err)
	}
	frameOff, err := readPresenceFrame(humanConn, agent.ID)
	if err != nil {
		t.Fatalf("read agent presence offline frame: %v", err)
	}
	if frameOff.Status != "offline" {
		t.Fatalf("expected status=offline after plugin disconnect, got %q", frameOff.Status)
	}
}

// presenceFrame mirrors the wire schema emitted by ws/client.go and
// (post-fix) ws/plugin.go — {type:"presence", user_id, status}.
type presenceFrame struct {
	Type   string `json:"type"`
	UserID string `json:"user_id"`
	Status string `json:"status"`
}

// readPresenceFrame skips any non-matching frames (e.g. the observer's
// own self-presence echo, pings, unrelated broadcasts) until it sees one
// for the target user, or the read deadline fires.
func readPresenceFrame(c *websocket.Conn, targetUserID string) (presenceFrame, error) {
	for {
		_, raw, err := c.ReadMessage()
		if err != nil {
			return presenceFrame{}, err
		}
		var pf presenceFrame
		if jerr := json.Unmarshal(raw, &pf); jerr != nil {
			continue
		}
		if pf.Type != "presence" || pf.UserID != targetUserID {
			continue
		}
		return pf, nil
	}
}

// drainOwnPresence reads frames until either the observer's own
// self-presence frame is consumed or the read deadline fires. Non-self,
// non-presence frames are skipped (e.g. an unrelated heartbeat ping).
// Uses the same deadline already set on the connection.
func drainOwnPresence(c *websocket.Conn, selfUserID string) error {
	for {
		_, raw, err := c.ReadMessage()
		if err != nil {
			return err
		}
		var pf presenceFrame
		if jerr := json.Unmarshal(raw, &pf); jerr != nil {
			continue
		}
		if pf.Type == "presence" && pf.UserID == selfUserID {
			return nil
		}
		// Other broadcast — keep draining.
	}
}

// unused but kept to discourage accidental import-purge — ensures the
// context dependency stays linked if a follow-up adds a ctx-bound test.
var _ = context.Background
