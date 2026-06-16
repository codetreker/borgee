package ws

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"

	"borgee-server/internal/idgen"
)

type PluginConn struct {
	hub     *Hub
	conn    *websocket.Conn
	agentID string
	apiKey  string
	send    chan []byte
	done    chan struct{}
	alive   bool

	// lastSeenAt: BPP-4.1 watchdog liveness — updated on every inbound
	// frame (ping/pong/api_request/api_response/response/BPP frame).
	// hub.SnapshotPluginLastSeen() reads this for the bpp watchdog.
	// Mutex-protected because the watchdog ticker reads concurrently
	// with the read loop's writes.
	lastSeenMu sync.Mutex
	lastSeenAt time.Time

	pendingMu sync.Mutex
	pending   map[string]chan PluginResponse

	// apiReqBucket: per-connection token bucket gating the api_request
	// goroutine spawn (#1108 F4). Each inbound api_request must take a
	// token BEFORE `go pc.handleAPIRequest` runs, so a single plugin
	// connection cannot fire an unbounded number of concurrent
	// goroutines (and concurrent re-entries into the HTTP stack). The
	// math mirrors server/middleware.go rateLimiter.allow; kept
	// self-contained here so internal/ws does not import internal/server.
	apiReqBucket apiReqBucket
}

// apiReqBucket is a self-contained leaky-token bucket for api_request
// admission. rate=tokens/sec refill, max=burst ceiling. The zero value is NOT
// ready — HandlePlugin seeds tokens/max/rate/lastTime from config so a fresh
// connection starts with a full burst.
type apiReqBucket struct {
	mu       sync.Mutex
	tokens   float64
	max      float64
	rate     float64
	lastTime time.Time
}

