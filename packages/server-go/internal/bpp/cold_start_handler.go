// Package bpp — cold_start_handler.go: BPP-6 plugin cold-start handshake
// dispatcher. Wired into the BPP-3 #489 PluginFrameDispatcher boundary
// to handle FrameTypeBPPColdStartHandshake.
//
// Blueprint reference: docs/blueprint/current/plugin-protocol.md §1.6
// (disconnected and failure states: process death vs. network reconnect) +
// §2.1 control-plane handshake.
// Spec: docs/implementation/modules/bpp-6-spec.md §0+§1 BPP-6.2.
// Acceptance: docs/qa/acceptance-templates/bpp-6.md §2.
//
// Design (matching agreement §2+§3+§4):
//   - **cold-start ≠ reconnect**. Its field set is disjoint from
//     ReconnectHandshakeFrame: it carries no cursor and does not expect resume.
//     See spec §0.1.
//   - **agent state is derived again**. When the server receives
//     cold_start_handshake, it clears in-memory state with
//     agent.Tracker.Clear(agentID), appends AL-1 #492 single-gate
//     AppendAgentStateTransition(any→online, runtime_crashed) to state-log, and
//     **does not replay historical frames**. This is the opposite of BPP-5:
//     BPP-5 performs incremental recovery, while BPP-6 is a fresh start. See
//     spec §0.2.
//   - **restart count is audit-only**. The reason reuses `runtime_crashed`
//     as the aligned value for previous error → current recovery.
//     reasons single source #496 stays a 6-dict and does not add a 7th reason.
//     BPP-6 is the 11th AL-1a reason alignment point. See spec §0.3.
//   - **best-effort and not resent** (consistent with BPP-4 §0.3 and BPP-5
//     §0.6). The server does not maintain a cold-start retry queue or
//     persistent state. AST scan must find 0 forbidden tokens
//     (pendingColdStart/coldStartQueue/deadLetterColdStart).
//
// Constraints (acceptance §2+§3+§4):
//   - cross-owner reject, matching the BPP-3 / BPP-4 / BPP-5 ACL pattern.
//   - Do not replay history: the handler carries no cursor
//     (TestBPP6_Handler_DoesNotInvokeResolveResume AST scan).
//   - Do not add a plugin_restart_count column: restart count is derived from
//     state-log COUNT(WHERE to_state='online' AND reason='runtime_crashed')
//     (TestBPP6_RestartCount_DerivedFromStateLog).

package bpp

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"borgee-server/internal/agent/reasons"
	"borgee-server/internal/store"
)

// AgentStateAppender is the interface seam to *store.Store's
// AppendAgentStateTransition + ListAgentStateLog (AL-1 #492 single-gate).
// This follows the same interface-boundary pattern as AgentErrorClearer and
// OwnerResolver: the bpp package uses interface injection and does not directly
// import the store business boundary.
type AgentStateAppender interface {
	AppendAgentStateTransition(agentID string, from, to store.AgentState, reason, taskID string) (int64, error)
	ListAgentStateLog(agentID string, limit int) ([]store.AgentStateLogRow, error)
}

// ColdStartHandler is the BPP-6 PluginFrameDispatcher entry. Construct
// via NewColdStartHandler(stateAppender, ownerResolver, errClearer,
// logger). All three wiring deps panic on nil — boot bug (跟 BPP-3
// NewAckDispatcher / BPP-4 NewHeartbeatWatchdog / BPP-5 NewReconnectHandler
// 同模式).
type ColdStartHandler struct {
	state   AgentStateAppender
	owner   OwnerResolver
	clearer AgentErrorClearer
	logger  *slog.Logger
}

// NewColdStartHandler wires the BPP-6 cold-start handler. logger may
// be nil (defaults to discard, useful in tests with captured handler).
func NewColdStartHandler(state AgentStateAppender, owner OwnerResolver,
	clearer AgentErrorClearer, logger *slog.Logger) *ColdStartHandler {
	if state == nil {
		panic("bpp: NewColdStartHandler state must not be nil")
	}
	if owner == nil {
		panic("bpp: NewColdStartHandler owner must not be nil")
	}
	if clearer == nil {
		panic("bpp: NewColdStartHandler clearer must not be nil")
	}
	return &ColdStartHandler{
		state:   state,
		owner:   owner,
		clearer: clearer,
		logger:  logger,
	}
}

