// Package bpp — lifecycle_audit.go: BPP-8.2 plugin lifecycle auditor.
//
// Records 5 plugin lifecycle events (connect / disconnect / reconnect /
// cold_start / heartbeat_timeout) into the existing admin_actions table
// with `actor_id="system"` and `action="plugin_<event>"`. Reuses the
// ADM-2.1 #484 admin_actions audit table; it does NOT introduce a separate
// `plugin_lifecycle_events` table.
//
// Blueprint: docs/blueprint/current/plugin-protocol.md §1.6 + §3 plugin lifecycle.
// Spec: docs/implementation/modules/bpp-8-spec.md §0 + §1 BPP-8.2.
//
// Design constraints:
//
//   - **① reuse admin_actions** — auditor calls Store.InsertAdminAction
//     with actor='system', action='plugin_<event>', target=<agent_id>,
//     metadata=JSON{plugin_id, reason, ...}. Audit remains forward-only.
//   - **② reason reuses the AL-1a reason set** — heartbeat_timeout reason=
//     reasons.NetworkUnreachable; cold_start reason=reasons.RuntimeCrashed
//     aligned with BPP-6 #522 + BPP-7 SDK.
//   - **④ single insert path** — all 5 methods go through this auditor; reverse grep
//     `InsertAdminAction.*"plugin_` must return zero hits outside lifecycle_audit.go.
//   - **⑥ best-effort** — log.Warn on InsertAdminAction errors and do not fail
//     the handler; no retry queue and no persistent deferred audit table.
//   - **⑦ actor='system' alignment** — matches BPP-4 watchdog + AP-2 sweeper.
//
// Constraints (acceptance §3):
//   - admin API must not mount plugin lifecycle endpoints (ADM-0 §1.3);
//     lifecycle GET in internal/api/bpp_8_lifecycle_list.go remains owner-only.
//   - AST scan forbidden: `pendingLifecycleAudit\|lifecycleQueue\|
//     deadLetterLifecycle` 0 hit (TestBPP83_NoLifecycleQueueOrAuditTable).

package bpp

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"borgee-server/internal/agent/reasons"
)

// LifecycleSystemActor matches BPP-4 watchdog and AP-2 sweeper
// actor='system'. Changes must be coordinated with BPP-4 and AP-2.
const LifecycleSystemActor = "system"

// Action constants — the 5 plugin_* literals in the admin_actions CHECK enum
// match migration v=31 (bpp_8_1_admin_actions_plugin_actions.go)
// CHECK literals. Any change must update the migration CHECK, these consts, and
// acceptance §1 together.
const (
	LifecycleActionConnect          = "plugin_connect"
	LifecycleActionDisconnect       = "plugin_disconnect"
	LifecycleActionReconnect        = "plugin_reconnect"
	LifecycleActionColdStart        = "plugin_cold_start"
	LifecycleActionHeartbeatTimeout = "plugin_heartbeat_timeout"
)

// LifecycleAuditor is the single-gate interface for recording the 5
// plugin lifecycle events. Implementations write rows to admin_actions
// (or fan to other audit sinks). Default impl is
// AdminActionsLifecycleAuditor.
type LifecycleAuditor interface {
	RecordConnect(pluginID, agentID string)
	RecordDisconnect(pluginID, agentID, reason string)
	RecordReconnect(pluginID, agentID string, lastKnownCursor int64)
	RecordColdStart(pluginID, agentID, restartReason string)
	RecordHeartbeatTimeout(pluginID, agentID string)
}

// LifecycleAuditStore is the seam to *store.Store's InsertAdminAction
// helper. The bpp package does not import store directly across the business
// boundary; it uses interface injection, matching BPP-3/4/5/6.
type LifecycleAuditStore interface {
	InsertAdminAction(actorID, targetUserID, action, metadata string) (string, error)
}

// AdminActionsLifecycleAuditor implements LifecycleAuditor by writing
// rows to admin_actions via Store.InsertAdminAction. Construct via
// NewAdminActionsLifecycleAuditor (nil store / nil logger panics —
// fail-fast constructor validation, matching the BPP-3/4/5/6 constructor pattern).
type AdminActionsLifecycleAuditor struct {
	store  LifecycleAuditStore
	logger *slog.Logger
}

