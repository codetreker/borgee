// Package bpp — plugin_frame_dispatcher.go: BPP-3 unified plugin-upstream
// BPP frame dispatcher boundary.
//
// Blueprint reference: docs/blueprint/current/plugin-protocol.md §2.2 (Plugin → Server data
// plane) — BPP frames carry `{type, ...payload-direct-fields}`, distinct
// from the RPC envelope `{type, id, data}` used by api_request/
// api_response.
//
// Spec: BPP-3 plugin connection lifecycle + unified frame dispatcher
// boundary. This completes the ws/plugin.go read-loop path for AL-2b ack
// ingress, which was previously deferred to the BPP-3 plugin implementation.
//
// Why this file exists:
//
//   - Before BPP-3, the plugin read loop in `internal/ws/plugin.go`
//     handled only the `{type, id, data}` RPC envelope (api_request /
//     api_response / ping / pong / response). Any other `type` was
//     silently dropped.
//   - BPP-1 #304 defined 9 envelope frames with shape
//     `{type, ...payload-direct-fields}` (no `id` / `data` wrappers).
//     AL-2b #481 added `agent_config_ack` frame (FrameTypeBPPAgentConfigAck)
//     but couldn't wire its ingress because there was no shared
//     dispatcher boundary in the plugin read loop.
//   - BPP-3 ships PluginFrameDispatcher: a thin router that takes a raw
//     wire `{type: ...}` payload, peeks `type`, and routes to the
//     per-frame dispatcher (currently AL-2b ack only; future: BPP-2
//     task lifecycle frames will register here).
//
// Boundary stance (BPP-1 envelope contract):
//
//   - This file does NOT touch `{type, id, data}` RPC frames — those
//     stay in plugin.go (api_request / api_response / ping / pong /
//     response). RPC envelope is request-reply, BPP envelope is
//     fire-and-forget event stream; mixing them in one dispatcher
//     would break the direction lock invariant (frame_schemas_test.go
//     reflection lint).
//   - This file does NOT import internal/api or internal/ws — bpp
//     package owns the boundary; ws/plugin.go calls into it via a
//     thin facade. Same pattern as BPP-2.1 ActionHandler.
//   - Each frame type has a single registered dispatcher (no broadcast).
//     Unknown frame type → log warn + skip (forward-compat: plugin may
//     send frames the server doesn't yet understand; reject would break
//     plugin upgrade rollouts).
//
// Negative constraints:
//
//   - No schema/migration changes (BPP-3 is wire-routing only).
//   - No new frame types added here — frames live in envelope.go,
//     dispatchers register on the boundary.
//   - Direction lock: only DirectionPluginToServer frames may
//     register. Attempt to register a server→plugin frame panics at
//     boot (defense-in-depth, prevents silent direction drift).

package bpp

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
)

// FrameDispatcher is the per-frame-type ingress handler. Each BPP frame
// `type` (FrameTypeBPP*) registers exactly one dispatcher. The router
// peeks `type` from the raw wire payload, looks up the dispatcher, and
// delegates Dispatch(rawJSON, sess).
//
// Implementations decode the raw JSON into their concrete frame struct
// (e.g. AgentConfigAckFrame) and invoke per-frame validation +
// downstream handler. Returning a non-nil error logs warn but does not
// drop the connection (acceptance §3.2 — fail-soft on plugin frame
// validation failures, drop only on protocol-level violations).
type FrameDispatcher interface {
	// Dispatch decodes raw JSON into the dispatcher's frame type and
	// processes it under the session context. Returns wrapped sentinel
	// errors so callers can errors.Is to map to wire-level error codes
	// using the same pattern as BPP-2.1 Dispatch and BPP-2.2
	// ValidateTaskFinished.
	Dispatch(raw json.RawMessage, sess PluginSessionContext) error
}

