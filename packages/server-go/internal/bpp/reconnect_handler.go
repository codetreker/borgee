// Package bpp — reconnect_handler.go: BPP-5 plugin reconnect handshake
// dispatcher. Wired into the BPP-3 #489 PluginFrameDispatcher boundary
// to handle FrameTypeBPPReconnectHandshake.
//
// Blueprint reference: docs/blueprint/current/plugin-protocol.md §1.6
// (reconnect recovery) + §2.1 (inherits the control-plane connect path) +
// RT-1.3 #296 cursor replay (reuses ResolveResume incremental mode).
// Spec: docs/implementation/modules/bpp-5-spec.md §0+§1 BPP-5.2.
// Acceptance: docs/qa/acceptance-templates/bpp-5.md §2.
//
// Stance (byte-identical with stance §2+§3+§4):
//   - **cursor resume reuses RT-1.3** by calling
//     ResolveResume(SessionResumeRequest{Mode: ResumeModeIncremental,
//     Since: LastKnownCursor}, …). Do not add another sequence or dictionary.
//   - **AL-1 five-state error → online chain** reuses the existing #492 valid
//     edge. There is no persisted "connecting" intermediate state; that is
//     only a spec concept. agent.Tracker.Clear is sufficient because once
//     hub.GetPlugin(agentID) != nil, ResolveAgentState moves error to online.
//   - **Do not add a 7th reason**. The connecting intermediate state is
//     reason-less; BPP-5 is the 10th test lock in the AL-1a 6-dict chain.
//   - **best-effort and not resent** (inherits BPP-4 §0.3 stance). The server
//     does not maintain a reconnect retry queue. The AST scan must find 0
//     forbidden tokens.
//
// Negative constraints (acceptance §4):
//   - cross-owner reject, matching the BPP-3 / BPP-4 ACL pattern.
//   - cursor regression uses trust-but-log: warn `bpp.reconnect_cursor_regression`
//     but do not reject; strict rejection is deferred to v2.

package bpp

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
)

// AgentErrorClearer is the interface boundary to *agent.Tracker.Clear. This is
// the counterpart to BPP-4 #499 AgentErrorSink.SetError: the bpp package does
// not import internal/agent at the reconnect boundary, and uses interface
// injection instead.
type AgentErrorClearer interface {
	Clear(agentID string)
}

// ChannelScopeResolver returns the permitted channel ids for the
// authenticated owner. This uses the same scope as RT-1.3 acceptance §2.5:
// the caller's channels. Same interface-boundary pattern as OwnerResolver and
// AgentErrorClearer.
type ChannelScopeResolver interface {
	ChannelIDsForOwner(ownerUserID string) ([]string, error)
}

// ReconnectHandler is the BPP-5 PluginFrameDispatcher entry. Construct
// via NewReconnectHandler(eventLister, scopeResolver, ownerResolver,
// errClearer, logger). All four wiring deps panic on nil — boot bug
// (跟 BPP-3 NewAckDispatcher / BPP-4 NewHeartbeatWatchdog 同模式).
type ReconnectHandler struct {
	events  EventLister
	scope   ChannelScopeResolver
	owner   OwnerResolver
	clearer AgentErrorClearer
	logger  *slog.Logger
}

// NewReconnectHandler wires the BPP-5 reconnect handler. logger may
// be nil (defaults to discard, useful in tests with captured handler).
func NewReconnectHandler(events EventLister, scope ChannelScopeResolver,
	owner OwnerResolver, clearer AgentErrorClearer, logger *slog.Logger) *ReconnectHandler {
	if events == nil {
		panic("bpp: NewReconnectHandler events must not be nil")
	}
	if scope == nil {
		panic("bpp: NewReconnectHandler scope must not be nil")
	}
	if owner == nil {
		panic("bpp: NewReconnectHandler owner must not be nil")
	}
	if clearer == nil {
		panic("bpp: NewReconnectHandler clearer must not be nil")
	}
	return &ReconnectHandler{
		events:  events,
		scope:   scope,
		owner:   owner,
		clearer: clearer,
		logger:  logger,
	}
}

// errReconnectCrossOwnerReject — cross-owner ACL failure, matching the BPP-3 ack
// dispatcher errAckCrossOwnerReject pattern.
var errReconnectCrossOwnerReject = errors.New(
	"bpp: reconnect_handshake cross-owner reject")

// IsReconnectCrossOwnerReject — sentinel matcher.
func IsReconnectCrossOwnerReject(err error) bool {
	return errors.Is(err, errReconnectCrossOwnerReject)
}

// ReconnectErrCodeCrossOwnerReject — wire-level error code, named with the
// same pattern as BPP-3 AckErrCodeCrossOwnerReject.
const ReconnectErrCodeCrossOwnerReject = "bpp.reconnect_cross_owner_reject"

