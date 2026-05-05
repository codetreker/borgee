// Package bpp — heartbeat_watchdog.go: BPP-4.1 plugin liveness 监测 +
// 状态翻转 (lastSeenAt > 30s → mark agent error/network_unreachable).
//
// Blueprint锚: docs/blueprint/current/plugin-protocol.md §1.6 (失联与故障状态 +
// 故障 UX 区分表 — "runtime_disconnected" 平台问题). Spec brief:
// docs/implementation/modules/bpp-4-spec.md §0.2. Acceptance:
// docs/qa/acceptance-templates/bpp-4.md §1.
//
// 立场 (跟 stance §1+§2 byte-identical):
//   - **Borgee 不取消 in-flight 任务** (蓝图 §1.6 字面). watchdog 仅触发
//     状态翻转, 不下发 cancel/abort/kill frame. 反向 grep `cancel.*task\|
//     abort.*inflight\|server.*kill.*runtime` 0 hit 守门.
//   - **30s 单源阈值锁** (跟蓝图 BPP-4 module acceptance "kill plugin →
//     30s 内 agent 显示 error" byte-identical). 改 = 改三处单测锁
//     (此常量 + bpp-4-spec.md §0.2 + content-lock §1.①).
//   - **AL-1a 6-dict reason 不扩第 7** — watchdog 触发的 reason 选既有
//     `network_unreachable` (跟蓝图 §1.6 故障 UX 区分表 "runtime_disconnected
//     → 重连中…" 文案对齐, 平台层判网络失联). BPP-4 = AL-1a reason 字典
//     第 9 处单测锁链 (跟 BPP-2.2 #485 第 7 处 + AL-2b #481 第 8 处链承袭).
//
// 反约束 (acceptance §4):
//   - 不直写 presence_sessions 列 (AL-1b 边界守, watchdog 走 agent.Tracker
//     SetError SSOT, 跟 #457 PATCH endpoint 同 source).
//   - 不开新 BPP envelope frame (whitelist 不变, BPP-4 仅复用 HeartbeatFrame
//     做 watchdog 触发源).
//   - admin god-mode 不入 watchdog (admin 不持有 PluginConn).

package bpp

import (
	"context"
	"log/slog"
	"time"

	agentpkg "borgee-server/internal/agent"
)

// BPP_HEARTBEAT_TIMEOUT_SECONDS — single source of truth for the
// plugin heartbeat liveness threshold. Byte-identical 跟蓝图 BPP-4
// module acceptance "kill plugin → 30s 内 agent 显示 error" 字面.
//
// 改 = 改三处:
//   1. 此常量
//   2. docs/implementation/modules/bpp-4-spec.md §0.2
//   3. docs/qa/bpp-4-content-lock.md §1.①
//
// 反向 grep CI lint 守: `bpp.*heartbeat.*60|heartbeat.*timeout.*[5-9][0-9]+s`
// count==0 (防隐式调高).
const BPP_HEARTBEAT_TIMEOUT_SECONDS = 30

// BPP_HEARTBEAT_TICKER_INTERVAL — watchdog 扫描周期, 必须 ≤ 阈值/3 防错
// 过窗口. 跟蓝图 §1.6 "缺心跳按未知" 立场承袭 (检测延迟 ≤ 阈值的容差).
const BPP_HEARTBEAT_TICKER_INTERVAL = 10 * time.Second

// PluginLivenessSource — interface seam, hub.go 实现, watchdog 消费.
// 跟 BPP-3 PluginFrameRouter / BPP-2.1 ActionHandler / cv-4.2
// IterationStatePusher 同 interface seam 模式 — bpp 包不 import
// internal/ws.
//
// SnapshotLastSeen returns a copy of the per-plugin lastSeenAt map
// (key = agent_id, value = last frame/ping receive time). Empty map
// means no plugins registered. Implementation MUST be safe for
// concurrent calls (watchdog ticker + connect/disconnect).
type PluginLivenessSource interface {
	SnapshotLastSeen() map[string]time.Time
}

// AgentErrorSink — interface seam to *agent.Tracker.SetError. bpp 包
// 不 import internal/agent at the package boundary; the wire-up at
// server boot injects the concrete tracker. Same seam pattern as
// PluginLivenessSource above.
type AgentErrorSink interface {
	SetError(agentID, reason string)
}

// HeartbeatWatchdog periodically checks plugin liveness against the
// 30s threshold. When a plugin's lastSeenAt is older than the threshold,
// the watchdog marks the corresponding agent as error/network_unreachable
// via the AgentErrorSink (跟 AL-1a 6-dict reason 第 9 处单测锁链承袭).
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
// 反约束: Run 仅触发 SetError; **不**调任何 cancel/abort/kill 路径
// (蓝图 §1.6 立场 ① — server 不取消 in-flight 任务).
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
// 6-dict). 改 = 改九处单测锁:
//   1. internal/agent/state.go (#249 source-of-truth)
//   2. internal/agent/state_test.go
//   3. AL-3 #305 / CV-4 #380 / AL-2a #454 / AL-1b #458 / AL-4 #387/#461
//   4. internal/bpp/agent_config_ack_dispatcher.go (#481 AL-2b 第 8 处)
//   5. internal/bpp/heartbeat_watchdog.go (本文件, BPP-4 第 9 处)
//
// 反约束: BPP-4 不另起 reason 字典 — `runtime_disconnected` 字面**不**
// 入代码层 (蓝图 §1.6 故障 UX 区分表 "runtime_disconnected → 重连中…"
// 是 client 侧 UI 文案, server 侧仍走 AL-1a 6-dict `network_unreachable`).
