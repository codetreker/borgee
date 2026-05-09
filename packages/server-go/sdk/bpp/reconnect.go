// Package bpp (sdk/bpp) — reconnect.go: BPP-7 SDK Reconnect / ColdStart /
// HeartbeatLoop / GrantRetry implementations.
//
// These are the runtime methods that actually exercise BPP-3.2 / BPP-4 /
// BPP-5 / BPP-6 protocols from the plugin side.
//
// 原则 (跟 docs/_archive/qa/bpp-7-stance-checklist.md §3 + spec §0.3):
//
//   - Reconnect (BPP-5 #503) — socket dropped, plugin process alive,
//     SDK still holds last_known_cursor. Sends ReconnectHandshakeFrame
//     so server resumes via RT-1.3 ResolveResume incremental.
//   - ColdStart (BPP-6 #522) — process restarted, no cursor, no resume.
//     Sends ColdStartHandshakeFrame; reason is `reasons.RuntimeCrashed`
//     byte-identical (AL-1a 6-dict 守护链第 12 处, reasons 单一来源 #496).
//     字段集与 ReconnectHandshakeFrame 互斥反断 (BPP-6 spec §0.1 原则).
//   - HeartbeatLoop — 30s ticker per HeartbeatInterval const (跟 BPP-4
//     watchdog 周期 byte-identical).
//   - GrantRetry (BPP-3.2.3) — same MaxPermissionRetries = 3 + RetryBackoff
//     = 30s as server's RequestRetryCache (const re-used directly).
//
// Reverse 反向约束 (acceptance §2.4): AST scan forbidden tokens
// `pendingSDKReconnect / sdkRetryQueue / deadLetterSDK / runtime_recovered
// / sdk_specific_reason / 7th.*reason` 0 hit (best-effort 守护链延伸第 4 处).

package bpp

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"

	"borgee-server/internal/agent/reasons"
	srvbpp "borgee-server/internal/bpp"
)

// MaxPermissionRetries / RetryBackoff / HeartbeatInterval are pinned to
// server constants for byte-identical guarantee (acceptance §2.2 + §2.3).
// Re-export here so SDK consumers can construct retry-aware code paths
// without importing internal packages directly (godoc surface stays clean).
const (
	MaxPermissionRetries = srvbpp.MaxPermissionRetries
	RetryBackoff         = srvbpp.RetryBackoff
)

// Reconnect dials a fresh ws.Conn and sends a ReconnectHandshakeFrame
// carrying the SDK's last-known cursor. Server-side BPP-5 handler
// (`internal/bpp/reconnect_handler.go`) calls ResolveResume incremental.
//
// Use this when the previous Connect's ws.Conn was dropped but the
// plugin process is still alive. For full process restart (cursor lost),
// use ColdStart instead — the two frames are 字段集 互斥反断 by design
// (BPP-6 spec §0.1).
func (c *Client) Reconnect(ctx context.Context, url string) error {
	conn, _, err := websocket.Dial(ctx, url, &websocket.DialOptions{})
	if err != nil {
		return fmt.Errorf("sdk/bpp: reconnect dial: %w", err)
	}
	now := time.Now().UnixMilli()
	frame := srvbpp.ReconnectHandshakeFrame{
		Type:            srvbpp.FrameTypeBPPReconnectHandshake,
		PluginID:        c.PluginID,
		AgentID:         c.AgentID,
		LastKnownCursor: c.LastKnownCursor(),
		DisconnectAt:    now, // best-effort wall clock; server is authoritative
		ReconnectAt:     now,
	}
	if err := writeFrame(ctx, conn, frame); err != nil {
		_ = conn.Close(websocket.StatusInternalError, "reconnect frame send failed")
		return fmt.Errorf("sdk/bpp: reconnect send: %w", err)
	}
	c.mu.Lock()
	if c.conn != nil {
		_ = c.conn.Close(websocket.StatusNormalClosure, "")
	}
	c.conn = conn
	c.mu.Unlock()
	c.logger.Info("sdk.bpp.reconnect_sent",
		"plugin_id", c.PluginID, "agent_id", c.AgentID,
		"last_known_cursor", frame.LastKnownCursor)
	return nil
}

