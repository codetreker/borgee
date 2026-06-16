package ws

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"borgee-server/internal/config"
	"borgee-server/internal/store"

	"github.com/coder/websocket"
)

func newInternalHub(t *testing.T) (*Hub, *store.Store) {
	t.Helper()
	s := store.MigratedStoreFromTemplate(t)
	t.Cleanup(func() { s.Close() })

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := &config.Config{JWTSecret: "test", NodeEnv: "development", DevAuthBypass: true}
	return NewHub(s, logger, cfg), s
}

func TestInternalClientSendAndAliveEdges(t *testing.T) {
	t.Parallel()
	c := &Client{
		send:       make(chan []byte, 1),
		done:       make(chan struct{}),
		subscribed: map[string]bool{},
		alive:      true,
	}

	c.SendJSON(map[string]string{"type": "first"})
	c.SendJSON(map[string]string{"type": "dropped"})
	if got := len(c.send); got != 1 {
		t.Fatalf("expected send buffer to stay at 1, got %d", got)
	}
	<-c.send

	c.SendJSON(func() {})
	if got := len(c.send); got != 0 {
		t.Fatalf("invalid json should not enqueue, got %d", got)
	}

	c.SendPing()
	var ping map[string]string
	if err := json.Unmarshal(<-c.send, &ping); err != nil {
		t.Fatal(err)
	}
	if ping["type"] != "ping" {
		t.Fatalf("expected ping, got %q", ping["type"])
	}

	if !c.CheckAlive() {
		t.Fatal("first alive check should pass")
	}
	if c.CheckAlive() {
		t.Fatal("second alive check should fail until pong")
	}
	c.setAlive()
	if !c.CheckAlive() {
		t.Fatal("alive check should pass after setAlive")
	}

	close(c.send)
	c.writePump(context.Background())

	c2 := &Client{send: make(chan []byte), done: make(chan struct{})}
	close(c2.done)
	c2.writePump(context.Background())
}

func TestInternalClientCloseWithWebSocket(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true})
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "")
		_, _, _ = conn.Read(r.Context())
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	conn, _, err := websocket.Dial(ctx, "ws"+server.URL[len("http"):], nil)
	if err != nil {
		t.Fatal(err)
	}

	c := &Client{conn: conn, done: make(chan struct{})}
	c.Close()
	c.Close()

	select {
	case <-c.done:
	default:
		t.Fatal("Close should close done channel")
	}
}

func TestInternalHubBroadcastBranches(t *testing.T) {
	t.Parallel()
	hub, _ := newInternalHub(t)
	c1 := &Client{userID: "u1", send: make(chan []byte, 4), subscribed: map[string]bool{"ch": true}}
	c2 := &Client{userID: "u1", send: make(chan []byte, 4), subscribed: map[string]bool{"ch": true}}
	c3 := &Client{userID: "u2", send: make(chan []byte, 4), subscribed: map[string]bool{}}

	hub.Register(c1)
	hub.Register(c2)
	hub.Register(c3)

	hub.BroadcastToChannel("ch", map[string]string{"type": "channel"}, c1)
	if got := len(c1.send); got != 0 {
		t.Fatalf("excluded client should not receive channel broadcast, got %d", got)
	}
	if got := len(c2.send); got != 1 {
		t.Fatalf("subscribed client should receive channel broadcast, got %d", got)
	}
	if got := len(c3.send); got != 0 {
		t.Fatalf("unsubscribed client should not receive channel broadcast, got %d", got)
	}

	hub.BroadcastToUser("u1", map[string]string{"type": "user"})
	if got := len(c1.send); got != 1 {
		t.Fatalf("user client should receive direct broadcast, got %d", got)
	}
	if got := len(c2.send); got != 2 {
		t.Fatalf("second user client should receive direct broadcast, got %d", got)
	}

	hub.BroadcastToAll(map[string]string{"type": "all"})
	if got := len(c3.send); got != 1 {
		t.Fatalf("all broadcast should reach every client, got %d", got)
	}

	hub.UnsubscribeUserFromChannel("u1", "ch")
	if c1.IsSubscribed("ch") || c2.IsSubscribed("ch") {
		t.Fatal("expected user clients to be unsubscribed from channel")
	}

	ids := hub.GetOnlineUserIDs()
	if len(ids) != 2 {
		t.Fatalf("expected 2 online users, got %d", len(ids))
	}

	hub.BroadcastToChannel("ch", func() {}, nil)
	hub.BroadcastToUser("u1", func() {})
	hub.BroadcastToAll(func() {})

	hub.Unregister(c2)
	hub.Unregister(c1)
	hub.Unregister(c3)
	if got := hub.ClientCount(); got != 0 {
		t.Fatalf("expected no clients after unregister, got %d", got)
	}
}

