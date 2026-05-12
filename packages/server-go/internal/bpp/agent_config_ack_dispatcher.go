// Package bpp — agent_config_ack_dispatcher.go: AL-2b ack frame inbound
// dispatcher source-of-truth.
//
// Blueprint: docs/blueprint/current/plugin-protocol.md §1.5 (hot reload,
// idempotent reload, ack response) + §2.2 (data plane, Plugin → Server).
// Spec brief: docs/implementation/modules/al-2b-spec.md +
// docs/implementation/modules/al-2b.2-server-hook-spec.md §1.
// Acceptance: docs/qa/acceptance-templates/al-2b.md §1.2 + §2.5 + §3.2.
//
// What this file does:
//  1. Validate AgentConfigAckFrame.Status ∈ 3-enum {applied, rejected,
//     stale}; enum-out values reject with AckErrCodeStatusUnknown.
//  2. When Status ∈ {rejected, stale} and Reason is non-empty: validate Reason
//     against the AL-1a reason set that must match exactly.
//  3. cross-owner reject — sess.OwnerUserID from the authenticated BPP-1
//     connection must match frame.AgentID's owner, otherwise reject + log warn
//     `bpp.ack_cross_owner_reject`.
//  4. ActionHandler-style interface seam — bpp does not import internal/api;
//     api registers AgentConfigAckHandler.
//
// Negative constraints (acceptance §3.2 + §4 reverse grep):
//   - admin API does not enter this owner-only path.
//   - cursor remains the only trusted ordering source.
//   - AL-2a polling path is removed to avoid dual paths.
//   - reason values reuse the internal/agent/reasons single source.
//   - bpp has no internal/api dependency; the interface seam matches BPP-2.1.
package bpp

import (
	"errors"
	"fmt"

	"borgee-server/internal/agent/reasons"
)

// AckErrCode* — error code literals match the BPP-2.2
// task_outcome_unknown / BPP-2.3 config_field_disallowed naming pattern.
const (
	AckErrCodeStatusUnknown    = "bpp.ack_status_unknown"
	AckErrCodeReasonUnknown    = "bpp.ack_reason_unknown"
	AckErrCodeCrossOwnerReject = "bpp.ack_cross_owner_reject"
)

// errAckStatusUnknown / errAckReasonUnknown / errAckCrossOwnerReject
// — sentinels callers can errors.Is against to map to wire-level error
// codes, matching BPP-2.1 errSemanticOpUnknown / BPP-2.2 errOutcomeUnknown.
var (
	errAckStatusUnknown    = errors.New("bpp: agent_config_ack status unknown (3-enum: applied/rejected/stale)")
	errAckReasonUnknown    = errors.New("bpp: agent_config_ack reason unknown (not in AL-1a 6 dict)")
	errAckCrossOwnerReject = errors.New("bpp: agent_config_ack cross-owner reject")
)

// IsAckStatusUnknown / IsAckReasonUnknown / IsAckCrossOwnerReject —
// sentinel matchers, matching BPP-2.1 IsSemanticOpUnknown / BPP-2.2
// IsTaskOutcomeUnknown.
func IsAckStatusUnknown(err error) bool { return errors.Is(err, errAckStatusUnknown) }
func IsAckReasonUnknown(err error) bool { return errors.Is(err, errAckReasonUnknown) }
func IsAckCrossOwnerReject(err error) bool {
	return errors.Is(err, errAckCrossOwnerReject)
}

// validAckStatuses — 3-enum membership set matching acceptance
// §1.2 CHECK enum (same source as al_2b_frames_test.go::isValidAckStatus;
// this is the production path reference).
var validAckStatuses = map[string]bool{
	AgentConfigAckStatusApplied:  true,
	AgentConfigAckStatusRejected: true,
	AgentConfigAckStatusStale:    true,
}

// validAL1aReason — REFACTOR-REASONS moved the single source to
// internal/agent/reasons.
//
// History: this file previously had inline 6 literals matching
// agent/state.go Reason* (#249/#305/#321/#380/#454/#458/#481/#492 test-lock
// chain). REFACTOR-REASONS deduped them to the internal/agent/reasons single
// source of truth.
func validAL1aReason(s string) bool { return reasons.IsValid(s) }

// AckSessionContext is the per-plugin-connection context the
// AckDispatcher passes to the registered handler. Carries the
// authenticated plugin owner UUID (resolved via BPP-1 connect handshake)
// + the plugin id (audit trail).
//
// cross-owner reject compares OwnerUserID with frame.AgentID's owner. This
// mirrors BPP-2.1 SessionContext but remains a separate type because ack frames
// do not use the semantic action path.
type AckSessionContext struct {
	OwnerUserID string // resolved via BPP-1 connect handshake
	PluginID    string // for audit / log only
}

