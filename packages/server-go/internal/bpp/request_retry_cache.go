// Package bpp — request_retry_cache.go: BPP-3.2.3 plugin SDK in-memory
// retry cache for permission_denied → owner grant → auto-retry flow.
//
// Blueprint出处: docs/blueprint/current/auth-permissions.md §1.3 主入口字面跟随
// + plugin-protocol.md §1.6 失联与故障状态. Spec: bpp-3.2-spec.md §1
// 原则 ③ + bpp-3.2-stance §3 + content-lock §4 错码字面锁定.
//
// Behaviour contract (反向约束 spec §3 #3 + content-lock §4):
//
//   1. TTL 5 min — entries expire on read (lazy GC); 防 cache 膨胀.
//   2. ≤3 次重试 (MaxPermissionRetries const, 反向 grep MaxPermissionRetries.*[4-9]
//      在 packages/plugin-sdk/ count==0).
//   3. 30s 固定退避 (RetryBackoff const, 反向 grep `expBackoff|exponential.*retry`
//      count==0 — 蓝图 §1.6 字面 server-side timing 单一来源, plugin 端不
//      增添新 timing 信号).
//   4. 上限超 → `bpp.retry_exhausted` 错码 byte-identical 跟 content-lock §4
//      (改 = 改两处: content-lock + 此 const).
//   5. 不复用 BPP-4 server-side watchdog 队列 — 真分清三路径 原则 §3 +
//      spec §3 #3. CI lint reverse-grep 等价单测守 (见
//      request_retry_cache_test.go).
//
// 反向约束注意:
//   - Cache state lives in plugin SDK process memory; server stateless.
//   - retry trigger: 仅 `agent_config_update` frame (BPP-2.3) → cache 扫
//     (跟原则 ②⑧ 复用既有 frame, 不另起 capability_granted).
//   - admin god-mode 不入此路径 (admin 不通过 plugin SDK upload semantic
//     action).

package bpp

import (
	"errors"
	"sync"
	"time"
)

// MaxPermissionRetries is the upper bound on retry attempts per request_id
// (content-lock §4 + bpp-3.2-spec.md §1 原则 ③). After this count is
// reached, the next ShouldRetry call returns ErrRetryExhausted.
//
// 反向 grep CI lint: `MaxPermissionRetries.*[4-9]` count==0 (锁定 ≤3).
const MaxPermissionRetries = 3

// RetryBackoff is the FIXED retry interval (content-lock §4 + spec §1
// 原则 ③). Not exponential — 蓝图 plugin-protocol.md §1.6 字面
// server-side timing 单一来源, plugin 端不增添新 timing 信号.
//
// 反向 grep CI lint: `expBackoff|exponential.*retry` count==0.
const RetryBackoff = 30 * time.Second

// RequestRetryCacheTTL is the cache entry TTL — entries older than 5 min
// are reaped on read (lazy GC). Defends against memory growth when
// owner never responds to permission_denied DM.
const RequestRetryCacheTTL = 5 * time.Minute

// RetryExhaustedErrCode is the error code emitted when MaxPermissionRetries
// is exceeded. byte-identical 跟 docs/qa/bpp-3.2-content-lock.md §4
// (改 = 改两处: content-lock + 此 const).
//
// 跟 BPP-2.2 bpp.task_subject_empty / BPP-2.3 bpp.config_field_disallowed /
// BPP-3.2.1 bpp.grant_capability_disallowed 命名同模式.
const RetryExhaustedErrCode = "bpp.retry_exhausted"

// ErrRetryExhausted sentinel returned by RequestRetryCache.ShouldRetry
// when the request has exceeded MaxPermissionRetries. Callers map to
// wire-level error code via IsRetryExhausted (跟 IsSemanticOpUnknown 同模式).
var ErrRetryExhausted = errors.New("bpp: retry exhausted (MaxPermissionRetries reached)")

// IsRetryExhausted lets callers map the sentinel to the wire-level
// error code RetryExhaustedErrCode without exporting cache state.
func IsRetryExhausted(err error) bool {
	return errors.Is(err, ErrRetryExhausted)
}

