// Package datalayer — DL-2 events store + retention sweeper unit tests.
//
// Spec: docs/implementation/modules/dl-2-spec.md §0 policy + §1 DL2.2.
//
// Pins:
//   - hot stream behavior unchanged (DL-1 #609 Subscribe/Publish behavior unchanged)
//   - cold stream async INSERT (channel_events / global_events routing)
//   - mustPersistKinds 4 categories pinned exactly (perm.* / impersonate.* / agent.state / admin.force_*)
//   - retention sweeper per-kind reaping (must-persist never deleted / channel.* 30d / agent_task.* 60d / default 90d)

package datalayer

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"borgee-server/internal/store"

	"gorm.io/gorm"
)

func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	s, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	if err := s.Migrate(); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return s.DB()
}

// TestIsMustPersistKind pins the exact 4 must-persist prefixes
// (blueprint §3.4 privacy contract categories).
func TestIsMustPersistKind(t *testing.T) {
	t.Parallel()
	want := map[string]bool{
		"perm.grant":          true,
		"perm.revoke":         true,
		"impersonate.start":   true,
		"impersonate.end":     true,
		"agent.state":         true,
		"admin.force_delete":  true,
		"admin.force_disable": true,
		// non-must-persist
		"channel.archived":   false,
		"message.created":    false,
		"agent_task.started": false,
		"artifact.committed": false,
		"random.kind":        false,
	}
	for k, w := range want {
		if got := IsMustPersistKind(k); got != w {
			t.Errorf("IsMustPersistKind(%q) = %v, want %v", k, got, w)
		}
	}
}

// TestRetentionDaysForKind pins per-kind defaults (sweeper §0 policy ②).
func TestRetentionDaysForKind(t *testing.T) {
	t.Parallel()
	tests := []struct {
		kind string
		want int
	}{
		{"perm.grant", -1},        // must-persist: never reap
		{"impersonate.start", -1}, // must-persist
		{"agent.state", -1},       // must-persist
		{"admin.force_delete", -1},
		{"channel.archived", 30},
		{"message.created", 30},
		{"agent_task.started", 60},
		{"artifact.committed", 60},
		{"random.kind", 90},
	}
	for _, tc := range tests {
		if got := RetentionDaysForKind(tc.kind); got != tc.want {
			t.Errorf("RetentionDaysForKind(%q) = %d, want %d", tc.kind, got, tc.want)
		}
	}
}

// TestIsChannelScopedKind pins routing rule.
func TestIsChannelScopedKind(t *testing.T) {
	t.Parallel()
	if !IsChannelScopedKind("channel.archived") {
		t.Error("channel.archived must be channel-scoped")
	}
	if !IsChannelScopedKind("message.created") {
		t.Error("message.created must be channel-scoped")
	}
	if IsChannelScopedKind("perm.grant") {
		t.Error("perm.grant must not be channel-scoped")
	}
}

// TestSQLiteEventStore_PersistChannel writes to channel_events.
func TestSQLiteEventStore_PersistChannel(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	store := NewSQLiteEventStore(db, nil)

	if err := store.PersistChannel(context.Background(), "ch-1", "channel.archived", []byte(`{"reason":"test"}`)); err != nil {
		t.Fatalf("persist: %v", err)
	}
	var n int64
	db.Raw(`SELECT COUNT(*) FROM channel_events WHERE channel_id = ? AND kind = ?`, "ch-1", "channel.archived").Row().Scan(&n)
	if n != 1 {
		t.Errorf("channel_events row count = %d, want 1", n)
	}
}

// TestSQLiteEventStore_PersistGlobal writes to global_events.
func TestSQLiteEventStore_PersistGlobal(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	store := NewSQLiteEventStore(db, nil)

	if err := store.PersistGlobal(context.Background(), "perm.grant", []byte(`{"user":"u1"}`)); err != nil {
		t.Fatalf("persist: %v", err)
	}
	var n int64
	db.Raw(`SELECT COUNT(*) FROM global_events WHERE kind = ?`, "perm.grant").Row().Scan(&n)
	if n != 1 {
		t.Errorf("global_events row count = %d, want 1", n)
	}
}

