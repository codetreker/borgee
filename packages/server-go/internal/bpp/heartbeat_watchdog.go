// Package bpp — heartbeat_watchdog.go: BPP-4.1 plugin liveness check and
// state transition (lastSeenAt > 30s → mark agent error/network_unreachable).
//
// Blueprint reference: docs/blueprint/current/plugin-protocol.md §1.6
// (disconnected and failure states + failure UX distinction table for the
// platform-level "runtime_disconnected" case). Spec brief:
// docs/implementation/modules/bpp-4-spec.md §0.2. Acceptance:
// docs/qa/acceptance-templates/bpp-4.md §1.
//
// Principles (matching BPP-4 principles §1+§2):
//   - **Borgee does not cancel in-flight tasks** (blueprint §1.6). The
//     watchdog only triggers a state transition; it does not send a
//     cancel/abort/kill frame. Reverse grep `cancel.*task\|abort.*inflight\|
//     server.*kill.*runtime` must return 0 hits.
//   - **30s threshold has one source of truth** (matching blueprint
//     BPP-4 module acceptance "kill plugin → 30s 内 agent 显示 error"). Any
//     change must update three locks together: this constant,
//     bpp-4-spec.md §0.2, and content-lock §1.①.
//   - **Do not add a 7th AL-1a reason**. The watchdog uses the existing
//     `network_unreachable` reason, aligned with the blueprint §1.6 failure UX
//     row "runtime_disconnected → 重连中…" for platform-level network loss.
//     BPP-4 is the 9th test lock in the AL-1a reason chain, after BPP-2.2
//     #485 and AL-2b #481.
//
// Negative constraints (acceptance §4):
//   - Do not write presence_sessions columns directly. AL-1b keeps that
//     boundary; the watchdog uses agent.Tracker.SetError as the single source
//     of truth, matching the #457 PATCH endpoint source.
//   - Do not add a new BPP envelope frame. The allow-list is unchanged; BPP-4
//     only reuses HeartbeatFrame as the watchdog trigger source.
//   - Admin users do not enter the watchdog path because admins do not hold a
//     PluginConn.

package bpp

import (
	"context"
	"log/slog"
	"time"

	agentpkg "borgee-server/internal/agent"
)

// BPP_HEARTBEAT_TIMEOUT_SECONDS — single source of truth for the
// plugin heartbeat liveness threshold. Matches blueprint BPP-4
// module acceptance "kill plugin → 30s 内 agent 显示 error".
//
// Any change must update three places:
//  1. This constant.
//  2. docs/implementation/modules/bpp-4-spec.md §0.2
//  3. docs/qa/bpp-4-content-lock.md §1.①
//
// Reverse grep CI lint: `bpp.*heartbeat.*60|heartbeat.*timeout.*[5-9][0-9]+s`
// count==0, preventing an implicit threshold increase.
const BPP_HEARTBEAT_TIMEOUT_SECONDS = 30

// BPP_HEARTBEAT_TICKER_INTERVAL is the watchdog scan interval. It must be <=
// threshold/3 so the watchdog does not miss the timeout window. This matches
// blueprint §1.6 "缺心跳按未知": detection delay is tolerated only up to the
// threshold.
const BPP_HEARTBEAT_TICKER_INTERVAL = 10 * time.Second

// PluginLivenessSource is the interface boundary implemented by hub.go and
// consumed by the watchdog. This follows the same interface-boundary pattern as
// BPP-3 PluginFrameRouter, BPP-2.1 ActionHandler, and cv-4.2
// IterationStatePusher; the bpp package does not import internal/ws.
//
// SnapshotLastSeen returns a copy of the per-plugin lastSeenAt map
// (key = agent_id, value = last frame/ping receive time). Empty map
// means no plugins registered. Implementation MUST be safe for
// concurrent calls (watchdog ticker + connect/disconnect).
type PluginLivenessSource interface {
	SnapshotLastSeen() map[string]time.Time
}

// AgentErrorSink is the interface boundary to *agent.Tracker.SetError. The bpp
// package does not import internal/agent at the package boundary; server boot
// wire-up injects the concrete tracker. Same boundary pattern as
// PluginLivenessSource.
type AgentErrorSink interface {
	SetError(agentID, reason string)
}

