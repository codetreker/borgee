// Package grants — HB-2 read-only consumer interface for HB-3 host_grants
// 单一来源 (hb-2-spec.md §3.2). HB-2 不写 grants — 仅查; HB-3 持 schema +
// 弹窗写路径. v0(C) 提供 mock impl, HB-3 落地后真接 SQLite consumer.
//
// 反向约束 (hb-2-spec.md §4 #3): 不缓存 — 每次 IPC call 重查; grep 检查
// `grantsCache|cachedGrants` 0 hit (撤销 < 100ms 锁定 HB-4 release gate).
package grants

import (
	"context"
	"sync"
	"time"
)

// Grant 是 HB-3 host_grants 表行 (read-only view).
type Grant struct {
	AgentID   string // 持 grant 的 agent (cross-agent ACL 出处)
	Scope     string // e.g. "fs:/Users/me/projects" 或 "egress:api.example.com"
	TTLUntil  int64  // unix millis; 0 表无 TTL (永久, v1 不支持)
	GrantedAt int64  // unix millis
}

// Consumer 是 HB-3 grants store 的 read-only 查询接口 (HB-2 daemon 不
// 缓存; 每次 IPC call 重查; HB-3 落地后由 SQLite-backed 实现).
type Consumer interface {
	// Lookup 按 (agent_id, scope) 查 grant; 不存在返回 (zero, false).
	Lookup(ctx context.Context, agentID, scope string) (Grant, bool, error)
}

// MemoryConsumer 是 v0(C) in-memory mock; HB-3 落地后替换 SQLite consumer.
type MemoryConsumer struct {
	mu    sync.RWMutex
	rows  map[string]Grant
	nowFn func() int64
}

// NewMemoryConsumer 构造 mock; nowFn 默认走 time.Now().UnixMilli (test 可注入).
func NewMemoryConsumer() *MemoryConsumer {
	return &MemoryConsumer{
		rows:  map[string]Grant{},
		nowFn: func() int64 { return time.Now().UnixMilli() },
	}
}

// SetNowFn 注入时间源 (单测 TTL 边界用).
func (m *MemoryConsumer) SetNowFn(f func() int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nowFn = f
}

// Put 插入 grant (mock 准备数据用; HB-3 SQLite consumer 不暴露此 API).
func (m *MemoryConsumer) Put(g Grant) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.rows[g.AgentID+"|"+g.Scope] = g
}

// Delete 撤销 grant (mock; HB-3 真撤销走 SQL DELETE + audit).
func (m *MemoryConsumer) Delete(agentID, scope string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.rows, agentID+"|"+scope)
}

// Lookup 按 (agent_id, scope) 查 — TTL 过期返回 (zero, false) 不返回行
// (caller 区分 not_found / expired 走 ACL gate 同源 reason).
func (m *MemoryConsumer) Lookup(_ context.Context, agentID, scope string) (Grant, bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	g, ok := m.rows[agentID+"|"+scope]
	if !ok {
		return Grant{}, false, nil
	}
	if g.TTLUntil != 0 && g.TTLUntil <= m.nowFn() {
		return g, false, nil // 行存在但 expired (caller 走 grant_expired reason)
	}
	return g, true, nil
}

// LookupRaw 按 (agent_id, scope) 返回 (grant, exists, expired, err) — caller
// 据此区分 grant_not_found vs grant_expired reason.
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
