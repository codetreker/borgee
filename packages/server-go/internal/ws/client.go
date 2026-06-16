package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"borgee-server/internal/auth"
	"borgee-server/internal/store"

	"github.com/coder/websocket"

	"borgee-server/internal/idgen"
)

const (
	sendBufSize = 256
)

type Client struct {
	hub    *Hub
	conn   *websocket.Conn
	userID string
	user   *store.User
	// sessionID is the per-connection UUID that AL-3.2 keys the
	// presence_sessions row by. UNIQUE-constrained server-side; the
	// hub's TrackOnline / TrackOffline write end consumes it as the
	// row identity for last-wins offline accounting.
	sessionID string
	// agentID is non-nil iff the connecting user has role="agent" —
	// the partial index `idx_presence_sessions_agent_id` (#310) covers
	// this column so DM-2.2 fallback's IsOnline(agent.id) OR-query
	// path resolves. nil for human sessions.
	agentID *string
	send    chan []byte
	done    chan struct{}

	subscribedMu sync.RWMutex
	subscribed   map[string]bool

	aliveMu sync.Mutex
	alive   bool
}

func newClient(hub *Hub, conn *websocket.Conn, user *store.User) *Client {
	// Agent sessions write `users.id` into both user_id (owner key) and
	// agent_id (partial-index key) so DM-2.2 mention fallback's OR
	// query (`WHERE user_id = ? OR agent_id = ?`) hits the right row
	// regardless of which key the caller passes. concept-model §1.1:
	// agents are users with role="agent".
	var agentID *string
	if user != nil && user.Role == "agent" {
		id := user.ID
		agentID = &id
	}
	return &Client{
		hub:        hub,
		conn:       conn,
		userID:     user.ID,
		user:       user,
		sessionID:  idgen.NewID(),
		agentID:    agentID,
		send:       make(chan []byte, sendBufSize),
		done:       make(chan struct{}),
		subscribed: make(map[string]bool),
		alive:      true,
	}
}

func (c *Client) Send(data []byte) {
	select {
	case c.send <- data:
	default:
	}
}

func (c *Client) SendReliable(data []byte) {
	select {
	case c.send <- data:
	case <-c.done:
	case <-time.After(250 * time.Millisecond):
	}
}

func (c *Client) SendJSON(v any) {
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	c.Send(data)
}

func (c *Client) SendPing() {
	c.SendJSON(map[string]string{"type": "ping"})
}

func (c *Client) IsSubscribed(channelID string) bool {
	c.subscribedMu.RLock()
	defer c.subscribedMu.RUnlock()
	return c.subscribed[channelID]
}

func (c *Client) Subscribe(channelID string) {
	c.subscribedMu.Lock()
	defer c.subscribedMu.Unlock()
	c.subscribed[channelID] = true
}

func (c *Client) Unsubscribe(channelID string) {
	c.subscribedMu.Lock()
	defer c.subscribedMu.Unlock()
	delete(c.subscribed, channelID)
}

func (c *Client) CheckAlive() bool {
	c.aliveMu.Lock()
	defer c.aliveMu.Unlock()
	if !c.alive {
		return false
	}
	c.alive = false
	return true
}

func (c *Client) setAlive() {
	c.aliveMu.Lock()
	defer c.aliveMu.Unlock()
	c.alive = true
}

func (c *Client) Close() {
	// TEST-FIX-3-COV: nil-safe guard — production paths always set conn via
	// newClient (real websocket from upgrader); test paths constructing
	// bare *Client{send,done} (e.g. ws_internal_test.go +
	// hub_heartbeat_test.go) leave conn==nil. Heartbeat dead-branch async
	// Close goroutine panics on nil conn → test process kill. Guard
	// preserves byte-identical production behavior (real conn always
	// non-nil) while letting unit tests exercise the close-on-dead path.
	if c.conn != nil {
		c.conn.Close(websocket.StatusNormalClosure, "closing")
	}
	select {
	case <-c.done:
	default:
		close(c.done)
	}
}

func (c *Client) writePump(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-c.done:
			return
		case msg, ok := <-c.send:
			if !ok {
				return
			}
			c.conn.Write(ctx, websocket.MessageText, msg)
		}
	}
}

var builtinCommandNames = map[string]bool{
	"help": true, "leave": true, "topic": true, "invite": true,
	"dm": true, "status": true, "clear": true, "nick": true,
}

var commandNameRe = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)

type wsMessage struct {
	Type            string          `json:"type"`
	ChannelID       string          `json:"channel_id,omitempty"`
	Content         string          `json:"content,omitempty"`
	ContentType     string          `json:"content_type,omitempty"`
	ClientID        string          `json:"client_id,omitempty"`
	ClientMessageID string          `json:"client_message_id,omitempty"`
	ReplyToID       string          `json:"reply_to_id,omitempty"`
	Mentions        []string        `json:"mentions,omitempty"`
	Commands        json.RawMessage `json:"commands,omitempty"`
}

