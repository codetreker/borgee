package ws_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"borgee-server/internal/config"
	"borgee-server/internal/store"
	"borgee-server/internal/ws"

	"github.com/coder/websocket"
)

// newPluginWSHarness builds a real plugin-WS rail backed by NewHub +
// HandlePlugin, mounts it on an httptest server, and wires hub.SetHandler to
// the given re-entry handler (the thing handleAPIRequest replays into). It
// seeds one agent (with apiKey) per requested agentID-slot and returns the
// server URL plus the created agents. This mirrors the production wiring
// (server.go: NewHub → hub.SetHandler(srv.Handler()) → mux /ws/plugin =
// HandlePlugin) so api_request frames travel the SAME read loop, not a helper
// invoked in isolation.
func newPluginWSHarness(t *testing.T, cfg *config.Config, reentry http.Handler, nAgents int) (string, []*store.User) {
	t.Helper()

	s := store.MigratedStoreFromTemplate(t)
	t.Cleanup(func() { s.Close() })
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	hub := ws.NewHub(s, logger, cfg)
	// handleAPIRequest re-enters THIS handler; in production it's the full mux
	// (server.go:100 hub.SetHandler(srv.Handler())).
	hub.SetHandler(reentry)

	agents := make([]*store.User, 0, nAgents)
	for i := 0; i < nAgents; i++ {
		apiKey, err := store.GenerateAPIKey()
		if err != nil {
			t.Fatalf("generate api key: %v", err)
		}
		agent := &store.User{
			DisplayName: "PluginRLBot",
			Role:        "agent",
			APIKey:      &apiKey,
		}
		if err := s.CreateUser(agent); err != nil {
			t.Fatalf("create agent %d: %v", i, err)
		}
		// Stash the plaintext key on the returned struct for the dialer (the
		// store hashes it server-side, so reload would not expose it).
		agent.APIKey = &apiKey
		agents = append(agents, agent)
	}

	ts := httptest.NewServer(http.HandlerFunc(ws.HandlePlugin(hub)))
	t.Cleanup(ts.Close)
	return ts.URL, agents
}

func dialPluginWS(t *testing.T, ctx context.Context, serverURL, apiKey string) *websocket.Conn {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(serverURL, "http") + "/ws/plugin"
	conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{
		HTTPHeader: http.Header{"Authorization": []string{"Bearer " + apiKey}},
	})
	if err != nil {
		t.Fatalf("plugin ws dial: %v", err)
	}
	t.Cleanup(func() { conn.Close(websocket.StatusNormalClosure, "") })
	return conn
}

// TestPluginWSReadLoopGatesAPIRequestSpawn is the #1108 F4 LAYER A red→green,
// rewritten to drive the REAL HandlePlugin read loop end-to-end (the prior
// version only invoked pc.allowAPIRequest/pc.send429APIResponse in isolation,
// so deleting the read-loop gate left it green).
//
// Wiring: a per-PluginConn bucket seeded from RatePluginAPIReqBurst=3,
// RatePluginAPIReqPerSec=0 (no mid-test refill). The re-entry handler always
// returns 200, so it can NEVER produce a 429 itself — the ONLY source of a 429
// api_response frame is the read loop's
// `if !pc.allowAPIRequest() { pc.send429APIResponse(id); continue }` gate.
//
// We fire 5 api_request frames over one connection. With burst=3, exactly the
// 4th and 5th must come back as 429 api_response frames. If the read-loop gate
// (plugin.go) is reverted, every frame reaches the always-200 handler and NO
// 429 is ever produced → this test fails (proven RED by reverting the gate).
func TestPluginWSReadLoopGatesAPIRequestSpawn(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{
		JWTSecret: "test",
		NodeEnv:   "development",
		// Small per-conn bucket, no refill → deterministic: exactly 3 admits.
		RatePluginAPIReqPerSec: 0,
		RatePluginAPIReqBurst:  3,
	}
	// Re-entry handler that ALWAYS returns 200 → it can never emit a 429, so
	// any 429 api_response observed must come from the read-loop spawn gate.
	always200 := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	serverURL, agents := newPluginWSHarness(t, cfg, always200, 1)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn := dialPluginWS(t, ctx, serverURL, *agents[0].APIKey)

	const total = 5
	for i := 0; i < total; i++ {
		frame, _ := json.Marshal(map[string]any{
			"type": "api_request",
			"id":   "req-" + string(rune('a'+i)),
			"data": map[string]any{"method": "GET", "path": "/api/v1/channels"},
		})
		if err := conn.Write(ctx, websocket.MessageText, frame); err != nil {
			t.Fatalf("write api_request %d: %v", i, err)
		}
	}

	// Collect the responses keyed by request id (read-loop spawns are
	// concurrent goroutines, so ordering across distinct ids is not stable;
	// keying by id makes the burst boundary deterministic).
	statuses := make(map[string]float64, total)
	for len(statuses) < total {
		readCtx, readCancel := context.WithTimeout(ctx, 5*time.Second)
		_, data, err := conn.Read(readCtx)
		readCancel()
		if err != nil {
			t.Fatalf("read api_response (got %d/%d): %v", len(statuses), total, err)
		}
		var msg struct {
			Type string `json:"type"`
			ID   string `json:"id"`
			Data struct {
				Status float64 `json:"status"`
			} `json:"data"`
		}
		if err := json.Unmarshal(data, &msg); err != nil {
			t.Fatalf("unmarshal frame: %v (raw=%s)", err, data)
		}
		if msg.Type != "api_response" {
			continue // ignore presence/pong noise
		}
		statuses[msg.ID] = msg.Data.Status
	}

	var ok200, rl429 int
	for id, st := range statuses {
		switch st {
		case http.StatusOK:
			ok200++
		case http.StatusTooManyRequests:
			rl429++
		default:
			t.Fatalf("req %s: unexpected status %v", id, st)
		}
	}

	// burst=3 → exactly 3 admitted (200 from always200 handler) and 2 rejected
	// by the read-loop gate (429). If the gate is removed, all 5 reach the
	// always-200 handler → rl429==0 → this assertion fails.
	if ok200 != 3 {
		t.Fatalf("expected exactly 3 admitted api_requests (200), got %d (statuses=%v)", ok200, statuses)
	}
	if rl429 != 2 {
		t.Fatalf("expected exactly 2 read-loop 429 rejections, got %d (statuses=%v) — "+
			"the api_request spawn gate is not bounding the read loop", rl429, statuses)
	}
}