// ColdStart dials a fresh ws.Conn and sends a ColdStartHandshakeFrame.
// 原则 ② cold-start ≠ reconnect — frame 字段集互斥反断 (no cursor).
// reason is `reasons.RuntimeCrashed` byte-identical 跟 server BPP-6
// handler 同源 (AL-1a 守护链第 12 处). Resets SDK lastKnownCursor to 0
// since process restart drops in-memory state.
func (c *Client) ColdStart(ctx context.Context, url string) error {
	conn, _, err := websocket.Dial(ctx, url, &websocket.DialOptions{})
	if err != nil {
		return fmt.Errorf("sdk/bpp: cold-start dial: %w", err)
	}
	frame := srvbpp.ColdStartHandshakeFrame{
		Type:          srvbpp.FrameTypeBPPColdStartHandshake,
		PluginID:      c.PluginID,
		AgentID:       c.AgentID,
		RestartAt:     time.Now().UnixMilli(),
		RestartReason: reasons.RuntimeCrashed, // 6-dict byte-identical, 守护链第 12 处
	}
	if err := writeFrame(ctx, conn, frame); err != nil {
		_ = conn.Close(websocket.StatusInternalError, "cold-start frame send failed")
		return fmt.Errorf("sdk/bpp: cold-start send: %w", err)
	}
	c.mu.Lock()
	if c.conn != nil {
		_ = c.conn.Close(websocket.StatusNormalClosure, "")
	}
	c.conn = conn
	c.lastKnownCursor = 0 // fresh start, BPP-6 spec §0.2 原则一致
	c.mu.Unlock()
	c.logger.Info("sdk.bpp.cold_start_sent",
		"plugin_id", c.PluginID, "agent_id", c.AgentID,
		"restart_reason", frame.RestartReason)
	return nil
}

// HeartbeatLoop ticks every HeartbeatInterval and SendHeartbeat. Returns
// when ctx is canceled. Errors during a single tick are logged and the
// loop continues (best-effort 原则一致 BPP-4 §0.3).
func (c *Client) HeartbeatLoop(ctx context.Context) {
	tick := time.NewTicker(HeartbeatInterval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			if err := c.SendHeartbeat(ctx, "online", ""); err != nil {
				c.logger.Warn("sdk.bpp.heartbeat_send_failed",
					"plugin_id", c.PluginID, "error", err)
			}
		}
	}
}

// GrantRetry encapsulates the BPP-3.2.3 client-side retry loop. Caller
// supplies the request operation (returns nil on success, non-nil on
// transient failure). GrantRetry attempts up to MaxPermissionRetries
// times with RetryBackoff between attempts; returns nil on success or
// `ErrSDKGrantRetryExhausted` after the third failure.
//
// Best-effort 原则: no persistent queue, no exponential backoff, no
// dead-letter table — single in-memory counter. AST scan守门 forbidden
// `pendingSDKReconnect/sdkRetryQueue/deadLetterSDK` 0 hit.
func (c *Client) GrantRetry(ctx context.Context, op func(context.Context) error) error {
	var attempts int32
	var lastErr error
	for atomic.LoadInt32(&attempts) < int32(MaxPermissionRetries) {
		if err := op(ctx); err == nil {
			return nil
		} else {
			lastErr = err
			atomic.AddInt32(&attempts, 1)
			c.logger.Warn("sdk.bpp.grant_retry_attempt",
				"plugin_id", c.PluginID,
				"attempt", atomic.LoadInt32(&attempts),
				"error", err)
		}
		// Don't sleep after the final failure.
		if atomic.LoadInt32(&attempts) >= int32(MaxPermissionRetries) {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(RetryBackoff):
		}
	}
	return fmt.Errorf("%w: last error: %v", ErrSDKGrantRetryExhausted, lastErr)
}

// ErrSDKGrantRetryExhausted is returned by GrantRetry after
// MaxPermissionRetries attempts have all failed.
var ErrSDKGrantRetryExhausted = errors.New("sdk/bpp: grant retry exhausted (MaxPermissionRetries reached)")
