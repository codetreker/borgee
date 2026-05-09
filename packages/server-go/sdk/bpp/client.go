// Package bpp (sdk/bpp) — BPP-7 plugin SDK Go client.
//
// This package is the in-tree Go SDK for borgee plugin runtimes. It
// lives inside the borgee-server module so envelope schemas (`internal/bpp`)
// are shared by-import — no separate go.mod, no go.work overhead, byte-
// identical frame definitions guaranteed at compile time.
//
// 设计 (跟 spec brief docs/implementation/modules/bpp-7-spec.md §0 +
// stance docs/qa/bpp-7-stance-checklist.md §1+§2+§3 byte-identical):
//
//   - ① **frame schema 跟 server byte-identical** — SDK 不重定义任何
//     envelope struct, 必走 import "borgee-server/internal/bpp". reflect
//     drift 反断 + AST scan `type.*Frame.*struct` 在 sdk/bpp/ 0 hit.
//   - ② **SDK Go module + ws 库同源** — 在 borgee-server 同 module 内
//     (sdk/bpp/) + `github.com/coder/websocket` 跟 server 同源. AST scan
//     forbidden tokens (pendingSDKReconnect / sdkRetryQueue /
//     deadLetterSDK) 0 hit (best-effort 锁链延伸第 4 处 — BPP-4 + BPP-5
//     + BPP-6 + BPP-7).
//   - ③ **BPP-3.2.3 retry + BPP-4 watchdog + BPP-5/6 reconnect/cold-start
//     SDK side 真实施** — reason 复用 reasons SSOT 6-dict (#496 字面承袭,
//     AL-1a reason 锁链 BPP-7 = 第 12 处). SDK ColdStart 走
//     reasons.RuntimeCrashed byte-identical 跟 server BPP-6 handler.
//
// 反约束:
//   - admin god-mode 不挂 SDK 路径 (ADM-0 §1.3 红线)
//   - SDK 不另开 client-side dispatcher (server-only 边界, BPP-3 #489)

package bpp

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"

	srvbpp "borgee-server/internal/bpp"
)

// HeartbeatInterval — BPP-4 #499 watchdog 周期 byte-identical (server
// side stale threshold = 30s, SDK 主动发送周期匹配, 设计 ⑥). 改 = 改
// server watchdog + 设计 ⑥ stance + 此 SDK const 同步.
const HeartbeatInterval = 30 * time.Second

// Client is the BPP-7 plugin SDK client. One Client per (plugin
// process × server connection) — Reconnect / ColdStart are per-Client
// methods that swap the underlying ws.Conn but preserve cursor state
// (Reconnect) or reset it (ColdStart).
//
// Construct via NewClient + Connect (4 deps panic on nil — boot bug,
// 跟 server BPP-3/4/5/6 同模式 ctor pattern).
type Client struct {
	// PluginID identifies the plugin process to the server (BPP-1
	// connect handshake field).
	PluginID string
	// AgentID is the agent this Client is bound to (BPP-5 reconnect /
	// BPP-6 cold-start frame field). One Client = one agent in v1
	// (multi-agent multiplexing 留 v2).
	AgentID string

	logger *slog.Logger
	conn   *websocket.Conn
	mu     sync.Mutex

	// lastKnownCursor advances as the SDK receives data-plane frames
	// from the server. On Reconnect the SDK sends this in a
	// ReconnectHandshakeFrame so the server can resume via RT-1.3
	// ResolveResume (BPP-5 设计 ② 复用 RT-1.3, 不另起 sequence).
	lastKnownCursor int64
}

// NewClient constructs a SDK Client. logger may be nil (defaults to
// slog.Default). pluginID + agentID required (empty → panic boot bug).
func NewClient(pluginID, agentID string, logger *slog.Logger) *Client {
	if pluginID == "" {
		panic("sdk/bpp: NewClient pluginID must not be empty")
	}
	if agentID == "" {
		panic("sdk/bpp: NewClient agentID must not be empty")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Client{PluginID: pluginID, AgentID: agentID, logger: logger}
}

// Connect dials the server's BPP socket and sends a ConnectFrame
// (BPP-1 §2.1 control plane handshake — Type/PluginID/Token/Version/
// Capabilities 5 字段 byte-identical 跟 server srvbpp.ConnectFrame).
//
// On success the underlying ws.Conn is stored on the Client; callers
// then loop on ReadFrame / Send for data-plane traffic.
func (c *Client) Connect(ctx context.Context, url, token, version, capabilities string) error {
	conn, _, err := websocket.Dial(ctx, url, &websocket.DialOptions{
		HTTPClient: http.DefaultClient,
	})
	if err != nil {
		return err
	}
	frame := srvbpp.ConnectFrame{
		Type:         srvbpp.FrameTypeBPPConnect,
		PluginID:     c.PluginID,
		Token:        token,
		Version:      version,
		Capabilities: capabilities,
	}
	if err := writeFrame(ctx, conn, frame); err != nil {
		_ = conn.Close(websocket.StatusInternalError, "connect frame send failed")
		return err
	}
	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()
	c.logger.Info("sdk.bpp.connected",
		"plugin_id", c.PluginID, "agent_id", c.AgentID, "url", url)
	return nil
}

// Close terminates the ws.Conn cleanly.
func (c *Client) Close() error {
	c.mu.Lock()
	conn := c.conn
	c.conn = nil
	c.mu.Unlock()
	if conn == nil {
		return nil
	}
	return conn.Close(websocket.StatusNormalClosure, "")
}

// LastKnownCursor returns the highest cursor value the SDK has seen.
// Used by Reconnect to construct a ReconnectHandshakeFrame.
func (c *Client) LastKnownCursor() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastKnownCursor
}

// AdvanceCursor monotonically advances the SDK's last-known cursor.
// Callers invoke this when a data-plane frame with a cursor field
// arrives. Reverse-monotonic input is silently dropped (RT-1.3 单调
// 设计沿用).
func (c *Client) AdvanceCursor(cursor int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if cursor > c.lastKnownCursor {
		c.lastKnownCursor = cursor
	}
}

// errSDKConnClosed — used internally when callers Send before Connect.
var errSDKConnClosed = errors.New("sdk/bpp: client not connected (call Connect first)")

// writeFrame is the SDK's single send path. Marshals any envelope to
// JSON and writes a binary ws message. Reserved private to keep the
// Send surface small (callers go through typed Send* helpers).
func writeFrame(ctx context.Context, conn *websocket.Conn, frame any) error {
	b, err := json.Marshal(frame)
	if err != nil {
		return err
	}
	return conn.Write(ctx, websocket.MessageText, b)
}

// SendHeartbeat writes a HeartbeatFrame (BPP-4 30s ticker pairs with
// server watchdog 30s threshold byte-identical, HeartbeatInterval const).
// Status defaults to "online" when status is empty; reason is empty
// for the online state per AL-1a. Server-side watchdog records the
// timestamp under server's clock — SDK only sets it for diagnostics.
func (c *Client) SendHeartbeat(ctx context.Context, status, reason string) error {
	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()
	if conn == nil {
		return errSDKConnClosed
	}
	if status == "" {
		status = "online"
	}
	return writeFrame(ctx, conn, srvbpp.HeartbeatFrame{
		Type:      srvbpp.FrameTypeBPPHeartbeat,
		PluginID:  c.PluginID,
		AgentID:   c.AgentID,
		Status:    status,
		Reason:    reason,
		Timestamp: time.Now().UnixMilli(),
	})
}
