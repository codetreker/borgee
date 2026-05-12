// Package throttle enforces the B.1 throttling invariant: at most one
// offline-mention system message per (channel_id, agent_id) within
// ThrottleWindow. G2.3 (#221) from #229.
// v0 uses an in-memory map + Mutex; data-layer.md row 75 reserves v1 storage
// for this policy. Keeping the decision inside Allow keeps that storage swap
// local. The throttle is not in the ws hub because G2.6's BPP schema lock keeps
// policy outside transport.
package throttle

import (
	"sync"
	"time"

	"borgee-server/internal/testutil/clock"
)

// ThrottleWindow is pinned by concept-model.md §4.1 (B.1). REG-CHECK audits it
// with grep; do not inline this duration at call sites.
const ThrottleWindow = 5 * time.Minute

type key struct{ channelID, agentID string }

// Throttle: per-(channel, agent) suppression. Concurrent-safe.
type Throttle struct {
	mu    sync.Mutex
	clock clock.Clock
	last  map[key]time.Time
}

// New uses clock.NewReal() in production and clock.NewFake() in tests. G2.3
// blocking constraint #4 rejects test sleeps over 100ms because they slow CI.
func New(c clock.Clock) *Throttle {
	if c == nil {
		c = clock.NewReal()
	}
	return &Throttle{clock: c, last: make(map[key]time.Time)}
}

// Allow returns true on first call for a (channel, agent) pair, then false
// until ThrottleWindow elapses since the previous accepted call. Two-dimensional
// isolation means distinct channel_id OR agent_id values get independent windows.
func (t *Throttle) Allow(channelID, agentID string) bool {
	now := t.clock.Now()
	t.mu.Lock()
	defer t.mu.Unlock()
	k := key{channelID, agentID}
	if last, ok := t.last[k]; ok && now.Sub(last) < ThrottleWindow {
		return false
	}
	t.last[k] = now
	return true
}
