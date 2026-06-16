package ws

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"

	"borgee-server/internal/idgen"
)

type RemoteConn struct {
	hub    *Hub
	conn   *websocket.Conn
	nodeID string
	userID string
	send   chan []byte
	done   chan struct{}
	alive  bool

	pendingMu sync.Mutex
	pending   map[string]chan json.RawMessage
}

func HandleRemote(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// #1031 concern-2: remote WS auth is `Authorization: Bearer <token>`
		// ONLY. The deprecated `?token=` query form was removed for the same
		// reason as the plugin rail — a token in the URL leaks into access logs
		// / proxies / referrers. The in-repo remote client (remotews/client.go,
		// #1112) dials with the header, so the query fallback was dead code.
		token := ""
		if authHeader := r.Header.Get("Authorization"); strings.HasPrefix(authHeader, "Bearer ") {
			token = strings.TrimPrefix(authHeader, "Bearer ")
		}
		if token == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		node, err := hub.store.GetRemoteNodeByToken(token)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		hub.store.UpdateRemoteNodeLastSeen(node.ID)

		conn, err := websocket.Accept(w, r, wsAcceptOptions(hub.config))
		if err != nil {
			hub.logger.Error("remote ws accept failed", "error", err)
			return
		}

		rc := &RemoteConn{
			hub:     hub,
			conn:    conn,
			nodeID:  node.ID,
			userID:  node.UserID,
			send:    make(chan []byte, sendBufSize),
			done:    make(chan struct{}),
			alive:   true,
			pending: make(map[string]chan json.RawMessage),
		}

		hub.RegisterRemote(node.ID, rc)

		ctx := r.Context()
		go rc.writePump(ctx)

		defer func() {
			hub.UnregisterRemote(node.ID)
			conn.Close(websocket.StatusNormalClosure, "")
		}()

		for {
			_, data, err := conn.Read(ctx)
			if err != nil {
				return
			}

			var msg struct {
				Type  string          `json:"type"`
				ID    string          `json:"id,omitempty"`
				Data  json.RawMessage `json:"data,omitempty"`
				Error string          `json:"error,omitempty"`
			}
			if err := json.Unmarshal(data, &msg); err != nil {
				continue
			}

			rc.alive = true

			switch msg.Type {
			case "ping":
				rc.sendJSON(map[string]string{"type": "pong"})
			case "pong":
				// alive already set
			case "response":
				rc.resolveRequest(msg.ID, msg.Data)
			}
		}
	}
}

func (rc *RemoteConn) SendRequest(data any) (json.RawMessage, error) {
	id := idgen.NewID()
	ch := make(chan json.RawMessage, 1)

	rc.pendingMu.Lock()
	rc.pending[id] = ch
	rc.pendingMu.Unlock()

	defer func() {
		rc.pendingMu.Lock()
		delete(rc.pending, id)
		rc.pendingMu.Unlock()
	}()

	payload, _ := json.Marshal(data)
	rc.sendJSON(map[string]any{
		"type": "request",
		"id":   id,
		"data": json.RawMessage(payload),
	})

	select {
	case resp := <-ch:
		return resp, nil
	case <-time.After(10 * time.Second):
		return nil, context.DeadlineExceeded
	}
}

func (rc *RemoteConn) resolveRequest(id string, data json.RawMessage) {
	rc.pendingMu.Lock()
	ch, ok := rc.pending[id]
	rc.pendingMu.Unlock()
	if ok {
		select {
		case ch <- data:
		default:
		}
	}
}

func (rc *RemoteConn) sendJSON(v any) {
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	select {
	case rc.send <- data:
	default:
	}
}

func (rc *RemoteConn) Send(data []byte) {
	select {
	case rc.send <- data:
	default:
	}
}

func (rc *RemoteConn) writePump(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-rc.done:
			return
		case msg, ok := <-rc.send:
			if !ok {
				return
			}
			rc.conn.Write(ctx, websocket.MessageText, msg)
		}
	}
}
