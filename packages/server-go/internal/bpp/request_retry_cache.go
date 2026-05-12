// Package bpp — request_retry_cache.go: BPP-3.2.3 plugin SDK in-memory
// retry cache for permission_denied → owner grant → auto-retry flow.
//
// Blueprint reference: docs/blueprint/current/auth-permissions.md §1.3 main
// entrypoint wording + plugin-protocol.md §1.6 disconnected and failure
// states. Spec: bpp-3.2-spec.md §1 principle ③ + bpp-3.2-stance §3 +
// content-lock §4 error-code literal lock.
//
// Behaviour contract (negative constraint spec §3 #3 + content-lock §4):
//
//   1. TTL 5 min — entries expire on read (lazy GC), preventing cache growth.
//   2. ≤3 retries (MaxPermissionRetries const; reverse grep
//      MaxPermissionRetries.*[4-9] in packages/plugin-sdk/ must return 0).
//   3. 30s fixed backoff (RetryBackoff const; reverse grep
//      `expBackoff|exponential.*retry` must return 0). Blueprint §1.6 keeps
//      server-side timing as the single source of truth; the plugin side does
//      not add a new timing signal.
//   4. When the limit is exceeded, emit `bpp.retry_exhausted`, byte-identical
//      with content-lock §4. Any change must update both content-lock and this
//      const.
//   5. Keep this cache separate from the server-side liveness queue; principle
//      §3 and spec §3 #3 keep the three paths separate. CI lint has an
//      equivalent reverse-grep test in request_retry_cache_test.go.
//
// Negative constraint notes:
//   - Cache state lives in plugin SDK process memory; server stateless.
//   - Retry trigger: only the `agent_config_update` frame (BPP-2.3) scans the
//     cache, reusing the existing frame per principles ②⑧ instead of adding a
//     capability_granted frame.
//   - Admin users do not enter this path because they do not upload through
//     plugin SDK semantic actions.

package bpp

import (
	"errors"
	"sync"
	"time"
)

// MaxPermissionRetries is the upper bound on retry attempts per request_id
// (content-lock §4 + bpp-3.2-spec.md §1 principle ③). After this count is
// reached, the next ShouldRetry call returns ErrRetryExhausted.
//
// Reverse grep CI lint: `MaxPermissionRetries.*[4-9]` count==0 (locks ≤3).
const MaxPermissionRetries = 3

// RetryBackoff is the FIXED retry interval (content-lock §4 + spec §1
// principle ③). Not exponential: blueprint plugin-protocol.md §1.6 makes
// server-side timing the single source of truth, and the plugin side does not
// add a new timing signal.
//
// Reverse grep CI lint: `expBackoff|exponential.*retry` count==0.
const RetryBackoff = 30 * time.Second

// RequestRetryCacheTTL is the cache entry TTL — entries older than 5 min
// are reaped on read (lazy GC). Defends against memory growth when
// owner never responds to permission_denied DM.
const RequestRetryCacheTTL = 5 * time.Minute

// RetryExhaustedErrCode is the error code emitted when MaxPermissionRetries
// is exceeded. It must remain byte-identical with
// docs/qa/bpp-3.2-content-lock.md §4. Any change must update both content-lock
// and this const.
//
// Naming follows the same pattern as BPP-2.2 bpp.task_subject_empty,
// BPP-2.3 bpp.config_field_disallowed, and BPP-3.2.1
// bpp.grant_capability_disallowed.
const RetryExhaustedErrCode = "bpp.retry_exhausted"

// ErrRetryExhausted sentinel returned by RequestRetryCache.ShouldRetry
// when the request has exceeded MaxPermissionRetries. Callers map to
// wire-level error code via IsRetryExhausted, matching the IsSemanticOpUnknown
// pattern.
var ErrRetryExhausted = errors.New("bpp: retry exhausted (MaxPermissionRetries reached)")

// IsRetryExhausted lets callers map the sentinel to the wire-level
// error code RetryExhaustedErrCode without exporting cache state.
func IsRetryExhausted(err error) bool {
	return errors.Is(err, ErrRetryExhausted)
}

// RetryEntry is a single in-flight permission_denied retry record.
// Stored in RequestRetryCache keyed by request_id (BPP-3.1 frame
// trace UUID must remain byte-identical with PermissionDeniedFrame.RequestID
// and CapabilityGrantPayload.RequestID across PRs).
type RetryEntry struct {
	RequestID    string    // BPP-3.1 frame trace UUID
	AgentID      string    // target agent (same source as frame.AgentID)
	Capability   string    // capability denied (same source as frame.RequiredCapability)
	Scope        string    // scope (same source as frame.CurrentScope)
	AttemptCount int       // completed retry count (0 = newly inserted, not retried yet)
	NextRetryAt  time.Time // next allowed retry time (= now + RetryBackoff)
	CreatedAt    time.Time // entry insertion time (for TTL comparison)
}

// RequestRetryCache is the plugin SDK in-memory permission_denied retry
// cache. Thread-safe (mutex-guarded map).
//
// Lifecycle:
//  1. Plugin receives BPP-3.1 PermissionDeniedFrame → caller calls Add(entry).
//  2. Plugin receives BPP-2.3 AgentConfigUpdateFrame (server push after owner
//     grant) → caller calls ShouldRetry(requestID, now). A non-nil entry with
//     nil error means retry is allowed and AttemptCount has been incremented;
//     ErrRetryExhausted means the limit was exceeded.
//  3. Retry succeeds → caller calls Remove(requestID) to clear the entry.
//
// Negative constraint: cache is not persisted. The server remains stateless;
// cache state lives only in plugin SDK process memory. Process restart clears
// the cache; after plugin reconnect the owner-side DM remains valid, and a
// manual retry can use a new request_id.
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
