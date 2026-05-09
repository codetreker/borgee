// Package bpp — reconnect_handler.go: BPP-5 plugin reconnect handshake
// dispatcher. Wired into the BPP-3 #489 PluginFrameDispatcher boundary
// to handle FrameTypeBPPReconnectHandshake.
//
// Blueprint锚: docs/blueprint/current/plugin-protocol.md §1.6 (重连恢复) +
// §2.1 (control-plane connect 路径承袭) + RT-1.3 #296 cursor replay
// (复用 ResolveResume incremental mode).
// Spec: docs/implementation/modules/bpp-5-spec.md §0+§1 BPP-5.2.
// Acceptance: docs/qa/acceptance-templates/bpp-5.md §2.
//
// 设计 (跟 stance §2+§3+§4 byte-identical):
//   - **cursor resume 复用 RT-1.3** — 调 ResolveResume(SessionResumeRequest{
//     Mode: ResumeModeIncremental, Since: LastKnownCursor}, …). 不另起
//     sequence, 不另起 dictionary.
//   - **AL-1 5-state 反向链 error → online** — 复用 #492 既有 valid edge
//     (无 persisted "connecting" 中间态; spec 概念名). agent.Tracker.Clear
//     即可 (因为 hub.GetPlugin(agentID) != nil 后, ResolveAgentState 自动
//     从 error 转 online).
//   - **不另起第 7 reason** — connecting 中间态 reason-less; AL-1a 6-dict
//     第 10 处单测锁链承袭 (BPP-2.2 #485 第 7 + AL-2b #481 第 8 + BPP-4
//     #499 第 9 + BPP-5 第 10).
//   - **best-effort 不重发** (跟 BPP-4 §0.3 设计沿用) — server 端不挂
//     reconnect retry queue. AST scan 反向断言 forbidden tokens 0 hit.
//
// 反约束 (acceptance §4):
//   - cross-owner reject (跟 BPP-3 / BPP-4 ACL 同模式).
//   - cursor 倒退 trust-but-log (warn `bpp.reconnect_cursor_regression`
//     但不 reject; 严格 reject 留 v2).

package bpp

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
)

// AgentErrorClearer is the interface seam to *agent.Tracker.Clear
// (跟 BPP-4 #499 AgentErrorSink.SetError 反向同模式 — bpp 包不直
// import internal/agent 在 reconnect 边界, 走 interface 注入).
type AgentErrorClearer interface {
	Clear(agentID string)
}

// ChannelScopeResolver returns the permitted channel ids for the
// authenticated owner (跟 RT-1.3 acceptance §2.5 同 scope: caller's
// channels). 跟 OwnerResolver / AgentErrorClearer 同 interface seam
// 模式.
type ChannelScopeResolver interface {
	ChannelIDsForOwner(ownerUserID string) ([]string, error)
}

// ReconnectHandler is the BPP-5 PluginFrameDispatcher entry. Construct
// via NewReconnectHandler(eventLister, scopeResolver, ownerResolver,
// errClearer, logger). All four wiring deps panic on nil — boot bug
// (跟 BPP-3 NewAckDispatcher / BPP-4 NewHeartbeatWatchdog 同模式).
type ReconnectHandler struct {
	events    EventLister
	scope     ChannelScopeResolver
	owner     OwnerResolver
	clearer   AgentErrorClearer
	logger    *slog.Logger
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

// errReconnectCrossOwnerReject — cross-owner ACL fail (跟 BPP-3 ack
// dispatcher errAckCrossOwnerReject 同模式).
var errReconnectCrossOwnerReject = errors.New(
	"bpp: reconnect_handshake cross-owner reject")

// IsReconnectCrossOwnerReject — sentinel matcher.
func IsReconnectCrossOwnerReject(err error) bool {
	return errors.Is(err, errReconnectCrossOwnerReject)
}

// ReconnectErrCodeCrossOwnerReject — wire-level error code (跟 BPP-3
// AckErrCodeCrossOwnerReject 同命名模式).
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
//     `bpp.reconnect_cursor_regression` (but DO NOT reject; v2 留账).
//  4. Resolve channel scope: scope.ChannelIDsForOwner(sess.OwnerUserID).
//  5. Replay via ResolveResume(SessionResumeRequest{Mode: incremental,
//     Since: frame.LastKnownCursor}, channelIDs, DefaultResumeLimit).
//     The replayed events are NOT pushed back here — callers (server.go
//     wire-up) decide how to surface them. BPP-5 just resumes the
//     cursor and clears the agent error state.
//  6. Clear agent error: clearer.Clear(frame.AgentID). agent.Tracker
//     auto-flips error → online because hub.GetPlugin(frame.AgentID)
//     != nil (跟 #492 5-state graph valid edge byte-identical).
//
// Returns nil on success; wrapped sentinel errors on failure
// (callers errors.Is to map to wire-level codes). 反向 dispatch (此
// handler 永不写持久 retry state — best-effort 设计沿用 BPP-4).
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

	// 5. Replay via RT-1.3 ResolveResume (incremental mode, byte-identical
	// 设计沿用 spec §0.2).
	if _, _, err := ResolveResume(h.events, SessionResumeRequest{
		Type:  FrameTypeSessionResume,
		Mode:  ResumeModeIncremental,
		Since: frame.LastKnownCursor,
	}, channelIDs, DefaultResumeLimit); err != nil {
		return fmt.Errorf("bpp.reconnect_resume_failed: %w", err)
	}

	// 6. Clear agent error (AL-1 5-state error → online valid edge,
	// agent.Tracker.Clear is the SSOT — hub.GetPlugin(agentID) != nil
	// + tracker.Clear → ResolveAgentState returns online).
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