// Dispatch — bpp.FrameDispatcher impl, registered on
// PluginFrameDispatcher for FrameTypeBPPReconnectHandshake.
//
// Validation order:
//
//  1. Decode raw → ReconnectHandshakeFrame (malformed → error wrapped).
//  2. cross-owner check: owner.OwnerOf(frame.AgentID) == sess.OwnerUserID.
//     Mismatch → errReconnectCrossOwnerReject + log warn.
//  3. cursor monotonic check (trust-but-log): if frame.LastKnownCursor
//     > server's current high-water → log warn
//     `bpp.reconnect_cursor_regression` (but DO NOT reject; v2 tracks the
//     stricter behavior).
//  4. Resolve channel scope: scope.ChannelIDsForOwner(sess.OwnerUserID).
//  5. Replay via ResolveResume(SessionResumeRequest{Mode: incremental,
//     Since: frame.LastKnownCursor}, channelIDs, DefaultResumeLimit).
//     The replayed events are NOT pushed back here — callers (server.go
//     wire-up) decide how to surface them. BPP-5 just resumes the
//     cursor and clears the agent error state.
//  6. Clear agent error: clearer.Clear(frame.AgentID). agent.Tracker
//     auto-flips error → online because hub.GetPlugin(frame.AgentID)
//     != nil, byte-identical with the #492 five-state graph valid edge.
//
// Returns nil on success; wrapped sentinel errors on failure
// (callers errors.Is to map to wire-level codes). Negative dispatch invariant:
// this handler never writes persistent retry state; it inherits BPP-4
// best-effort behavior.
func (h *ReconnectHandler) Dispatch(raw json.RawMessage, sess PluginSessionContext) error {
	var frame ReconnectHandshakeFrame
	if err := json.Unmarshal(raw, &frame); err != nil {
		return fmt.Errorf("bpp.reconnect_frame_decode: %w", err)
	}
	if frame.AgentID == "" {
		return errors.New("bpp.reconnect_handshake_invalid: agent_id required")
	}

	// 2. cross-owner check.
	owner, err := h.owner.OwnerOf(frame.AgentID)
	if err != nil {
		return fmt.Errorf("%w: agent_id=%q resolve failed: %v",
			errReconnectCrossOwnerReject, frame.AgentID, err)
	}
	if owner != sess.OwnerUserID {
		if h.logger != nil {
			h.logger.Warn(ReconnectErrCodeCrossOwnerReject,
				"agent_id", frame.AgentID,
				"owner", owner,
				"sess_owner", sess.OwnerUserID)
		}
		return fmt.Errorf("%w: agent_id=%q owner=%q sess_owner=%q",
			errReconnectCrossOwnerReject, frame.AgentID, owner, sess.OwnerUserID)
	}

	// 3. cursor monotonic check (trust-but-log).
	highWater := h.events.GetLatestCursor()
	if frame.LastKnownCursor > highWater {
		if h.logger != nil {
			h.logger.Warn("bpp.reconnect_cursor_regression",
				"agent_id", frame.AgentID,
				"last_known_cursor", frame.LastKnownCursor,
				"server_high_water", highWater,
				"action", "trust-but-log (v1, strict reject 留 v2)")
		}
	}

	// 4. Resolve channel scope.
	channelIDs, err := h.scope.ChannelIDsForOwner(sess.OwnerUserID)
	if err != nil {
		return fmt.Errorf("bpp.reconnect_channel_scope_failed: %w", err)
	}

	// 5. Replay via RT-1.3 ResolveResume (incremental mode, byte-identical with
	// spec §0.2 stance).
	if _, _, err := ResolveResume(h.events, SessionResumeRequest{
		Type:  FrameTypeSessionResume,
		Mode:  ResumeModeIncremental,
		Since: frame.LastKnownCursor,
	}, channelIDs, DefaultResumeLimit); err != nil {
		return fmt.Errorf("bpp.reconnect_resume_failed: %w", err)
	}

	// 6. Clear agent error (AL-1 5-state error → online valid edge,
	// agent.Tracker.Clear is the single source of truth: hub.GetPlugin(agentID)
	// != nil + tracker.Clear → ResolveAgentState returns online.
	h.clearer.Clear(frame.AgentID)

	if h.logger != nil {
		h.logger.Info("bpp.reconnect_handshake_resolved",
			"agent_id", frame.AgentID,
			"plugin_id", frame.PluginID,
			"last_known_cursor", frame.LastKnownCursor,
			"server_high_water", highWater,
			"channel_count", len(channelIDs))
	}
	return nil
}