// NewAdminActionsLifecycleAuditor wires the auditor. logger defaults to
// slog.Default when nil.
func NewAdminActionsLifecycleAuditor(store LifecycleAuditStore, logger *slog.Logger) *AdminActionsLifecycleAuditor {
	if store == nil {
		panic("bpp: NewAdminActionsLifecycleAuditor store must not be nil")
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &AdminActionsLifecycleAuditor{store: store, logger: logger}
}

// recordEvent — internal helper, single insert path. Marshals metadata
// to JSON; on InsertAdminAction error logs.Warn and returns
// (best-effort behavior).
func (a *AdminActionsLifecycleAuditor) recordEvent(action, agentID string, metadata map[string]any) {
	mdJSON, err := json.Marshal(metadata)
	if err != nil {
		a.logger.Warn("bpp.lifecycle_audit_metadata_marshal_failed",
			"action", action, "agent_id", agentID, "error", err)
		return
	}
	if _, err := a.store.InsertAdminAction(LifecycleSystemActor, agentID, action, string(mdJSON)); err != nil {
		a.logger.Warn("bpp.lifecycle_audit_insert_failed",
			"action", action, "agent_id", agentID, "error", err)
	}
}

// RecordConnect — BPP-1 connect handshake handler hook.
func (a *AdminActionsLifecycleAuditor) RecordConnect(pluginID, agentID string) {
	a.recordEvent(LifecycleActionConnect, agentID, map[string]any{
		"plugin_id": pluginID,
	})
}

// RecordDisconnect — hub Cleanup hook (ws.Conn close).
func (a *AdminActionsLifecycleAuditor) RecordDisconnect(pluginID, agentID, reason string) {
	a.recordEvent(LifecycleActionDisconnect, agentID, map[string]any{
		"plugin_id": pluginID,
		"reason":    reason,
	})
}

// RecordReconnect — BPP-5 #503 reconnect_handler.go hook.
func (a *AdminActionsLifecycleAuditor) RecordReconnect(pluginID, agentID string, lastKnownCursor int64) {
	a.recordEvent(LifecycleActionReconnect, agentID, map[string]any{
		"plugin_id":         pluginID,
		"last_known_cursor": lastKnownCursor,
	})
}

// RecordColdStart — BPP-6 #522 cold_start_handler.go hook.
//
// Design ②: reason reuses the AL-1a 6-dict. Caller passes restartReason
// (typically reasons.RuntimeCrashed, aligned with BPP-6 + BPP-7 SDK and
// the AL-1a reason alignment point). Negative assertion: caller must use the
// reasons.* const instead of hardcoding the "runtime_crashed" string.
func (a *AdminActionsLifecycleAuditor) RecordColdStart(pluginID, agentID, restartReason string) {
	a.recordEvent(LifecycleActionColdStart, agentID, map[string]any{
		"plugin_id":      pluginID,
		"restart_reason": restartReason,
	})
}

// RecordHeartbeatTimeout — BPP-4 #499 watchdog hook.
//
// Design ②: reason literal matches reasons.NetworkUnreachable,
// matching the BPP-4 watchdog SetError reason and AL-1a alignment point.
func (a *AdminActionsLifecycleAuditor) RecordHeartbeatTimeout(pluginID, agentID string) {
	a.recordEvent(LifecycleActionHeartbeatTimeout, agentID, map[string]any{
		"plugin_id": pluginID,
		"reason":    reasons.NetworkUnreachable, // AL-1a alignment point, kept aligned
	})
}

// Compile-time assertion that AdminActionsLifecycleAuditor implements
// LifecycleAuditor (reverse-grep guard for interface mismatch).
var _ LifecycleAuditor = (*AdminActionsLifecycleAuditor)(nil)

// formatColdStartReason — convenience for callers: returns
// reasons.RuntimeCrashed (the matching 6-dict literal). Exposed
// so test harnesses can assert the literal without importing reasons.*
// twice. Returns string for direct use in RecordColdStart.
func formatColdStartReason() string { return reasons.RuntimeCrashed }

// _ used only as a documentation hook.
var _ = fmt.Sprintf