// PluginSessionContext is the per-connection context passed to every
// frame dispatcher. Carries the authenticated owner UUID (from BPP-1
// connect handshake) so per-frame dispatchers can run cross-owner ACL.
// This is byte-identical with AckSessionContext for the shared field, with a
// broader scope.
//
// Auth stance: OwnerUserID is set ONCE at connection accept (plugin.go
// hub.RegisterPlugin time, after API key auth) and never mutates. A
// dispatcher receiving sess with empty OwnerUserID is a server boot
// bug (panics defensively in Dispatch).
type PluginSessionContext struct {
	OwnerUserID string // resolved via BPP-1 connect handshake
}

// PluginFrameDispatcher routes plugin-upstream BPP frames to per-type
// FrameDispatcher implementations. Construct via NewPluginFrameDispatcher,
// register frames via Register, route via Route.
//
// Thread-safe for concurrent Route calls (RWMutex on the registry).
// Register is intended to be called once at server boot (via wire-up
// in server.go) — concurrent Register is allowed but wasteful.
type PluginFrameDispatcher struct {
	mu       sync.RWMutex
	registry map[string]FrameDispatcher
	logger   *slog.Logger
}

// NewPluginFrameDispatcher creates an empty dispatcher with the given
// logger. logger may be nil (defaults to discard, useful in tests).
func NewPluginFrameDispatcher(logger *slog.Logger) *PluginFrameDispatcher {
	return &PluginFrameDispatcher{
		registry: make(map[string]FrameDispatcher),
		logger:   logger,
	}
}