func HandleClient(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := authenticateWS(hub, r)
		if user == nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		conn, err := websocket.Accept(w, r, wsAcceptOptions(hub.config))
		if err != nil {
			hub.logger.Error("ws accept failed", "error", err)
			return
		}

		client := newClient(hub, conn, user)
		hub.Register(client)

		hub.store.UpdateLastSeen(user.ID)

		hub.BroadcastToAll(map[string]any{
			"type":    "presence",
			"user_id": user.ID,
			"status":  "online",
		})

		ctx := r.Context()
		go client.writePump(ctx)

		defer func() {
			hub.Unregister(client)
			hub.cmdStore.UnregisterByConnection(fmt.Sprintf("%p", client))
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

			var msg wsMessage
			if err := json.Unmarshal(data, &msg); err != nil {
				continue
			}

			client.setAlive()

			switch msg.Type {
			case "ping":
				client.SendJSON(map[string]string{"type": "pong"})
			case "pong":
				// alive already set
			case "subscribe":
				handleSubscribe(client, msg)
			case "unsubscribe":
				handleUnsubscribe(client, msg)
			case "typing":
				handleTyping(client, msg)
			case "send_message":
				handleSendMessage(client, msg)
			case "register_commands":
				handleRegisterCommands(client, msg)
			}
		}
	}
}

func authenticateWS(hub *Hub, r *http.Request) *store.User {
	if proto := r.Header.Get("Sec-WebSocket-Protocol"); strings.HasPrefix(proto, "Bearer,") {
		apiKey := strings.TrimSpace(strings.TrimPrefix(proto, "Bearer,"))
		if user, err := hub.store.GetUserByAPIKey(apiKey); err == nil && user.DeletedAt == nil && !user.Disabled {
			return user
		}
	}

	if authHeader := r.Header.Get("Authorization"); strings.HasPrefix(authHeader, "Bearer ") {
		apiKey := strings.TrimPrefix(authHeader, "Bearer ")
		if user, err := hub.store.GetUserByAPIKey(apiKey); err == nil && user.DeletedAt == nil && !user.Disabled {
			return user
		}
	}

	// WS-auth-unify: the `?token=` apiKey query form on /ws is removed.
	// Production browsers authenticate with the session cookie (below);
	// process clients (Go/Node agents) use the Authorization / Sec-WebSocket-
	// Protocol Bearer header above. Unlike plugin `?apiKey` / remote `?token`,
	// /ws had no npx-distributed old-binary consumer of the query form, so it
	// is dropped outright rather than deprecated.

	if cookie, err := r.Cookie(auth.CookieName); err == nil {
		if user := auth.ValidateJWT(hub.store, hub.config.JWTSecret, cookie.Value); user != nil {
			return user
		}
	}

	if hub.config.IsDevelopment() && hub.config.DevAuthBypass {
		if userID := r.URL.Query().Get("user_id"); userID != "" {
			if user, err := hub.store.GetUserByID(userID); err == nil {
				return user
			}
		}
		if devUserID := r.Header.Get("X-Dev-User-Id"); devUserID != "" {
			if user, err := hub.store.GetUserByID(devUserID); err == nil {
				return user
			}
		}
		users, err := hub.store.ListUsers()
		if err == nil {
			for i := range users {
				// ADM-0.3: pick first member (users.role enum collapsed).
				if users[i].Role == "member" {
					return &users[i]
				}
			}
		}
	}

	return nil
}

func handleSubscribe(c *Client, msg wsMessage) {
	ch, err := c.hub.store.GetChannelByID(msg.ChannelID)
	if err != nil {
		c.SendJSON(map[string]any{"type": "error", "code": "NOT_FOUND", "message": "Channel not found"})
		return
	}
	if ch.Visibility == "private" && !c.hub.store.IsChannelMember(msg.ChannelID, c.userID) {
		c.SendJSON(map[string]any{"type": "error", "code": "NOT_MEMBER", "message": "Not a member"})
		return
	}
	c.Subscribe(msg.ChannelID)
	c.SendJSON(map[string]any{"type": "subscribed", "channel_id": msg.ChannelID})
}

func handleUnsubscribe(c *Client, msg wsMessage) {
	c.Unsubscribe(msg.ChannelID)
	c.SendJSON(map[string]any{"type": "unsubscribed", "channel_id": msg.ChannelID})
}

func handleTyping(c *Client, msg wsMessage) {
	c.hub.BroadcastToChannel(msg.ChannelID, map[string]any{
		"type":         "typing",
		"channel_id":   msg.ChannelID,
		"user_id":      c.userID,
		"display_name": c.user.DisplayName,
	}, c)
}

