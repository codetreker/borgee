// Package bpp_test — request_retry_cache_test.go: BPP-3.2.3 retry cache
// 5 unit pins (acceptance §3.1-§3.3 + content-lock §4 + spec §3 反向约束).
//
// Pins:
//   3.1.a TTL 5 min lazy GC reaps stale entries
//   3.1.b ≤3 次重试 (MaxPermissionRetries const lock)
//   3.1.c 30s 固定退避 (RetryBackoff const lock + backoff window enforcement)
//   3.2.a 上限超 → ErrRetryExhausted + bpp.retry_exhausted 错码字面 byte-identical
//   3.3.a 反向约束 grep: 不复用 BPP-4 watchdog 队列
package bpp_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"borgee-server/internal/bpp"
)

// REG-BPP32-201 (acceptance §3.1.a) — TTL 5 min lazy GC.
// After 5min30s of clock advance, ShouldRetry returns (nil, nil) — entry
// reaped on read. cache.Len() drops to 0 after the read.
func TestBPP_RetryCache_TTLLazyGC(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	c := bpp.NewRequestRetryCacheWithClock(clock)

	c.Add(&bpp.RetryEntry{
		RequestID: "req-ttl",
		AgentID:   "agent-A",
	})
	if c.Len() != 1 {
		t.Fatalf("Len after Add: got %d, want 1", c.Len())
	}

	// Advance past TTL (5 min) + past backoff (30s).
	now = now.Add(bpp.RequestRetryCacheTTL + 30*time.Second)

	entry, err := c.ShouldRetry("req-ttl")
	if entry != nil || err != nil {
		t.Errorf("ShouldRetry after TTL: got (%v, %v), want (nil, nil) — lazy GC", entry, err)
	}
	if c.Len() != 0 {
		t.Errorf("Len after lazy GC: got %d, want 0", c.Len())
	}
}

// REG-BPP32-202 (acceptance §3.1.b + content-lock §4) — ≤3 次重试 lock.
// 1st/2nd/3rd ShouldRetry succeed (after backoff each); 4th returns
// ErrRetryExhausted + entry removed (terminal).
func TestBPP_RetryCache_3RetryThenExhaust(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	c := bpp.NewRequestRetryCacheWithClock(clock)

	c.Add(&bpp.RetryEntry{RequestID: "req-3max", AgentID: "ag-1"})

	// Sanity: const literal lock.
	if bpp.MaxPermissionRetries != 3 {
		t.Fatalf("MaxPermissionRetries const 脱节: got %d, want 3 (content-lock §4)", bpp.MaxPermissionRetries)
	}

	// 3 successful retries, each after RetryBackoff.
	for i := 1; i <= 3; i++ {
		now = now.Add(bpp.RetryBackoff)
		entry, err := c.ShouldRetry("req-3max")
		if err != nil || entry == nil {
			t.Fatalf("attempt %d: got (%v, %v), want (entry, nil)", i, entry, err)
		}
		if entry.AttemptCount != i {
			t.Errorf("attempt %d AttemptCount: got %d, want %d", i, entry.AttemptCount, i)
		}
	}

	// 4th attempt — exhausted.
	now = now.Add(bpp.RetryBackoff)
	entry, err := c.ShouldRetry("req-3max")
	if entry != nil {
		t.Errorf("4th attempt entry: got %+v, want nil", entry)
	}
	if !bpp.IsRetryExhausted(err) {
		t.Errorf("4th attempt err: got %v, want ErrRetryExhausted", err)
	}
	// Terminal — entry removed.
	if c.Len() != 0 {
		t.Errorf("Len after exhaust: got %d, want 0 (terminal removal)", c.Len())
	}
	// 错码字面锁定 byte-identical 跟 content-lock §4.
	if bpp.RetryExhaustedErrCode != "bpp.retry_exhausted" {
		t.Errorf("RetryExhaustedErrCode 脱节: got %q, want %q",
			bpp.RetryExhaustedErrCode, "bpp.retry_exhausted")
	}
}

// REG-BPP32-203 (acceptance §3.1.c) — 30s 固定退避 lock + backoff 窗口
// enforcement. ShouldRetry 在 backoff 内返 (nil, nil) — 不抢退避窗口.
func TestBPP_RetryCache_30sFixedBackoffEnforced(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	c := bpp.NewRequestRetryCacheWithClock(clock)

	// Const literal lock — 反向 grep `RetryBackoff.*=.*60\|RetryBackoff.*=.*15`
	// 等漂值 0 hit (content-lock §4 字面 30s).
	if bpp.RetryBackoff != 30*time.Second {
		t.Fatalf("RetryBackoff const 脱节: got %v, want 30s (content-lock §4)", bpp.RetryBackoff)
	}

	c.Add(&bpp.RetryEntry{RequestID: "req-bo", AgentID: "ag-2"})

	// Within backoff window — should NOT retry.
	now = now.Add(29 * time.Second)
	entry, err := c.ShouldRetry("req-bo")
	if entry != nil || err != nil {
		t.Errorf("within backoff (T+29s): got (%v, %v), want (nil, nil)", entry, err)
	}

	// At backoff edge — should retry.
	now = now.Add(time.Second) // T+30s total
	entry, err = c.ShouldRetry("req-bo")
	if err != nil || entry == nil {
		t.Errorf("at backoff edge (T+30s): got (%v, %v), want (entry, nil)", entry, err)
	}
	if entry.AttemptCount != 1 {
		t.Errorf("AttemptCount after first retry: got %d, want 1", entry.AttemptCount)
	}

	// Immediately after — should NOT retry (backoff bump).
	entry, err = c.ShouldRetry("req-bo")
	if entry != nil || err != nil {
		t.Errorf("immediately after retry: got (%v, %v), want (nil, nil) — backoff bumped", entry, err)
	}
}

