// Package api — agent_config_ack_handler.go: BPP-3 concrete
// AgentConfigAckHandler + OwnerResolver wiring the plugin-upstream
// agent_config_ack BPP frame into the SSOT plane.
//
// Blueprint锚: docs/blueprint/current/plugin-protocol.md §1.5 (热更新 + 幂等
// reload + ack 回执) + §2.2 (data plane, Plugin → Server).
// Spec brief: docs/implementation/modules/al-2b.2-server-hook-spec.md §1
// (落点 `internal/api/agent_config_ack_handler.go`).
// Acceptance: docs/qa/acceptance-templates/al-2b.md §2.5 + §3.2.
//
// Why this file exists (BPP-3 后续 to AL-2b #481):
//
//   - AL-2b #481 shipped the *outbound* AgentConfigUpdateFrame
//     (server→plugin) + the bpp.AckDispatcher seam, but deferred the
//     *inbound* `agent_config_ack` ingress because plugin.go's read
//     loop only handled the {type, id, data} RPC envelope. BPP-3 ships
//     the unified frame dispatcher boundary (internal/bpp/
//     plugin_frame_dispatcher.go); this file is the concrete handler
//     the dispatcher delegates to.
//
//   - Owner-only ACL gate: the OwnerResolver looks up agent_id → owner
//     UUID via the existing store.GetAgent path (跟 anchor #360 owner-
//     only / agent_config.go PATCH owner ACL 同模式 字面同源).
//
//   - SSOT compare semantics (best-effort, 反约束 §3.2):
//       * applied  — log info (audit trail; last_applied_at column
//                    reserved for future schema bump, not in scope for
//                    BPP-3 wire-routing milestone — bpp.dispatcher godoc
//                    referenced this; intentionally log-only).
//       * stale    — log warn (plugin reports an older schema_version
//                    than current; plugin will GET /agents/:id/config
//                    on next poll, runtime 不缓存 蓝图 §1.5).
//       * rejected — log warn with reason (AL-1a 6-dict already
//                    validated by bpp.AckDispatcher upstream).
//
// 反约束:
//   - admin god-mode 不入此路径 — handler 用 OwnerResolver 走 owner-only
//     ACL (跟 ADM-0 §1.3 红线 + REG-INV-002 fail-closed 同模式).
//   - 不写 events 表 — ack 是 plugin → server 回执 audit, 不是 channel
//     broadcast (RT-1 条原则反约束: 不另起 plugin-only 推送通道).
//   - bpp 包零 internal/api 依赖 — handler 通过 bpp.AgentConfigAckHandler
//     interface seam 注入 (跟 BPP-2.1 ActionHandler / cv-4.2
//     IterationStatePusher 同模式).
package api

import (
	"fmt"
	"log/slog"

	"borgee-server/internal/bpp"
	"borgee-server/internal/store"
)

// AgentConfigAckHandlerImpl is the concrete bpp.AgentConfigAckHandler
// wired in server.go boot. Logger may be nil (defaults to discard for
// unit tests).
type AgentConfigAckHandlerImpl struct {
	Store  *store.Store
	Logger *slog.Logger
}

// HandleAck processes a validated AgentConfigAckFrame. bpp.AckDispatcher
// has already gated:
//   - Status ∈ {applied, rejected, stale}
//   - Reason ∈ AL-1a 6-dict (when Status != applied && Reason != "")
//   - cross-owner check via OwnerResolver
//
// This handler is the terminal sink — log + best-effort book-keeping,
// no error returned for log-only outcomes (returning error here would
// only re-log via bpp.PluginFrameDispatcher.Route, double noise).
func (h *AgentConfigAckHandlerImpl) HandleAck(frame bpp.AgentConfigAckFrame, sess bpp.AckSessionContext) error {
	if h.Logger == nil {
		return nil
	}
	switch frame.Status {
	case bpp.AgentConfigAckStatusApplied:
		h.Logger.Info("bpp.agent_config_ack_applied",
			"agent_id", frame.AgentID,
			"owner_id", sess.OwnerUserID,
			"schema_version", frame.SchemaVersion,
			"applied_at", frame.AppliedAt)
	case bpp.AgentConfigAckStatusStale:
		h.Logger.Warn("bpp.agent_config_ack_stale",
			"agent_id", frame.AgentID,
			"owner_id", sess.OwnerUserID,
			"schema_version", frame.SchemaVersion,
			"reason", frame.Reason)
	case bpp.AgentConfigAckStatusRejected:
		h.Logger.Warn("bpp.agent_config_ack_rejected",
			"agent_id", frame.AgentID,
			"owner_id", sess.OwnerUserID,
			"schema_version", frame.SchemaVersion,
			"reason", frame.Reason)
	}
	return nil
}

// AgentOwnerResolver implements bpp.OwnerResolver against the existing
// store.GetAgent path. The agents row's OwnerID is the SSOT for owner
// ACL — same field anchor #360 / DM-2 #372 / agent_config.go PATCH
// owner gate consult.
type AgentOwnerResolver struct {
	Store *store.Store
}

// OwnerOf returns the owner UUID for an agent_id. Returns ("", error)
// when:
//   - the agent row doesn't exist (deleted, wrong id) — bpp dispatcher
//     soft-rejects via errAckCrossOwnerReject (audit log only).
//   - the agent row has no OwnerID (legacy data; should be backfilled
//     by CM-3 #176 org enrichment).
func (r *AgentOwnerResolver) OwnerOf(agentID string) (string, error) {
	agent, err := r.Store.GetAgent(agentID)
	if err != nil {
		return "", fmt.Errorf("agent lookup failed: %w", err)
	}
	if agent.OwnerID == nil {
		return "", fmt.Errorf("agent %s has no owner", agentID)
	}
	return *agent.OwnerID, nil
}