func handleSendMessage(c *Client, msg wsMessage) {
	clientMsgID := msg.ClientMessageID
	if clientMsgID == "" {
		clientMsgID = msg.ClientID
	}

	nack := func(code, message string) {
		c.SendJSON(map[string]any{
			"type":              "message_nack",
			"client_id":         clientMsgID,
			"client_message_id": clientMsgID,
			"code":              code,
			"message":           message,
		})
	}

	content := strings.TrimSpace(msg.Content)
	if content == "" {
		nack("INVALID_CONTENT_TYPE", "Empty message")
		return
	}

	ch, err := c.hub.store.GetChannelByID(msg.ChannelID)
	if err != nil {
		nack("NOT_FOUND", "Channel not found")
		return
	}

	if ch.Visibility == "private" && !c.hub.store.CanAccessChannel(msg.ChannelID, c.userID) {
		nack("NOT_FOUND", "Channel not found")
		return
	}

	if !c.hub.store.IsChannelMember(msg.ChannelID, c.userID) {
		nack("NOT_MEMBER", "Not a member of this channel")
		return
	}

	ct := msg.ContentType
	if ct == "" {
		ct = "text"
		if strings.HasPrefix(content, "/") {
			ct = "command"
		}
	}
	if ct != "text" && ct != "image" && ct != "command" {
		nack("INVALID_CONTENT_TYPE", "content_type must be 'text', 'image', or 'command'")
		return
	}

	// borgee #1108 F5: image content must be an http(s) URL or a same-origin
	// relative path — reject javascript:/data:/protocol-relative at write time
	// so neither rail stores a render-time phishing/inert-anchor vector. Same
	// allowlist as the REST rail (api/messages.go) and the client guard.
	if ct == "image" && !store.IsAllowedImageContentURL(content) {
		nack("INVALID_CONTENT", "image content must be an http(s) URL or same-origin path")
		return
	}

	if ct == "command" {
		var cmdCheck struct {
			Command string `json:"command"`
			Params  any    `json:"params"`
		}
		if !strings.HasPrefix(content, "/") {
			if err := json.Unmarshal([]byte(content), &cmdCheck); err != nil {
				nack("INVALID_CONTENT_TYPE", "Command content_type requires valid JSON with command/params or /prefix")
				return
			}
		}
	}

	var replyTo *string
	if msg.ReplyToID != "" {
		replyTo = &msg.ReplyToID
	}

	created, err := c.hub.store.CreateMessageFull(msg.ChannelID, c.userID, content, ct, replyTo, msg.Mentions)
	if err != nil {
		c.hub.logger.Error("ws message create failed", "error", err)
		nack("INTERNAL_ERROR", "Failed to create message")
		return
	}

	c.SendJSON(map[string]any{
		"type":              "message_ack",
		"client_id":         clientMsgID,
		"client_message_id": clientMsgID,
		"message_id":        created.ID,
		"message":           created,
	})

	payload := mustJSON(map[string]any{
		"message": created,
	})

	c.hub.store.CreateEvent(&store.Event{
		Kind:      "new_message",
		ChannelID: msg.ChannelID,
		Payload:   payload,
	})

	c.hub.SignalNewEvents()

	c.hub.BroadcastToChannel(msg.ChannelID, map[string]any{
		"type":    "new_message",
		"message": created,
	}, nil)
}

func handleRegisterCommands(c *Client, msg wsMessage) {
	if c.user.Role != "agent" {
		c.SendJSON(map[string]any{"type": "error", "code": "FORBIDDEN", "message": "Only agents can register commands"})
		return
	}

	var cmds []AgentCommand
	if err := json.Unmarshal(msg.Commands, &cmds); err != nil {
		c.SendJSON(map[string]any{"type": "error", "message": "Invalid commands payload"})
		return
	}

	var registered []string
	var skipped []string
	var valid []AgentCommand

	for _, cmd := range cmds {
		if !commandNameRe.MatchString(cmd.Name) {
			skipped = append(skipped, cmd.Name)
			continue
		}
		if builtinCommandNames[cmd.Name] {
			skipped = append(skipped, cmd.Name)
			continue
		}
		if len(cmd.Description) > 200 {
			skipped = append(skipped, cmd.Name)
			continue
		}
		if len(cmd.Params) > 0 {
			paramsJSON, _ := json.Marshal(cmd.Params)
			if len(paramsJSON) > 16384 {
				skipped = append(skipped, cmd.Name)
				continue
			}
		}
		valid = append(valid, cmd)
		registered = append(registered, cmd.Name)
	}

	connID := fmt.Sprintf("%p", c)
	c.hub.cmdStore.Register(connID, c.userID, c.user.DisplayName, valid)

	c.SendJSON(map[string]any{"type": "commands_registered", "registered": registered, "skipped": skipped})

	c.hub.BroadcastToAll(map[string]any{"type": "commands_updated"})
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func newID() string {
	return idgen.NewID()
}

func nowMs() int64 {
	return time.Now().UnixMilli()
}

// exported for auth reuse
func AuthenticateWS(hub *Hub, r *http.Request) *store.User {
	return authenticateWS(hub, r)
}