// Register binds a FrameDispatcher to a BPP frame type. Panics on:
//   - empty frameType (boot bug)
//   - duplicate registration of same type (boot bug — only one
//     dispatcher per frame type)
//   - frameType is not in the BPP envelope whitelist (forces caller to
//     define the frame in envelope.go first, prevents typo'd routes)
//
// Direction lock: only DirectionPluginToServer frames may be
// registered. Attempting to register a server→plugin frame panics
// (defense-in-depth: the plugin doesn't *send* server→plugin frames,
// so registering one here is a definitional bug).
func (d *PluginFrameDispatcher) Register(frameType string, fd FrameDispatcher) {
	if frameType == "" {
		panic("bpp: PluginFrameDispatcher.Register frameType must not be empty")
	}
	if fd == nil {
		panic("bpp: PluginFrameDispatcher.Register dispatcher must not be nil")
	}
	// Validate against envelope whitelist + direction lock.
	if !isPluginToServerFrame(frameType) {
		panic(fmt.Sprintf("bpp: PluginFrameDispatcher.Register %q is not a Plugin→Server BPP frame (envelope.go whitelist + DirectionPluginToServer required)",
			frameType))
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, exists := d.registry[frameType]; exists {
		panic(fmt.Sprintf("bpp: PluginFrameDispatcher.Register duplicate frameType %q", frameType))
	}
	d.registry[frameType] = fd
}

// Route inspects the raw wire payload's `type` field and dispatches
// to the registered FrameDispatcher. Behavior on unknown type:
//   - logs warn `bpp.plugin_frame_unknown_type` (forward-compat: plugin
//     may send frames the server doesn't understand on rolling upgrade)
//   - returns nil (no error — reject would break plugin upgrade flow)
//
// Behavior on registered type returning an error:
//   - logs warn with the wrapped error (per-frame dispatcher already
//     wraps with sentinel for code mapping)
//   - returns the wrapped error so callers can errors.Is for metrics
//
// Returns:
//   - (true, nil)   — type matched a registered dispatcher; Dispatch OK
//   - (true, error) — type matched, dispatcher returned err (logged)
//   - (false, nil)  — type unknown OR raw payload missing `type` (logged
//     warn, soft-skip, plugin upgrade tolerance)
func (d *PluginFrameDispatcher) Route(raw []byte, sess PluginSessionContext) (handled bool, err error) {
	if len(raw) == 0 {
		return false, nil
	}
	// Peek `type` field without full decoding — frame payload may be
	// large (e.g. inbound_message), avoid double parse.
	var head struct {
		Type string `json:"type"`
	}
	if jerr := json.Unmarshal(raw, &head); jerr != nil {
		// Malformed JSON — plugin.go's earlier json.Unmarshal would
		// have caught this for the {type, id, data} envelope, but
		// callers may pass partial frames. Soft-skip + log.
		d.logf("warn", "bpp.plugin_frame_malformed_json", "error", jerr)
		return false, nil
	}
	if head.Type == "" {
		return false, nil
	}

	d.mu.RLock()
	fd, ok := d.registry[head.Type]
	d.mu.RUnlock()
	if !ok {
		// Forward-compat soft-skip — plugin upgrade may send frames the
		// server doesn't yet understand; rejecting would break rolling
		// rollouts. Log warn for visibility.
		d.logf("warn", "bpp.plugin_frame_unknown_type", "type", head.Type)
		return false, nil
	}

	if derr := fd.Dispatch(raw, sess); derr != nil {
		d.logf("warn", "bpp.plugin_frame_dispatch_failed",
			"type", head.Type, "error", derr)
		return true, derr
	}
	return true, nil
}

func (d *PluginFrameDispatcher) logf(level, msg string, args ...any) {
	if d.logger == nil {
		return
	}
	switch level {
	case "warn":
		d.logger.Warn(msg, args...)
	case "info":
		d.logger.Info(msg, args...)
	case "error":
		d.logger.Error(msg, args...)
	default:
		d.logger.Info(msg, args...)
	}
}

// isPluginToServerFrame returns true iff frameType is in the BPP
// envelope whitelist AND has FrameDirection == DirectionPluginToServer.
//
// This walks the AllBPPEnvelopes() reflection list — small (10
// frames), called only at server boot via Register, no perf concern.
func isPluginToServerFrame(frameType string) bool {
	for _, frame := range AllBPPEnvelopes() {
		if frame.FrameType() == frameType && frame.FrameDirection() == DirectionPluginToServer {
			return true
		}
	}
	return false
}

// AckFrameAdapter wraps an *AckDispatcher (the AL-2b agent_config_ack
// dispatcher, defined in agent_config_ack_dispatcher.go) into the
// FrameDispatcher interface so it can register on PluginFrameDispatcher.
//
// Stance: keep the AckDispatcher API surface focused on the validated
// frame struct (typed AgentConfigAckFrame) and let this adapter handle
// the raw → typed decoding boundary. Same pattern: BPP-2.1 ActionHandler
// wraps server-go business logic, this wraps the typed dispatcher into
// a wire-level adapter.
//
// Wire-up: server.go boot does
//
//	pfd := bpp.NewPluginFrameDispatcher(logger)
//	pfd.Register(bpp.FrameTypeBPPAgentConfigAck,
//	             bpp.NewAckFrameAdapter(ackDispatcher))
//	hub.SetPluginFrameDispatcher(pfd)  // optional lookup for plugin.go
type AckFrameAdapter struct {
	dispatcher *AckDispatcher
}

// NewAckFrameAdapter wires an AckDispatcher (typed) into the
// FrameDispatcher (raw) interface. Panics on nil — boot bug.
func NewAckFrameAdapter(d *AckDispatcher) *AckFrameAdapter {
	if d == nil {
		panic("bpp: NewAckFrameAdapter dispatcher must not be nil")
	}
	return &AckFrameAdapter{dispatcher: d}
}

// Dispatch decodes raw → AgentConfigAckFrame and delegates to the
// typed AckDispatcher. AckSessionContext mirrors PluginSessionContext
// (same OwnerUserID semantics).
func (a *AckFrameAdapter) Dispatch(raw json.RawMessage, sess PluginSessionContext) error {
	var frame AgentConfigAckFrame
	if err := json.Unmarshal(raw, &frame); err != nil {
		return fmt.Errorf("bpp.ack_frame_decode: %w", err)
	}
	return a.dispatcher.Dispatch(frame, AckSessionContext{
		OwnerUserID: sess.OwnerUserID,
	})
}