// REG-BPP32-204 (acceptance §3.2 + spec §1 原则 ③ + content-lock §4) —
// IsRetryExhausted sentinel matcher + Remove (post-success cleanup).
func TestBPP_RetryCache_RemoveAfterSuccess(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 4, 30, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	c := bpp.NewRequestRetryCacheWithClock(clock)

	c.Add(&bpp.RetryEntry{RequestID: "req-rm", AgentID: "ag-3"})
	now = now.Add(bpp.RetryBackoff)

	// 1st retry, then caller removes (simulating successful retry).
	if _, err := c.ShouldRetry("req-rm"); err != nil {
		t.Fatalf("1st retry: %v", err)
	}
	c.Remove("req-rm")
	if c.Len() != 0 {
		t.Errorf("Len after Remove: got %d, want 0", c.Len())
	}

	// Re-issue (e.g. owner revoke + re-deny) — Add resets state.
	c.Add(&bpp.RetryEntry{RequestID: "req-rm", AgentID: "ag-3"})
	if c.Len() != 1 {
		t.Fatalf("Re-Add: Len got %d, want 1", c.Len())
	}
	now = now.Add(bpp.RetryBackoff)
	entry, err := c.ShouldRetry("req-rm")
	if err != nil || entry == nil {
		t.Errorf("Re-Add then retry: got (%v, %v), want (entry, nil) — count reset", entry, err)
	}
	if entry.AttemptCount != 1 {
		t.Errorf("Re-Add AttemptCount: got %d, want 1 (fresh re-issue)", entry.AttemptCount)
	}
}

// REG-BPP32-205 (acceptance §3.3 + spec §3 反向约束 #3) — 反向约束 grep:
// 不复用 BPP-4 watchdog 队列. CI lint 等价单测守 future 脱节.
//
// 反向断言 in this package:
//   - `pendingAcks` (BPP-4 watchdog ack 队列名)
//   - `deadLetterQueue` (BPP-4 失败队列)
//   - `cancel.*in.flight` (BPP-4 取消机制)
//   - `BPP-?4.*watchdog` 引用注释 (说明跨 milestone 复用)
func TestBPP_RetryCache_ReverseGrep_NoBPP4Reuse(t *testing.T) {
	t.Parallel()
	bppDir := filepath.Join(".")
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`pendingAcks`),
		regexp.MustCompile(`deadLetterQueue`),
		regexp.MustCompile(`cancel.*in.flight`),
		regexp.MustCompile(`BPP-?4.*watchdog.*reuse|reuse.*BPP-?4.*watchdog`),
	}
	hits := []string{}
	_ = filepath.Walk(bppDir, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		// Only scan request_retry_cache.go itself — that's the BPP-3.2.3
		// 原则 §3 boundary.
		if !strings.HasSuffix(p, "request_retry_cache.go") {
			return nil
		}
		body, err := os.ReadFile(p)
		if err != nil {
			return nil
		}
		for _, pat := range patterns {
			if loc := pat.FindIndex(body); loc != nil {
				hits = append(hits, p+":"+pat.String())
			}
		}
		return nil
	})
	if len(hits) > 0 {
		t.Errorf("反向约束 spec §3 #3 broken — request_retry_cache.go 不应复用 BPP-4 队列, hit: %v", hits)
	}
}

// TestNewRequestRetryCache_RealClock — exercises the production
// constructor (default 0% covered before this patch). Validates Add +
// ShouldRetry round-trip uses time.Now wall clock without injection.
func TestNewRequestRetryCache_RealClock(t *testing.T) {
	t.Parallel()
	c := bpp.NewRequestRetryCache()
	entry := &bpp.RetryEntry{RequestID: "req-real-clock-1"}
	c.Add(entry)
	// Newly added: NextRetryAt is now+30s, so ShouldRetry returns
	// (nil, nil) — not yet retryable.
	got, err := c.ShouldRetry("req-real-clock-1")
	if err != nil {
		t.Fatalf("unexpected ShouldRetry err: %v", err)
	}
	if got != nil {
		t.Fatalf("expected backoff window to suppress retry; got %#v", got)
	}
}