// TestEventBusWithStore_HotAndCold pins double-stream — hot Subscribe
// receives event AND cold store gets persisted (deterministic via WaitGroup).
func TestEventBusWithStore_HotAndCold(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	// Custom store with WG to await async cold-stream INSERT.
	var wg sync.WaitGroup
	wg.Add(1)
	wrapped := &asyncWaitStore{inner: NewSQLiteEventStore(db, nil), done: &wg}
	bus := NewInProcessEventBusWithStore(wrapped)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sub, err := bus.Subscribe(ctx, "channel.archived:ch-1")
	if err != nil {
		t.Fatal(err)
	}

	if err := bus.Publish(ctx, "channel.archived:ch-1", []byte(`{"x":1}`)); err != nil {
		t.Fatalf("publish: %v", err)
	}

	// hot: subscriber gets event
	select {
	case got := <-sub:
		if got.Topic != "channel.archived:ch-1" {
			t.Errorf("hot topic = %q, want %q", got.Topic, "channel.archived:ch-1")
		}
	case <-time.After(time.Second):
		t.Fatal("hot stream timeout")
	}

	// cold: wait for async persist
	wg.Wait()
	var n int64
	db.Raw(`SELECT COUNT(*) FROM channel_events WHERE channel_id = ? AND kind = ?`,
		"ch-1", "channel.archived").Row().Scan(&n)
	if n != 1 {
		t.Errorf("cold stream channel_events count = %d, want 1", n)
	}
}

// asyncWaitStore wraps an EventStore with a WaitGroup to make cold-stream
// writes deterministic in tests.
type asyncWaitStore struct {
	inner EventStore
	done  *sync.WaitGroup
}

func (s *asyncWaitStore) PersistChannel(ctx context.Context, channelID, kind string, payload []byte) error {
	defer s.done.Done()
	return s.inner.PersistChannel(ctx, channelID, kind, payload)
}
func (s *asyncWaitStore) PersistGlobal(ctx context.Context, kind string, payload []byte) error {
	defer s.done.Done()
	return s.inner.PersistGlobal(ctx, kind, payload)
}

// TestEventBusWithStore_GlobalRoute confirms non-channel-scoped kind goes
// to global_events.
func TestEventBusWithStore_GlobalRoute(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	var wg sync.WaitGroup
	wg.Add(1)
	wrapped := &asyncWaitStore{inner: NewSQLiteEventStore(db, nil), done: &wg}
	bus := NewInProcessEventBusWithStore(wrapped)

	if err := bus.Publish(context.Background(), "perm.grant", []byte(`{"u":"u1"}`)); err != nil {
		t.Fatal(err)
	}
	wg.Wait()

	var n int64
	db.Raw(`SELECT COUNT(*) FROM global_events WHERE kind = ?`, "perm.grant").Row().Scan(&n)
	if n != 1 {
		t.Errorf("global_events count = %d, want 1", n)
	}
}

// TestEventsRetentionSweeper_RunOnce_ReapsExpired sweeps rows past retention.
func TestEventsRetentionSweeper_RunOnce_ReapsExpired(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	now := int64(2_000_000_000_000) // fixed UnixMilli
	// Seed 3 channel_events: 1 expired (45d old, retention=30) + 1 fresh + 1 must-persist sentinel
	must := func(sql string, args ...any) {
		if err := db.Exec(sql, args...).Error; err != nil {
			t.Fatal(err)
		}
	}
	const day = int64(86400000)
	must(`INSERT INTO channel_events (lex_id, channel_id, kind, payload, created_at, retention_days) VALUES (?, ?, ?, ?, ?, ?)`,
		"l-old", "ch-1", "channel.archived", "", now-45*day, 30)
	must(`INSERT INTO channel_events (lex_id, channel_id, kind, payload, created_at, retention_days) VALUES (?, ?, ?, ?, ?, ?)`,
		"l-new", "ch-1", "channel.archived", "", now-10*day, 30)
	// must-persist sentinel: retention_days NULL (never reaped, regardless of kind)
	must(`INSERT INTO channel_events (lex_id, channel_id, kind, payload, created_at, retention_days) VALUES (?, ?, ?, ?, ?, NULL)`,
		"l-perm", "ch-1", "perm.grant", "", now-365*day)

	// Seed global_events: 1 expired + 1 must-persist (NULL retention)
	must(`INSERT INTO global_events (lex_id, kind, payload, created_at, retention_days) VALUES (?, ?, ?, ?, ?)`,
		"g-old", "random.kind", "", now-100*day, 90)
	must(`INSERT INTO global_events (lex_id, kind, payload, created_at, retention_days) VALUES (?, ?, ?, ?, NULL)`,
		"g-perm", "perm.grant", "", now-365*day)

	sw := NewEventsRetentionSweeper(db, nil, 0) // 0 disables ticker
	sw.now = func() time.Time { return time.UnixMilli(now) }
	cReaped, gReaped, err := sw.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if cReaped != 1 {
		t.Errorf("channel reaped = %d, want 1 (l-old only)", cReaped)
	}
	if gReaped != 1 {
		t.Errorf("global reaped = %d, want 1 (g-old only)", gReaped)
	}

	// Verify must-persist rows survived.
	var n int64
	db.Raw(`SELECT COUNT(*) FROM channel_events WHERE lex_id = ?`, "l-perm").Row().Scan(&n)
	if n != 1 {
		t.Error("must-persist channel_events row reaped — privacy contract broken")
	}
	db.Raw(`SELECT COUNT(*) FROM global_events WHERE lex_id = ?`, "g-perm").Row().Scan(&n)
	if n != 1 {
		t.Error("must-persist global_events row reaped — privacy contract broken")
	}
}