// HeartbeatWatchdog periodically checks plugin liveness against the
// 30s threshold. When a plugin's lastSeenAt is older than the threshold,
// the watchdog marks the corresponding agent as error/network_unreachable
// via the AgentErrorSink. This is the 9th test lock in the AL-1a 6-dict reason
// chain.
//
// Construction: NewHeartbeatWatchdog(source, sink, logger). Run(ctx)
// blocks until ctx is cancelled (typically run from server boot in a
// goroutine, mirrors hub.StartHeartbeat shape).
type HeartbeatWatchdog struct {
	source    PluginLivenessSource
	sink      AgentErrorSink
	logger    *slog.Logger
	now       func() time.Time
	threshold time.Duration

	// markedErr tracks agents already flipped to error to avoid spammy
	// repeated SetError calls every tick while plugin remains offline.
	// Cleared when the plugin reconnects (lastSeenAt advances past the
	// threshold).
	markedErr map[string]bool
}

// NewHeartbeatWatchdog wires source + sink + logger. logger may be nil
// (defaults to discard, useful in unit tests with a captured handler).
func NewHeartbeatWatchdog(source PluginLivenessSource, sink AgentErrorSink, logger *slog.Logger) *HeartbeatWatchdog {
	if source == nil {
		panic("bpp: NewHeartbeatWatchdog source must not be nil")
	}
	if sink == nil {
		panic("bpp: NewHeartbeatWatchdog sink must not be nil")
	}
	return &HeartbeatWatchdog{
		source:    source,
		sink:      sink,
		logger:    logger,
		now:       time.Now,
		threshold: time.Duration(BPP_HEARTBEAT_TIMEOUT_SECONDS) * time.Second,
		markedErr: make(map[string]bool),
	}
}

// Run blocks until ctx is cancelled. Ticker fires every
// BPP_HEARTBEAT_TICKER_INTERVAL; each tick scans the source's
// lastSeenAt snapshot and flips stale agents to error.
//
// Negative constraint: Run only triggers SetError; it does not call any
// cancel/abort/kill path (blueprint §1.6 principle ①: the server does not
// cancel in-flight tasks).
func (w *HeartbeatWatchdog) Run(ctx context.Context) {
	ticker := time.NewTicker(BPP_HEARTBEAT_TICKER_INTERVAL)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.scanOnce()
		}
	}
}

// scanOnce performs a single liveness scan + state flip pass. Exported
// to package via lower-case for test injection (see heartbeat_watchdog_test.go
// fake clock + manual tick simulation).
func (w *HeartbeatWatchdog) scanOnce() {
	now := w.now()
	snap := w.source.SnapshotLastSeen()
	stillAlive := make(map[string]bool, len(snap))
	for agentID, lastSeen := range snap {
		if now.Sub(lastSeen) > w.threshold {
			if !w.markedErr[agentID] {
				w.sink.SetError(agentID, agentpkg.ReasonNetworkUnreachable)
				w.markedErr[agentID] = true
				if w.logger != nil {
					w.logger.Warn("bpp.heartbeat_timeout",
						"agent_id", agentID,
						"last_seen_ms_ago", now.Sub(lastSeen).Milliseconds(),
						"reason", agentpkg.ReasonNetworkUnreachable)
				}
			}
		} else {
			stillAlive[agentID] = true
		}
	}
	// Reconnect / new heartbeat received → clear markedErr so the next
	// disconnect cycle re-flips. (Tracker.Clear is called separately on
	// RegisterPlugin, BPP-4 watchdog only owns the disconnect direction;
	// reconnect flow stays in hub.RegisterPlugin → tracker.Clear path.)
	for agentID := range w.markedErr {
		if stillAlive[agentID] {
			delete(w.markedErr, agentID)
		}
	}
}

// ReasonNetworkUnreachable: BPP-4 watchdog uses
// `agentpkg.ReasonNetworkUnreachable` directly (single source of truth
// in `internal/agent/state.go::ReasonNetworkUnreachable`, AL-1a #249
// 6-dict). Any change must update nine test locks:
//   1. internal/agent/state.go (#249 source-of-truth)
//   2. internal/agent/state_test.go
//   3. AL-3 #305 / CV-4 #380 / AL-2a #454 / AL-1b #458 / AL-4 #387/#461
//   4. internal/bpp/agent_config_ack_dispatcher.go (#481 AL-2b 第 8 处)
//   5. internal/bpp/heartbeat_watchdog.go (本文件, BPP-4 第 9 处)
//
// Negative constraint: BPP-4 does not add another reason dictionary. The
// `runtime_disconnected` literal does not enter server code; blueprint §1.6
// failure UX row "runtime_disconnected → 重连中…" is client-side UI wording,
// while the server still uses AL-1a 6-dict `network_unreachable`.
