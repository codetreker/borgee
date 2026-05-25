//go:build linux || darwin

// Package rootdclient — typed client for the `borgee rootd` companion
// daemon's UDS IPC. The main daemon (`borgee daemon`, User=borgee) uses
// this to forward root-requiring jobs to rootd (User=root).
//
// PR-1 shipped the Ping() smoke method. PR-4 (#1033) added the three
// typed root-command methods (InstallPlugin, ServiceLifecycle,
// DelegationRevoke) used by the install_plugin / service.lifecycle /
// delegation.revoke executors.
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

// Ping is the smoke command. Returns the decoded result map on success
// (`{"pong": true, "time": <unix ms>}`).
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

// InstallPluginRequest mirrors rootd.InstallPluginParams so callers
// never hand-craft the params map. Kept as a duplicate type so this
// package does not import the rootd package directly (the rootd package
// only builds on linux+darwin; the client should compile everywhere
// these executors do — same constraint).
type InstallPluginRequest struct {
	ManifestURL           string `json:"manifest_url"`
	PubKeyBase64          string `json:"pubkey_base64"`
	PluginID              string `json:"plugin_id"`
	TargetPath            string `json:"target_path"`
	HelperUser            string `json:"helper_user,omitempty"`
	HelperGroup           string `json:"helper_group,omitempty"`
	DryRun                bool   `json:"dry_run,omitempty"`
	AllowInsecureManifest bool   `json:"allow_insecure_manifest,omitempty"`
}

// InstallPluginResponse mirrors rootd.InstallPluginResult.
type InstallPluginResponse struct {
	Installed     bool   `json:"installed"`
	TargetPath    string `json:"target_path"`
	StdoutSummary string `json:"stdout_summary,omitempty"`
	StderrSummary string `json:"stderr_summary,omitempty"`
}

// InstallPlugin asks rootd to fetch + verify + place a signed plugin
// binary. The daemon-side executor builds opts from the leased job's
// payload + manifest binding.
func (c *Client) InstallPlugin(ctx context.Context, opts InstallPluginRequest) (*InstallPluginResponse, error) {
	params, err := json.Marshal(opts)
	if err != nil {
		return nil, fmt.Errorf("rootdclient: install_plugin marshal: %w", err)
	}
	raw, err := c.call(ctx, "install_plugin", "install-"+nowMS(), params)
	if err != nil {
		return nil, err
	}
	var out InstallPluginResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("rootdclient: install_plugin decode result: %w", err)
	}
	return &out, nil
}

// ServiceLifecycleRequest mirrors rootd.ServiceLifecycleParams.
type ServiceLifecycleRequest struct {
	Manager   string `json:"manager"`
	Unit      string `json:"unit"`
	Operation string `json:"operation"`
}

// ServiceLifecycleResponse mirrors rootd.ServiceLifecycleResult.
type ServiceLifecycleResponse struct {
	ExitCode int    `json:"exit_code"`
	Stdout   string `json:"stdout,omitempty"`
	Stderr   string `json:"stderr,omitempty"`
}

// ServiceLifecycle asks rootd to exec systemctl / launchctl against
// the (manager, unit, operation) triple the daemon executor resolved
// from the signed manifest's ServiceDeclaration. Unit names never come
// from operator free-form strings — only from server-signed manifest.
func (c *Client) ServiceLifecycle(ctx context.Context, opts ServiceLifecycleRequest) (*ServiceLifecycleResponse, error) {
	params, err := json.Marshal(opts)
	if err != nil {
		return nil, fmt.Errorf("rootdclient: service_lifecycle marshal: %w", err)
	}
	raw, err := c.call(ctx, "service_lifecycle", "svc-"+nowMS(), params)
	if err != nil {
		return nil, err
	}
	var out ServiceLifecycleResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("rootdclient: service_lifecycle decode result: %w", err)
	}
	return &out, nil
}

// DelegationRevokeRequest mirrors rootd.DelegationRevokeParams.
type DelegationRevokeRequest struct {
	EnrollmentID        string   `json:"enrollment_id"`
	DrainTimeoutSeconds int      `json:"drain_timeout_seconds,omitempty"`
	ServiceName         string   `json:"service_name,omitempty"`
	ServiceManager      string   `json:"service_manager,omitempty"`
	CredentialPaths     []string `json:"credential_paths,omitempty"`
}

// DelegationRevokeResponse mirrors rootd.DelegationRevokeResult.
type DelegationRevokeResponse struct {
	Disabled        bool     `json:"disabled"`
	CredentialWiped bool     `json:"credential_wiped"`
	WipedPaths      []string `json:"wiped_paths,omitempty"`
}

// DelegationRevoke asks rootd to disable borgee.service + wipe
// credential files. The daemon executor calls this AFTER draining its
// dispatcher; the daemon process then exits gracefully so the WS Result
// frame can carry the terminal status back to the server before SIGTERM.
func (c *Client) DelegationRevoke(ctx context.Context, opts DelegationRevokeRequest) (*DelegationRevokeResponse, error) {
	params, err := json.Marshal(opts)
	if err != nil {
		return nil, fmt.Errorf("rootdclient: delegation_revoke marshal: %w", err)
	}
	raw, err := c.call(ctx, "delegation_revoke", "revoke-"+nowMS(), params)
	if err != nil {
		return nil, err
	}
	var out DelegationRevokeResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("rootdclient: delegation_revoke decode result: %w", err)
	}
	return &out, nil
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