// AgentConfigAckHandler is the seam between the bpp package and the api
// package for processing a validated AgentConfigAckFrame. The api
// package implements one handler that:
//   - Looks up the current agent_configs.schema_version single-source value;
//   - Compares against frame.SchemaVersion (mismatch → log stale, skip
//     last_applied_at update);
//   - For Status==applied: UPDATE agent_configs.last_applied_at;
//   - For Status∈{rejected,stale}: log warn (best-effort, non-blocking).
//
// Negative constraint: bpp does not import internal/api; handler is injected
// through an interface.
type AgentConfigAckHandler interface {
	HandleAck(frame AgentConfigAckFrame, sess AckSessionContext) error
}

// OwnerResolver resolves an agent_id to its owner user UUID for cross-
// owner ACL. The api package wires this to the agents table, using the same
// gate as existing REST handler owner-only ACL (#360 / DM-2 #372 pattern).
//
// Returns ("", error) when agent_id does not exist; the dispatcher treats this
// as a soft reject (frame from disconnected agent — log warn but don't
// crash).
type OwnerResolver interface {
	OwnerOf(agentID string) (string, error)
}

// AckDispatcher routes validated AgentConfigAckFrame instances to the
// registered AgentConfigAckHandler. Validation order:
//
//  1. frame.Status ∈ validAckStatuses (3-enum). Values outside the enum → errAckStatusUnknown.
//  2. when Status ∈ {rejected, stale} and Reason is non-empty: Reason ∈
//     validAL1aReasons (AL-1a 6-dict). Values outside the dictionary → errAckReasonUnknown.
//  3. cross-owner check: resolver.OwnerOf(frame.AgentID) == sess.OwnerUserID.
//     mismatch → errAckCrossOwnerReject.
//  4. Delegate to handler.HandleAck(frame, sess).
//
// Negative constraints (acceptance §4):
//   - admin routes do not enter this path. handler.HandleAck uses owner-only
//     ACL, and CI reverse grep guards al-2b-spec §3 line 3.
//   - no raw HTTP / REST endpoint here. The interface seam keeps dispatcher with
//     zero internal/api imports, matching BPP-2.1.
type AckDispatcher struct {
	handler  AgentConfigAckHandler
	resolver OwnerResolver
}

// NewAckDispatcher creates a dispatcher wired to the given handler +
// owner resolver. Both MUST be non-nil; nil arguments are a server boot
// bug (panics — defense-in-depth, prevents 0-coverage routes from
// silently entering production).
func NewAckDispatcher(h AgentConfigAckHandler, r OwnerResolver) *AckDispatcher {
	if h == nil {
		panic("bpp: NewAckDispatcher handler must not be nil")
	}
	if r == nil {
		panic("bpp: NewAckDispatcher resolver must not be nil")
	}
	return &AckDispatcher{handler: h, resolver: r}
}

// Dispatch validates a plugin-upstream AgentConfigAckFrame and routes
// it to the registered handler. See type doc for validation order.
//
// Returns wrapped sentinel errors so callers can errors.Is to map to
// wire-level error codes (跟 BPP-2.1 Dispatch / BPP-2.2 ValidateTaskFinished
// 同模式).
func (d *AckDispatcher) Dispatch(frame AgentConfigAckFrame, sess AckSessionContext) error {
	// 1. Status 3-enum.
	if !validAckStatuses[frame.Status] {
		return fmt.Errorf("%w: status=%q (3-enum: applied/rejected/stale)",
			errAckStatusUnknown, frame.Status)
	}

	// 2. Reason dictionary (validate only when rejected/stale and Reason is non-empty).
	if frame.Status != AgentConfigAckStatusApplied && frame.Reason != "" {
		if !validAL1aReason(frame.Reason) {
			return fmt.Errorf("%w: reason=%q (AL-1a 6-dict: api_key_invalid/quota_exceeded/network_unreachable/runtime_crashed/runtime_timeout/unknown)",
				errAckReasonUnknown, frame.Reason)
		}
	}

	// 3. cross-owner check.
	owner, err := d.resolver.OwnerOf(frame.AgentID)
	if err != nil {
		return fmt.Errorf("%w: agent_id=%q resolve failed: %v",
			errAckCrossOwnerReject, frame.AgentID, err)
	}
	if owner != sess.OwnerUserID {
		return fmt.Errorf("%w: agent_id=%q owner=%q sess_owner=%q",
			errAckCrossOwnerReject, frame.AgentID, owner, sess.OwnerUserID)
	}

	// 4. Delegate.
	return d.handler.HandleAck(frame, sess)
}