func TestInternalCommandStoreReplacementAndLimits(t *testing.T) {
	t.Parallel()
	cs := NewCommandStore()
	cs.Register("conn-1", "agent-1", "Agent", []AgentCommand{{Name: "same"}, {Name: "old"}})
	cs.Register("conn-1", "agent-1", "Agent", []AgentCommand{{Name: "same"}, {Name: "new"}})

	if got := len(cs.GetByName("old")); got != 0 {
		t.Fatalf("re-register should remove old command name, got %d", got)
	}
	if got := len(cs.GetAll()[0].Commands); got != 2 {
		t.Fatalf("expected replacement commands only, got %d", got)
	}
	if cs.UnregisterByConnection("missing") {
		t.Fatal("missing connection unregister should return false")
	}

	cmds := make([]AgentCommand, 100)
	for i := range cmds {
		cmds[i] = AgentCommand{Name: "cmd" + string(rune('a'+i/26)) + string(rune('a'+i%26))}
	}
	cs.Register("conn-2", "agent-2", "Agent2", cmds)
	cs.Register("conn-3", "agent-2", "Agent2", []AgentCommand{{Name: "overflow"}})
	if got := len(cs.GetByName("overflow")); got != 0 {
		t.Fatalf("overflow command should be clipped, got %d", got)
	}
}

