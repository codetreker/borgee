// Package bpp — cold_start_handler.go: BPP-6 plugin cold-start handshake
// dispatcher. Wired into the BPP-3 #489 PluginFrameDispatcher boundary
// to handle FrameTypeBPPColdStartHandshake.
//
// Blueprint锚: docs/blueprint/current/plugin-protocol.md §1.6 (失联与故障状态 —
// 进程死亡 vs 网络重连) + §2.1 control-plane handshake.
// Spec: docs/implementation/modules/bpp-6-spec.md §0+§1 BPP-6.2.
// Acceptance: docs/qa/acceptance-templates/bpp-6.md §2.
//
// 设计 (跟 约定 §2+§3+§4 byte-identical):
//   - **cold-start ≠ reconnect** — 字段集与 ReconnectHandshakeFrame 互斥
//     (不带 cursor / 不 expect resume). spec §0.1.
//   - **agent state 重新 derive** — server 收 cold_start_handshake →
//     ① agent.Tracker.Clear(agentID) 清 in-memory state +
//     ② AL-1 #492 single-gate AppendAgentStateTransition(any→online,
//     runtime_crashed) 翻 state-log + ③ **不重放历史 frame** (反向
//     BPP-5: BPP-5 走增量恢复; BPP-6 fresh start). spec §0.2.
//   - **restart count 仅 audit** — reason 复用 `runtime_crashed`
//     byte-identical (反映上次 error → 此次复活语义). reasons 单一来源 #496
//     6-dict 不扩第 7. AL-1a reason 对齐位 BPP-6 = 第 11 处 (BPP-2.2 第 7
//     + AL-2b 第 8 + BPP-4 第 9 + BPP-5 第 10 + BPP-6 第 11). spec §0.3.
//   - **best-effort 不重发** (跟 BPP-4 §0.3 + BPP-5 §0.6 设计跟历史一致) —
//     server 端不挂 cold-start retry queue / persistent state. AST scan
//     反向断言 forbidden tokens (pendingColdStart/coldStartQueue/
//     deadLetterColdStart) 0 hit (延伸第 3 处).
//
// 约束 (acceptance §2+§3+§4):
//   - cross-owner reject (跟 BPP-3 / BPP-4 / BPP-5 ACL 同模式).
//   - 不重放历史 — handler 不携 cursor (TestBPP6_Handler_DoesNotInvokeResolveResume
//     AST scan 守).
//   - 不另开 plugin_restart_count 列 — restart 计数走 state-log
//     COUNT(WHERE to_state='online' AND reason='runtime_crashed') 反向
//     derive (TestBPP6_RestartCount_DerivedFromStateLog 守).

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
// 跟 AgentErrorClearer / OwnerResolver 同 interface seam 模式 — bpp 包
// 走 interface 注入, 不直 import store 业务边界.
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

// errColdStartCrossOwnerReject — cross-owner ACL fail (跟 BPP-5
// errReconnectCrossOwnerReject 同模式).
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
//       - no history → from = AgentStateInitial
//       - last row → from = last.ToState
//     If from == AgentStateOnline already, skip transition (no-op,
//     ValidateTransition rejects same-state). Tracker.Clear still
//     called to ensure in-memory state is fresh.
//  4. Append AL-1 #492 single-gate transition any→online with reason
//     `runtime_crashed` byte-identical (复用 6-dict, 不扩 单一来源).
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
			reasons.RuntimeCrashed, // AL-1a 6-dict, reasons 单一来源 #496 字面 byte-identical (对齐位第 11 处)
			"",
		); err != nil {
			return fmt.Errorf("bpp.cold_start_state_append_failed: %w", err)
		}
	}

	// 5. Clear agent in-memory state — Tracker.Clear is the 单一来源
	// (跟 BPP-5 reconnect handler 同语义, 反向 BPP-4 SetError).
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
