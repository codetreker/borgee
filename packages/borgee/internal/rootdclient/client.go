//go:build linux || darwin

// Package rootdclient — typed client for the `borgee rootd` companion
// daemon's UDS IPC. The main daemon (`borgee daemon`, User=borgee) uses
// this to forward root-requiring jobs to rootd (User=root).
//
// PR-1 ships only a Ping() method that round-trips the placeholder ping
// command rootd's whitelist contains. PR-4 adds typed methods for
// install_plugin / service_lifecycle / delegation_revoke; no daemon code
// calls this client yet.
//
// Wire protocol matches internal/cli/rootd.Server: line-delimited JSON,
// one request per connection, the connection closes after the response.
package rootdclient

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"runtime"
	"time"
)

const (
	DefaultSocketLinux  = "/run/borgee/borgee-rootd.sock"
	DefaultSocketDarwin = "/Users/Shared/Borgee/borgee-rootd.sock"
)

// DefaultSocket returns the canonical UDS path baked into the production
// borgee-rootd.service unit / launchd plist for the running platform.
func DefaultSocket() string {
	if runtime.GOOS == "darwin" {
		return DefaultSocketDarwin
	}
	return DefaultSocketLinux
}

// Client is the typed RPC client. Zero value is unusable; callers must
// set SocketPath (use DefaultSocket() in production).
type Client struct {
	SocketPath  string
	DialTimeout time.Duration
}

// request and response mirror rootd.Server's wire types. We keep them
// internal so callers of Client never hand-craft a payload.
type request struct {
	Cmd       string          `json:"cmd"`
	RequestID string          `json:"request_id"`
	Params    json.RawMessage `json:"params,omitempty"`
}

type response struct {
	RequestID string          `json:"request_id"`
	OK        bool            `json:"ok"`
	Result    json.RawMessage `json:"result,omitempty"`
	Error     string          `json:"error,omitempty"`
}

// Ping is the PR-1 smoke command. Returns the decoded result map on
// success (`{"pong": true, "time": <unix ms>}`). Real ops (InstallPlugin,
// ServiceLifecycle, DelegationRevoke) land in PR-4 as additional typed
// methods on Client.
func (c *Client) Ping(ctx context.Context) (map[string]any, error) {
	raw, err := c.call(ctx, "ping", "ping-"+nowMS(), nil)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("rootdclient: ping decode result: %w", err)
	}
	return out, nil
}

// call performs a single round-trip. Internal so callers can only invoke
// the typed methods on Client (which is the whole point of a narrow
// client API).
func (c *Client) call(ctx context.Context, cmd, requestID string, params json.RawMessage) (json.RawMessage, error) {
	if c.SocketPath == "" {
		return nil, errors.New("rootdclient: SocketPath is empty")
	}
	d := net.Dialer{Timeout: c.dialTimeout()}
	conn, err := d.DialContext(ctx, "unix", c.SocketPath)
	if err != nil {
		return nil, fmt.Errorf("rootdclient: dial %s: %w", c.SocketPath, err)
	}
	defer conn.Close()

	// Honor ctx deadline + a sane default to avoid hanging if rootd is
	// stuck — the per-cmd handlers are supposed to be fast (no blocking
	// IO).
	deadline, hasDeadline := ctx.Deadline()
	if !hasDeadline {
		deadline = time.Now().Add(5 * time.Second)
	}
	_ = conn.SetDeadline(deadline)

	if err := json.NewEncoder(conn).Encode(request{
		Cmd:       cmd,
		RequestID: requestID,
		Params:    params,
	}); err != nil {
		return nil, fmt.Errorf("rootdclient: write req: %w", err)
	}

	line, err := bufio.NewReader(conn).ReadBytes('\n')
	if err != nil {
		return nil, fmt.Errorf("rootdclient: read resp: %w", err)
	}
	var resp response
	if err := json.Unmarshal(line, &resp); err != nil {
		return nil, fmt.Errorf("rootdclient: decode resp: %w", err)
	}
	if !resp.OK {
		if resp.Error == "" {
			resp.Error = "unspecified"
		}
		return nil, fmt.Errorf("rootdclient: rootd rejected cmd=%q: %s", cmd, resp.Error)
	}
	return resp.Result, nil
}

func (c *Client) dialTimeout() time.Duration {
	if c.DialTimeout > 0 {
		return c.DialTimeout
	}
	return 2 * time.Second
}

func nowMS() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
