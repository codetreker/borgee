// Package agent implements the AL-1a (#R3 Phase 2 start) agent runtime
// three-state model.
//
// Phase 2 only commits to online / offline plus the error-side path. busy /
// idle depend on the BPP/Phase 4 task_started / task_finished frames; without
// BPP they would be stubs that v1 would later remove. See docs/blueprint/
// agent-lifecycle.md §2.3 and the 2026-04-28 four-person review #5 decision.
//
// Design:
//
//   - online / offline are derived from hub plugin presence (GetPlugin(id) != nil)
//     and are not stored in Tracker, so reconnects do not create a mismatch window.
//   - error is Tracker's only retained state. SetError(id, reason) comes from
//     the runtime error-side path (HTTP 500 / api_key_invalid /
//     network_unreachable / runtime_crashed). Clear(id) runs when RegisterPlugin
//     establishes a new connection.
//   - State is not persisted in Phase 2; AL-3 moves it to storage.
//
// Copy lock (#190 §11): "在线" / "已离线" / "故障 (api_key_invalid)" and related labels.
// 客户端见 packages/client/src/components/AgentManager.tsx + Sidebar.tsx.
package agent

import (
	"strings"
	"sync"
	"time"

	"borgee-server/internal/agent/reasons"
)

// RuntimeState is the Phase 2 three-state model.
type RuntimeState string

const (
	StateOnline  RuntimeState = "online"
	StateOffline RuntimeState = "offline"
	StateError   RuntimeState = "error"
)

// Reason codes describe runtime failure causes. The UI uses them to route users
// to the repair entry point (blueprint §2.3 key design).
// String values are bound to the client copy table; changing them also requires
// updating AgentManager.tsx reasonLabel.
//
// REFACTOR-REASONS: the single source of truth moved to internal/agent/reasons.
// This block only re-exports values to preserve byte-identical call semantics
// and existing import sites (api/al_1b_2_status.go / api/runtimes.go /
// api/iterations.go, etc.). Changing a literal means changing reasons.ALL and
// the eight synchronized tests. New code should import internal/agent/reasons directly.
const (
	ReasonAPIKeyInvalid      = reasons.APIKeyInvalid
	ReasonQuotaExceeded      = reasons.QuotaExceeded
	ReasonNetworkUnreachable = reasons.NetworkUnreachable
	ReasonRuntimeCrashed     = reasons.RuntimeCrashed
	ReasonRuntimeTimeout     = reasons.RuntimeTimeout
	ReasonUnknown            = reasons.Unknown
)

// Snapshot is the result of one state query. It marshals directly into the
// agent.state / agent.reason fields returned by GET /api/v1/agents.
type Snapshot struct {
	State     RuntimeState `json:"state"`
	Reason    string       `json:"reason,omitempty"`
	UpdatedAt int64        `json:"updated_at,omitempty"` // Unix ms; 0 means not updated yet (default offline)
}

// Tracker is a thread-safe in-memory map from agentID to error snapshot.
// online / offline are not stored in Tracker; they are derived from hub presence.
type Tracker struct {
	mu       sync.RWMutex
	errors   map[string]Snapshot
	now      func() time.Time // injectable for tests; defaults to time.Now.
}

// NewTracker constructs the production tracker.
func NewTracker() *Tracker {
	return &Tracker{
		errors: make(map[string]Snapshot),
		now:    time.Now,
	}
}

// SetError marks an agent as error. Empty reason falls back to ReasonUnknown.
func (t *Tracker) SetError(agentID, reason string) {
	if agentID == "" {
		return
	}
	if reason == "" {
		reason = ReasonUnknown
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.errors[agentID] = Snapshot{
		State:     StateError,
		Reason:    reason,
		UpdatedAt: t.now().UnixMilli(),
	}
}

// Clear removes the error state after a successful new connection or owner reset.
// Empty agentID is a no-op.
func (t *Tracker) Clear(agentID string) {
	if agentID == "" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.errors, agentID)
}

// Lookup returns an agent's error state. ok=false means there is no error record;
// callers should use hub presence to choose online vs offline.
func (t *Tracker) Lookup(agentID string) (Snapshot, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	s, ok := t.errors[agentID]
	return s, ok
}

// Resolve returns the final snapshot for an agentID and current plugin presence.
// Priority is error > online > offline: an error record displays error; without
// an error, plugin presence displays online; otherwise the result is offline.
//
// This is the only state query entry point for GET /api/v1/agents.
func (t *Tracker) Resolve(agentID string, hasPlugin bool) Snapshot {
	if s, ok := t.Lookup(agentID); ok {
		return s
	}
	if hasPlugin {
		return Snapshot{State: StateOnline}
	}
	return Snapshot{State: StateOffline}
}

// ClassifyProxyError is the runtime error-side classifier: it maps the
// (status, err) returned by ProxyPluginRequest to a reason code. Callers should
// call SetError when the returned reason is non-empty.
//
// Rules (blueprint §2.3 error-state reason codes):
//   - status == 401 or err contains "api key" → api_key_invalid
//   - status == 429 → quota_exceeded
//   - status >= 500 → runtime_crashed
//   - err contains "timeout" / "deadline" → runtime_timeout
//   - err contains "not connected" / "no route" → network_unreachable
//   - any other non-nil err → unknown
//   - no match → "" (no error).
func ClassifyProxyError(status int, err error) string {
	if err == nil && status < 400 {
		return ""
	}
	if status == 401 {
		return ReasonAPIKeyInvalid
	}
	if status == 429 {
		return ReasonQuotaExceeded
	}
	if err != nil {
		msg := strings.ToLower(err.Error())
		if containsAny(msg, "api key", "api_key", "unauthorized") {
			return ReasonAPIKeyInvalid
		}
		if containsAny(msg, "timeout", "deadline exceeded") {
			return ReasonRuntimeTimeout
		}
		if containsAny(msg, "not connected", "no route", "connection refused", "unreachable") {
			return ReasonNetworkUnreachable
		}
	}
	if status >= 500 {
		return ReasonRuntimeCrashed
	}
	if err != nil {
		return ReasonUnknown
	}
	return ""
}

// containsAny — true if s contains any of subs. Caller passes pre-lowercased s
// for case-insensitive matching.
func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