// TestEventsRetentionSweeper_StartStop covers the ctx-aware lifecycle.
// It guards against goroutine leaks, matching #608.
func TestEventsRetentionSweeper_StartStop(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	sw := NewEventsRetentionSweeper(db, nil, 10*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	sw.Start(ctx)
	// Let at least one tick fire.
	time.Sleep(30 * time.Millisecond)
	cancel()
	select {
	case <-sw.Done():
	case <-time.After(time.Second):
		t.Fatal("sweeper did not stop within 1s after ctx cancel — goroutine leak")
	}
}

// TestEventsRetentionSweeper_StartZeroInterval — interval=0 short-circuits
// the background loop, Done() closes immediately.
func TestEventsRetentionSweeper_StartZeroInterval(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	sw := NewEventsRetentionSweeper(db, nil, 0)
	sw.Start(context.Background())
	select {
	case <-sw.Done():
	case <-time.After(time.Second):
		t.Fatal("interval=0 should close Done() immediately")
	}
}

// TestEventsRetentionSweeper_RunOnce_DBError covers the error log branch
// — closing DB makes RunOnce return error, runOnceLog logs it.
func TestEventsRetentionSweeper_RunOnce_DBError(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sw := NewEventsRetentionSweeper(db, logger, 0)

	// Close underlying SQL DB so DELETE fails.
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	_ = sqlDB.Close()

	if _, _, err := sw.RunOnce(context.Background()); err == nil {
		t.Error("expected RunOnce err on closed DB")
	}
	// Drive runOnceLog via Start with short interval to hit error log path
	// — but DB already closed, so the goroutine just logs error each tick.
	// Direct invocation via exported wrapper is simpler; we exercise the
	// branch by calling the private runOnceLog through Start.
	sw2 := NewEventsRetentionSweeper(db, logger, 5*time.Millisecond)
	ctx, cancel := context.WithCancel(context.Background())
	sw2.Start(ctx)
	time.Sleep(20 * time.Millisecond)
	cancel()
	<-sw2.Done()
}

// TestEventsRetentionSweeper_RunOnce_LogsReaped covers the info log branch
// (cReaped>0 || gReaped>0 + logger != nil).
func TestEventsRetentionSweeper_RunOnce_LogsReaped(t *testing.T) {
	t.Parallel()
	db := openTestDB(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	sw := NewEventsRetentionSweeper(db, logger, 5*time.Millisecond)
	now := int64(2_000_000_000_000)
	sw.now = func() time.Time { return time.UnixMilli(now) }

	// Seed 1 expired global_events row to ensure reap > 0.
	const day = int64(86400000)
	if err := db.Exec(
		`INSERT INTO global_events (lex_id, kind, payload, created_at, retention_days) VALUES (?, ?, ?, ?, ?)`,
		"g-old-log", "random.kind", "", now-100*day, 90,
	).Error; err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	sw.Start(ctx)
	// First tick fires immediately within Start; let it finish.
	time.Sleep(20 * time.Millisecond)
	cancel()
	<-sw.Done()

	// Verify row was reaped.
	var n int64
	db.Raw(`SELECT COUNT(*) FROM global_events WHERE lex_id = ?`, "g-old-log").Row().Scan(&n)
	if n != 0 {
		t.Errorf("row not reaped, got count=%d", n)
	}
}
