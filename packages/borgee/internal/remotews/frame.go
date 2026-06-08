// Package remotews implements the borgee daemon's reverse-WebSocket client:
// it dials the Borgee server's /ws/remote endpoint, keeps the connection
// alive with reconnect + heartbeat, and serves inbound ls/read/stat requests
// from the allowed directories via the fsops package.
//
// The wire contract is owned by the server (packages/server-go internal/ws +
// internal/api) and is read-only context here. Request frames are FLAT —
// the action and path live directly under data, with no params nesting.
package remotews

import "encoding/json"

// Frame is the single envelope used for BOTH encode and decode, so the
// marshal test asserts the exact bytes the daemon puts on the wire. Mirrors
// the inline struct on the server side (packages/server-go internal/ws/
// remote.go): type / id / data / error.
type Frame struct {
	Type  string          `json:"type"`
	ID    string          `json:"id,omitempty"`
	Data  json.RawMessage `json:"data,omitempty"`
	Error string          `json:"error,omitempty"`
}

// RequestData is the FLAT request payload (no params nesting). This is the
// load-bearing invariant: marshaled JSON carries data.action + data.path and
// NO params. It aligns literally with the server's flat {action,path} request
// builder so both ends assert the same wire shape.
type RequestData struct {
	Action string `json:"action"`
	Path   string `json:"path"`
}
