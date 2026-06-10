// Package bpp — dead_letter.go: BPP-4.2 audit log for failed server→plugin
// pushes. Best effort: log a warning, do not write to a persistent queue, and
// rely on RT-1.3 cursor replay for recovery.
//
// Blueprint reference: docs/blueprint/current/plugin-protocol.md §1.5 (runtime
// does not cache frames) + RT-1.3 #296 cursor replay (after reconnect, the
// plugin actively pulls missing frames).
// Spec brief: docs/implementation/modules/bpp-4-spec.md §0.3 + §1
// BPP-4.2. Acceptance: docs/qa/acceptance-templates/bpp-4.md §2.
//
// Design contract (matching the design checklist §3):
//   - **ack is best-effort and is not resent** (inherits blueprint §1.5).
//     When a server→plugin push fails (sent=false, plugin offline), log a
//     warning plus an audit hint, but do not queue it. After reconnect, the
//     plugin uses RT-1.3 cursor replay to pull; the server does not proactively
//     resend.
//   - **dead-letter audit log schema is self-contained** (5 fields:
//     actor / action / target / when / scope). It was originally drafted to
//     match the planned HB-1/HB-2 host-bridge audit structs, but that
//     host-bridge/Helper rail was CUT (its tables dropped at migration v=54)
//     and those audit structs were never built — so this schema now stands on
//     its own. Any change must update the three test locks, matching HB-4 §1.5
//     release gate line 4, which locks the audit log format.
//
// Negative constraints (acceptance §4.3):
//   - Reverse grep `pendingAcks\|retryQueue\|deadLetterQueue\|ackTimeout.*resend`
//     must return 0 hits. The CI lint prevents an implicit v2 retry path from
//     moving into this layer.
//   - Reverse grep `time.*Ticker.*resend\|retry.*frame.*backoff` must return 0
//     hits. This file has no ticker and no retry; it only logs and returns.

package bpp

import (
	"log/slog"
)

// DeadLetterAuditEntry is the 5-field audit log schema. It was originally
// drafted to share a shape with the planned HB-1 install-butler and HB-2
// host-bridge IPC audit structs, but that host-bridge/Helper rail was CUT
// (tables dropped at migration v=54) and neither audit struct was ever built,
// so this schema is now standalone.
//
// Any change must update:
//  1. This struct's field names and JSON tags (BPP-4).
//  2. The three BPP-4 test locks.
//
// HB-4 §1.5 release gate line 4 enforces the same contract: locked audit-log
// JSON schema, including actor / action / target / when / scope.
type DeadLetterAuditEntry struct {
	Actor  string `json:"actor"`  // "server" (only BPP-4 dead-letter actor)
	Action string `json:"action"` // "frame_drop"
	Target string `json:"target"` // "<agent_id>"
	When   int64  `json:"when"`   // Unix ms
	Scope  string `json:"scope"`  // "<frame_type>:cursor=<cursor>"
}

// LogFrameDroppedPluginOffline is the single dead-letter entry point. Push
// failure paths such as al_2b_2_agent_config_push.go call it when sent=false.
//
// It does not write to a persistent queue, does not resend, and does not start
// a timer; it only logs a warning plus an audit hint. After reconnect, the
// plugin uses RT-1.3 #296 cursor replay to actively pull missing frames.
//
// log key `bpp.frame_dropped_plugin_offline` must stay aligned with
// content-lock §1.③. Any change must update this function, content-lock, and
// acceptance.
func LogFrameDroppedPluginOffline(logger *slog.Logger, entry DeadLetterAuditEntry) {
	if logger == nil {
		return
	}
	logger.Warn("bpp.frame_dropped_plugin_offline",
		"actor", entry.Actor,
		"action", entry.Action,
		"target", entry.Target,
		"when", entry.When,
		"scope", entry.Scope)
}
