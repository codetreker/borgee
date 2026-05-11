// Package grants defines the HB-2 read-only consumer interface for the HB-3
// host_grants single source (hb-2-spec.md §3.2). HB-2 only reads grants; HB-3
// owns the schema and dialog write path. v0(C) provides a mock implementation,
// and the landed HB-3 path uses the SQLite consumer.
//
// Negative constraint (hb-2-spec.md §4 #3): no caching. Each IPC call must
// re-query; grep check `grantsCache|cachedGrants` returns 0 hit. This protects
// the revoke < 100ms HB-4 release gate.
package grants

import (
	"context"
	"sync"
	"time"
)

// Grant is a read-only view of an HB-3 host_grants row.
type Grant struct {
	AgentID   string // agent that holds the grant (source for cross-agent ACL)
	Scope     string // e.g. "fs:/Users/me/projects" or "egress:api.example.com"
	TTLUntil  int64  // unix millis; 0 means no expiration (ttl_kind=always)
	GrantedAt int64  // unix millis
}

// Consumer is the read-only query interface for the HB-3 grants store. The
// HB-2 daemon does not cache; each IPC call re-queries. The landed HB-3 path is
// SQLite-backed.
type Consumer interface {
	// Lookup returns the grant for (agent_id, scope); absence returns (zero, false).
	Lookup(ctx context.Context, agentID, scope string) (Grant, bool, error)
}

// MemoryConsumer is the v0(C) in-memory mock; production uses the SQLite consumer.
type MemoryConsumer struct {
	mu    sync.RWMutex
	rows  map[string]Grant
	nowFn func() int64
}

// NewMemoryConsumer constructs the mock; tests may replace the default clock.
func NewMemoryConsumer() *MemoryConsumer {
	return &MemoryConsumer{
		rows:  map[string]Grant{},
		nowFn: func() int64 { return time.Now().UnixMilli() },
	}
}

// SetNowFn injects a clock for TTL boundary tests.
func (m *MemoryConsumer) SetNowFn(f func() int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nowFn = f
}

// Put inserts a grant for mock setup; the HB-3 SQLite consumer does not expose this API.
func (m *MemoryConsumer) Put(g Grant) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rows[g.AgentID+"|"+g.Scope] = g
}

// Delete removes a mock grant; HB-3 production revoke stamps revoked_at plus audit.
func (m *MemoryConsumer) Delete(agentID, scope string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.rows, agentID+"|"+scope)
}

// Lookup checks (agent_id, scope). An expired TTL returns allowed=false;
// callers that need not_found vs expired use the ACL gate reason path.
func (m *MemoryConsumer) Lookup(_ context.Context, agentID, scope string) (Grant, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	g, ok := m.rows[agentID+"|"+scope]
	if !ok {
		return Grant{}, false, nil
	}
	if g.TTLUntil != 0 && g.TTLUntil <= m.nowFn() {
		return g, false, nil // row exists but expired; caller uses grant_expired reason
	}
	return g, true, nil
}

// LookupRaw returns (grant, exists, expired, err) for (agent_id, scope), letting
// callers distinguish grant_not_found from grant_expired reason.
func (m *MemoryConsumer) LookupRaw(_ context.Context, agentID, scope string) (Grant, bool, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	g, ok := m.rows[agentID+"|"+scope]
	if !ok {
		return Grant{}, false, false, nil
	}
	if g.TTLUntil != 0 && g.TTLUntil <= m.nowFn() {
		return g, true, true, nil
	}
	return g, true, false, nil
}