// TestPluginWSAPIRequestKeysAuthBucketPerAgent is the #1108 F4 LAYER B
// red→green, rewritten to drive the REAL HandlePlugin read loop +
// handleAPIRequest re-entry (the prior version hardcoded RemoteAddr in the test
// body and asserted clientIP, never exercising the production line).
//
// handleAPIRequest replays the api_request into hub.handler via
// httptest.NewRequest, which HARDCODES RemoteAddr to "192.0.2.1:1234". The
// LAYER B production line `httpReq.RemoteAddr = pc.agentID + ":0"` overrides it
// so each agent's auth re-entry keys auth:<agentID>, not the shared
// 192.0.2.1 bucket.
//
// Here the re-entry handler is a PROBE that records the inbound r.RemoteAddr.
// We fire an api_request to an auth path (/api/v1/auth/login) from TWO
// distinct-agentID plugin connections and assert each probe recording equals
// that agent's own ID — never the shared httptest constant. If the LAYER B line
// is reverted, both recordings are "192.0.2.1:1234" → assertion fails (proven
// RED by reverting the line).
func TestPluginWSAPIRequestKeysAuthBucketPerAgent(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	// recorded[agentID] = the r.RemoteAddr the re-entry handler saw for that
	// agent's api_request (matched by the Authorization Bearer apiKey is
	// fragile; instead we match by the request body we send below).
	recorded := make([]string, 0, 2)
	probe := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		recorded = append(recorded, r.RemoteAddr)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	cfg := &config.Config{
		JWTSecret: "test",
		NodeEnv:   "development",
		// Generous bucket so neither agent is gated by LAYER A in this test.
		RatePluginAPIReqPerSec: 100,
		RatePluginAPIReqBurst:  100,
	}
	serverURL, agents := newPluginWSHarness(t, cfg, probe, 2)
	agentA, agentB := agents[0], agents[1]

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	fireAuthRequest := func(apiKey string) {
		conn := dialPluginWS(t, ctx, serverURL, apiKey)
		frame, _ := json.Marshal(map[string]any{
			"type": "api_request",
			"id":   "auth-req",
			"data": map[string]any{"method": "POST", "path": "/api/v1/auth/login"},
		})
		if err := conn.Write(ctx, websocket.MessageText, frame); err != nil {
			t.Fatalf("write auth api_request: %v", err)
		}
		// Block until the api_response comes back so the re-entry (and thus the
		// probe recording) has definitely happened before we move on.
		for {
			readCtx, readCancel := context.WithTimeout(ctx, 5*time.Second)
			_, data, err := conn.Read(readCtx)
			readCancel()
			if err != nil {
				t.Fatalf("read auth api_response: %v", err)
			}
			var msg struct {
				Type string `json:"type"`
				ID   string `json:"id"`
			}
			_ = json.Unmarshal(data, &msg)
			if msg.Type == "api_response" && msg.ID == "auth-req" {
				return
			}
		}
	}

	fireAuthRequest(*agentA.APIKey)
	fireAuthRequest(*agentB.APIKey)

	mu.Lock()
	got := append([]string(nil), recorded...)
	mu.Unlock()

	if len(got) != 2 {
		t.Fatalf("expected exactly 2 probe recordings, got %d: %v", len(got), got)
	}

	// Each api_request must have re-entered with RemoteAddr keyed to its OWN
	// agentID (LAYER B: httpReq.RemoteAddr = pc.agentID + ":0"). agentID ==
	// user.ID. Pre-fix both would be the httptest constant "192.0.2.1:1234".
	wantA := agentA.ID + ":0"
	wantB := agentB.ID + ":0"
	gotSet := map[string]bool{got[0]: true, got[1]: true}

	if !gotSet[wantA] {
		t.Fatalf("agentA api_request did not key auth bucket per-agent: want RemoteAddr %q in %v "+
			"(LAYER B RemoteAddr override missing → shared httptest 192.0.2.1)", wantA, got)
	}
	if !gotSet[wantB] {
		t.Fatalf("agentB api_request did not key auth bucket per-agent: want RemoteAddr %q in %v "+
			"(LAYER B RemoteAddr override missing → shared httptest 192.0.2.1)", wantB, got)
	}
	if got[0] == got[1] {
		t.Fatalf("two distinct agents collapsed onto ONE shared RemoteAddr key %q — "+
			"per-agent auth bucket keying is broken", got[0])
	}
	for _, addr := range got {
		if strings.HasPrefix(addr, "192.0.2.1") {
			t.Fatalf("auth re-entry RemoteAddr is the httptest constant %q, not the agentID — "+
				"LAYER B per-agent keying reverted", addr)
		}
	}
}