// errColdStartCrossOwnerReject — cross-owner ACL fail, matching the BPP-5
// errReconnectCrossOwnerReject pattern.
var errColdStartCrossOwnerReject = errors.New(
	"bpp: cold_start_handshake cross-owner reject")

// IsColdStartCrossOwnerReject — sentinel matcher.
func IsColdStartCrossOwnerReject(err error) bool {
	return errors.Is(err, errColdStartCrossOwnerReject)
}

// ColdStartErrCodeCrossOwnerReject — wire-level error code.
const ColdStartErrCodeCrossOwnerReject = "bpp.cold_start_cross_owner_reject"

// Dispatch — bpp.FrameDispatcher impl, registered on
// PluginFrameDispatcher for FrameTypeBPPColdStartHandshake.
//
// Validation order:
//
//  1. Decode raw → ColdStartHandshakeFrame (malformed → error wrapped).
//  2. cross-owner check: owner.OwnerOf(frame.AgentID) == sess.OwnerUserID.
//     Mismatch → errColdStartCrossOwnerReject + log warn.
//  3. Resolve current state via ListAgentStateLog(agentID, 1):
//     - no history → from = AgentStateInitial
//     - last row → from = last.ToState
//     If from == AgentStateOnline already, skip transition (no-op,
//     ValidateTransition rejects same-state). Tracker.Clear still
//     called to ensure in-memory state is fresh.
//  4. Append AL-1 #492 single-gate transition any→online with reason
//     `runtime_crashed`, reusing the aligned 6-dict without expanding
//     the single source of truth.
//  5. Clear agent in-memory state: clearer.Clear(frame.AgentID).
//
// Returns nil on success; wrapped sentinel errors on failure
// (callers errors.Is to map to wire-level codes).
func (h *ColdStartHandler) Dispatch(raw json.RawMessage, sess PluginSessionContext) error {
	var frame ColdStartHandshakeFrame
	if err := json.Unmarshal(raw, &frame); err != nil {
		return fmt.Errorf("bpp.cold_start_frame_decode: %w", err)
	}
	if frame.AgentID == "" {
		return errors.New("bpp.cold_start_handshake_invalid: agent_id required")
	}

	// 2. cross-owner check.
	owner, err := h.owner.OwnerOf(frame.AgentID)
	if err != nil {
		return fmt.Errorf("%w: agent_id=%q resolve failed: %v",
			errColdStartCrossOwnerReject, frame.AgentID, err)
	}
	if owner != sess.OwnerUserID {
		if h.logger != nil {
			h.logger.Warn(ColdStartErrCodeCrossOwnerReject,
				"agent_id", frame.AgentID,
				"owner", owner,
				"sess_owner", sess.OwnerUserID)
		}
		return fmt.Errorf("%w: agent_id=%q owner=%q sess_owner=%q",
			errColdStartCrossOwnerReject, frame.AgentID, owner, sess.OwnerUserID)
	}

	// 3. Resolve current state from state-log (most recent row).
	rows, err := h.state.ListAgentStateLog(frame.AgentID, 1)
	if err != nil {
		return fmt.Errorf("bpp.cold_start_state_lookup_failed: %w", err)
	}
	from := store.AgentStateInitial
	if len(rows) > 0 {
		from = store.AgentState(rows[0].ToState)
	}

	// 4. Append transition any→online via AL-1 #492 single-gate. Skip
	// when already online (ValidateTransition rejects same-state, by
	// design — cold-start from already-online is a no-op + tracker
	// clear only).
	if from != store.AgentStateOnline {
		if _, err := h.state.AppendAgentStateTransition(
			frame.AgentID,
			from,
			store.AgentStateOnline,
			reasons.RuntimeCrashed, // AL-1a 6-dict; reasons single source #496, alignment point 11
			"",
		); err != nil {
			return fmt.Errorf("bpp.cold_start_state_append_failed: %w", err)
		}
	}

	// 5. Clear agent in-memory state. Tracker.Clear is the single source of
	// truth, with the same semantics as the BPP-5 reconnect handler and the
	// opposite direction from BPP-4 SetError.
	h.clearer.Clear(frame.AgentID)

	if h.logger != nil {
		h.logger.Info("bpp.cold_start_handshake_received",
			"agent_id", frame.AgentID,
			"plugin_id", frame.PluginID,
			"restart_at", frame.RestartAt,
			"restart_reason", frame.RestartReason,
			"from_state", string(from))
	}
	return nil
}