// RetryEntry is a single in-flight permission_denied retry record.
// Stored in RequestRetryCache keyed by request_id (BPP-3.1 frame
// trace UUID byte-identical 跟 PermissionDeniedFrame.RequestID +
// CapabilityGrantPayload.RequestID 跨 PR 脱节 同源).
type RetryEntry struct {
	RequestID    string    // BPP-3.1 frame trace UUID
	AgentID      string    // target agent (跟 frame.AgentID 同源)
	Capability   string    // capability denied (跟 frame.RequiredCapability 同源)
	Scope        string    // scope (跟 frame.CurrentScope 同源)
	AttemptCount int       // 已重试次数 (0 = 初次写入未重试)
	NextRetryAt  time.Time // 下次允许重试的时间 (= now + RetryBackoff)
	CreatedAt    time.Time // entry 写入时间 (TTL 比较)
}

// RequestRetryCache is the plugin SDK in-memory permission_denied retry
// cache. Thread-safe (mutex-guarded map).
//
// Lifecycle:
//   1. plugin 收 BPP-3.1 PermissionDeniedFrame → caller 调 Add(entry).
//   2. plugin 收 BPP-2.3 AgentConfigUpdateFrame (owner grant 后 server
//      推) → caller 调 ShouldRetry(requestID, now); 返 (true, nil) 即
//      可重试 + AttemptCount++; (false, ErrRetryExhausted) 即上限超.
//   3. 重试成功 → caller 调 Remove(requestID) 清 entry.
//
// 反向约束: cache 不持久化 (server stateless 守); cache state 仅 plugin
// SDK 进程内存; 进程重启 = cache 清空 (plugin 重连后 owner 端 DM 仍
// 有效, 用户手动重试可走新 request_id).
type RequestRetryCache struct {
	mu      sync.Mutex
	entries map[string]*RetryEntry
	now     func() time.Time // injectable clock for tests
}

// NewRequestRetryCache constructs a cache with real wall-clock time.
// Tests should use NewRequestRetryCacheWithClock for determinism.
func NewRequestRetryCache() *RequestRetryCache {
	return &RequestRetryCache{
		entries: make(map[string]*RetryEntry),
		now:     time.Now,
	}
}

// NewRequestRetryCacheWithClock constructs a cache with an injectable
// clock function (test seam). Used by request_retry_cache_test.go to
// pin TTL + RetryBackoff timing without sleeping.
func NewRequestRetryCacheWithClock(clock func() time.Time) *RequestRetryCache {
	return &RequestRetryCache{
		entries: make(map[string]*RetryEntry),
		now:     clock,
	}
}

// Add registers a new permission_denied entry in the cache. Idempotent
// on requestID — re-adding the same key resets AttemptCount to 0 (fresh
// re-issue from owner side, e.g. after grant + revoke + re-deny).
func (c *RequestRetryCache) Add(entry *RetryEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	entry.CreatedAt = now
	entry.NextRetryAt = now.Add(RetryBackoff)
	entry.AttemptCount = 0
	c.entries[entry.RequestID] = entry
}

// ShouldRetry checks whether a request_id is eligible for retry NOW.
// Returns (entry, nil) if all gates pass (cache hit + TTL valid +
// retry budget not exhausted + backoff window elapsed). Side-effect:
// on (true, nil), AttemptCount++ and NextRetryAt is bumped by RetryBackoff.
//
// Possible (entry, err) pairs:
//   - (entry, nil): caller MAY retry now; AttemptCount has been bumped.
//   - (nil, ErrRetryExhausted): request_id exists but AttemptCount >=
//     MaxPermissionRetries; entry is REMOVED from cache (terminal).
//   - (nil, nil): cache miss OR entry TTL-expired (lazy GC) OR backoff
//     window not yet elapsed; caller should not retry.
func (c *RequestRetryCache) ShouldRetry(requestID string) (*RetryEntry, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	entry, ok := c.entries[requestID]
	if !ok {
		return nil, nil
	}
	// Lazy TTL GC.
	if now.Sub(entry.CreatedAt) >= RequestRetryCacheTTL {
		delete(c.entries, requestID)
		return nil, nil
	}
	// Backoff window.
	if now.Before(entry.NextRetryAt) {
		return nil, nil
	}
	// Budget check — exhaustion is terminal.
	if entry.AttemptCount >= MaxPermissionRetries {
		delete(c.entries, requestID)
		return nil, ErrRetryExhausted
	}
	// Approve retry: bump count + backoff window.
	entry.AttemptCount++
	entry.NextRetryAt = now.Add(RetryBackoff)
	return entry, nil
}

// Remove deletes the entry for requestID (called after a successful
// retry confirms grant landed). No-op if not present.
func (c *RequestRetryCache) Remove(requestID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, requestID)
}

// Len returns the live entry count (test seam — assert lazy GC).
func (c *RequestRetryCache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.entries)
}