func TestInternalPluginConnRequestResponseBranches(t *testing.T) {
	t.Parallel()
	hub, _ := newInternalHub(t)
	hub.SetHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/json" {
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"ok":true}`))
			return
		}
		_, _ = w.Write([]byte("plain body"))
	}))
	pc := &PluginConn{
		hub:     hub,
		apiKey:  "key",
		send:    make(chan []byte, 2),
		done:    make(chan struct{}),
		pending: make(map[string]chan PluginResponse),
	}

	pc.sendJSON(func() {})
	pc.Send([]byte(`{"type":"manual"}`))
	pc.Send([]byte(`{"type":"dropped"}`))
	if got := len(pc.send); got != 2 {
		t.Fatalf("expected full send buffer, got %d", got)
	}
	<-pc.send
	<-pc.send

	pc.handleAPIResponse("bad", json.RawMessage(`{`))
	pc.pending["resp-1"] = make(chan PluginResponse, 1)
	pc.handleAPIResponse("resp-1", json.RawMessage(`{"status":202,"body":{"done":true}}`))
	resp := <-pc.pending["resp-1"]
	if resp.Status != http.StatusAccepted || string(resp.Body) != `{"done":true}` {
		t.Fatalf("unexpected plugin response: %#v body=%s", resp, resp.Body)
	}

	pc.handleAPIRequest("bad-req", json.RawMessage(`{`))
	if msg := readPluginSend(t, pc); msg["type"] != "api_response" {
		t.Fatalf("expected api_response for invalid request, got %v", msg)
	}

	pc.handleAPIRequest("json-req", json.RawMessage(`{"method":"POST","path":"/json","body":{"x":1}}`))
	jsonMsg := readPluginSend(t, pc)
	if jsonMsg["id"] != "json-req" {
		t.Fatalf("expected json-req response, got %v", jsonMsg["id"])
	}
	if data := jsonMsg["data"].(map[string]any); data["status"].(float64) != http.StatusCreated {
		t.Fatalf("expected status 201, got %v", data["status"])
	}

	pc.handleAPIRequest("plain-req", json.RawMessage(`{"path":"/plain"}`))
	plainMsg := readPluginSend(t, pc)
	if data := plainMsg["data"].(map[string]any); data["body"] != "plain body" {
		t.Fatalf("expected plain body, got %#v", data["body"])
	}

	go func() {
		msg := readPluginSend(t, pc)
		id := msg["id"].(string)
		pc.resolveRequest(id, http.StatusTeapot, []byte(`{"tea":true}`))
	}()
	got, err := pc.SendRequest("GET", "/plugin", []byte(`{"q":1}`))
	if err != nil {
		t.Fatalf("SendRequest: %v", err)
	}
	if got.Status != http.StatusTeapot || string(got.Body) != `{"tea":true}` {
		t.Fatalf("unexpected SendRequest result: %#v body=%s", got, got.Body)
	}

	close(pc.send)
	pc.writePump(context.Background())
	pc2 := &PluginConn{send: make(chan []byte), done: make(chan struct{})}
	close(pc2.done)
	pc2.writePump(context.Background())
}

func readPluginSend(t *testing.T, pc *PluginConn) map[string]any {
	t.Helper()
	select {
	case data := <-pc.send:
		var msg map[string]any
		if err := json.Unmarshal(data, &msg); err != nil {
			t.Fatal(err)
		}
		return msg
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for plugin send")
	}
	return nil
}

// TestInternalPluginAPIReqBucketBoundsSpawn is the #1108 F4 LAYER A red→green:
// the per-PluginConn token bucket gates the api_request goroutine spawn. With
// rate=0 (no refill mid-test) and max=3, the first 3 dispatch decisions must be
// admitted and the 4th/5th must be rejected — and a rejection must emit a 429
// api_response frame (status 429, body {"error":"rate limit exceeded"}, same
// envelope as a real api_response). Before the fix the read loop spawned
// handleAPIRequest unconditionally, so no bucket existed and no 429 frame was
// ever produced.
func TestInternalPluginAPIReqBucketBoundsSpawn(t *testing.T) {
	t.Parallel()
	pc := &PluginConn{
		agentID: "agent-1",
		send:    make(chan []byte, 8),
		done:    make(chan struct{}),
		pending: make(map[string]chan PluginResponse),
		apiReqBucket: apiReqBucket{
			tokens:   3,
			max:      3,
			rate:     0, // no refill: deterministic, exactly 3 admissions
			lastTime: time.Now(),
		},
	}

	// Drive the read loop's api_request gate decision 5 times. This mirrors
	// plugin.go's `if !pc.allowAPIRequest() { pc.send429APIResponse(id); ... }`.
	results := make([]bool, 5)
	for i := 0; i < 5; i++ {
		id := "req-" + string(rune('a'+i))
		if pc.allowAPIRequest() {
			results[i] = true
			continue
		}
		pc.send429APIResponse(id)
	}

	if !results[0] || !results[1] || !results[2] {
		t.Fatalf("first 3 api_requests should be admitted (max=3), got %v", results)
	}
	if results[3] || results[4] {
		t.Fatalf("4th/5th api_requests should be rejected (bucket empty), got %v", results)
	}

	// Exactly two 429 frames should have been emitted (one per rejection),
	// each with the over-rate envelope.
	for i := 3; i <= 4; i++ {
		msg := readPluginSend(t, pc)
		if msg["type"] != "api_response" {
			t.Fatalf("rejection %d: expected api_response frame, got %v", i, msg)
		}
		data, ok := msg["data"].(map[string]any)
		if !ok {
			t.Fatalf("rejection %d: missing data object, got %v", i, msg)
		}
		if data["status"].(float64) != float64(http.StatusTooManyRequests) {
			t.Fatalf("rejection %d: expected status 429, got %v", i, data["status"])
		}
		if data["body"] != `{"error":"rate limit exceeded"}` {
			t.Fatalf("rejection %d: expected rate-limit body, got %v", i, data["body"])
		}
	}

	// No extra frames: the 3 admissions did NOT emit anything here (their
	// spawn path is exercised elsewhere); only rejections emit.
	select {
	case extra := <-pc.send:
		t.Fatalf("unexpected extra frame after 2 rejections: %s", extra)
	default:
	}
}

func TestInternalRemoteConnRequestResponseBranches(t *testing.T) {
	t.Parallel()
	rc := &RemoteConn{
		send:    make(chan []byte, 2),
		done:    make(chan struct{}),
		pending: make(map[string]chan json.RawMessage),
	}

	rc.sendJSON(func() {})
	rc.Send([]byte(`{"type":"manual"}`))
	rc.Send([]byte(`{"type":"dropped"}`))
	if got := len(rc.send); got != 2 {
		t.Fatalf("expected full remote send buffer, got %d", got)
	}
	<-rc.send
	<-rc.send

	rc.pending["resp-1"] = make(chan json.RawMessage, 1)
	rc.resolveRequest("resp-1", json.RawMessage(`{"ok":true}`))
	if got := string(<-rc.pending["resp-1"]); got != `{"ok":true}` {
		t.Fatalf("unexpected remote response %s", got)
	}
	rc.resolveRequest("missing", json.RawMessage(`{"ignored":true}`))

	go func() {
		msg := readRemoteSend(t, rc)
		id := msg["id"].(string)
		rc.resolveRequest(id, json.RawMessage(`{"remote":true}`))
	}()
	got, err := rc.SendRequest(map[string]any{"action": "run"})
	if err != nil {
		t.Fatalf("SendRequest: %v", err)
	}
	if string(got) != `{"remote":true}` {
		t.Fatalf("unexpected SendRequest response %s", got)
	}

	close(rc.send)
	rc.writePump(context.Background())
	rc2 := &RemoteConn{send: make(chan []byte), done: make(chan struct{})}
	close(rc2.done)
	rc2.writePump(context.Background())
}

func readRemoteSend(t *testing.T, rc *RemoteConn) map[string]any {
	t.Helper()
	select {
	case data := <-rc.send:
		var msg map[string]any
		if err := json.Unmarshal(data, &msg); err != nil {
			t.Fatal(err)
		}
		return msg
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for remote send")
	}
	return nil
}

// TestWSAcceptOptionsCSWSH proves the CSWSH (Cross-Site WebSocket Hijacking)
// Origin defense on the shared accept-options helper:
//   - development → InsecureSkipVerify (old permissive behavior, so the e2e
//     Playwright browser + unit-test dials keep working);
//   - production → OriginPatterns sourced from CORS_ORIGIN, never an
//     unconditional skip.
func TestWSAcceptOptionsCSWSH(t *testing.T) {
	t.Parallel()

	dev := &config.Config{NodeEnv: "development", JWTSecret: "x"}
	devOpts := wsAcceptOptions(dev)
	if !devOpts.InsecureSkipVerify {
		t.Fatal("dev: expected InsecureSkipVerify=true (permissive dev/e2e path)")
	}
	if len(devOpts.OriginPatterns) != 0 {
		t.Fatalf("dev: expected no OriginPatterns, got %v", devOpts.OriginPatterns)
	}

	prod := &config.Config{NodeEnv: "production", JWTSecret: "x", CORSOrigin: "https://app.example.com"}
	prodOpts := wsAcceptOptions(prod)
	if prodOpts.InsecureSkipVerify {
		t.Fatal("prod: InsecureSkipVerify must NOT be unconditionally true")
	}
	if len(prodOpts.OriginPatterns) != 1 || prodOpts.OriginPatterns[0] != "https://app.example.com" {
		t.Fatalf("prod: expected OriginPatterns=[CORS_ORIGIN], got %v", prodOpts.OriginPatterns)
	}
}

// TestWSCSWSHHandshakeRejectsDisallowedOrigin drives a real handshake against
// all three rails mounted with a PRODUCTION config (NodeEnv != development) and
// asserts:
//   - a disallowed cross-origin browser handshake → 403 (CSWSH blocked);
//   - no Origin header (Go/Node/openclaw PROCESS clients) → upgrade allowed;
//   - same-origin (request Host) → upgrade allowed;
//   - the CORS_ORIGIN allow-listed Origin → upgrade allowed.
//
// It sends a hand-built WebSocket upgrade request via net/http so it can read
// the raw handshake status code on each rail (coder/websocket's Dial hides it).
func TestWSCSWSHHandshakeRejectsDisallowedOrigin(t *testing.T) {
	t.Parallel()

	s := store.MigratedStoreFromTemplate(t)
	t.Cleanup(func() { s.Close() })
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	allowedOrigin := "https://app.example.com"
	cfg := &config.Config{
		NodeEnv:    "production",
		JWTSecret:  "prod-secret",
		CORSOrigin: allowedOrigin,
	}
	hub := NewHub(s, logger, cfg)

	// Seed an agent + apiKey so each rail's auth gate passes and execution
	// reaches the Origin check inside websocket.Accept.
	apiKey, err := store.GenerateAPIKey()
	if err != nil {
		t.Fatal(err)
	}
	agent := &store.User{DisplayName: "CSWSHBot", Role: "agent", APIKey: &apiKey}
	if err := s.CreateUser(agent); err != nil {
		t.Fatalf("create agent: %v", err)
	}

	// A remote-node token so /ws/remote auth passes.
	node, err := s.CreateRemoteNode(agent.ID, "cswsh-node")
	if err != nil {
		t.Fatalf("create remote node: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", HandleClient(hub))
	mux.HandleFunc("/ws/plugin", HandlePlugin(hub))
	mux.HandleFunc("/ws/remote", HandleRemote(hub))
	ts := httptest.NewServer(mux)
	defer ts.Close()

	host := strings.TrimPrefix(ts.URL, "http://")
	wsBase := "ws://" + host

	// rejectStatus sends a hand-built WS upgrade via net/http and returns the
	// HTTP response status. Used for the disallowed-Origin path, which 403s
	// before any hijack (so the http client sees a normal error response).
	rejectStatus := func(t *testing.T, path, origin string, auth func(*http.Request)) int {
		t.Helper()
		req, err := http.NewRequest(http.MethodGet, ts.URL+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Connection", "Upgrade")
		req.Header.Set("Upgrade", "websocket")
		req.Header.Set("Sec-WebSocket-Version", "13")
		req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
		auth(req)
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("handshake %s origin=%q: %v", path, origin, err)
		}
		defer resp.Body.Close()
		return resp.StatusCode
	}

	// dialOK dials the rail via coder/websocket with the given Origin and
	// reports whether the upgrade succeeded. Used for the allow paths.
	dialOK := func(t *testing.T, path, origin string, hdr http.Header) bool {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		h := http.Header{}
		for k, v := range hdr {
			h[k] = v
		}
		if origin != "" {
			h.Set("Origin", origin)
		}
		conn, _, err := websocket.Dial(ctx, wsBase+path, &websocket.DialOptions{HTTPHeader: h})
		if err != nil {
			return false
		}
		conn.Close(websocket.StatusNormalClosure, "")
		return true
	}

	rails := []struct {
		path string
		hdr  http.Header
	}{
		{"/ws", http.Header{"Authorization": []string{"Bearer " + apiKey}}},
		{"/ws/plugin", http.Header{"Authorization": []string{"Bearer " + apiKey}}},
		{"/ws/remote", http.Header{"Authorization": []string{"Bearer " + node.ConnectionToken}}},
	}

	for _, rail := range rails {
		rail := rail
		applyAuth := func(r *http.Request) {
			for k, v := range rail.hdr {
				r.Header[k] = v
			}
		}
		t.Run(rail.path, func(t *testing.T) {
			// Subtests are NOT t.Parallel() here: the parent's deferred
			// ts.Close() would otherwise tear down the server before these
			// run (parallel subtests execute after the parent returns).

			// Disallowed cross-origin browser → 403 (CSWSH blocked).
			if got := rejectStatus(t, rail.path, "https://evil.example.com", applyAuth); got != http.StatusForbidden {
				t.Fatalf("%s disallowed origin: expected 403, got %d", rail.path, got)
			}

			// No Origin header (process clients) → upgrade allowed.
			if !dialOK(t, rail.path, "", rail.hdr) {
				t.Fatalf("%s no-origin: expected upgrade to succeed", rail.path)
			}

			// Same-origin (Origin host == request Host) → allowed.
			if !dialOK(t, rail.path, "http://"+host, rail.hdr) {
				t.Fatalf("%s same-origin: expected upgrade to succeed", rail.path)
			}

			// CORS_ORIGIN allow-listed Origin → allowed.
			if !dialOK(t, rail.path, allowedOrigin, rail.hdr) {
				t.Fatalf("%s allowed origin: expected upgrade to succeed", rail.path)
			}
		})
	}
}

func TestInternalAuthenticateWSDevFallbacksAndHelpers(t *testing.T) {
	t.Parallel()
	hub, s := newInternalHub(t)
	email := "dev@example.com"
	user := &store.User{ID: "dev-user", Email: &email, DisplayName: "Dev User", Role: "member"}
	if err := s.CreateUser(user); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/ws?user_id=dev-user", nil)
	if got := authenticateWS(hub, req); got == nil || got.ID != user.ID {
		t.Fatalf("expected dev query auth, got %#v", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Header.Set("X-Dev-User-Id", "dev-user")
	if got := authenticateWS(hub, req); got == nil || got.ID != user.ID {
		t.Fatalf("expected dev header auth, got %#v", got)
	}

	if mustJSON(func() {}) != "{}" {
		t.Fatal("mustJSON should return empty object on marshal failure")
	}
	if newID() == "" {
		t.Fatal("newID should not be empty")
	}
	if nowMs() <= 0 {
		t.Fatal("nowMs should be positive")
	}
}
