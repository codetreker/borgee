// Package bpp — lifecycle_audit.go: BPP-8.2 plugin lifecycle auditor.
//
// Records 5 plugin lifecycle events (connect / disconnect / reconnect /
// cold_start / heartbeat_timeout) into the existing admin_actions table
// with `actor_id="system"` and `action="plugin_<event>"`. Reuses the
// ADM-2.1 #484 admin_actions audit table — does NOT introduce a separate
// `plugin_lifecycle_events` table (设计 ①).
//
// Blueprint锚: docs/blueprint/current/plugin-protocol.md §1.6 + §3 plugin lifecycle.
// Spec: docs/implementation/modules/bpp-8-spec.md §0 设计 ①+②+③ + §1
// 拆段 BPP-8.2.
//
// 设计 (跟 stance §1+§2+§3+§4 byte-identical):
//
//   - **① 复用 admin_actions 表** — auditor 调 Store.InsertAdminAction
//     with actor='system', action='plugin_<event>', target=<agent_id>,
//     metadata=JSON{plugin_id, reason, ...}. audit forward-only 跟
//     ADM-2.1 + AP-2 + BPP-4 watchdog 跨四 milestone 同精神 (锁链第 5 处).
//   - **② reason 复用 AL-1a 6-dict** — heartbeat_timeout reason=
//     reasons.NetworkUnreachable; cold_start reason=reasons.RuntimeCrashed
//     byte-identical 跟 BPP-6 #522 + BPP-7 SDK 同源. AL-1a reason 锁链
//     BPP-8 = 第 13 处.
//   - **④ single-gate** — 5 method 全走此 auditor; grep 检查
//     `InsertAdminAction.*"plugin_` 在 lifecycle_audit.go 外 0 hit.
//   - **⑥ best-effort** — fire-and-forget (log.Warn on InsertAdminAction
//     error, 不 fail handler); 无 retry queue / 无持久化 deferred audit
//     (跟 BPP-4/5/6/7 best-effort 设计沿用, AST 锁链延伸第 5 处).
//   - **⑦ actor='system' byte-identical** — 跟 BPP-4 watchdog + AP-2
//     sweeper actor='system' 跨五 milestone 同源.
//
// 反约束 (acceptance §3):
//   - admin god-mode 不挂 SDK 路径 (ADM-0 §1.3 红线, lifecycle GET endpoint
//     在 internal/api/bpp_8_lifecycle_list.go owner-only, admin-api 不挂).
//   - AST scan forbidden: `pendingLifecycleAudit\|lifecycleQueue\|
//     deadLetterLifecycle` 0 hit (TestBPP83_NoLifecycleQueueOrAuditTable).

package bpp

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"borgee-server/internal/agent/reasons"
)

// LifecycleSystemActor — actor_id 字面 byte-identical 跟 BPP-4 watchdog +
// AP-2 sweeper actor='system' 跨五 milestone 同源 (设计 ⑦). 改 = 改 BPP-4
// + AP-2 同步.
const LifecycleSystemActor = "system"

// Action constants — admin_actions CHECK enum 5 条 plugin_* 字面
// byte-identical 跟 migration v=31 (bpp_8_1_admin_actions_plugin_actions.go)
// CHECK 字面同源. 改 = 改 migration CHECK + 此 const + acceptance §1
// 同步.
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
// helper — bpp 包不直 import store 业务边界, 走 interface 注入 (跟
// BPP-3/4/5/6 同模式).
type LifecycleAuditStore interface {
	InsertAdminAction(actorID, targetUserID, action, metadata string) (string, error)
}

// AdminActionsLifecycleAuditor implements LifecycleAuditor by writing
// rows to admin_actions via Store.InsertAdminAction. Construct via
// NewAdminActionsLifecycleAuditor (nil store / nil logger panics —
// boot bug, 跟 BPP-3/4/5/6 同模式 ctor pattern).
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
// (best-effort 设计 ⑥, fire-and-forget).
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
// 设计 ② reason 复用 AL-1a 6-dict — caller passes restartReason (typically
// reasons.RuntimeCrashed byte-identical 跟 BPP-6 + BPP-7 SDK 同源, AL-1a
// reason 锁链第 13 处). 反向断言: caller 必走 reasons.* const 不
// hardcode "runtime_crashed" 字符串.
func (a *AdminActionsLifecycleAuditor) RecordColdStart(pluginID, agentID, restartReason string) {
	a.recordEvent(LifecycleActionColdStart, agentID, map[string]any{
		"plugin_id":      pluginID,
		"restart_reason": restartReason,
	})
}

// RecordHeartbeatTimeout — BPP-4 #499 watchdog hook.
//
// 设计 ② reason 字面 byte-identical=reasons.NetworkUnreachable (跟 BPP-4
// watchdog SetError reason byte-identical, AL-1a 锁链第 13 处).
func (a *AdminActionsLifecycleAuditor) RecordHeartbeatTimeout(pluginID, agentID string) {
	a.recordEvent(LifecycleActionHeartbeatTimeout, agentID, map[string]any{
		"plugin_id": pluginID,
		"reason":    reasons.NetworkUnreachable, // AL-1a 锁链第 13 处 byte-identical
	})
}

// Compile-time assertion that AdminActionsLifecycleAuditor implements
// LifecycleAuditor (reverse-grep guard for interface drift).
var _ LifecycleAuditor = (*AdminActionsLifecycleAuditor)(nil)

// formatColdStartReason — convenience for callers: returns
// reasons.RuntimeCrashed (the 6-dict byte-identical literal). Exposed
// so test harnesses can assert the literal without importing reasons.*
// twice. Returns string for direct use in RecordColdStart.
func formatColdStartReason() string { return reasons.RuntimeCrashed }

// _ used only as a documentation hook.
var _ = fmt.Sprintf