// allowAPIRequest takes one token from the per-connection bucket, refilling
// by elapsed*rate (capped at max) first. Returns false when the bucket is
// empty — the caller must then NOT spawn handleAPIRequest. Mirrors
// server/middleware.go rateLimiter.allow.
func (pc *PluginConn) allowAPIRequest() bool {
	b := &pc.apiReqBucket
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(b.lastTime).Seconds()
	b.tokens += elapsed * b.rate
	if b.tokens > b.max {
		b.tokens = b.max
	}
	b.lastTime = now

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// send429APIResponse emits the over-rate api_response frame for a dropped
// api_request (#1108 F4). Same envelope shape as handleAPIRequest's success
// frame so the plugin's response demux handles it uniformly.
func (pc *PluginConn) send429APIResponse(id string) {
	pc.sendJSON(map[string]any{
		"type": "api_response",
		"id":   id,
		"data": map[string]any{
			"status": http.StatusTooManyRequests,
			"body":   `{"error":"rate limit exceeded"}`,
		},
	})
}

type PluginResponse struct {
	Status int
	Body   []byte
}

func HandlePlugin(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// #1031 concern-2: plugin WS auth is `Authorization: Bearer <key>` ONLY.
		// The deprecated `?apiKey=` query form was removed — an api_key in the
		// URL leaks into access logs / proxies / referrers / browser history.
		// The in-repo + published (@codetreker/borgee-openclaw-plugin 0.1.3)
		// clients dial with the header, so this breaks only aged third-party
		// builds still pinned to a query-dialing distribution.
		var apiKey string
		if authHeader := r.Header.Get("Authorization"); strings.HasPrefix(authHeader, "Bearer ") {
			apiKey = strings.TrimPrefix(authHeader, "Bearer ")
		}
		if apiKey == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		user, err := hub.store.GetUserByAPIKey(apiKey)
		if err != nil || user.DeletedAt != nil || user.Disabled {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		conn, err := websocket.Accept(w, r, wsAcceptOptions(hub.config))
		if err != nil {
			hub.logger.Error("plugin ws accept failed", "error", err)
			return
		}

		pc := &PluginConn{
			hub:        hub,
			conn:       conn,
			agentID:    user.ID,
			apiKey:     apiKey,
			send:       make(chan []byte, sendBufSize),
			done:       make(chan struct{}),
			alive:      true,
			lastSeenAt: time.Now(),
			pending:    make(map[string]chan PluginResponse),
			apiReqBucket: apiReqBucket{
				tokens:   float64(hub.config.RatePluginAPIReqBurst),
				max:      float64(hub.config.RatePluginAPIReqBurst),
				rate:     float64(hub.config.RatePluginAPIReqPerSec),
				lastTime: time.Now(),
			},
		}

		hub.RegisterPlugin(user.ID, pc)

		// agent 上线信号 — 跟 client.go::HandleClient 同款 (UpdateLastSeen +
		// BroadcastToAll presence)。漏写一边 agent 通过 plugin WS 连入时:
		//   - users.last_seen_at 不更, REST /api/v1/online 5min 窗口不含 agent
		//     → AppContext.onlineUserIds 永远缺这个 ID → Sidebar 灰头像;
		//   - presence 帧不广播, 别的客户端的 usePresence cache 拿不到 state
		//     → ChannelMembersModal 的 PresenceDot fallback 到 offline (灰).
		// 跟 client.go:208-214, 222-226 字面对齐 (帧 schema 一致, 复用
		// useWebSocket.ts:295-297 现有 mirror, 别新创 `presence.changed`).
		hub.store.UpdateLastSeen(user.ID)
		hub.BroadcastToAll(map[string]any{
			"type":    "presence",
			"user_id": user.ID,
			"status":  "online",
		})

		ctx := r.Context()
		go pc.writePump(ctx)

		defer func() {
			hub.UnregisterPlugin(user.ID)
			hub.BroadcastToAll(map[string]any{
				"type":    "presence",
				"user_id": user.ID,
				"status":  "offline",
			})
			conn.Close(websocket.StatusNormalClosure, "")
		}()

		for {
			_, data, err := conn.Read(ctx)
			if err != nil {
				return
			}

			var msg struct {
				Type string          `json:"type"`
				ID   string          `json:"id,omitempty"`
				Data json.RawMessage `json:"data,omitempty"`
			}
			if err := json.Unmarshal(data, &msg); err != nil {
				continue
			}

			pc.alive = true
			pc.touchLastSeen()

			switch msg.Type {
			case "ping":
				pc.sendJSON(map[string]string{"type": "pong"})
			case "pong":
				// alive already set
			case "api_request":
				// #1108 F4: bound the goroutine spawn per connection. The
				// re-entry still passes rateLimitMiddleware (user:<userID>),
				// but without this gate one plugin could spawn unbounded
				// goroutines. Over-rate → emit a 429 api_response frame and
				// drop the request (do NOT close the connection).
				if !pc.allowAPIRequest() {
					pc.send429APIResponse(msg.ID)
					continue
				}
				go pc.handleAPIRequest(msg.ID, msg.Data)
			case "api_response":
				go pc.handleAPIResponse(msg.ID, msg.Data)
			case "response":
				pc.resolveRequest(msg.ID, 200, msg.Data)
			default:
				// BPP-3 (this PR) unified BPP frame dispatcher boundary.
				// AL-2b ack ingress (`agent_config_ack`) and any future
				// Plugin→Server BPP frames (BPP-2 task lifecycle, etc.)
				// land here.
				//
				// 立场: RPC envelope above ({type, id, data}) is request-
				// reply; BPP envelope here is fire-and-forget event
				// stream — different shapes, different lifecycle, hence
				// the dispatch boundary split.
				//
				// nil-safe: if SetPluginFrameRouter never called (early
				// boot or unit tests not exercising plugin BPP frames),
				// soft-skip. Same forward-compat semantics as router
				// receiving unknown frame type.
				router := hub.pluginFrameRouterSnapshot()
				if router == nil {
					continue
				}
				// Pass the FULL raw wire payload (`data`, not the inner
				// `msg.Data`) — BPP frames have shape `{type, ...payload-
				// direct-fields}`, no `data` wrapper. plugin.go's
				// json.Unmarshal above into the {type, id, data} struct
				// only peeks `type`; the raw bytes are still the full
				// frame.
				if _, err := router.Route(data, PluginSessionContext{OwnerUserID: user.ID}); err != nil {
					hub.logger.Warn("bpp.plugin_frame_route_failed",
						"agent_id", user.ID, "type", msg.Type, "error", err)
				}
			}
		}
	}
}

// touchLastSeen marks the connection alive at time.Now() for the BPP-4
// watchdog (hub.SnapshotPluginLastSeen consumer). Mutex-guarded for
// concurrent watchdog reads.
func (pc *PluginConn) touchLastSeen() {
	pc.lastSeenMu.Lock()
	pc.lastSeenAt = time.Now()
	pc.lastSeenMu.Unlock()
}

// LastSeen returns the last inbound-frame timestamp under lock.
// Exported for hub.SnapshotPluginLastSeen consumption only.
func (pc *PluginConn) LastSeen() time.Time {
	pc.lastSeenMu.Lock()
	defer pc.lastSeenMu.Unlock()
	return pc.lastSeenAt
}

func (pc *PluginConn) handleAPIResponse(id string, data json.RawMessage) {
	var resp struct {
		Status int             `json:"status"`
		Body   json.RawMessage `json:"body"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return
	}
	pc.resolveRequest(id, resp.Status, resp.Body)
}

func (pc *PluginConn) handleAPIRequest(id string, data json.RawMessage) {
	var req struct {
		Method string          `json:"method"`
		Path   string          `json:"path"`
		Body   json.RawMessage `json:"body,omitempty"`
	}
	if err := json.Unmarshal(data, &req); err != nil {
		pc.sendJSON(map[string]any{
			"type": "api_response",
			"id":   id,
			"data": map[string]any{"status": 400, "body": `{"error":"invalid request"}`},
		})
		return
	}

	method := req.Method
	if method == "" {
		method = "GET"
	}

	var bodyReader io.Reader
	if len(req.Body) > 0 {
		bodyReader = bytes.NewReader(req.Body)
	}

	httpReq := httptest.NewRequest(method, req.Path, bodyReader)
	httpReq.Header.Set("Authorization", "Bearer "+pc.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	// #1108 F4 LAYER B: httptest.NewRequest hardcodes RemoteAddr to
	// "192.0.2.1:1234", so every plugin's api_request to an auth path
	// (/api/v1/auth/*) would share one auth:192.0.2.1 brute-force bucket —
	// one noisy agent could throttle auth re-entries for ALL agents, and a
	// single agent's own bucket key wouldn't isolate it. Key the bucket per
	// agent instead. clientIP/hostOnly (server/middleware.go) strips at the
	// last ':' → yields the agentID (a UUID, no internal colons). With
	// TrustedProxyCount default 0, XFF is ignored so this is not spoofable.
	// Non-auth paths still key user:<userID> via AuthenticateFlexible.
	httpReq.RemoteAddr = pc.agentID + ":0"

	rec := httptest.NewRecorder()
	pc.hub.handler.ServeHTTP(rec, httpReq)

	var responseBody any
	bodyStr := rec.Body.String()
	if json.Valid([]byte(bodyStr)) {
		responseBody = json.RawMessage(bodyStr)
	} else {
		responseBody = bodyStr
	}

	pc.sendJSON(map[string]any{
		"type": "api_response",
		"id":   id,
		"data": map[string]any{
			"status": rec.Code,
			"body":   responseBody,
		},
	})
}

func (pc *PluginConn) sendJSON(v any) {
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	select {
	case pc.send <- data:
	default:
	}
}

func (pc *PluginConn) Send(data []byte) {
	select {
	case pc.send <- data:
	default:
	}
}

func (pc *PluginConn) SendRequest(method, path string, body []byte) (PluginResponse, error) {
	id := idgen.NewID()
	ch := make(chan PluginResponse, 1)

	pc.pendingMu.Lock()
	pc.pending[id] = ch
	pc.pendingMu.Unlock()

	defer func() {
		pc.pendingMu.Lock()
		delete(pc.pending, id)
		pc.pendingMu.Unlock()
	}()

	req := map[string]any{
		"type": "request",
		"id":   id,
		"data": map[string]any{
			"action": method,
			"path":   path,
		},
	}
	if body != nil {
		req["data"].(map[string]any)["body"] = json.RawMessage(body)
	}
	pc.sendJSON(req)

	select {
	case resp := <-ch:
		return resp, nil
	case <-time.After(10 * time.Second):
		return PluginResponse{}, context.DeadlineExceeded
	}
}

func (pc *PluginConn) resolveRequest(id string, status int, body []byte) {
	pc.pendingMu.Lock()
	ch, ok := pc.pending[id]
	pc.pendingMu.Unlock()
	if ok {
		select {
		case ch <- PluginResponse{Status: status, Body: body}:
		default:
		}
	}
}

func (pc *PluginConn) writePump(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-pc.done:
			return
		case msg, ok := <-pc.send:
			if !ok {
				return
			}
			pc.conn.Write(ctx, websocket.MessageText, msg)
		}
	}
}
